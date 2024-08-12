package protocol

import (
	"testing"
)

func TestEncodeStorage(t *testing.T) {
	tests := []struct {
		name string
		cmd  StorageCommand
		want string
	}{
		{
			name: "set ascii",
			cmd:  StorageCommand{Command: "set", Key: "testkey", Flags: 0, Exptime: 0, Data: []byte("testvalue")},
			want: "set testkey 0 0 9\r\ntestvalue\r\n",
		},
		{
			name: "set with flags and exptime",
			cmd:  StorageCommand{Command: "set", Key: "k", Flags: 42, Exptime: 3600, Data: []byte("hi")},
			want: "set k 42 3600 2\r\nhi\r\n",
		},
		{
			name: "empty value",
			cmd:  StorageCommand{Command: "set", Key: "empty", Data: []byte("")},
			want: "set empty 0 0 0\r\n\r\n",
		},
		{
			name: "binary value with embedded CRLF",
			cmd:  StorageCommand{Command: "set", Key: "bin", Data: []byte("a\r\nb")},
			want: "set bin 0 0 4\r\na\r\nb\r\n",
		},
		{
			name: "add",
			cmd:  StorageCommand{Command: "add", Key: "k", Data: []byte("v")},
			want: "add k 0 0 1\r\nv\r\n",
		},
		{
			name: "noreply",
			cmd:  StorageCommand{Command: "set", Key: "k", Data: []byte("v"), NoReply: true},
			want: "set k 0 0 1 noreply\r\nv\r\n",
		},
		{
			name: "cas",
			cmd:  StorageCommand{Command: "cas", Key: "k", Data: []byte("v"), CAS: 12345, HasCAS: true},
			want: "cas k 0 0 1 12345\r\nv\r\n",
		},
		{
			name: "cas with noreply",
			cmd:  StorageCommand{Command: "cas", Key: "k", Data: []byte("vv"), CAS: 7, HasCAS: true, NoReply: true},
			want: "cas k 0 0 2 7 noreply\r\nvv\r\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(EncodeStorage(tc.cmd))
			if got != tc.want {
				t.Errorf("EncodeStorage()\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestEncodeRetrieval(t *testing.T) {
	tests := []struct {
		name    string
		command string
		keys    []string
		want    string
	}{
		{"single", "get", []string{"foo"}, "get foo\r\n"},
		{"multi", "get", []string{"a", "b", "c"}, "get a b c\r\n"},
		{"gets", "gets", []string{"foo", "bar"}, "gets foo bar\r\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(EncodeRetrieval(tc.command, tc.keys))
			if got != tc.want {
				t.Errorf("EncodeRetrieval()\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestEncodeDelete(t *testing.T) {
	if got := string(EncodeDelete("foo", false)); got != "delete foo\r\n" {
		t.Errorf("EncodeDelete = %q", got)
	}
	if got := string(EncodeDelete("foo", true)); got != "delete foo noreply\r\n" {
		t.Errorf("EncodeDelete noreply = %q", got)
	}
}

func TestEncodeIncrDecr(t *testing.T) {
	if got := string(EncodeIncrDecr("incr", "k", 5, false)); got != "incr k 5\r\n" {
		t.Errorf("EncodeIncrDecr incr = %q", got)
	}
	if got := string(EncodeIncrDecr("decr", "k", 2, true)); got != "decr k 2 noreply\r\n" {
		t.Errorf("EncodeIncrDecr decr noreply = %q", got)
	}
}

func TestEncodeStats(t *testing.T) {
	if got := string(EncodeStats("")); got != "stats\r\n" {
		t.Errorf("EncodeStats() = %q", got)
	}
	if got := string(EncodeStats("items")); got != "stats items\r\n" {
		t.Errorf("EncodeStats(items) = %q", got)
	}
}
