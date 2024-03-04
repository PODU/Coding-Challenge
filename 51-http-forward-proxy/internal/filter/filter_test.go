package filter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostForbidden(t *testing.T) {
	f := New()
	for _, h := range []string{"example.com", "ads.example.net"} {
		f.forbiddenHosts[h] = struct{}{}
	}

	cases := []struct {
		host string
		want bool
	}{
		{"example.com", true},          // exact
		{"www.example.com", true},      // subdomain suffix
		{"deep.sub.example.com", true}, // deeper subdomain
		{"example.com:8080", true},     // with port
		{"notexample.com", false},      // not a dotted suffix
		{"example.org", false},         // different TLD
		{"ads.example.net", true},      // exact second entry
		{"x.ads.example.net", true},    // subdomain of second
		{"example.net", false},         // parent of blocked sub is not blocked
		{"", false},                    // empty
	}
	for _, c := range cases {
		if got := f.HostForbidden(c.host); got != c.want {
			t.Errorf("HostForbidden(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestHostForbiddenEmptyList(t *testing.T) {
	f := New()
	if f.HostForbidden("anything.com") {
		t.Error("empty filter should block nothing")
	}
}

func TestHostForbiddenIPv6(t *testing.T) {
	f := New()
	f.forbiddenHosts["::1"] = struct{}{}
	if !f.HostForbidden("[::1]:8080") {
		t.Error("expected bracketed IPv6 with port to match")
	}
	if !f.HostForbidden("::1") {
		t.Error("expected bare IPv6 to match")
	}
}

func TestBodyHasBanned(t *testing.T) {
	f := New()
	f.bannedWords = []string{"malware", "phishing"}

	if w, hit := f.BodyHasBanned([]byte("this page has MALWARE in it")); !hit || w != "malware" {
		t.Errorf("case-insensitive hit failed: word=%q hit=%v", w, hit)
	}
	if _, hit := f.BodyHasBanned([]byte("a perfectly clean response")); hit {
		t.Error("clean body should not be flagged")
	}
	if _, hit := f.BodyHasBanned(nil); hit {
		t.Error("empty body should not be flagged")
	}

	empty := New()
	if _, hit := empty.BodyHasBanned([]byte("malware everywhere")); hit {
		t.Error("empty word list should flag nothing")
	}
}

func TestHasBannedWords(t *testing.T) {
	if New().HasBannedWords() {
		t.Error("empty filter reports banned words")
	}
	f := New()
	f.bannedWords = []string{"x"}
	if !f.HasBannedWords() {
		t.Error("expected HasBannedWords true")
	}
}

func TestLoadFilesParsing(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts.txt")
	wordsPath := filepath.Join(dir, "words.txt")

	if err := os.WriteFile(hostsPath, []byte("# comment\n\n  Example.COM  \nads.test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wordsPath, []byte("# c\nMalware\n\n  Secret \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := New()
	if existed, err := f.LoadHosts(hostsPath); err != nil || !existed {
		t.Fatalf("LoadHosts: existed=%v err=%v", existed, err)
	}
	if existed, err := f.LoadWords(wordsPath); err != nil || !existed {
		t.Fatalf("LoadWords: existed=%v err=%v", existed, err)
	}

	if !f.HostForbidden("example.com") {
		t.Error("expected normalized lower-cased host to be loaded")
	}
	if !f.HostForbidden("ads.test") {
		t.Error("expected second host loaded")
	}
	if _, hit := f.BodyHasBanned([]byte("contains a SECRET word")); !hit {
		t.Error("expected trimmed/lower-cased word loaded")
	}
}

func TestLoadMissingFile(t *testing.T) {
	f := New()
	existed, err := f.LoadHosts(filepath.Join(t.TempDir(), "nope.txt"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if existed {
		t.Error("missing file should report existed=false")
	}
	if f.HostForbidden("anything") {
		t.Error("missing file should leave filter empty")
	}
}
