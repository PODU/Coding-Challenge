// Package hackernews implements source.Source over the public HN Firebase API.
package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/codingchallenges/topstories/internal/model"
	"github.com/codingchallenges/topstories/internal/source"
)

// DefaultBaseURL is the public Hacker News Firebase API root.
const DefaultBaseURL = "https://hacker-news.firebaseio.com/v0"

// Client implements source.Source against the HN Firebase API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	retryWait  time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API root (used by tests with httptest).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithRetries configures retry attempts and the base backoff wait.
func WithRetries(n int, wait time.Duration) Option {
	return func(c *Client) { c.maxRetries = n; c.retryWait = wait }
}

// New builds a Client with sane defaults.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		maxRetries: 3,
		retryWait:  200 * time.Millisecond,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Name implements source.Source.
func (c *Client) Name() string { return "hackernews" }

// categoryPath maps a Category onto its Firebase endpoint segment.
func categoryPath(cat source.Category) (string, error) {
	switch cat {
	case source.CatTop:
		return "topstories", nil
	case source.CatNew:
		return "newstories", nil
	case source.CatBest:
		return "beststories", nil
	case source.CatAsk:
		return "askstories", nil
	case source.CatShow:
		return "showstories", nil
	default:
		return "", fmt.Errorf("hackernews: unknown category %q", cat)
	}
}

// StoryIDs implements source.Source.
func (c *Client) StoryIDs(ctx context.Context, cat source.Category) ([]int64, error) {
	path, err := categoryPath(cat)
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := c.getJSON(ctx, fmt.Sprintf("%s/%s.json", c.baseURL, path), &ids); err != nil {
		return nil, fmt.Errorf("hackernews: story ids for %s: %w", cat, err)
	}
	return ids, nil
}

// hnItem mirrors the raw HN Firebase item JSON.
type hnItem struct {
	ID      int64  `json:"id"`
	Deleted bool   `json:"deleted"`
	Dead    bool   `json:"dead"`
	Type    string `json:"type"`
	By      string `json:"by"`
	Time    int64  `json:"time"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Score   int    `json:"score"`
}

// Fetch implements source.Source. Deleted/dead items return source.Skip.
func (c *Client) Fetch(ctx context.Context, id int64) (model.Story, error) {
	var it hnItem
	url := fmt.Sprintf("%s/item/%d.json", c.baseURL, id)
	if err := c.getJSON(ctx, url, &it); err != nil {
		return model.Story{}, fmt.Errorf("hackernews: fetch item %d: %w", id, err)
	}
	if it.ID == 0 {
		// HN returns literal null for missing items.
		return model.Story{}, source.Skip("missing item")
	}
	if it.Deleted || it.Dead {
		return model.Story{}, source.Skip("deleted or dead")
	}
	author := it.By
	if author == "" {
		author = "unknown"
	}
	typ := it.Type
	if typ == "" {
		typ = "story"
	}
	return model.Story{
		ID:        it.ID,
		Title:     it.Title,
		URL:       it.URL,
		Score:     it.Score,
		Author:    author,
		Timestamp: time.Unix(it.Time, 0).UTC(),
		Type:      typ,
		Source:    c.Name(),
	}, nil
}

// getJSON performs a GET with bounded retry/backoff and decodes the body.
func (c *Client) getJSON(ctx context.Context, url string, dst any) error {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryWait * time.Duration(attempt)):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		// Retry on 5xx; fail fast on 4xx.
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		err = json.NewDecoder(resp.Body).Decode(dst)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("decode: %w", err)
		}
		return nil
	}
	return fmt.Errorf("after %d retries: %w", c.maxRetries, lastErr)
}
