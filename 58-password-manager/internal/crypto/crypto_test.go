package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func testParams(t *testing.T) KDFParams {
	t.Helper()
	p, err := DefaultParams()
	if err != nil {
		t.Fatalf("DefaultParams: %v", err)
	}
	// Lower cost so the test suite stays fast while still exercising Argon2id.
	p.Time = 1
	p.Memory = 8 * 1024
	return p
}

func TestRoundTrip(t *testing.T) {
	p := testParams(t)
	key, err := DeriveKey([]byte("correct horse battery staple"), p)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	aad := []byte("header")

	cases := map[string][]byte{
		"empty": {},
		"small": []byte("hello"),
		"large": bytes.Repeat([]byte("A"), 100000),
	}
	for name, pt := range cases {
		t.Run(name, func(t *testing.T) {
			nonce, ct, err := Seal(key, pt, aad)
			if err != nil {
				t.Fatalf("Seal: %v", err)
			}
			got, err := Open(key, nonce, ct, aad)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if !bytes.Equal(got, pt) {
				t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(got), len(pt))
			}
		})
	}
}

func TestWrongKeyFails(t *testing.T) {
	p := testParams(t)
	good, _ := DeriveKey([]byte("right-password"), p)
	bad, _ := DeriveKey([]byte("wrong-password"), p)
	aad := []byte("header")

	nonce, ct, err := Seal(good, []byte("secret data"), aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	got, err := Open(bad, nonce, ct, aad)
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got err=%v", err)
	}
	if got != nil {
		t.Fatalf("expected no plaintext on failure, got %d bytes", len(got))
	}
}

func TestTamperDetection(t *testing.T) {
	p := testParams(t)
	key, _ := DeriveKey([]byte("pw"), p)
	aad := []byte("header-aad")
	nonce, ct, err := Seal(key, []byte("important"), aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	t.Run("ciphertext", func(t *testing.T) {
		bad := append([]byte(nil), ct...)
		bad[0] ^= 0x01
		if _, err := Open(key, nonce, bad, aad); !errors.Is(err, ErrAuth) {
			t.Fatalf("expected ErrAuth, got %v", err)
		}
	})
	t.Run("nonce", func(t *testing.T) {
		bad := append([]byte(nil), nonce...)
		bad[0] ^= 0x01
		if _, err := Open(key, bad, ct, aad); !errors.Is(err, ErrAuth) {
			t.Fatalf("expected ErrAuth, got %v", err)
		}
	})
	t.Run("aad", func(t *testing.T) {
		if _, err := Open(key, nonce, ct, []byte("different-aad")); !errors.Is(err, ErrAuth) {
			t.Fatalf("expected ErrAuth, got %v", err)
		}
	})
}

func TestNonceUniqueness(t *testing.T) {
	p := testParams(t)
	key, _ := DeriveKey([]byte("pw"), p)
	n1, c1, _ := Seal(key, []byte("same"), nil)
	n2, c2, _ := Seal(key, []byte("same"), nil)
	if bytes.Equal(n1, n2) {
		t.Fatal("two seals produced identical nonces")
	}
	if bytes.Equal(c1, c2) {
		t.Fatal("two seals of identical data produced identical ciphertext")
	}
}

func TestKDFDeterminism(t *testing.T) {
	p := testParams(t)
	k1, _ := DeriveKey([]byte("pw"), p)
	k2, _ := DeriveKey([]byte("pw"), p)
	if !bytes.Equal(k1, k2) {
		t.Fatal("same password+salt+params produced different keys")
	}
	if len(k1) != KeyLen {
		t.Fatalf("key length = %d, want %d", len(k1), KeyLen)
	}

	p2 := testParams(t) // fresh random salt
	k3, _ := DeriveKey([]byte("pw"), p2)
	if bytes.Equal(k1, k3) {
		t.Fatal("different salts produced the same key")
	}
}

func TestParamsRoundTrip(t *testing.T) {
	p := testParams(t)
	b, err := MarshalParams(p)
	if err != nil {
		t.Fatalf("MarshalParams: %v", err)
	}
	got, err := UnmarshalParams(b)
	if err != nil {
		t.Fatalf("UnmarshalParams: %v", err)
	}
	if got.Time != p.Time || got.Memory != p.Memory || got.Threads != p.Threads || !bytes.Equal(got.Salt, p.Salt) {
		t.Fatalf("params round-trip mismatch: %+v vs %+v", got, p)
	}
}

func TestValidate(t *testing.T) {
	if err := (KDFParams{Algorithm: "scrypt"}).Validate(); err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
	if err := (KDFParams{Algorithm: "argon2id", Time: 0}).Validate(); err == nil {
		t.Fatal("expected error for zero time")
	}
}
