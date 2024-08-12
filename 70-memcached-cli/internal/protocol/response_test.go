package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func reader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestParseStorageReply(t *testing.T) {
	tests := []struct {
		in      string
		want    StoreStatus
		wantErr bool
		kind    string
	}{
		{in: "STORED\r\n", want: Stored},
		{in: "NOT_STORED\r\n", want: NotStored},
		{in: "EXISTS\r\n", want: Exists},
		{in: "NOT_FOUND\r\n", want: NotFound},
		{in: "ERROR\r\n", wantErr: true, kind: "ERROR"},
		{in: "CLIENT_ERROR bad data chunk\r\n", wantErr: true, kind: "CLIENT_ERROR"},
		{in: "SERVER_ERROR object too large for cache\r\n", wantErr: true, kind: "SERVER_ERROR"},
		{in: "WAT\r\n", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(strings.TrimSpace(tc.in), func(t *testing.T) {
			got, err := ParseStorageReply(reader(tc.in))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got status %v", got)
				}
				if tc.kind != "" {
					var perr *ProtocolError
					if !errors.As(err, &perr) || perr.Kind != tc.kind {
						t.Fatalf("expected ProtocolError kind %q, got %v", tc.kind, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseValues(t *testing.T) {
	t.Run("zero values (miss)", func(t *testing.T) {
		vals, err := ParseValues(reader("END\r\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 0 {
			t.Fatalf("expected 0 values, got %d", len(vals))
		}
	})

	t.Run("single value", func(t *testing.T) {
		vals, err := ParseValues(reader("VALUE foo 0 3\r\nbar\r\nEND\r\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 1 {
			t.Fatalf("expected 1 value, got %d", len(vals))
		}
		v := vals[0]
		if v.Key != "foo" || v.Flags != 0 || string(v.Data) != "bar" {
			t.Fatalf("unexpected value: %+v", v)
		}
		if v.HasCAS {
			t.Fatalf("did not expect CAS")
		}
	})

	t.Run("multiple values", func(t *testing.T) {
		in := "VALUE a 1 1\r\nx\r\nVALUE b 2 2\r\nyy\r\nEND\r\n"
		vals, err := ParseValues(reader(in))
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 2 {
			t.Fatalf("expected 2 values, got %d", len(vals))
		}
		if vals[0].Key != "a" || string(vals[0].Data) != "x" || vals[0].Flags != 1 {
			t.Fatalf("value 0 wrong: %+v", vals[0])
		}
		if vals[1].Key != "b" || string(vals[1].Data) != "yy" || vals[1].Flags != 2 {
			t.Fatalf("value 1 wrong: %+v", vals[1])
		}
	})

	t.Run("value containing CRLF", func(t *testing.T) {
		// data is exactly 4 bytes "a\r\nb"
		in := "VALUE k 0 4\r\na\r\nb\r\nEND\r\n"
		vals, err := ParseValues(reader(in))
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 1 || string(vals[0].Data) != "a\r\nb" {
			t.Fatalf("framing failed: %+v", vals)
		}
	})

	t.Run("gets with CAS", func(t *testing.T) {
		vals, err := ParseValues(reader("VALUE foo 0 3 99\r\nbar\r\nEND\r\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(vals) != 1 || !vals[0].HasCAS || vals[0].CAS != 99 {
			t.Fatalf("expected CAS 99, got %+v", vals[0])
		}
	})

	t.Run("truncated data block", func(t *testing.T) {
		_, err := ParseValues(reader("VALUE foo 0 10\r\nbar")) // declares 10, has 3
		if !errors.Is(err, ErrTruncated) {
			t.Fatalf("expected ErrTruncated, got %v", err)
		}
	})

	t.Run("error line in retrieval", func(t *testing.T) {
		_, err := ParseValues(reader("ERROR\r\n"))
		var perr *ProtocolError
		if !errors.As(err, &perr) || perr.Kind != "ERROR" {
			t.Fatalf("expected ERROR, got %v", err)
		}
	})
}

func TestParseDeleteReply(t *testing.T) {
	if ok, err := ParseDeleteReply(reader("DELETED\r\n")); err != nil || !ok {
		t.Fatalf("DELETED: ok=%v err=%v", ok, err)
	}
	if ok, err := ParseDeleteReply(reader("NOT_FOUND\r\n")); err != nil || ok {
		t.Fatalf("NOT_FOUND: ok=%v err=%v", ok, err)
	}
	if _, err := ParseDeleteReply(reader("ERROR\r\n")); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseNumericReply(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		v, found, err := ParseNumericReply(reader("11\r\n"))
		if err != nil || !found || v != 11 {
			t.Fatalf("got v=%d found=%v err=%v", v, found, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		_, found, err := ParseNumericReply(reader("NOT_FOUND\r\n"))
		if err != nil || found {
			t.Fatalf("expected not found, got found=%v err=%v", found, err)
		}
	})
	t.Run("client error non-numeric", func(t *testing.T) {
		_, _, err := ParseNumericReply(reader("CLIENT_ERROR cannot increment or decrement non-numeric value\r\n"))
		var perr *ProtocolError
		if !errors.As(err, &perr) || !perr.IsClientError() {
			t.Fatalf("expected CLIENT_ERROR, got %v", err)
		}
	})
}

func TestParseStats(t *testing.T) {
	in := "STAT pid 1234\r\nSTAT uptime 4200\r\nSTAT curr_items 7\r\nEND\r\n"
	stats, err := ParseStats(reader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []Stat{{"pid", "1234"}, {"uptime", "4200"}, {"curr_items", "7"}}
	if len(stats) != len(want) {
		t.Fatalf("got %d stats, want %d", len(stats), len(want))
	}
	for i := range want {
		if stats[i] != want[i] {
			t.Fatalf("stat %d: got %+v want %+v", i, stats[i], want[i])
		}
	}
}

// TestRoundTrip encodes a storage command and a retrieval reply, then parses
// the reply back, asserting the data survives exact framing including binary.
func TestRoundTrip(t *testing.T) {
	data := []byte{0x00, '\r', '\n', 0xff, 'x'}
	enc := EncodeStorage(StorageCommand{Command: "set", Key: "bin", Data: data})
	// The data block in the request must be exactly len(data) bytes.
	idx := bytes.Index(enc, []byte("\r\n"))
	body := enc[idx+2 : len(enc)-2]
	if !bytes.Equal(body, data) {
		t.Fatalf("request body mismatch: %q vs %q", body, data)
	}

	// Build a matching VALUE reply and parse it.
	var b bytes.Buffer
	b.WriteString("VALUE bin 0 ")
	b.WriteString("5\r\n")
	b.Write(data)
	b.WriteString("\r\nEND\r\n")
	vals, err := ParseValues(bufio.NewReader(&b))
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 1 || !bytes.Equal(vals[0].Data, data) {
		t.Fatalf("round-trip data mismatch: %+v", vals)
	}
}
