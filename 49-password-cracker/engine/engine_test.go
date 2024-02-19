package engine

import (
	"context"
	"strings"
	"testing"

	"passcrack/bruteforce"
	"passcrack/hashreg"
	"passcrack/saltutil"
	"passcrack/wordlist"
)

func md5algo(t *testing.T) hashreg.Algo {
	t.Helper()
	a, err := hashreg.Get("md5")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

const testWords = "apple\npassword\nhunter2\ncorrecthorse\nbanana\n"

func newDictCracker(t *testing.T, targets []string, salter saltutil.Salter, workers int) *Cracker {
	t.Helper()
	cr, err := New(Config{Algo: md5algo(t), Salter: salter, Targets: targets, Workers: workers})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return cr
}

// Dictionary mode finds a known hash.
func TestDictionaryCrack(t *testing.T) {
	target := hashreg.HexSum(md5algo(t), []byte("hunter2"))
	cr := newDictCracker(t, []string{target}, saltutil.New("", saltutil.Prefix), 0)
	cand, _ := wordlist.Stream(context.Background(), strings.NewReader(testWords))
	cracked, uncracked := cr.Run(context.Background(), cand, nil)
	if cracked[target] != "hunter2" {
		t.Fatalf("cracked = %v, want hunter2", cracked)
	}
	if len(uncracked) != 0 {
		t.Fatalf("uncracked = %v, want none", uncracked)
	}
}

// Test-plan #2 negative: a hash not in the wordlist stays UNCRACKED.
func TestDictionaryNotFound(t *testing.T) {
	target := hashreg.HexSum(md5algo(t), []byte("absent-word"))
	cr := newDictCracker(t, []string{target}, saltutil.New("", saltutil.Prefix), 0)
	cand, _ := wordlist.Stream(context.Background(), strings.NewReader(testWords))
	cracked, uncracked := cr.Run(context.Background(), cand, nil)
	if len(cracked) != 0 {
		t.Fatalf("cracked = %v, want none", cracked)
	}
	if len(uncracked) != 1 || uncracked[0] != target {
		t.Fatalf("uncracked = %v, want [%s]", uncracked, target)
	}
}

// Test-plan #6: multiple targets, both recovered in one pass.
func TestMultipleHashes(t *testing.T) {
	a := hashreg.HexSum(md5algo(t), []byte("apple"))
	b := hashreg.HexSum(md5algo(t), []byte("banana"))
	cr := newDictCracker(t, []string{a, b}, saltutil.New("", saltutil.Prefix), 0)
	cand, _ := wordlist.Stream(context.Background(), strings.NewReader(testWords))
	cracked, uncracked := cr.Run(context.Background(), cand, nil)
	if cracked[a] != "apple" || cracked[b] != "banana" {
		t.Fatalf("cracked = %v, want apple+banana", cracked)
	}
	if len(uncracked) != 0 {
		t.Fatalf("uncracked = %v", uncracked)
	}
}

// Test-plan #5: salted hash cracks only with the matching salt/mode.
func TestSaltedCrack(t *testing.T) {
	prefixSalt := saltutil.New("s4lt", saltutil.Prefix)
	target := hashreg.HexSum(md5algo(t), prefixSalt.Apply("hunter2"))

	// With the correct salt -> cracked.
	cr := newDictCracker(t, []string{target}, prefixSalt, 0)
	cand, _ := wordlist.Stream(context.Background(), strings.NewReader(testWords))
	cracked, _ := cr.Run(context.Background(), cand, nil)
	if cracked[target] != "hunter2" {
		t.Fatalf("salted crack failed: %v", cracked)
	}

	// Without salt (plain) -> uncracked.
	cr2 := newDictCracker(t, []string{target}, saltutil.New("", saltutil.Prefix), 0)
	cand2, _ := wordlist.Stream(context.Background(), strings.NewReader(testWords))
	cracked2, uncracked2 := cr2.Run(context.Background(), cand2, nil)
	if len(cracked2) != 0 || len(uncracked2) != 1 {
		t.Fatalf("plain attack should fail on salted hash: cracked=%v", cracked2)
	}
}

// Test-plan #3: brute-force over a small space.
func TestBruteForceCrack(t *testing.T) {
	target := hashreg.HexSum(md5algo(t), []byte("cab"))
	cr := newDictCracker(t, []string{target}, saltutil.New("", saltutil.Prefix), 0)
	cand := bruteforce.Generate(context.Background(), "abc", 1, 3)
	cracked, uncracked := cr.Run(context.Background(), cand, nil)
	if cracked[target] != "cab" {
		t.Fatalf("brute crack = %v, want cab", cracked)
	}
	if len(uncracked) != 0 {
		t.Fatalf("uncracked = %v", uncracked)
	}
}

// Test-plan #4: parity with the challenge's published brute-force MD5 vectors.
func TestChallengeBruteVectors(t *testing.T) {
	const (
		passHash = "7a95bf926a0333f57705aeac07a362a2" // MD5("PASS")
		codeHash = "08054846bbc9933fd0395f8be516a9f9" // MD5("CODE")
	)
	cr := newDictCracker(t, []string{passHash, codeHash}, saltutil.New("", saltutil.Prefix), 0)
	cand := bruteforce.Generate(context.Background(), "ABCDEFGHIJKLMNOPQRSTUVWXYZ", 1, 4)
	cracked, uncracked := cr.Run(context.Background(), cand, nil)
	if cracked[passHash] != "PASS" || cracked[codeHash] != "CODE" {
		t.Fatalf("challenge vectors = %v, want PASS+CODE", cracked)
	}
	if len(uncracked) != 0 {
		t.Fatalf("uncracked = %v", uncracked)
	}
}

// Targets may be upper-case or padded with whitespace.
func TestTargetNormalization(t *testing.T) {
	target := hashreg.HexSum(md5algo(t), []byte("apple"))
	cr := newDictCracker(t, []string{"  " + strings.ToUpper(target) + "\n"}, saltutil.New("", saltutil.Prefix), 0)
	cand, _ := wordlist.Stream(context.Background(), strings.NewReader(testWords))
	cracked, _ := cr.Run(context.Background(), cand, nil)
	if cracked[target] != "apple" {
		t.Fatalf("normalization failed: %v", cracked)
	}
}

// Test-plan #9: identical results regardless of worker count.
func TestWorkerCountInvariance(t *testing.T) {
	a := hashreg.HexSum(md5algo(t), []byte("apple"))
	b := hashreg.HexSum(md5algo(t), []byte("hunter2"))
	for _, w := range []int{1, 2, 4, 8} {
		cr := newDictCracker(t, []string{a, b}, saltutil.New("", saltutil.Prefix), w)
		cand, _ := wordlist.Stream(context.Background(), strings.NewReader(testWords))
		cracked, uncracked := cr.Run(context.Background(), cand, nil)
		if cracked[a] != "apple" || cracked[b] != "hunter2" || len(uncracked) != 0 {
			t.Fatalf("workers=%d gave cracked=%v uncracked=%v", w, cracked, uncracked)
		}
	}
}

func TestNewValidation(t *testing.T) {
	// no targets
	if _, err := New(Config{Algo: md5algo(t)}); err == nil {
		t.Error("expected error with no targets")
	}
	// bad length
	if _, err := New(Config{Algo: md5algo(t), Targets: []string{"abc"}}); err == nil {
		t.Error("expected error for short hash")
	}
	// non-hex
	bad := strings.Repeat("z", 32)
	if _, err := New(Config{Algo: md5algo(t), Targets: []string{bad}}); err == nil {
		t.Error("expected error for non-hex hash")
	}
	// nil algo
	if _, err := New(Config{Targets: []string{strings.Repeat("a", 32)}}); err == nil {
		t.Error("expected error for nil algo")
	}
}
