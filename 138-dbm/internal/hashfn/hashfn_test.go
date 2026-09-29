package hashfn

import "testing"

func TestFNV1aVectors(t *testing.T) {
	cases := map[string]uint64{
		"":       0xcbf29ce484222325,
		"a":      0xaf63dc4c8601ec8c,
		"foobar": 0x85944171f73967e8,
	}
	for in, want := range cases {
		if got := FNV1a64([]byte(in)); got != want {
			t.Errorf("fnv1a(%q) = %#x, want %#x", in, got, want)
		}
	}
}

func TestDJB2Vectors(t *testing.T) {
	if got := DJB2Hash(nil); got != 5381 {
		t.Fatalf("djb2(\"\") = %d, want 5381", got)
	}
	// h("a") = 5381*33 + 97
	if got := DJB2Hash([]byte("a")); got != 5381*33+97 {
		t.Fatalf("djb2(\"a\") = %d", got)
	}
}

func TestMurmur3Vectors(t *testing.T) {
	cases := []struct {
		in   string
		seed uint32
		want uint32
	}{
		{"", 0, 0},
		{"", 1, 0x514e28b7},
		{"hello", 0, 0x248bfa47},
		{"The quick brown fox jumps over the lazy dog", 0, 0x2e4ff723},
	}
	for _, c := range cases {
		if got := Murmur3x86_32([]byte(c.in), c.seed); got != c.want {
			t.Errorf("murmur3(%q, %d) = %#x, want %#x", c.in, c.seed, got, c.want)
		}
	}
}

func TestParseAndNames(t *testing.T) {
	for _, name := range Names() {
		id, err := Parse(name)
		if err != nil {
			t.Fatalf("Parse(%q): %v", name, err)
		}
		if id.String() != name || !id.Valid() || id.Func() == nil {
			t.Fatalf("round trip failed for %q", name)
		}
	}
	if _, err := Parse("sha256"); err == nil {
		t.Fatal("expected error for unknown hash")
	}
	if ID(99).Valid() || ID(99).Func() != nil {
		t.Fatal("ID 99 should be invalid")
	}
	if id, _ := Parse(" FNV1A "); id != FNV1a {
		t.Fatal("Parse should be case/space insensitive")
	}
}

func TestLowBitsSpread(t *testing.T) {
	// The store indexes buckets by low bits; sequential keys must not all
	// land in the same low-bit class for any registered function.
	for _, name := range Names() {
		id, _ := Parse(name)
		fn := id.Func()
		seen := map[uint64]bool{}
		for i := 0; i < 256; i++ {
			seen[fn([]byte("key-"+string(rune('A'+i%26))+string(rune('a'+i%7))+string(rune('0'+i%10))))&0xf] = true
		}
		if len(seen) < 12 {
			t.Errorf("%s: only %d of 16 low-bit classes hit", name, len(seen))
		}
	}
}
