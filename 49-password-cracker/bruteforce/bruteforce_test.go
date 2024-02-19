package bruteforce

import (
	"context"
	"testing"
)

func drain(ctx context.Context, charset string, min, max int) []string {
	var got []string
	for s := range Generate(ctx, charset, min, max) {
		got = append(got, s)
	}
	return got
}

func TestGenerateExactSet(t *testing.T) {
	got := drain(context.Background(), "ab", 1, 2)
	want := []string{"a", "b", "aa", "ab", "ba", "bb"}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGenerateCountMatches(t *testing.T) {
	// charset "abc", len 1..3 => 3 + 9 + 27 = 39
	got := drain(context.Background(), "abc", 1, 3)
	if Count(3, 1, 3) != 39 {
		t.Fatalf("Count = %d, want 39", Count(3, 1, 3))
	}
	if len(got) != 39 {
		t.Fatalf("generated %d, want 39", len(got))
	}
	// no duplicates
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s] {
			t.Fatalf("duplicate candidate %q", s)
		}
		seen[s] = true
	}
}

func TestGenerateCancellable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := Generate(ctx, "abcdefghij", 1, 8) // huge space
	count := 0
	for range ch {
		count++
		if count == 5 {
			cancel()
		}
	}
	// Must terminate promptly without exhausting the keyspace.
	if count > 100000 {
		t.Fatalf("did not stop on cancel, drained %d", count)
	}
}

func TestValidate(t *testing.T) {
	bad := []struct {
		charset  string
		min, max int
	}{
		{"", 1, 3},
		{"abc", 0, 3},
		{"abc", 3, 2},
		{"aab", 1, 3}, // duplicate chars
	}
	for _, c := range bad {
		if err := Validate(c.charset, c.min, c.max); err == nil {
			t.Errorf("Validate(%q,%d,%d) = nil, want error", c.charset, c.min, c.max)
		}
	}
	if err := Validate("abc", 1, 3); err != nil {
		t.Errorf("Validate good params: %v", err)
	}
}

func TestCountEdge(t *testing.T) {
	if Count(0, 1, 3) != 0 {
		t.Error("Count with empty charset must be 0")
	}
	if Count(36, 3, 2) != 0 {
		t.Error("Count with min>max must be 0")
	}
}
