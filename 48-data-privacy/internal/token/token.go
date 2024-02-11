// Package token generates the random tokens that stand in for sensitive
// values.
//
// Format-preserving tokens look like the original (an email, a phone number)
// so that downstream validation still passes. They are still random: nothing
// about the real value can be recovered from them.
package token

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// Prefix starts every token that isn't format-preserving.
const Prefix = "tok_"

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const (
	FormatEmail      = "email"
	FormatPhone      = "phone"
	FormatSSN        = "ssn"
	FormatCreditCard = "credit_card"
)

// NewRandom returns a URL-safe token like "tok_9f3aK2..." with about 131 bits
// of randomness.
func NewRandom() (string, error) {
	s, err := randomBase62(22)
	if err != nil {
		return "", err
	}
	return Prefix + s, nil
}

// New returns a token shaped like format, or a plain random token if format
// is empty or unknown.
func New(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatEmail:
		return newEmail()
	case FormatPhone:
		return newPhone()
	case FormatSSN:
		return newSSN()
	case FormatCreditCard:
		return newCreditCard()
	default:
		return NewRandom()
	}
}

// DetectFormat guesses which format value is in, or returns "".
func DetectFormat(value string) string {
	switch {
	case reEmail.MatchString(value):
		return FormatEmail
	case reCreditCard.MatchString(value):
		return FormatCreditCard
	case reSSN.MatchString(value):
		return FormatSSN
	case rePhone.MatchString(value):
		return FormatPhone
	default:
		return ""
	}
}

var (
	reEmail      = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	rePhone      = regexp.MustCompile(`^[+0-9().\-\s]{7,}$`)
	reSSN        = regexp.MustCompile(`^\d{3}-\d{2}-\d{4}$`)
	reCreditCard = regexp.MustCompile(`^\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}$`)
)

func newEmail() (string, error) {
	local, err := randomBase62(10)
	if err != nil {
		return "", err
	}
	domain, err := randomLowercase(8)
	if err != nil {
		return "", err
	}
	return strings.ToLower(local) + "@" + domain + ".example", nil
}

func newPhone() (string, error) {
	d, err := randomDigits(10)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(%s) %s-%s", d[0:3], d[3:6], d[6:10]), nil
}

func newSSN() (string, error) {
	d, err := randomDigits(9)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s", d[0:3], d[3:5], d[5:9]), nil
}

// newCreditCard returns a 16-digit number with a valid Luhn check digit, so
// card validation downstream accepts it.
func newCreditCard() (string, error) {
	d, err := randomDigits(15)
	if err != nil {
		return "", err
	}
	full := d + luhnCheckDigit(d)
	return fmt.Sprintf("%s-%s-%s-%s", full[0:4], full[4:8], full[8:12], full[12:16]), nil
}

// luhnCheckDigit returns the digit that makes number+digit pass the Luhn check.
func luhnCheckDigit(number string) string {
	sum := 0
	// The check digit will be the rightmost digit, so doubling starts with
	// the current last digit.
	double := true
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
	return fmt.Sprintf("%d", (10-(sum%10))%10)
}

func randomBase62(n int) (string, error) {
	return randomFromAlphabet(n, base62)
}

func randomDigits(n int) (string, error) {
	// No leading zero, so the number reads as having its full width.
	s, err := randomFromAlphabet(n, "0123456789")
	if err != nil {
		return "", err
	}
	if s[0] == '0' {
		s = "1" + s[1:]
	}
	return s, nil
}

func randomLowercase(n int) (string, error) {
	return randomFromAlphabet(n, "abcdefghijklmnopqrstuvwxyz")
}

func randomFromAlphabet(n int, alphabet string) (string, error) {
	var b strings.Builder
	b.Grow(n)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("token: random: %w", err)
		}
		b.WriteByte(alphabet[idx.Int64()])
	}
	return b.String(), nil
}
