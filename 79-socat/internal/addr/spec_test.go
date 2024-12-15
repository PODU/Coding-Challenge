package addr

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantKind   Kind
		wantNet    string
		wantParams []string
		wantOpts   map[string]string
		wantErr    bool
	}{
		{
			name: "dash is stdio", input: "-",
			wantKind: KindSTDIO, wantParams: nil, wantOpts: map[string]string{},
		},
		{
			name: "STDIO keyword", input: "STDIO",
			wantKind: KindSTDIO, wantOpts: map[string]string{},
		},
		{
			name: "STDIN!!STDOUT alias", input: "STDIN!!STDOUT",
			wantKind: KindSTDIO, wantOpts: map[string]string{},
		},
		{
			name: "TCP4 connect", input: "TCP4:eu.httpbin.org:80",
			wantKind: KindTCP, wantNet: "tcp4",
			wantParams: []string{"eu.httpbin.org", "80"}, wantOpts: map[string]string{},
		},
		{
			name: "TCP bare maps to tcp", input: "TCP:localhost:80",
			wantKind: KindTCP, wantNet: "tcp",
			wantParams: []string{"localhost", "80"}, wantOpts: map[string]string{},
		},
		{
			name: "TCP6 ipv6 literal", input: "TCP6:[::1]:80",
			wantKind: KindTCP, wantNet: "tcp6",
			wantParams: []string{"::1", "80"}, wantOpts: map[string]string{},
		},
		{
			name: "TCP4-LISTEN with options", input: "TCP4-LISTEN:3333,reuseaddr,fork",
			wantKind: KindTCPListen, wantNet: "tcp4",
			wantParams: []string{"3333"},
			wantOpts:   map[string]string{"reuseaddr": "", "fork": ""},
		},
		{
			name: "UDP4 connect", input: "UDP4:localhost:9999",
			wantKind: KindUDP, wantNet: "udp4",
			wantParams: []string{"localhost", "9999"}, wantOpts: map[string]string{},
		},
		{
			name: "UDP-LISTEN", input: "UDP4-LISTEN:9999",
			wantKind: KindUDPListen, wantNet: "udp4",
			wantParams: []string{"9999"}, wantOpts: map[string]string{},
		},
		{
			name: "FILE create", input: "FILE:test.txt,create",
			wantKind: KindFile, wantParams: []string{"test.txt"},
			wantOpts: map[string]string{"create": ""},
		},
		{
			name: "OPEN alias creat append", input: "OPEN:./test.log,creat,append",
			wantKind: KindFile, wantParams: []string{"./test.log"},
			wantOpts: map[string]string{"creat": "", "append": ""},
		},
		{
			name: "EXEC opaque command preserves spaces", input: "EXEC:'tail -f test.txt'",
			wantKind: KindExec, wantParams: []string{"tail -f test.txt"},
			wantOpts: map[string]string{},
		},
		{
			name: "EXEC without quotes", input: "EXEC:cat",
			wantKind: KindExec, wantParams: []string{"cat"},
			wantOpts: map[string]string{},
		},
		{
			name: "option with value", input: "FILE:x,create,append",
			wantKind: KindFile, wantParams: []string{"x"},
			wantOpts: map[string]string{"create": "", "append": ""},
		},

		// Error cases.
		{name: "empty", input: "", wantErr: true},
		{name: "unknown type", input: "BOGUS:foo", wantErr: true},
		{name: "tcp missing port", input: "TCP4:host", wantErr: true},
		{name: "unknown option", input: "TCP4-LISTEN:3333,frobnicate", wantErr: true},
		{name: "file no path", input: "FILE:", wantErr: true},
		{name: "exec empty", input: "EXEC:", wantErr: true},
		{name: "stdio with param", input: "STDIO:foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %+v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.wantKind)
			}
			if tt.wantNet != "" && got.Net != tt.wantNet {
				t.Errorf("Net = %q, want %q", got.Net, tt.wantNet)
			}
			if !reflect.DeepEqual(got.Params, tt.wantParams) {
				t.Errorf("Params = %#v, want %#v", got.Params, tt.wantParams)
			}
			if !reflect.DeepEqual(got.Options, tt.wantOpts) {
				t.Errorf("Options = %#v, want %#v", got.Options, tt.wantOpts)
			}
		})
	}
}

func TestAccessors(t *testing.T) {
	s, err := Parse("TCP4:example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	if s.Host() != "example.com" || s.Port() != "443" {
		t.Errorf("Host/Port = %q/%q", s.Host(), s.Port())
	}

	l, _ := Parse("TCP4-LISTEN:8080,reuseaddr")
	if l.Port() != "8080" {
		t.Errorf("listen Port = %q, want 8080", l.Port())
	}
	if !l.IsListener() || !l.HasOption("reuseaddr") || l.HasOption("fork") {
		t.Errorf("listener option checks failed: %+v", l)
	}

	f, _ := Parse("FILE:/var/log/x.log,append")
	if f.Path() != "/var/log/x.log" {
		t.Errorf("Path = %q", f.Path())
	}

	e, _ := Parse("EXEC:'tail -f a b'")
	if e.Command() != "tail -f a b" {
		t.Errorf("Command = %q", e.Command())
	}
}
