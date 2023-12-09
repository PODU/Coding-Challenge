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

const LinkedInMaxBody = 3000 // approximate

// LinkedIn posts through LinkedIn's UGC Posts API. No automated test calls it.
type LinkedIn struct {
	AccessToken string
	AuthorURN   string       // who the post is from, e.g. "urn:li:person:xxxx"
	HTTPClient  *http.Client // nil means a client with a 15s timeout
	BaseURL     string
}

func NewLinkedIn(accessToken, authorURN string) *LinkedIn {
	return &LinkedIn{
		AccessToken: accessToken,
		AuthorURN:   authorURN,
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
		BaseURL:     "https://api.linkedin.com",
	}
}

func (*LinkedIn) Name() string { return "linkedin" }

func (*LinkedIn) Validate(p model.Post) error {
	if len(p.Body) > LinkedInMaxBody {
		return fmt.Errorf("%w: body exceeds LinkedIn limit of %d characters", model.ErrValidation, LinkedInMaxBody)
	}
	return nil
}

func (l *LinkedIn) visibility(v model.Visibility) string {
	if v == model.VisibilityConnections {
		return "CONNECTIONS"
	}
	return "PUBLIC"
}

func (l *LinkedIn) Publish(ctx context.Context, p model.Post) (string, error) {
	if l.AccessToken == "" {
		return "", fmt.Errorf("%w: linkedin access token not configured", ErrAuth)
	}
	client := l.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	payload := map[string]any{
		"author":         l.AuthorURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]any{
			"com.linkedin.ugc.ShareContent": map[string]any{
				"shareCommentary":    map[string]any{"text": p.Body},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]any{
			"com.linkedin.ugc.MemberNetworkVisibility": l.visibility(p.Visibility),
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.BaseURL+"/v2/ugcPosts", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+l.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", fmt.Errorf("%w: linkedin %d: %s", ErrAuth, resp.StatusCode, respBody)
	case resp.StatusCode >= 300:
		return "", fmt.Errorf("linkedin publish failed: %d: %s", resp.StatusCode, respBody)
	}

	// LinkedIn normally returns the new post's URN in this header.
	if id := resp.Header.Get("X-RestLi-Id"); id != "" {
		return id, nil
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &out)
	return out.ID, nil
}
