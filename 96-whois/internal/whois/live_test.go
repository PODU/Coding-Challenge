//go:build live

// Package whois live test. Excluded from the default build; run with:
//
//	go test -tags live -run TestLive ./...
//
// It performs a real network lookup against whois.iana.org and is tolerant of
// registry text changes (smoke test only).
package whois

import (
	"context"
	"testing"
	"time"
)

func TestLiveIANAReferral(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in -short mode")
	}
	client := NewClient()
	client.Timeout = 15 * time.Second

	l := NewLookup(client)
	res, err := l.Do(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("live lookup: %v", err)
	}
	if len(res.Hops) < 2 {
		t.Fatalf("expected at least one referral hop, got %d hops", len(res.Hops))
	}
	if res.Record.Domain == "" {
		t.Errorf("expected a domain in the merged record, got empty")
	}
	t.Logf("hops: %d, domain: %s, registrar: %s",
		len(res.Hops), res.Record.Domain, res.Record.Registrar)
}
