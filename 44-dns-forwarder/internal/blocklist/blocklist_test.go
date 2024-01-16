package blocklist

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBlockedExactAndSubdomain(t *testing.T) {
	l := New([]string{"ads.example.com", "Tracker.NET."})

	cases := map[string]bool{
		"ads.example.com":      true,
		"ADS.EXAMPLE.COM":      true,  // case-insensitive
		"ads.example.com.":     true,  // trailing dot
		"deep.ads.example.com": true,  // subdomain
		"example.com":          false, // parent of a listed entry is not blocked
		"tracker.net":          true,
		"sub.tracker.net":      true,
		"google.com":           false,
	}
	for name, want := range cases {
		if got := l.Blocked(name); got != want {
			t.Errorf("Blocked(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestNilAndEmpty(t *testing.T) {
	var l *List
	if l.Blocked("anything.com") {
		t.Error("nil list should block nothing")
	}
	if New(nil).Blocked("anything.com") {
		t.Error("empty list should block nothing")
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "block.txt")
	content := "# comment\n\nads.example.com\n0.0.0.0 tracker.net\n  spaced.org  \n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if l.Len() != 3 {
		t.Errorf("Len = %d, want 3", l.Len())
	}
	for _, name := range []string{"ads.example.com", "tracker.net", "spaced.org"} {
		if !l.Blocked(name) {
			t.Errorf("%q should be blocked", name)
		}
	}
}
