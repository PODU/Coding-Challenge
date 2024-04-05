package xxd

import (
	"bytes"
	"testing"
)

func roundTrip(t *testing.T, in []byte, mutate func(*Config)) []byte {
	t.Helper()
	dcfg := NewConfig()
	if mutate != nil {
		mutate(&dcfg)
	}
	var dumped bytes.Buffer
	if err := Dump(bytes.NewReader(in), &dumped, dcfg); err != nil {
		t.Fatalf("Dump: %v", err)
	}

	rcfg := NewConfig()
	rcfg.Reverse = true
	rcfg.Plain = dcfg.Plain
	var restored bytes.Buffer
	if err := Reverse(&dumped, &restored, rcfg); err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	return restored.Bytes()
}

func TestReverseRoundTrip(t *testing.T) {
	inputs := map[string][]byte{
		"f1":        f1,
		"allbytes":  allBytes(),
		"one":       {'A'},
		"seventeen": []byte("0123456789abcdefX"),
		"empty":     {},
	}
	variants := []struct {
		name   string
		mutate func(*Config)
	}{
		{"default", nil},
		{"cols8", func(c *Config) { c.Cols = 8 }},
		{"group1", func(c *Config) { c.Group = 1 }},
		{"group0", func(c *Config) { c.Group = 0 }},
		{"cols8group4", func(c *Config) { c.Cols = 8; c.Group = 4 }},
		{"cols7", func(c *Config) { c.Cols = 7 }},
		{"plain", func(c *Config) { c.Plain = true; c.Cols = DefaultPlainCols }},
		{"plainCols16", func(c *Config) { c.Plain = true; c.Cols = 16 }},
	}

	for iname, in := range inputs {
		for _, v := range variants {
			t.Run(iname+"/"+v.name, func(t *testing.T) {
				got := roundTrip(t, in, v.mutate)
				if !bytes.Equal(got, in) {
					t.Errorf("round-trip mismatch\n got: %x\nwant: %x", got, in)
				}
			})
		}
	}
}

func TestReverseSeekedZeroFill(t *testing.T) {
	// A seeked dump reverses with the gap before the first offset zero-filled.
	dcfg := NewConfig()
	dcfg.Seek = 4
	var dumped bytes.Buffer
	if err := Dump(bytes.NewReader(f1), &dumped, dcfg); err != nil {
		t.Fatal(err)
	}
	rcfg := NewConfig()
	rcfg.Reverse = true
	var restored bytes.Buffer
	if err := Reverse(&dumped, &restored, rcfg); err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0, 0, 0, 0}, f1[4:]...)
	if !bytes.Equal(restored.Bytes(), want) {
		t.Errorf("seeked reverse\n got: %x\nwant: %x", restored.Bytes(), want)
	}
}

func TestReverseCRLF(t *testing.T) {
	in := "00000000: 4669 6c65 2031 2063 6f6e 7465 6e74 732e  File 1 contents.\r\n" +
		"00000010: 0a                                       .\r\n"
	rcfg := NewConfig()
	rcfg.Reverse = true
	var out bytes.Buffer
	if err := Reverse(bytes.NewReader([]byte(in)), &out, rcfg); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), f1) {
		t.Errorf("CRLF reverse\n got: %x\nwant: %x", out.Bytes(), f1)
	}
}

func TestReverseIgnoresGutter(t *testing.T) {
	// The ASCII gutter contains hex-looking text ("face") that must not be
	// decoded.
	in := "00000000: 6661 6365  face\n"
	rcfg := NewConfig()
	rcfg.Reverse = true
	var out bytes.Buffer
	if err := Reverse(bytes.NewReader([]byte(in)), &out, rcfg); err != nil {
		t.Fatal(err)
	}
	if string(out.Bytes()) != "face" {
		t.Errorf("got %q, want %q", out.String(), "face")
	}
}

func TestDecodeHexOddDigits(t *testing.T) {
	if _, err := decodeHex("abc"); err == nil {
		t.Error("expected error for odd hex digit count")
	}
}
