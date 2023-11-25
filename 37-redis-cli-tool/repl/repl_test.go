package repl

import (
	"reflect"
	"strings"
	"testing"

	"rediscli/resp"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"simple", "SET name John", []string{"SET", "name", "John"}, false},
		{"extra spaces", "  PING   ", []string{"PING"}, false},
		{"tabs", "SET\tk\tv", []string{"SET", "k", "v"}, false},
		{"double quotes", `SET k "two words"`, []string{"SET", "k", "two words"}, false},
		{"single quotes", `SET k 'two words'`, []string{"SET", "k", "two words"}, false},
		{"empty quoted", `SET k ""`, []string{"SET", "k", ""}, false},
		{"escape newline", `SET k "a\nb"`, []string{"SET", "k", "a\nb"}, false},
		{"escape quote", `SET k "a\"b"`, []string{"SET", "k", `a"b`}, false},
		{"hex escape", `SET k "\x41"`, []string{"SET", "k", "A"}, false},
		{"single quote literal backslash", `SET k 'a\nb'`, []string{"SET", "k", `a\nb`}, false},
		{"unterminated double", `SET k "oops`, nil, true},
		{"unterminated single", `SET k 'oops`, nil, true},
		{"empty line", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("tokenize(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tokenize(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

// fakeSender scripts replies for the REPL, recording what was sent.
type fakeSender struct {
	replies []resp.Value
	idx     int
	sent    [][]string
}

func (f *fakeSender) Addr() string { return "fake:0" }

func (f *fakeSender) Send(args []string) (resp.Value, error) {
	f.sent = append(f.sent, args)
	v := f.replies[f.idx]
	f.idx++
	return v, nil
}

func TestRunPipeStopsOnError(t *testing.T) {
	fs := &fakeSender{replies: []resp.Value{
		{Type: resp.SimpleString, Str: "OK"},
		{Type: resp.Error, Str: "ERR boom"},
		{Type: resp.SimpleString, Str: "SHOULD-NOT-RUN"},
	}}
	in := strings.NewReader("SET a 1\nBADCMD\nSET b 2\n")
	var out strings.Builder

	code := Run(in, &out, fs, false)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if len(fs.sent) != 2 {
		t.Errorf("sent %d commands, want 2 (must stop after error): %v", len(fs.sent), fs.sent)
	}
	wantOut := "OK\n(error) ERR boom\n"
	if out.String() != wantOut {
		t.Errorf("output = %q, want %q", out.String(), wantOut)
	}
}

func TestRunPipeSuccess(t *testing.T) {
	fs := &fakeSender{replies: []resp.Value{
		{Type: resp.SimpleString, Str: "OK"},
		{Type: resp.BulkString, Str: "1"},
	}}
	in := strings.NewReader("SET a 1\r\nGET a\r\n") // CRLF tolerated
	var out strings.Builder

	if code := Run(in, &out, fs, false); code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got, want := out.String(), "OK\n\"1\"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunLocalCommandsNotSent(t *testing.T) {
	fs := &fakeSender{replies: []resp.Value{{Type: resp.SimpleString, Str: "PONG"}}}
	in := strings.NewReader("help\nhelp GET\nPING\nquit\nPING\n")
	var out strings.Builder

	Run(in, &out, fs, false)

	if len(fs.sent) != 1 {
		t.Fatalf("sent %d commands, want 1 (only PING; help/quit are local): %v", len(fs.sent), fs.sent)
	}
	if !reflect.DeepEqual(fs.sent[0], []string{"PING"}) {
		t.Errorf("sent[0] = %v, want [PING]", fs.sent[0])
	}
	if !strings.Contains(out.String(), "Get the value of a key") {
		t.Errorf("help GET output missing summary; got:\n%s", out.String())
	}
}

func TestRunInteractivePrompt(t *testing.T) {
	fs := &fakeSender{replies: []resp.Value{{Type: resp.SimpleString, Str: "PONG"}}}
	in := strings.NewReader("PING\n")
	var out strings.Builder

	Run(in, &out, fs, true)

	if !strings.HasPrefix(out.String(), "fake:0> ") {
		t.Errorf("interactive output should start with prompt, got %q", out.String())
	}
}

func TestRunInteractiveContinuesOnError(t *testing.T) {
	fs := &fakeSender{replies: []resp.Value{
		{Type: resp.Error, Str: "ERR boom"},
		{Type: resp.SimpleString, Str: "PONG"},
	}}
	in := strings.NewReader("BADCMD\nPING\n")
	var out strings.Builder

	if code := Run(in, &out, fs, true); code != 0 {
		t.Errorf("interactive exit code = %d, want 0", code)
	}
	if len(fs.sent) != 2 {
		t.Errorf("sent %d, want 2 (interactive should keep going after error)", len(fs.sent))
	}
}
