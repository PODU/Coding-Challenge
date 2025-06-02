package smtp

import "testing"

func TestParseLine(t *testing.T) {
	tests := []struct {
		in       string
		wantVerb string
		wantArg  string
		wantErr  bool
	}{
		{"HELO example.com", "HELO", "example.com", false},
		{"mail from:<a@b>", "MAIL", "from:<a@b>", false},
		{"EHLO  spaced.example", "EHLO", "spaced.example", false},
		{"NOOP", "NOOP", "", false},
		{"QUIT\r\n", "QUIT", "", false},
		{"rcpt TO: <c@d.com>", "RCPT", "TO: <c@d.com>", false},
		{"   ", "", "", true},
		{"", "", "", true},
	}
	for _, tt := range tests {
		cmd, err := ParseLine(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLine(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
			continue
		}
		if tt.wantErr {
			continue
		}
		if cmd.Verb != tt.wantVerb || cmd.Arg != tt.wantArg {
			t.Errorf("ParseLine(%q) = {%q,%q}, want {%q,%q}", tt.in, cmd.Verb, cmd.Arg, tt.wantVerb, tt.wantArg)
		}
	}
}

func TestParseMailFrom(t *testing.T) {
	tests := []struct {
		arg     string
		want    string
		wantErr bool
	}{
		{"FROM:<a@b.com>", "a@b.com", false},
		{"FROM: <a@b.com>", "a@b.com", false},
		{"from:<a@b>", "a@b", false},
		{"FROM:<a@b> SIZE=1000", "a@b", false},
		{"FROM:<>", "", false}, // null reverse-path allowed for MAIL
		{"FROM:", "", true},
		{"FROM:a@b", "", true}, // missing angle brackets
		{"TO:<a@b>", "", true}, // wrong keyword
		{"<a@b>", "", true},    // missing keyword
	}
	for _, tt := range tests {
		got, err := ParseMailFrom(tt.arg)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseMailFrom(%q) err=%v wantErr=%v", tt.arg, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseMailFrom(%q) = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

func TestParseRcptTo(t *testing.T) {
	tests := []struct {
		arg     string
		want    string
		wantErr bool
	}{
		{"TO:<c@d.com>", "c@d.com", false},
		{"TO: <c@d.com>", "c@d.com", false},
		{"to:<c@d>", "c@d", false},
		{"TO:<c@d> ORCPT=rfc822;c@d", "c@d", false},
		{"TO:<>", "", true}, // null forward-path not allowed for RCPT
		{"TO:", "", true},
		{"FROM:<c@d>", "", true},
	}
	for _, tt := range tests {
		got, err := ParseRcptTo(tt.arg)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRcptTo(%q) err=%v wantErr=%v", tt.arg, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseRcptTo(%q) = %q, want %q", tt.arg, got, tt.want)
		}
	}
}
