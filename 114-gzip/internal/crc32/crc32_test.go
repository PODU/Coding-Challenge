package crc32

import (
	stdcrc "hash/crc32"
	"math/rand"
	"testing"
)

func TestCheckVector(t *testing.T) {
	// Canonical CRC-32 check value.
	got := Checksum([]byte("123456789"))
	if got != 0xCBF43926 {
		t.Fatalf("Checksum(\"123456789\") = %#x, want 0xCBF43926", got)
	}
}

func TestAgainstStdlib(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	for size := 0; size < 5000; size += 137 {
		buf := make([]byte, size)
		rng.Read(buf)
		got := Checksum(buf)
		want := stdcrc.ChecksumIEEE(buf)
		if got != want {
			t.Fatalf("size %d: got %#x want %#x", size, got, want)
		}
	}
}

func TestUpdateIncremental(t *testing.T) {
	data := []byte("the quick brown fox")
	full := Checksum(data)
	part := Update(0, data[:5])
	part = Update(part, data[5:])
	if full != part {
		t.Fatalf("incremental %#x != full %#x", part, full)
	}
}
