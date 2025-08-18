package whois

import (
	"context"
	"fmt"
	"testing"
)

func TestLookupFollowsReferralChain(t *testing.T) {
	// Second hop: the registrar/registry record (terminal, no further referral).
	registry := newFakeServer(t, func(req string) string {
		return "Domain Name: EXAMPLE.COM\n" +
			"Registrar: Example Registrar, LLC\n" +
			"Creation Date: 1995-08-14T04:00:00Z\n" +
			"Name Server: a.iana-servers.net\n"
	})

	// First hop: an IANA-style response whose refer points at the second server.
	seed := newFakeServer(t, func(req string) string {
		return fmt.Sprintf("%% IANA\nrefer: %s\ndomain: COM\n", registry.addr)
	})

	l := NewLookup(NewClient())
	l.Seed = seed.addr

	res, err := l.Do(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(res.Hops) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(res.Hops))
	}
	if res.Hops[0].Server != seed.addr || res.Hops[1].Server != registry.addr {
		t.Errorf("unexpected hop order: %s -> %s", res.Hops[0].Server, res.Hops[1].Server)
	}
	if res.Record.Registrar != "Example Registrar, LLC" {
		t.Errorf("merged Registrar = %q", res.Record.Registrar)
	}
	if res.Record.Created != "1995-08-14T04:00:00Z" {
		t.Errorf("merged Created = %q", res.Record.Created)
	}
	if res.NoMatch {
		t.Error("should not be flagged no-match")
	}
}

func TestLookupNoReferralStopsAtSeed(t *testing.T) {
	registry := newFakeServer(t, func(req string) string { return "Domain Name: X\n" })
	seed := newFakeServer(t, func(req string) string {
		return fmt.Sprintf("refer: %s\n", registry.addr)
	})

	l := NewLookup(NewClient())
	l.Seed = seed.addr
	l.Follow = false

	res, err := l.Do(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(res.Hops) != 1 {
		t.Fatalf("Follow=false should give 1 hop, got %d", len(res.Hops))
	}
}

func TestLookupCycleProtection(t *testing.T) {
	// Two servers that refer to each other; the visited-set must break the loop.
	var aAddr, bAddr string
	a := newFakeServer(t, func(req string) string { return "refer: " + bAddr + "\n" })
	aAddr = a.addr
	b := newFakeServer(t, func(req string) string { return "refer: " + aAddr + "\n" })
	bAddr = b.addr

	l := NewLookup(NewClient())
	l.Seed = aAddr

	res, err := l.Do(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	// a -> b -> (a already visited) stop = 2 hops.
	if len(res.Hops) != 2 {
		t.Errorf("expected cycle to stop at 2 hops, got %d", len(res.Hops))
	}
}

func TestLookupNoMatch(t *testing.T) {
	body := loadFixture(t, "no_match.txt")
	seed := newFakeServer(t, func(req string) string {
		return body
	})
	l := NewLookup(NewClient())
	l.Seed = seed.addr

	res, err := l.Do(context.Background(), "nosuchdomain-xyz-12345.com")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !res.NoMatch {
		t.Error("expected NoMatch to be true")
	}
}

func TestLookupFirstHopErrorReturns(t *testing.T) {
	l := NewLookup(NewClient())
	l.Client.Timeout = 200_000_000 // 200ms
	l.Seed = "127.0.0.1:1"         // refused

	if _, err := l.Do(context.Background(), "example.com"); err == nil {
		t.Fatal("expected transport error on first hop")
	}
}
