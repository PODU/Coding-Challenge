package mcp

import "testing"

func TestParseSolutions(t *testing.T) {
	got := parseSolutions(fixtureReadme)
	if len(got) != 6 {
		t.Fatalf("parsed %d solutions, want 6: %+v", len(got), got)
	}
	// Links before any heading must be ignored.
	pre := "Intro [Stray](https://example.com/stray)\n\n## A\n- [Go](https://example.com/a/go)\n"
	if n := len(parseSolutions(pre)); n != 1 {
		t.Errorf("links before heading not ignored: got %d, want 1", n)
	}
}

func TestFind_ByName(t *testing.T) {
	got := Find(fixtureReadme, "json", "")
	if len(got) != 2 {
		t.Fatalf("Find json = %d, want 2", len(got))
	}
	for _, s := range got {
		if s.Number != "2" {
			t.Errorf("challenge number = %q, want 2", s.Number)
		}
	}
}

func TestFind_ByNumber(t *testing.T) {
	got := Find(fixtureReadme, "104", "")
	if len(got) != 1 || got[0].URL != "https://example.com/mcp/go" {
		t.Fatalf("Find 104 = %+v", got)
	}
	// Numeric query is exact: "1" must not match "104".
	if n := len(Find(fixtureReadme, "1", "")); n != 3 {
		t.Errorf("Find 1 = %d, want 3 (wc only)", n)
	}
}

func TestFind_CaseInsensitive(t *testing.T) {
	if len(Find(fixtureReadme, "WC", "")) != 3 {
		t.Errorf("case-insensitive name match failed")
	}
}

func TestFind_LanguageFilter(t *testing.T) {
	got := Find(fixtureReadme, "wc", "go")
	if len(got) != 1 || got[0].Language != "Go" {
		t.Fatalf("language filter = %+v", got)
	}
	if n := len(Find(fixtureReadme, "wc", "haskell")); n != 0 {
		t.Errorf("non-matching language = %d, want 0", n)
	}
}

func TestCleanHeading(t *testing.T) {
	if got := cleanHeading("[Challenge 1](https://x)"); got != "Challenge 1" {
		t.Errorf("cleanHeading = %q, want 'Challenge 1'", got)
	}
}
