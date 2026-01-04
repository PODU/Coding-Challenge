package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DefaultReadmeURL is the raw README of the Coding Challenges Shared
// Solutions repository.
const DefaultReadmeURL = "https://raw.githubusercontent.com/CodingChallengesFYI/SharedSolutions/main/README.md"

const maxReadmeBytes = 4 << 20 // 4 MiB

// Fetcher retrieves the shared README. Tests swap in a fake so they can run
// offline.
type Fetcher interface {
	Fetch(ctx context.Context) (string, error)
}

// GitHubFetcher fetches the README over HTTP and caches it for TTL.
type GitHubFetcher struct {
	URL    string
	Client *http.Client
	TTL    time.Duration

	mu       sync.Mutex
	cached   string
	cachedAt time.Time
}

// NewGitHubFetcher returns a fetcher for DefaultReadmeURL.
func NewGitHubFetcher() *GitHubFetcher {
	return &GitHubFetcher{
		URL:    DefaultReadmeURL,
		Client: &http.Client{Timeout: 20 * time.Second},
		TTL:    5 * time.Minute,
	}
}

// Fetch returns the README text, serving a cached copy when it is fresh.
func (g *GitHubFetcher) Fetch(ctx context.Context) (string, error) {
	g.mu.Lock()
	if g.cached != "" && time.Since(g.cachedAt) < g.TTL {
		cached := g.cached
		g.mu.Unlock()
		return cached, nil
	}
	g.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.URL, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, g.URL)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadmeBytes))
	if err != nil {
		return "", err
	}
	content := string(body)

	g.mu.Lock()
	g.cached = content
	g.cachedAt = time.Now()
	g.mu.Unlock()

	return content, nil
}
