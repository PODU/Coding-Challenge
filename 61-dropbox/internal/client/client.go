// Package client implements the REST client and the sync executor that drives
// the pure planner against a real server.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"dropbox/internal/wire"
)

// Client-side errors.
var (
	ErrConflict = errors.New("version conflict")
	ErrNotFound = errors.New("not found")
	ErrAuth     = errors.New("authentication failed")
)

// Client talks to the dropd REST API.
type Client struct {
	base  string
	token string
	hc    *http.Client
}

// New returns a Client for the given server base URL and bearer token. A nil
// http.Client uses http.DefaultClient.
func New(base, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{base: strings.TrimRight(base, "/"), token: token, hc: hc}
}

func (c *Client) SetToken(t string) { c.token = t }

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.hc.Do(req)
}

// encodePath percent-escapes each path segment while preserving the slashes.
func encodePath(rel string) string {
	parts := strings.Split(rel, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

// Login exchanges credentials for a bearer token.
func (c *Client) Login(user, password string) (string, error) {
	body, _ := json.Marshal(wire.LoginRequest{User: user, Password: password})
	req, _ := http.NewRequest("POST", c.base+"/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrAuth
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login: %s", resp.Status)
	}
	var lr wire.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", err
	}
	return lr.Token, nil
}

func (c *Client) List() ([]wire.FileInfo, error) {
	req, _ := http.NewRequest("GET", c.base+"/v1/files", nil)
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrAuth
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list: %s", resp.Status)
	}
	var lr wire.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, err
	}
	return lr.Files, nil
}

// Download streams the remote file rel into w, returning its FileInfo.
func (c *Client) Download(rel string, w io.Writer) (wire.FileInfo, error) {
	req, _ := http.NewRequest("GET", c.base+"/v1/files/"+encodePath(rel), nil)
	resp, err := c.do(req)
	if err != nil {
		return wire.FileInfo{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return wire.FileInfo{}, ErrAuth
	case http.StatusNotFound:
		return wire.FileInfo{}, ErrNotFound
	default:
		return wire.FileInfo{}, fmt.Errorf("download %s: %s", rel, resp.Status)
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return wire.FileInfo{}, err
	}
	ver, _ := strconv.ParseInt(resp.Header.Get("X-Version"), 10, 64)
	return wire.FileInfo{Path: rel, Hash: resp.Header.Get("X-Content-SHA256"), Version: ver}, nil
}

// Upload PUTs body to rel. version (>= 0) becomes the If-Match precondition;
// hash is sent as X-Content-SHA256 for server-side verification.
func (c *Client) Upload(rel string, version int64, hash string, size int64, body io.Reader) (wire.FileInfo, error) {
	req, _ := http.NewRequest("PUT", c.base+"/v1/files/"+encodePath(rel), body)
	if version >= 0 {
		req.Header.Set("If-Match", strconv.FormatInt(version, 10))
	}
	if hash != "" {
		req.Header.Set("X-Content-SHA256", hash)
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.do(req)
	if err != nil {
		return wire.FileInfo{}, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return wire.FileInfo{}, ErrAuth
	case http.StatusConflict:
		return wire.FileInfo{}, ErrConflict
	default:
		b, _ := io.ReadAll(resp.Body)
		return wire.FileInfo{}, fmt.Errorf("upload %s: %s: %s", rel, resp.Status, strings.TrimSpace(string(b)))
	}
	var fi wire.FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&fi); err != nil {
		return wire.FileInfo{}, err
	}
	return fi, nil
}

// Delete removes rel. version (>= 0) becomes the If-Match precondition.
func (c *Client) Delete(rel string, version int64) error {
	req, _ := http.NewRequest("DELETE", c.base+"/v1/files/"+encodePath(rel), nil)
	if version >= 0 {
		req.Header.Set("If-Match", strconv.FormatInt(version, 10))
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrAuth
	case http.StatusConflict:
		return ErrConflict
	default:
		return fmt.Errorf("delete %s: %s", rel, resp.Status)
	}
}
