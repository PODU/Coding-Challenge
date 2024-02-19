package saltutil

import "testing"

func TestApply(t *testing.T) {
	cases := []struct {
		salt, mode, cand, want string
	}{
		{"", "prefix", "pw", "pw"},
		{"", "suffix", "pw", "pw"},
		{"s", "prefix", "pw", "spw"},
		{"s", "suffix", "pw", "pws"},
		{"salt", "prefix", "secret", "saltsecret"},
		{"salt", "suffix", "secret", "secretsalt"},
	}
	for _, c := range cases {
		mode, err := ParseMode(c.mode)
		if err != nil {
			t.Fatalf("ParseMode(%q): %v", c.mode, err)
		}
		got := string(New(c.salt, mode).Apply(c.cand))
		if got != c.want {
			t.Errorf("salt=%q mode=%s cand=%q = %q, want %q", c.salt, c.mode, c.cand, got, c.want)
		}
	}
}

func TestParseModeInvalid(t *testing.T) {
	if _, err := ParseMode("middle"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

// Apply must not alias internal state across calls (concurrency safety).
func TestApplyAllocatesFresh(t *testing.T) {
	s := New("salt", Prefix)
	a := s.Apply("aaaa")
	b := s.Apply("bbbb")
	if string(a) != "saltaaaa" || string(b) != "saltbbbb" {
		t.Fatalf("apply aliased: a=%q b=%q", a, b)
	}
}
