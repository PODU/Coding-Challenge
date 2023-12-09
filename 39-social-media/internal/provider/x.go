package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"socialmedia/internal/model"
)

const XMaxBody = 280

// X posts through the X API v2 tweets endpoint. No automated test calls it.
type X struct {
	AccessToken string       // OAuth2 user-context bearer token
	HTTPClient  *http.Client // nil means a client with a 15s timeout
	BaseURL     string
}

func NewX(accessToken string) *X {
	return &X{
		AccessToken: accessToken,
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
		BaseURL:     "https://api.twitter.com",
	}
}

func (*X) Name() string { return "x" }

func (*X) Validate(p model.Post) error {
	if len(p.Body) > XMaxBody {
		return fmt.Errorf("%w: body exceeds X limit of %d characters", model.ErrValidation, XMaxBody)
	}
	return nil
}

func (x *X) Publish(ctx context.Context, p model.Post) (string, error) {
	if x.AccessToken == "" {
		return "", fmt.Errorf("%w: x access token not configured", ErrAuth)
	}
	client := x.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	body, err := json.Marshal(map[string]any{"text": p.Body})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, x.BaseURL+"/2/tweets", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+x.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", fmt.Errorf("%w: x %d: %s", ErrAuth, resp.StatusCode, respBody)
	case resp.StatusCode >= 300:
		return "", fmt.Errorf("x publish failed: %d: %s", resp.StatusCode, respBody)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("x publish: decode response: %w", err)
	}
	return out.Data.ID, nil
}
