package protocol

import (
	"errors"
	"io"
	"testing"
)

func TestProtocolErrorError(t *testing.T) {
	if got := (&ProtocolError{Kind: "ERROR"}).Error(); got != "ERROR" {
		t.Fatalf("plain ERROR = %q", got)
	}
	if got := (&ProtocolError{Kind: "CLIENT_ERROR", Msg: "bad"}).Error(); got != "CLIENT_ERROR: bad" {
		t.Fatalf("with msg = %q", got)
	}
	se := &ProtocolError{Kind: "SERVER_ERROR", Msg: "boom"}
	if !se.IsServerError() || se.IsClientError() {
		t.Fatal("server/client classification wrong")
	}
}

func TestStoreStatusString(t *testing.T) {
	cases := map[StoreStatus]string{
		Stored:           "STORED",
		NotStored:        "NOT_STORED",
		Exists:           "EXISTS",
		NotFound:         "NOT_FOUND",
		NoReply:          "(noreply)",
		StoreStatus(999): "UNKNOWN",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("StoreStatus(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestReadLineEOFWithPartial(t *testing.T) {
	// A line with no trailing newline before EOF -> io.ErrUnexpectedEOF.
	_, err := ParseStorageReply(reader("STORED")) // no CRLF
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF for unterminated line, got %v", err)
	}
}

func TestReadLineEmptyEOF(t *testing.T) {
	// Immediate EOF -> io.EOF surfaced.
	_, err := ParseStorageReply(reader(""))
	if err == nil {
		t.Fatal("expected error on empty stream")
	}
}

func TestParseValuesBadHeader(t *testing.T) {
	// VALUE line missing the bytes field.
	if _, err := ParseValues(reader("VALUE foo 0\r\n")); !errors.Is(err, ErrUnexpectedReply) {
		t.Fatalf("short header: got %v", err)
	}
	// Non-numeric flags.
	if _, err := ParseValues(reader("VALUE foo x 3\r\nbar\r\nEND\r\n")); !errors.Is(err, ErrUnexpectedReply) {
		t.Fatalf("bad flags: got %v", err)
	}
	// Non-numeric byte count.
	if _, err := ParseValues(reader("VALUE foo 0 z\r\n")); !errors.Is(err, ErrUnexpectedReply) {
		t.Fatalf("bad bytes: got %v", err)
	}
}

func TestParseValuesMissingTrailingCRLF(t *testing.T) {
	// Data block not followed by CRLF (uses 'X' bytes then END instead).
	_, err := ParseValues(reader("VALUE foo 0 3\r\nbarXXEND\r\n"))
	if err == nil {
		t.Fatal("expected error for missing trailing CRLF")
	}
}

func TestParseStatsUnexpected(t *testing.T) {
	if _, err := ParseStats(reader("WAT\r\n")); !errors.Is(err, ErrUnexpectedReply) {
		t.Fatalf("got %v", err)
	}
	if _, err := ParseStats(reader("SERVER_ERROR busy\r\n")); err == nil {
		t.Fatal("expected server error")
	}
}

func TestParseDeleteUnexpected(t *testing.T) {
	if _, err := ParseDeleteReply(reader("WAT\r\n")); !errors.Is(err, ErrUnexpectedReply) {
		t.Fatalf("got %v", err)
	}
}

func TestParseNumericUnexpected(t *testing.T) {
	if _, _, err := ParseNumericReply(reader("notanumber\r\n")); !errors.Is(err, ErrUnexpectedReply) {
		t.Fatalf("got %v", err)
	}
}
