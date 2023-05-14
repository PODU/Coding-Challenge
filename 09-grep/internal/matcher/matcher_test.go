package matcher

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		ignoreCase bool
		wordMatch  bool
		line       string
		want       bool
	}{
		{"literal hit", "cat", false, false, "the cat sat", true},
		{"literal miss", "dog", false, false, "the cat sat", false},
		{"empty matches anything", "", false, false, "whatever", true},
		{"empty matches empty line", "", false, false, "", true},
		{"digit class hit", `\d`, false, false, "abc123", true},
		{"digit class miss", `\d`, false, false, "letters", false},
		{"word class hit", `\w`, false, false, "_", true},
		{"anchor start hit", "^The", false, false, "The end", true},
		{"anchor start miss", "^The", false, false, "Not The end", false},
		{"anchor end hit", "end$", false, false, "The end", true},
		{"anchor end miss", "end$", false, false, "endless", false},
		{"alternation", "cat|dog", false, false, "a dog", true},
		{"case sensitive miss", "nirvana", false, false, "NIRVANA", false},
		{"case insensitive hit", "nirvana", true, false, "NIRVANA", true},
		{"word match whole", "cat", false, true, "a cat.", true},
		{"word match rejects substring", "cat", false, true, "category", false},
		{"word match rejects concat", "cat", false, true, "concatenate", false},
		{"word + ignore case", "cat", true, true, "A CAT here", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := New(tc.pattern, tc.ignoreCase, tc.wordMatch)
			if err != nil {
				t.Fatalf("New(%q) error: %v", tc.pattern, err)
			}
			if got := m.Matches(tc.line); got != tc.want {
				t.Errorf("Matches(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestNewInvalidPattern(t *testing.T) {
	if _, err := New("(", false, false); err == nil {
		t.Fatal("expected error for unbalanced (, got nil")
	}
}
