package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCommandStorage(t *testing.T) {
	g := Globals{Flags: 7, Exptime: 60}
	cmd, err := ParseCommand([]string{"set", "k", "hello", "world"}, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "set" || cmd.Keys[0] != "k" {
		t.Fatalf("unexpected: %+v", cmd)
	}
	if string(cmd.Data) != "hello world" {
		t.Fatalf("data = %q, want joined args", cmd.Data)
	}
	if cmd.Flags != 7 || cmd.Exptime != 60 {
		t.Fatalf("globals not applied: %+v", cmd)
	}
	sc := cmd.StorageCommand()
	if sc.Command != "set" || sc.Flags != 7 || string(sc.Data) != "hello world" {
		t.Fatalf("StorageCommand wrong: %+v", sc)
	}
}

func TestParseCommandDataFromStdin(t *testing.T) {
	cmd, err := ParseCommand([]string{"set", "k"}, Globals{}, strings.NewReader("piped value"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cmd.Data) != "piped value" {
		t.Fatalf("expected stdin data, got %q", cmd.Data)
	}
}

func TestParseCommandEmptyValue(t *testing.T) {
	cmd, err := ParseCommand([]string{"set", "k"}, Globals{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Data) != 0 {
		t.Fatalf("expected empty value, got %q", cmd.Data)
	}
}

func TestParseCommandGetMulti(t *testing.T) {
	cmd, err := ParseCommand([]string{"get", "a", "b", "c"}, Globals{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmd.Keys) != 3 || cmd.Keys[1] != "b" {
		t.Fatalf("keys = %v", cmd.Keys)
	}
}

func TestParseCommandCAS(t *testing.T) {
	cmd, err := ParseCommand([]string{"cas", "k", "42", "newval"}, Globals{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cmd.HasCAS || cmd.CAS != 42 || string(cmd.Data) != "newval" {
		t.Fatalf("cas parse wrong: %+v", cmd)
	}
	sc := cmd.StorageCommand()
	if !sc.HasCAS || sc.CAS != 42 {
		t.Fatalf("StorageCommand cas wrong: %+v", sc)
	}
}

func TestParseCommandIncr(t *testing.T) {
	cmd, err := ParseCommand([]string{"incr", "k", "5"}, Globals{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Delta != 5 {
		t.Fatalf("delta = %d", cmd.Delta)
	}
}

func TestParseCommandErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no command", []string{}},
		{"unknown command", []string{"frobnicate", "k"}},
		{"set without key", []string{"set"}},
		{"get without key", []string{"get"}},
		{"delete extra args", []string{"delete", "a", "b"}},
		{"incr missing delta", []string{"incr", "k"}},
		{"incr bad delta", []string{"incr", "k", "notanumber"}},
		{"cas missing id", []string{"cas", "k"}},
		{"cas bad id", []string{"cas", "k", "xx", "v"}},
		{"empty key", []string{"get", ""}},
		{"key with space handled by shell, control char", []string{"get", "a\tb"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCommand(tc.args, Globals{}, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			var ue *UsageError
			if !errors.As(err, &ue) {
				t.Fatalf("expected UsageError, got %T: %v", err, err)
			}
		})
	}
}

func TestValidateKeyTooLong(t *testing.T) {
	long := strings.Repeat("a", 251)
	if err := validateKey(long); err == nil {
		t.Fatal("expected error for >250-byte key")
	}
	if err := validateKey(strings.Repeat("a", 250)); err != nil {
		t.Fatalf("250-byte key should be valid: %v", err)
	}
}
