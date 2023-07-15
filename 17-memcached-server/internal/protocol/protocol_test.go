package protocol

import (
	"strings"
	"testing"
)

func TestParseStorageValid(t *testing.T) {
	cmd, err := Parse("set foo 42 0 3\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Type != TypeStorage || cmd.Op != "set" || cmd.Key != "foo" {
		t.Fatalf("bad command: %+v", cmd)
	}
	if cmd.Flags != 42 || cmd.Exptime != 0 || cmd.Bytes != 3 || cmd.NoReply {
		t.Fatalf("bad fields: %+v", cmd)
	}
}

func TestParseStorageNoReply(t *testing.T) {
	cmd, err := Parse("set foo 0 0 3 noreply\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd.NoReply {
		t.Fatalf("expected noreply set")
	}
}

func TestParseCas(t *testing.T) {
	cmd, err := Parse("cas foo 0 0 3 12345\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Op != "cas" || cmd.CASUnique != 12345 {
		t.Fatalf("bad cas parse: %+v", cmd)
	}
	cmd2, err := Parse("cas foo 0 0 3 12345 noreply\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cmd2.NoReply || cmd2.CASUnique != 12345 {
		t.Fatalf("bad cas noreply parse: %+v", cmd2)
	}
}

func TestParseRetrievalMultiKey(t *testing.T) {
	cmd, err := Parse("gets a b c\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Type != TypeRetrieval || cmd.Op != "gets" {
		t.Fatalf("bad type: %+v", cmd)
	}
	if strings.Join(cmd.Keys, ",") != "a,b,c" {
		t.Fatalf("bad keys: %v", cmd.Keys)
	}
}

func TestParseDelete(t *testing.T) {
	cmd, err := Parse("delete foo noreply\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd.Type != TypeDelete || cmd.Key != "foo" || !cmd.NoReply {
		t.Fatalf("bad delete: %+v", cmd)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		line   string
		client bool // expect CLIENT_ERROR (true) vs ERROR (false)
	}{
		{"frobnicate foo\r\n", false},      // unknown command
		{"\r\n", false},                    // empty
		{"set foo 0 0\r\n", true},          // too few fields
		{"set foo x 0 3\r\n", true},        // bad flags
		{"set foo 0 y 3\r\n", true},        // bad exptime
		{"set foo 0 0 z\r\n", true},        // bad bytes
		{"set foo 0 0 -1\r\n", true},       // negative bytes
		{"cas foo 0 0 3\r\n", true},        // cas missing token
		{"cas foo 0 0 3 notnum\r\n", true}, // cas bad token
		{"get\r\n", true},                  // retrieval missing key
		{"delete\r\n", true},               // delete missing key
		{"set foo 0 0 3 bogus\r\n", true},  // bad trailing token
	}
	for _, c := range cases {
		_, err := Parse(c.line)
		if err == nil {
			t.Errorf("expected error for %q", c.line)
			continue
		}
		if err.Client != c.client {
			t.Errorf("%q: got client=%v want %v", c.line, err.Client, c.client)
		}
	}
}

func TestKeyTooLong(t *testing.T) {
	longKey := strings.Repeat("k", 251)
	if _, err := Parse("set " + longKey + " 0 0 3\r\n"); err == nil {
		t.Fatalf("expected error for over-long key")
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	lines := []string{
		"set foo 42 0 3\r\n",
		"add bar 0 100 5 noreply\r\n",
		"cas baz 1 0 2 999\r\n",
		"cas baz 1 0 2 999 noreply\r\n",
		"get a b c\r\n",
		"gets x\r\n",
		"delete foo\r\n",
		"delete foo noreply\r\n",
	}
	for _, line := range lines {
		cmd, err := Parse(line)
		if err != nil {
			t.Fatalf("parse %q: %v", line, err)
		}
		if got := cmd.Encode(); got != line {
			t.Errorf("round trip mismatch: parsed %q encoded %q", line, got)
		}
	}
}
