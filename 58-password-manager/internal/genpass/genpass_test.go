package genpass

import (
	"math"
	"strings"
	"testing"
)

func TestLengthHonored(t *testing.T) {
	for _, n := range []int{4, 16, 20, 64, 128} {
		o := DefaultOptions()
		o.Length = n
		pw, err := Generate(o)
		if err != nil {
			t.Fatalf("Generate(len=%d): %v", n, err)
		}
		if len(pw) != n {
			t.Fatalf("length = %d, want %d", len(pw), n)
		}
	}
}

func TestRespectsDisabledClasses(t *testing.T) {
	o := Options{Length: 40, Lower: true, Upper: false, Digits: false, Symbols: false}
	for i := 0; i < 50; i++ {
		pw, err := Generate(o)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if strings.ContainsAny(pw, upperSet+digitSet+symbolSet) {
			t.Fatalf("password %q contains a disabled class", pw)
		}
	}
}

func TestIncludesEachEnabledClass(t *testing.T) {
	o := DefaultOptions()
	o.Length = 8
	for i := 0; i < 200; i++ {
		pw, err := Generate(o)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !strings.ContainsAny(pw, lowerSet) ||
			!strings.ContainsAny(pw, upperSet) ||
			!strings.ContainsAny(pw, digitSet) ||
			!strings.ContainsAny(pw, symbolSet) {
			t.Fatalf("password %q missing an enabled class", pw)
		}
	}
}

func TestEmptyAlphabetRejected(t *testing.T) {
	o := Options{Length: 10}
	if _, err := Generate(o); err == nil {
		t.Fatal("expected error when all classes disabled")
	}
}

func TestLengthTooShortForClasses(t *testing.T) {
	o := DefaultOptions() // 4 classes
	o.Length = 3
	if _, err := Generate(o); err == nil {
		t.Fatal("expected error when length < number of classes")
	}
}

func TestZeroLengthRejected(t *testing.T) {
	o := DefaultOptions()
	o.Length = 0
	if _, err := Generate(o); err == nil {
		t.Fatal("expected error for zero length")
	}
}

func TestEntropy(t *testing.T) {
	o := DefaultOptions()
	o.Length = 20
	size := len(lowerSet) + len(upperSet) + len(digitSet) + len(symbolSet)
	want := 20 * math.Log2(float64(size))
	got := Entropy(o)
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("Entropy = %f, want %f", got, want)
	}
	if Entropy(Options{Length: 10}) != 0 {
		t.Fatal("empty alphabet should yield 0 entropy")
	}
}

// TestNoModuloBias is a statistical sanity check: over many samples each
// character of a single-class alphabet should appear with roughly uniform
// frequency. A biased generator (e.g. raw modulo) would skew low indices.
func TestNoModuloBias(t *testing.T) {
	o := Options{Length: 100, Lower: true}
	counts := map[rune]int{}
	const samples = 200
	for i := 0; i < samples; i++ {
		pw, err := Generate(o)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		for _, c := range pw {
			counts[c]++
		}
	}
	total := samples * o.Length
	expected := float64(total) / float64(len(lowerSet))
	for _, c := range lowerSet {
		got := float64(counts[c])
		// Allow generous +/-35% slack; a real modulo bias is far larger and
		// systematic, this only catches gross errors without flaking.
		if got < expected*0.65 || got > expected*1.35 {
			t.Fatalf("char %q frequency %f deviates from expected %f", c, got, expected)
		}
	}
}
