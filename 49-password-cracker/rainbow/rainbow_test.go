package rainbow

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"passcrack/hashreg"
	"passcrack/saltutil"
)

func TestBuildAndLookup(t *testing.T) {
	md5algo, _ := hashreg.Get("md5")
	noSalt := saltutil.New("", saltutil.Prefix)
	words := "apple\npassword\nhunter2\n\ncorrecthorse\n"

	var table bytes.Buffer
	n, err := Build(context.Background(), strings.NewReader(words), &table, md5algo, noSalt)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if n != 4 {
		t.Fatalf("built %d entries, want 4", n)
	}

	hunterHash := hashreg.HexSum(md5algo, []byte("hunter2"))
	cracked, miss, err := Lookup(bytes.NewReader(table.Bytes()),
		[]string{strings.ToUpper(hunterHash), "ffffffffffffffffffffffffffffffff"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if cracked[hunterHash] != "hunter2" {
		t.Fatalf("lookup hunter2 = %q, want hunter2 (cracked=%v)", cracked[hunterHash], cracked)
	}
	if len(miss) != 1 || miss[0] != "ffffffffffffffffffffffffffffffff" {
		t.Fatalf("miss = %v, want the unknown hash", miss)
	}
}

func TestBuildSalted(t *testing.T) {
	md5algo, _ := hashreg.Get("md5")
	salter := saltutil.New("xyz", saltutil.Suffix)
	var table bytes.Buffer
	if _, err := Build(context.Background(), strings.NewReader("hunter2\n"), &table, md5algo, salter); err != nil {
		t.Fatalf("Build: %v", err)
	}
	saltedHash := hashreg.HexSum(md5algo, []byte("hunter2xyz"))
	cracked, _, err := Lookup(bytes.NewReader(table.Bytes()), []string{saltedHash})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if cracked[saltedHash] != "hunter2" {
		t.Fatalf("salted rainbow lookup failed: %v", cracked)
	}
}
