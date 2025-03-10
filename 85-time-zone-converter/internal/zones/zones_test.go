package zones

import (
	"strings"
	"testing"
)

// T11: city alias variants all resolve to America/New_York.
func TestResolveCityAliasVariants(t *testing.T) {
	for _, tok := range []string{"new york", "New_York", "new york ", "NEW  YORK", "nyc"} {
		res, err := Resolve(tok)
		if err != nil {
			t.Fatalf("Resolve(%q) error: %v", tok, err)
		}
		if res.Name != "America/New_York" {
			t.Errorf("Resolve(%q) = %q, want America/New_York", tok, res.Name)
		}
	}
}

// T12: abbreviations resolve to their documented canonical zones.
func TestResolveAbbreviations(t *testing.T) {
	cases := map[string]string{
		"JST": "Asia/Tokyo",
		"IST": "Asia/Kolkata",
		"EST": "America/New_York",
		"jst": "Asia/Tokyo", // case-insensitive
	}
	for tok, want := range cases {
		res, err := Resolve(tok)
		if err != nil {
			t.Fatalf("Resolve(%q) error: %v", tok, err)
		}
		if res.Name != want {
			t.Errorf("Resolve(%q) = %q, want %q", tok, res.Name, want)
		}
	}
}

// T13: ambiguous abbreviation reports a note.
func TestResolveAmbiguousAbbrev(t *testing.T) {
	res, err := Resolve("CST")
	if err != nil {
		t.Fatalf("Resolve(CST) error: %v", err)
	}
	if res.Name != "America/Chicago" {
		t.Errorf("CST = %q, want America/Chicago", res.Name)
	}
	if res.Note == "" {
		t.Errorf("CST should report an ambiguity note")
	}
}

// T14: unknown token errors with suggestions.
func TestResolveUnknown(t *testing.T) {
	_, err := Resolve("Mars/Olympus")
	if err == nil {
		t.Fatal("expected error for unknown zone")
	}
	if !strings.Contains(err.Error(), "unknown zone") {
		t.Errorf("error %q should mention 'unknown zone'", err.Error())
	}
}

// IANA region names resolve directly.
func TestResolveIANA(t *testing.T) {
	res, err := Resolve("Asia/Kolkata")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Name != "Asia/Kolkata" || res.Via != "iana" {
		t.Errorf("got %+v", res)
	}
}

// Suggestions are produced for near-miss tokens.
func TestSuggest(t *testing.T) {
	s := Suggest("lndon", 5)
	found := false
	for _, x := range s {
		if x == "london" {
			found = true
		}
	}
	if !found {
		t.Errorf("Suggest(lndon) = %v, expected 'london'", s)
	}
}
