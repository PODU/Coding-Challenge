package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
)

// TargetOS and TargetArch are the platform picked from multi-arch images.
const (
	TargetOS   = "linux"
	TargetArch = "amd64"
)

// Client talks to a registry. Tests point it at an httptest server.
type Client struct {
	HTTP   *http.Client // nil means http.DefaultClient
	Scheme string       // empty means "https"
}

// Image is a pulled manifest and config. Token is needed to fetch the layers.
type Image struct {
	Ref       Ref
	Token     string
	Manifest  Manifest
	Config    ImageConfig
	ConfigRaw []byte
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) scheme() string {
	if c.Scheme != "" {
		return c.Scheme
	}
	return "https"
}

// Pull fetches the manifest (picking linux/amd64 from a multi-arch image) and
// the image config. Layers are left to FetchBlob so they can be streamed.
func (c *Client) Pull(ctx context.Context, ref Ref) (*Image, error) {
	body, contentType, token, err := c.getManifest(ctx, ref, "")
	if err != nil {
		return nil, err
	}

	if IsIndexMediaType(contentType) || (!IsManifestMediaType(contentType) && LooksLikeIndex(body)) {
		idx, err := ParseIndex(body)
		if err != nil {
			return nil, err
		}
		desc, err := SelectManifest(idx, TargetOS, TargetArch)
		if err != nil {
			return nil, err
		}
		child := ref
		child.Tag = ""
		child.Digest = desc.Digest
		body, _, token, err = c.getManifest(ctx, child, token)
		if err != nil {
			return nil, fmt.Errorf("fetch %s/%s child manifest: %w", TargetOS, TargetArch, err)
		}
		ref = child
	}

	manifest, err := ParseManifest(body)
	if err != nil {
		return nil, err
	}

	configRaw, err := c.fetchBlobBytes(ctx, ref, manifest.Config.Digest, token)
	if err != nil {
		return nil, fmt.Errorf("fetch image config: %w", err)
	}
	config, err := ParseImageConfig(configRaw)
	if err != nil {
		return nil, err
	}

	return &Image{
		Ref:       ref,
		Token:     token,
		Manifest:  manifest,
		Config:    config,
		ConfigRaw: configRaw,
	}, nil
}

// FetchBlob streams a blob. The reader checks the sha256 as it goes and
// returns an error at EOF on a mismatch.
func (c *Client) FetchBlob(ctx context.Context, ref Ref, digest, token string) (io.ReadCloser, error) {
	resp, _, err := c.getWithAuth(ctx, ref.BlobURL(c.scheme(), digest), "", ref, token)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("fetch blob %s: status %d: %s", digest, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return newVerifyReader(resp.Body, digest)
}

// getManifest fetches a manifest, getting a token first if needed. It returns
// the token so later requests can reuse it.
func (c *Client) getManifest(ctx context.Context, ref Ref, token string) (body []byte, contentType, outToken string, err error) {
	resp, usedToken, err := c.getWithAuth(ctx, ref.ManifestURL(c.scheme()), AcceptManifests, ref, token)
	if err != nil {
		return nil, "", token, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", usedToken, fmt.Errorf("fetch manifest %s: status %d: %s", ref.reference(), resp.StatusCode, strings.TrimSpace(string(b)))
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", usedToken, fmt.Errorf("read manifest body: %w", err)
	}
	contentType = resp.Header.Get("Content-Type")
	return body, contentType, usedToken, nil
}

// getWithAuth does a GET. On a 401 it gets a token from the challenge and
// retries once. It returns the token it ended up using.
func (c *Client) getWithAuth(ctx context.Context, url, accept string, ref Ref, token string) (*http.Response, string, error) {
	resp, err := c.get(ctx, url, accept, token)
	if err != nil {
		return nil, token, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		challenge, perr := ParseChallenge(resp.Header.Get("WWW-Authenticate"))
		resp.Body.Close()
		if perr != nil {
			return nil, token, perr
		}
		if challenge.Scope == "" {
			challenge.Scope = PullScope(ref.Repository)
		}
		newToken, terr := c.fetchToken(ctx, challenge)
		if terr != nil {
			return nil, token, terr
		}
		resp, err = c.get(ctx, url, accept, newToken)
		return resp, newToken, err
	}
	return resp, token, nil
}

func (c *Client) get(ctx context.Context, url, accept, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	return resp, nil
}

// fetchToken requests an anonymous token for challenge.
func (c *Client) fetchToken(ctx context.Context, challenge Challenge) (string, error) {
	tokenURL, err := TokenRequestURL(challenge)
	if err != nil {
		return "", err
	}
	resp, err := c.get(ctx, tokenURL, "", "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("token request: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var tok struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tok.Token != "" {
		return tok.Token, nil
	}
	if tok.AccessToken != "" {
		return tok.AccessToken, nil
	}
	return "", fmt.Errorf("token response contained no token")
}

// fetchBlobBytes reads a whole (small) blob into memory.
func (c *Client) fetchBlobBytes(ctx context.Context, ref Ref, digest, token string) ([]byte, error) {
	rc, err := c.FetchBlob(ctx, ref, digest, token)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// verifyReader checks a blob's sha256 once it has been read to the end.
type verifyReader struct {
	rc   io.ReadCloser
	h    hash.Hash
	want string // hex, without "sha256:"
	done bool
}

// newVerifyReader only supports sha256, which is all registries use in
// practice.
func newVerifyReader(rc io.ReadCloser, digest string) (*verifyReader, error) {
	want, ok := strings.CutPrefix(digest, "sha256:")
	if !ok {
		rc.Close()
		return nil, fmt.Errorf("unsupported digest algorithm in %q (want sha256:)", digest)
	}
	return &verifyReader{rc: rc, h: sha256.New(), want: want}, nil
}

func (v *verifyReader) Read(p []byte) (int, error) {
	n, err := v.rc.Read(p)
	if n > 0 {
		v.h.Write(p[:n])
	}
	if err == io.EOF && !v.done {
		v.done = true
		got := hex.EncodeToString(v.h.Sum(nil))
		if got != v.want {
			return n, fmt.Errorf("digest mismatch: got sha256:%s want sha256:%s", got, v.want)
		}
	}
	return n, err
}

func (v *verifyReader) Close() error { return v.rc.Close() }
