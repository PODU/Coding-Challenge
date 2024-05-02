package main

import "testing"

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Config
		wantErr bool
	}{
		{
			name: "tcp client positional",
			args: []string{"example.com", "80"},
			want: Config{Host: "example.com", Port: "80"},
		},
		{
			name: "tcp listen with -p",
			args: []string{"-l", "-p", "8888"},
			want: Config{Listen: true, Port: "8888"},
		},
		{
			name: "grouped flags lvp",
			args: []string{"-lvp", "8888"},
			want: Config{Listen: true, Verbose: true, Port: "8888"},
		},
		{
			name: "udp client",
			args: []string{"-u", "127.0.0.1", "9999"},
			want: Config{UDP: true, Host: "127.0.0.1", Port: "9999"},
		},
		{
			name: "udp server",
			args: []string{"-u", "-l", "-p", "9999"},
			want: Config{UDP: true, Listen: true, Port: "9999"},
		},
		{
			name: "scan range",
			args: []string{"-z", "localhost", "8880-8890"},
			want: Config{Zero: true, Host: "localhost", Port: "8880-8890"},
		},
		{
			name: "exec flag with value",
			args: []string{"-l", "-p", "1234", "-e", "/bin/sh"},
			want: Config{Listen: true, Port: "1234", Exec: "/bin/sh"},
		},
		{
			name: "hexdump flag",
			args: []string{"-x", "host", "22"},
			want: Config{Hex: true, Host: "host", Port: "22"},
		},
		{
			name: "listen positional port",
			args: []string{"-l", "8080"},
			want: Config{Listen: true, Port: "8080"},
		},
		{
			name: "help",
			args: []string{"-h"},
			want: Config{Help: true},
		},
		{name: "missing port", args: []string{"host"}, wantErr: true},
		{name: "listen missing port", args: []string{"-l"}, wantErr: true},
		{name: "bad port", args: []string{"host", "70000"}, wantErr: true},
		{name: "bad range", args: []string{"-z", "host", "90-80"}, wantErr: true},
		{name: "unknown flag", args: []string{"-Q"}, wantErr: true},
		{name: "missing flag value", args: []string{"-p"}, wantErr: true},
		{name: "zero with listen", args: []string{"-l", "-z", "-p", "80"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got cfg %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *got != tt.want {
				t.Fatalf("got %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestParsePortRange(t *testing.T) {
	lo, hi, err := parsePortRange("8880-8890")
	if err != nil || lo != 8880 || hi != 8890 {
		t.Fatalf("range: got (%d,%d,%v)", lo, hi, err)
	}
	lo, hi, err = parsePortRange("443")
	if err != nil || lo != 443 || hi != 443 {
		t.Fatalf("single: got (%d,%d,%v)", lo, hi, err)
	}
	if _, _, err := parsePortRange("0-10"); err == nil {
		t.Fatal("expected error for port 0")
	}
}
