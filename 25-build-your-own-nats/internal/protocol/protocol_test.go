package protocol

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseControlLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Command
	}{
		{"ping", "PING\r\n", PingCmd{}},
		{"pong", "PONG\r\n", PongCmd{}},
		{"ping lowercase", "ping\r\n", PingCmd{}},
		{"ping no crlf", "PING", PingCmd{}},
		{"sub", "SUB foo 1\r\n", SubCmd{Subject: "foo", Sid: "1"}},
		{"sub queue", "SUB foo bar 1\r\n", SubCmd{Subject: "foo", Queue: "bar", Sid: "1"}},
		{"pub", "PUB CodingChallenge 11\r\n", PubCmd{Subject: "CodingChallenge", Size: 11}},
		{"pub reply", "PUB foo INBOX.1 5\r\n", PubCmd{Subject: "foo", Reply: "INBOX.1", Size: 5}},
		{"unsub", "UNSUB 1\r\n", UnsubCmd{Sid: "1"}},
		{"unsub max", "UNSUB 1 5\r\n", UnsubCmd{Sid: "1", MaxMsgs: 5, HasMax: true}},
		{"unsub zero", "UNSUB 1 0\r\n", UnsubCmd{Sid: "1", MaxMsgs: 0, HasMax: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.in))
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseConnectDefaultsVerbose(t *testing.T) {
	got, err := Parse([]byte("CONNECT {}\r\n"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	cc, ok := got.(ConnectCmd)
	if !ok {
		t.Fatalf("want ConnectCmd, got %T", got)
	}
	if !cc.Options.Verbose {
		t.Fatalf("verbose should default to true for empty options")
	}
}

func TestParseConnectVerboseFalse(t *testing.T) {
	got, err := Parse([]byte(`CONNECT {"verbose":false,"name":"cli"}` + "\r\n"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	cc := got.(ConnectCmd)
	if cc.Options.Verbose {
		t.Fatalf("verbose should be false")
	}
	if cc.Options.Name != "cli" {
		t.Fatalf("name = %q, want cli", cc.Options.Name)
	}
}

func TestParseConnectTolerant(t *testing.T) {
	// Bare CONNECT and non-JSON options should still connect.
	for _, in := range []string{"CONNECT\r\n", "CONNECT garbage\r\n"} {
		got, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", in, err)
		}
		if _, ok := got.(ConnectCmd); !ok {
			t.Fatalf("Parse(%q) = %T, want ConnectCmd", in, got)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		in      string
		wantErr error
	}{
		{"", ErrEmptyLine},
		{"   \r\n", ErrEmptyLine},
		{"BOGUS foo\r\n", ErrUnknownOp},
		{"SUB\r\n", ErrInvalidArgs},
		{"PUB foo\r\n", ErrInvalidArgs},
		{"PUB foo notanumber\r\n", ErrInvalidArgs},
		{"UNSUB\r\n", ErrInvalidArgs},
		{"UNSUB 1 notanumber\r\n", ErrInvalidArgs},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			_, err := Parse([]byte(tt.in))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Parse(%q) err = %v, want %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestEncoders(t *testing.T) {
	if got := string(OK()); got != "+OK\r\n" {
		t.Fatalf("OK = %q", got)
	}
	if got := string(Pong()); got != "PONG\r\n" {
		t.Fatalf("Pong = %q", got)
	}
	if got := string(EncodeErr("Unknown Protocol Operation")); got != "-ERR 'Unknown Protocol Operation'\r\n" {
		t.Fatalf("EncodeErr = %q", got)
	}
}

func TestEncodeMsgRoundTrip(t *testing.T) {
	// The challenge's canonical example.
	got := string(EncodeMsg("CodingChallenge", "1", "", []byte("Hello John!")))
	want := "MSG CodingChallenge 1 11\r\nHello John!\r\n"
	if got != want {
		t.Fatalf("EncodeMsg = %q, want %q", got, want)
	}
}

func TestEncodeMsgWithReply(t *testing.T) {
	got := string(EncodeMsg("foo", "2", "INBOX.7", []byte("hi")))
	want := "MSG foo 2 INBOX.7 2\r\nhi\r\n"
	if got != want {
		t.Fatalf("EncodeMsg = %q, want %q", got, want)
	}
}

func TestParsePubThenPayloadRoundTrip(t *testing.T) {
	// Parse the control line of "PUB CodingChallenge 11\r\nHello John!\r\n";
	// payload is read separately by the caller using Size.
	cmd, err := Parse([]byte("PUB CodingChallenge 11\r\n"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	pub := cmd.(PubCmd)
	if pub.Size != len("Hello John!") {
		t.Fatalf("Size = %d, want %d", pub.Size, len("Hello John!"))
	}
}

func TestEncodeInfoIsValidFrame(t *testing.T) {
	b := EncodeInfo(Info{ServerID: "x", Version: "0.1.0", Proto: 1, MaxPayload: 1024})
	s := string(b)
	if len(s) < len("INFO {}\r\n") || s[:5] != "INFO " || s[len(s)-2:] != "\r\n" {
		t.Fatalf("INFO frame malformed: %q", s)
	}
}
