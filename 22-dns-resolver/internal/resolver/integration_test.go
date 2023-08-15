//go:build integration

// Package resolver live integration test. This file is built only with the
// "integration" tag so the default `go test ./...` stays fully offline:
//
//	go test -tags=integration ./internal/resolver/...
//
// It performs real recursive resolution from the root nameservers and requires
// outbound UDP/53 (and TCP/53 for truncated replies) to the internet.
package resolver

import (
	"testing"
	"time"

	"dnsresolve/internal/message"
	"dnsresolve/internal/transport"
)

func TestLiveResolveDNSGoogle(t *testing.T) {
	x := transport.UDP{Timeout: 5 * time.Second, Retries: 2}
	res, err := New(x).Resolve("dns.google.com", message.TypeA)
	if err != nil {
		t.Fatalf("live resolve failed: %v", err)
	}
	if len(res.Addrs) == 0 {
		t.Fatalf("expected at least one A record, got none")
	}
	t.Logf("dns.google.com -> %v (%d hops)", res.Addrs, len(res.Trace))

	// dns.google.com is anycast 8.8.8.8 / 8.8.4.4; assert a known address is present.
	found := false
	for _, a := range res.Addrs {
		if a.String() == "8.8.8.8" || a.String() == "8.8.4.4" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 8.8.8.8 or 8.8.4.4 among %v", res.Addrs)
	}
}

func TestLiveResolveCNAME(t *testing.T) {
	x := transport.UDP{Timeout: 5 * time.Second, Retries: 2}
	res, err := New(x).Resolve("www.github.com", message.TypeA)
	if err != nil {
		t.Fatalf("live resolve failed: %v", err)
	}
	if len(res.Addrs) == 0 {
		t.Fatalf("expected at least one A record for www.github.com")
	}
	t.Logf("www.github.com -> %v (cname=%q)", res.Addrs, res.CNAME)
}
