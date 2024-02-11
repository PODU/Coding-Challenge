package token

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewRandomShapeAndUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	re := regexp.MustCompile(`^tok_[0-9A-Za-z]{22}$`)
	for i := 0; i < 10000; i++ {
		tok, err := NewRandom()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(tok) {
			t.Fatalf("token %q does not match expected shape", tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %q", tok)
		}
		seen[tok] = true
	}
}

func TestFormatPreservingShapes(t *testing.T) {
	cases := []struct {
		format string
		re     *regexp.Regexp
	}{
		{FormatEmail, regexp.MustCompile(`^[a-z0-9]+@[a-z0-9]+\.example$`)},
		{FormatPhone, regexp.MustCompile(`^\(\d{3}\) \d{3}-\d{4}$`)},
		{FormatSSN, regexp.MustCompile(`^\d{3}-\d{2}-\d{4}$`)},
		{FormatCreditCard, regexp.MustCompile(`^\d{4}-\d{4}-\d{4}-\d{4}$`)},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				tok, err := New(c.format)
				if err != nil {
					t.Fatal(err)
				}
				if !c.re.MatchString(tok) {
					t.Fatalf("format %s: token %q does not match shape %s", c.format, tok, c.re)
				}
			}
		})
	}
}

func TestCreditCardLuhnValid(t *testing.T) {
	for i := 0; i < 1000; i++ {
		tok, err := New(FormatCreditCard)
		if err != nil {
			t.Fatal(err)
		}
		digits := strings.ReplaceAll(tok, "-", "")
		if !luhnValid(digits) {
			t.Fatalf("generated card %q fails Luhn", tok)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	cases := map[string]string{
		"alice@example.com":    FormatEmail,
		"123-45-6789":          FormatSSN,
		"4111-1111-1111-1111":  FormatCreditCard,
		"(415) 555-0100":       FormatPhone,
		"just some free text!": "",
	}
	for in, want := range cases {
		if got := DetectFormat(in); got != want {
			t.Errorf("DetectFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnknownFormatFallsBackToRandom(t *testing.T) {
	tok, err := New("not-a-real-format")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok, Prefix) {
		t.Fatalf("expected random fallback token, got %q", tok)
	}
}

func luhnValid(number string) bool {
	sum := 0
	double := false
	for i := len(number) - 1; i >= 0; i-- {
		d := int(number[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
