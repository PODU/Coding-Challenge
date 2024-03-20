package bloom

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
)

// randomWord builds a deterministic pseudo-random lowercase token.
func randomWord(r *rand.Rand) string {
	const alpha = "abcdefghijklmnopqrstuvwxyz"
	n := 6 + r.Intn(12)
	b := make([]byte, n)
	for i := range b {
		b[i] = alpha[r.Intn(len(alpha))]
	}
	return string(b)
}

// TestNoFalseNegatives is the core guarantee: every word added must report
// present. This must hold across a range of (n, p) sizes and must never fail.
func TestNoFalseNegatives(t *testing.T) {
	cases := []struct {
		n int
		p float64
	}{
		{100, 0.1},
		{1000, 0.01},
		{5000, 0.001},
		{20000, 0.01},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("n=%d_p=%g", tc.n, tc.p), func(t *testing.T) {
			r := rand.New(rand.NewSource(int64(tc.n)))
			bf, err := New(tc.n, tc.p)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			words := make([]string, tc.n)
			for i := range words {
				words[i] = randomWord(r)
				bf.Add(words[i])
			}
			for _, w := range words {
				if !bf.Contains(w) {
					t.Fatalf("false negative: %q added but Contains==false", w)
				}
			}
		})
	}
}

// TestFalsePositiveRate builds a filter for n items at target p, then queries a
// large disjoint set of items known not to be present and asserts the empirical
// false-positive fraction is within tolerance of p.
func TestFalsePositiveRate(t *testing.T) {
	cases := []struct {
		n int
		p float64
	}{
		{10000, 0.01},
		{20000, 0.001},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("n=%d_p=%g", tc.n, tc.p), func(t *testing.T) {
			r := rand.New(rand.NewSource(1))
			bf, err := New(tc.n, tc.p)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			added := make(map[string]bool, tc.n)
			for len(added) < tc.n {
				w := "in_" + randomWord(r)
				if !added[w] {
					added[w] = true
					bf.Add(w)
				}
			}

			const trials = 200000
			fp := 0
			for i := 0; i < trials; i++ {
				// "out_" prefix guarantees disjointness from the "in_" set.
				w := "out_" + randomWord(r) + fmt.Sprintf("_%d", i)
				if added[w] {
					continue
				}
				if bf.Contains(w) {
					fp++
				}
			}
			observed := float64(fp) / float64(trials)
			// Generous band: empirical rate should not exceed 1.5x target.
			if observed > tc.p*1.5 {
				t.Fatalf("false-positive rate %.5f exceeds tolerance %.5f (target p=%g)",
					observed, tc.p*1.5, tc.p)
			}
			t.Logf("observed FP rate %.5f vs target %g (m=%d k=%d)", observed, tc.p, bf.M(), bf.K())
		})
	}
}

// TestOptimalMath checks the m/k formulas against hand-computed reference values
// for known (n, p) pairs.
func TestOptimalMath(t *testing.T) {
	cases := []struct {
		n     int
		p     float64
		wantM uint32
		wantK uint16
	}{
		// n=10000, p=0.01: m = ceil(-10000*ln(0.01)/ln(2)^2) = 95851, k = round(m/n*ln2) = 7
		{10000, 0.01, 95851, 7},
		// n=235886, p=0.01 (size of /usr/share/dict/words)
		{235886, 0.01, 2260982, 7},
		// n=1000, p=0.001: m = ceil(-1000*ln(0.001)/ln2^2) = 14378, k = round(14.378*ln2)=10
		{1000, 0.001, 14378, 10},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("n=%d_p=%g", tc.n, tc.p), func(t *testing.T) {
			m, k := Optimal(tc.n, tc.p)
			if m != tc.wantM {
				t.Errorf("m = %d, want %d", m, tc.wantM)
			}
			if k != tc.wantK {
				t.Errorf("k = %d, want %d", k, tc.wantK)
			}
		})
	}
}

func TestOptimalEdgeCases(t *testing.T) {
	// n=0 clamps to 1; must not divide by zero or produce empty array.
	m, k := Optimal(0, 0.01)
	if m < 1 {
		t.Errorf("n=0: m = %d, want >= 1", m)
	}
	if k < 1 {
		t.Errorf("n=0: k = %d, want >= 1", k)
	}
	bf := NewWithParams(m, k)
	if len(bf.Bits()) < 1 {
		t.Errorf("n=0: bit array empty")
	}
	// k never below 1 even for large p where the formula would round to 0.
	_, k = Optimal(1000, 0.99)
	if k < 1 {
		t.Errorf("large p: k = %d, want >= 1", k)
	}
}

func TestNewInvalidP(t *testing.T) {
	for _, p := range []float64{0, -0.5, 1, 1.5} {
		if _, err := New(100, p); err == nil {
			t.Errorf("New(_, %g): expected error, got nil", p)
		} else {
			var pe *ParamError
			if !errors.As(err, &pe) {
				t.Errorf("New(_, %g): expected *ParamError, got %T", p, err)
			}
		}
	}
}

func TestNormalizeConsistency(t *testing.T) {
	bf, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	bf.Add("  Hello  ")
	if !bf.Contains("hello") {
		t.Errorf("normalization mismatch: added '  Hello  ', Contains('hello')==false")
	}
	if !bf.Contains("HELLO") {
		t.Errorf("normalization mismatch: Contains('HELLO')==false")
	}
}

func TestUnicode(t *testing.T) {
	bf, err := New(100, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	words := []string{"café", "naïve", "日本語", "Ωmega"}
	for _, w := range words {
		bf.Add(w)
	}
	for _, w := range words {
		if !bf.Contains(w) {
			t.Errorf("unicode false negative for %q", w)
		}
	}
}

func BenchmarkAdd(b *testing.B) {
	bf, _ := New(b.N+1, 0.01)
	r := rand.New(rand.NewSource(1))
	words := make([]string, b.N)
	for i := range words {
		words[i] = randomWord(r)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(words[i])
	}
}

func BenchmarkContains(b *testing.B) {
	bf, _ := New(100000, 0.01)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100000; i++ {
		bf.Add(randomWord(r))
	}
	probe := randomWord(r)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Contains(probe)
	}
}
