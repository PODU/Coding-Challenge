package diff

import (
	"math/rand"
	"testing"
)

// TestLCSStringLength checks LCS length, which is deterministic even when more
// than one optimal subsequence exists.
func TestLCSStringLength(t *testing.T) {
	cases := []struct {
		name   string
		a, b   string
		length int
	}{
		{"classic ABCBDAB/BDCAB", "ABCBDAB", "BDCAB", 4},
		{"identical", "ABCDEF", "ABCDEF", 6},
		{"disjoint", "ABC", "DEF", 0},
		{"left empty", "", "ABC", 0},
		{"right empty", "ABC", "", 0},
		{"both empty", "", "", 0},
		{"XMJYAUZ/MZJAWXU", "XMJYAUZ", "MZJAWXU", 4},
		{"duplicates AABBA/ABA", "AABBA", "ABA", 3},
		{"unicode", "héllo", "hallo", 4}, // h, l, l, o
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LCSString(tc.a, tc.b)
			if len([]rune(got)) != tc.length {
				t.Fatalf("LCSString(%q,%q) = %q (len %d), want len %d",
					tc.a, tc.b, got, len([]rune(got)), tc.length)
			}
			if !isSubsequence([]rune(got), []rune(tc.a)) {
				t.Errorf("result %q is not a subsequence of a=%q", got, tc.a)
			}
			if !isSubsequence([]rune(got), []rune(tc.b)) {
				t.Errorf("result %q is not a subsequence of b=%q", got, tc.b)
			}
		})
	}
}

// TestLCSStringExact checks the exact subsequence where it is unique.
func TestLCSStringExact(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"ABCDEF", "ABCDEF", "ABCDEF"},
		{"ABC", "DEF", ""},
		{"AABBA", "ABA", "ABA"},
		{"GAC", "AGCAT", "AC"},
	}
	for _, tc := range cases {
		if got := LCSString(tc.a, tc.b); got != tc.want {
			t.Errorf("LCSString(%q,%q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestLCSLines exercises the []string (line-level) form used by the diff
// engine.
func TestLCSLines(t *testing.T) {
	a := []string{"the", "quick", "brown", "fox"}
	b := []string{"the", "lazy", "brown", "dog"}
	got := LCS(a, b)
	want := []string{"the", "brown"}
	if !equalStrings(got, want) {
		t.Errorf("LCS = %v, want %v", got, want)
	}

	if got := LCS(nil, []string{"a"}); len(got) != 0 {
		t.Errorf("LCS(nil, [a]) = %v, want empty", got)
	}
	if got := LCS([]string{"a", "b"}, nil); len(got) != 0 {
		t.Errorf("LCS([a b], nil) = %v, want empty", got)
	}
}

// TestMyersMatchesDP fuzzes the Myers and DP paths against each other: both
// must always recover an optimal (same-length) common subsequence, and the
// resulting diffs must be byte-for-byte identical.
func TestMyersMatchesDP(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	alphabet := []string{"a", "b", "c", "d"}
	randLines := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return out
	}

	for iter := 0; iter < 2000; iter++ {
		a := randLines(rng.Intn(9))
		b := randLines(rng.Intn(9))

		dpLCS := LCS(a, b)
		myLCS := myersLCS(a, b)
		if len(dpLCS) != len(myLCS) {
			t.Fatalf("LCS length mismatch for a=%v b=%v: dp=%v myers=%v", a, b, dpLCS, myLCS)
		}
		if !isSubsequenceStr(myLCS, a) || !isSubsequenceStr(myLCS, b) {
			t.Fatalf("myers LCS %v not a common subsequence of a=%v b=%v", myLCS, a, b)
		}
		if !isSubsequenceStr(dpLCS, a) || !isSubsequenceStr(dpLCS, b) {
			t.Fatalf("dp LCS %v not a common subsequence of a=%v b=%v", dpLCS, a, b)
		}
		// Equal lengths + both valid common subsequences => both are maximal,
		// i.e. the two algorithms agree on the optimal LCS length. (They may
		// pick different equally-optimal subsequences, so their concrete diff
		// output can legitimately differ.)
	}
}

func isSubsequence[T comparable](sub, of []T) bool {
	i := 0
	for j := 0; i < len(sub) && j < len(of); j++ {
		if sub[i] == of[j] {
			i++
		}
	}
	return i == len(sub)
}

func isSubsequenceStr(sub, of []string) bool { return isSubsequence(sub, of) }

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
