package parser

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    Message
		numeric bool
	}{
		{
			name: "ping no prefix",
			line: "PING :LAG1234567",
			want: Message{
				Raw:     "PING :LAG1234567",
				Command: "PING",
				Params:  []string{"LAG1234567"},
			},
		},
		{
			name: "numeric welcome",
			line: ":*.libera.chat 001 CCIRC :Welcome to the Libera.Chat Network CCIRC",
			want: Message{
				Raw:     ":*.libera.chat 001 CCIRC :Welcome to the Libera.Chat Network CCIRC",
				Prefix:  "*.libera.chat",
				Nick:    "*.libera.chat",
				Command: "001",
				Params:  []string{"CCIRC", "Welcome to the Libera.Chat Network CCIRC"},
			},
			numeric: true,
		},
		{
			name: "notice with star target",
			line: ":*.libera.chat NOTICE * :*** Looking up your ident...",
			want: Message{
				Raw:     ":*.libera.chat NOTICE * :*** Looking up your ident...",
				Prefix:  "*.libera.chat",
				Nick:    "*.libera.chat",
				Command: "NOTICE",
				Params:  []string{"*", "*** Looking up your ident..."},
			},
		},
		{
			name: "join with leading colon on channel",
			line: ":CCIRC!~guest@example.com JOIN :#cc",
			want: Message{
				Raw:     ":CCIRC!~guest@example.com JOIN :#cc",
				Prefix:  "CCIRC!~guest@example.com",
				Nick:    "CCIRC",
				User:    "~guest",
				Host:    "example.com",
				Command: "JOIN",
				Params:  []string{"#cc"},
			},
		},
		{
			name: "join without leading colon on channel",
			line: ":CCIRC!~guest@example.com JOIN #cc",
			want: Message{
				Raw:     ":CCIRC!~guest@example.com JOIN #cc",
				Prefix:  "CCIRC!~guest@example.com",
				Nick:    "CCIRC",
				User:    "~guest",
				Host:    "example.com",
				Command: "JOIN",
				Params:  []string{"#cc"},
			},
		},
		{
			name: "part",
			line: ":Bob!~bob@host PART #cc",
			want: Message{
				Raw:     ":Bob!~bob@host PART #cc",
				Prefix:  "Bob!~bob@host",
				Nick:    "Bob",
				User:    "~bob",
				Host:    "host",
				Command: "PART",
				Params:  []string{"#cc"},
			},
		},
		{
			name: "nick change",
			line: ":Old!~guest@host NICK :New",
			want: Message{
				Raw:     ":Old!~guest@host NICK :New",
				Prefix:  "Old!~guest@host",
				Nick:    "Old",
				User:    "~guest",
				Host:    "host",
				Command: "NICK",
				Params:  []string{"New"},
			},
		},
		{
			name: "privmsg to channel",
			line: ":Alice!~a@host PRIVMSG #cc :Hello there world",
			want: Message{
				Raw:     ":Alice!~a@host PRIVMSG #cc :Hello there world",
				Prefix:  "Alice!~a@host",
				Nick:    "Alice",
				User:    "~a",
				Host:    "host",
				Command: "PRIVMSG",
				Params:  []string{"#cc", "Hello there world"},
			},
		},
		{
			name: "privmsg to user",
			line: ":Alice!~a@host PRIVMSG CCIRC :hi in private",
			want: Message{
				Raw:     ":Alice!~a@host PRIVMSG CCIRC :hi in private",
				Prefix:  "Alice!~a@host",
				Nick:    "Alice",
				User:    "~a",
				Host:    "host",
				Command: "PRIVMSG",
				Params:  []string{"CCIRC", "hi in private"},
			},
		},
		{
			name: "ctcp action",
			line: ":Alice!~a@host PRIVMSG #cc :\x01ACTION waves\x01",
			want: Message{
				Raw:     ":Alice!~a@host PRIVMSG #cc :\x01ACTION waves\x01",
				Prefix:  "Alice!~a@host",
				Nick:    "Alice",
				User:    "~a",
				Host:    "host",
				Command: "PRIVMSG",
				Params:  []string{"#cc", "\x01ACTION waves\x01"},
			},
		},
		{
			name: "quit with reason",
			line: ":Alice!~a@host QUIT :Quit: bye bye!",
			want: Message{
				Raw:     ":Alice!~a@host QUIT :Quit: bye bye!",
				Prefix:  "Alice!~a@host",
				Nick:    "Alice",
				User:    "~a",
				Host:    "host",
				Command: "QUIT",
				Params:  []string{"Quit: bye bye!"},
			},
		},
		{
			name: "trailing with spaces and colons",
			line: ":s 372 me :- visit https://example.com: now",
			want: Message{
				Raw:     ":s 372 me :- visit https://example.com: now",
				Prefix:  "s",
				Nick:    "s",
				Command: "372",
				Params:  []string{"me", "- visit https://example.com: now"},
			},
			numeric: true,
		},
		{
			name: "ircv3 tags ignored gracefully",
			line: "@time=2023-01-01T00:00:00.000Z;account=alice :Alice!~a@host PRIVMSG #cc :tagged",
			want: Message{
				Raw:     "@time=2023-01-01T00:00:00.000Z;account=alice :Alice!~a@host PRIVMSG #cc :tagged",
				Tags:    map[string]string{"time": "2023-01-01T00:00:00.000Z", "account": "alice"},
				Prefix:  "Alice!~a@host",
				Nick:    "Alice",
				User:    "~a",
				Host:    "host",
				Command: "PRIVMSG",
				Params:  []string{"#cc", "tagged"},
			},
		},
		{
			name: "command without params",
			line: ":nick!u@h JOIN",
			want: Message{
				Raw:     ":nick!u@h JOIN",
				Prefix:  "nick!u@h",
				Nick:    "nick",
				User:    "u",
				Host:    "h",
				Command: "JOIN",
			},
		},
		{
			name: "tolerates crlf",
			line: "PING :token\r\n",
			want: Message{
				Raw:     "PING :token",
				Command: "PING",
				Params:  []string{"token"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.line, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q)\n  got  %#v\n  want %#v", tt.line, got, tt.want)
			}
			if got.IsNumeric() != tt.numeric {
				t.Errorf("IsNumeric() = %v, want %v", got.IsNumeric(), tt.numeric)
			}
		})
	}
}

func TestParseMalformed(t *testing.T) {
	bad := []string{
		"",
		"   ",
		"\r\n",
		"@only=tags",
		":prefix-only",
	}
	for _, line := range bad {
		t.Run(line, func(t *testing.T) {
			if _, err := Parse(line); err == nil {
				t.Errorf("Parse(%q) expected error, got nil", line)
			}
		})
	}
}

func TestParseNoPanicFuzzySamples(t *testing.T) {
	samples := []string{
		"::::",
		":n!u@h",
		"PRIVMSG",
		"@a;b;c= PING",
		":a@b!c PRIVMSG #x :y",
	}
	for _, s := range samples {
		_, _ = Parse(s) // must not panic
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		name    string
		command string
		params  []string
		want    string
	}{
		{"nick", "NICK", []string{"CCClient"}, "NICK CCClient"},
		{"user", "USER", []string{"guest", "0", "*", "Coding Challenges Client"}, "USER guest 0 * :Coding Challenges Client"},
		{"join no colon", "JOIN", []string{"#cc"}, "JOIN #cc"},
		{"part trailing", "PART", []string{"#cc"}, "PART :#cc"},
		{"privmsg", "PRIVMSG", []string{"#cc", "Hello there"}, "PRIVMSG #cc :Hello there"},
		{"privmsg single word forced trailing", "PRIVMSG", []string{"#cc", "hi"}, "PRIVMSG #cc :hi"},
		{"pong", "PONG", []string{"LAG1234567"}, "PONG :LAG1234567"},
		{"quit", "QUIT", []string{"Leaving"}, "QUIT :Leaving"},
		{"no params", "LIST", nil, "LIST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Encode(tt.command, tt.params...); got != tt.want {
				t.Errorf("Encode(%q, %v) = %q, want %q", tt.command, tt.params, got, tt.want)
			}
		})
	}
}

// TestRoundTrip verifies that building outgoing lines produces exactly the
// expected wire bytes including CRLF, and that re-parsing yields the same
// command and parameters.
func TestRoundTrip(t *testing.T) {
	cases := []struct {
		command string
		params  []string
		wire    string
	}{
		{"JOIN", []string{"#cc"}, "JOIN #cc\r\n"},
		{"PART", []string{"#cc"}, "PART :#cc\r\n"},
		{"PRIVMSG", []string{"#cc", "Hello there"}, "PRIVMSG #cc :Hello there\r\n"},
		{"NICK", []string{"NewNick"}, "NICK NewNick\r\n"},
		{"QUIT", []string{"bye for now"}, "QUIT :bye for now\r\n"},
		{"PONG", []string{"abc"}, "PONG :abc\r\n"},
	}
	for _, c := range cases {
		line := Encode(c.command, c.params...)
		wire := line + "\r\n"
		if wire != c.wire {
			t.Errorf("wire for %s%v = %q, want %q", c.command, c.params, wire, c.wire)
		}
		m, err := Parse(line)
		if err != nil {
			t.Fatalf("re-parse %q: %v", line, err)
		}
		if m.Command != c.command || !reflect.DeepEqual(m.Params, c.params) {
			t.Errorf("re-parse %q => command %q params %v, want %q %v",
				line, m.Command, m.Params, c.command, c.params)
		}
	}
}
