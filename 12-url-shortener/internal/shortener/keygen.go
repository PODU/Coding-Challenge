package shortener

import (
	"crypto/sha256"
	"math/big"
	"strconv"
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// DefaultKeyLength gives 62^7 (about 3.5 trillion) possible keys.
const DefaultKeyLength = 7

// base62Encode returns exactly length base62 digits of data, padded with '0'.
func base62Encode(data []byte, length int) string {
	n := new(big.Int).SetBytes(data)
	base := big.NewInt(int64(len(base62Alphabet)))
	zero := big.NewInt(0)
	mod := new(big.Int)

	buf := make([]byte, 0, length)
	for n.Cmp(zero) > 0 && len(buf) < length {
		n.DivMod(n, base, mod)
		buf = append(buf, base62Alphabet[mod.Int64()])
	}
	for len(buf) < length {
		buf = append(buf, base62Alphabet[0])
	}
	return string(buf[:length])
}

// GenerateKey derives a key from the SHA-256 of longURL. If exists says the key
// is taken, it salts the URL and tries again, making the key one character
// longer every five attempts.
func GenerateKey(longURL string, exists func(string) bool) string {
	length := DefaultKeyLength
	for salt := 0; ; salt++ {
		seed := longURL
		if salt > 0 {
			seed = longURL + "#" + strconv.Itoa(salt)
		}
		sum := sha256.Sum256([]byte(seed))
		key := base62Encode(sum[:], length)
		if !exists(key) {
			return key
		}
		if salt > 0 && salt%5 == 0 {
			length++
		}
	}
}
