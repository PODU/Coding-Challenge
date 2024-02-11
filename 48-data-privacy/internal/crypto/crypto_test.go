package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func mustKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, KeySize)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	kr, err := NewKeyringSingle(mustKey(t))
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("123-45-6789")
	aad := []byte("tok_abc")

	version, nonce, ct, err := kr.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}
	got, err := kr.Decrypt(version, nonce, ct, aad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: %q != %q", got, plaintext)
	}
}

func TestDecryptTamperFails(t *testing.T) {
	kr, _ := NewKeyringSingle(mustKey(t))
	pt := []byte("secret")
	aad := []byte("tok_1")

	t.Run("tamper ciphertext", func(t *testing.T) {
		v, nonce, ct, _ := kr.Encrypt(pt, aad)
		ct[0] ^= 0xFF
		if _, err := kr.Decrypt(v, nonce, ct, aad); err == nil {
			t.Fatal("expected auth failure")
		}
	})
	t.Run("tamper nonce", func(t *testing.T) {
		v, nonce, ct, _ := kr.Encrypt(pt, aad)
		nonce[0] ^= 0xFF
		if _, err := kr.Decrypt(v, nonce, ct, aad); err == nil {
			t.Fatal("expected auth failure")
		}
	})
}

func TestDecryptWrongAADFails(t *testing.T) {
	kr, _ := NewKeyringSingle(mustKey(t))
	v, nonce, ct, _ := kr.Encrypt([]byte("secret"), []byte("tok_A"))
	if _, err := kr.Decrypt(v, nonce, ct, []byte("tok_B")); err == nil {
		t.Fatal("expected failure with wrong AAD (token transplant prevention)")
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	kr1, _ := NewKeyringSingle(mustKey(t))
	kr2, _ := NewKeyringSingle(mustKey(t))
	v, nonce, ct, _ := kr1.Encrypt([]byte("secret"), []byte("tok"))
	if _, err := kr2.Decrypt(v, nonce, ct, []byte("tok")); err == nil {
		t.Fatal("expected failure decrypting with a different key")
	}
}

func TestKeyRotationDecryptsOldRecords(t *testing.T) {
	oldKey, newKey := mustKey(t), mustKey(t)

	// Old keyring writes a record at version 1.
	oldKR, _ := NewKeyring(1, map[byte][]byte{1: oldKey})
	v, nonce, ct, _ := oldKR.Encrypt([]byte("legacy"), []byte("tok"))

	// New keyring: current version 2, but retains version 1 for decrypt.
	newKR, err := NewKeyring(2, map[byte][]byte{1: oldKey, 2: newKey})
	if err != nil {
		t.Fatal(err)
	}
	if newKR.CurrentVersion() != 2 {
		t.Fatalf("current version = %d, want 2", newKR.CurrentVersion())
	}
	got, err := newKR.Decrypt(v, nonce, ct, []byte("tok"))
	if err != nil {
		t.Fatalf("post-rotation decrypt of old record failed: %v", err)
	}
	if string(got) != "legacy" {
		t.Fatalf("got %q, want legacy", got)
	}
}

func TestBadKeySize(t *testing.T) {
	if _, err := NewKeyringSingle([]byte("short")); err == nil {
		t.Fatal("expected key size error")
	}
}
