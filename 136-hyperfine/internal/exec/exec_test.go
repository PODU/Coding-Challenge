package exec

import (
	"bytes"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"ls", []string{"ls"}},
		{"  ls   -l  ", []string{"ls", "-l"}},
		{`grep "two words" file`, []string{"grep", "two words", "file"}},
		{`grep 'two words' file`, []string{"grep", "two words", "file"}},
		{`echo ""`, []string{"echo", ""}},
		{`a"b"c`, []string{"abc"}},
	}
	for _, c := range cases {
		got, err := Split(c.in)
		if err != nil {
			t.Errorf("Split(%q) failed: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Split(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestSplitUnterminatedQuote(t *testing.T) {
	if _, err := Split(`echo "oops`); err == nil {
		t.Error("expected an error for an unterminated quote")
	}
}

func TestSplitBackslashIsPlatformDependent(t *testing.T) {
	got, err := Split(`C:\tools\x y`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtime.GOOS == "windows" {
		// Backslash is a path separator here, not an escape.
		want := []string{`C:\tools\x`, "y"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v, want %#v", got, want)
		}
	} else if len(got) != 2 {
		t.Errorf("got %#v, want two words", got)
	}
}

func TestParseShell(t *testing.T) {
	s, err := ParseShell("none")
	if err != nil {
		t.Fatal(err)
	}
	if s.Enabled() {
		t.Error(`ParseShell("none") should be disabled`)
	}

	s, err = ParseShell("zsh -f")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"zsh", "-f", "-c"}; !reflect.DeepEqual(s.Argv, want) {
		t.Errorf("argv = %v, want %v", s.Argv, want)
	}

	s, err = ParseShell("cmd.exe")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"cmd.exe", "/s", "/c"}; !reflect.DeepEqual(s.Argv, want) {
		t.Errorf("argv = %v, want %v", s.Argv, want)
	}

	if _, err := ParseShell(""); err == nil {
		t.Error("expected an error for an empty shell specification")
	}
}

func TestDefaultShellMatchesPlatform(t *testing.T) {
	s := DefaultShell()
	if !s.Enabled() {
		t.Fatal("default shell should be enabled")
	}
	if runtime.GOOS == "windows" && !strings.Contains(s.Argv[0], "cmd") {
		t.Errorf("windows default shell = %v", s.Argv)
	}
}

func TestRunCapturesTimesAndExitCode(t *testing.T) {
	var out bytes.Buffer
	tm, err := Run(DefaultShell(), "echo hi", nil, Streams{Stdout: &out})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if tm.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", tm.ExitCode)
	}
	if tm.Real <= 0 {
		t.Errorf("real time = %v, want a positive duration", tm.Real)
	}
	if !strings.Contains(out.String(), "hi") {
		t.Errorf("stdout = %q, want it to contain \"hi\"", out.String())
	}
}

func TestRunReportsNonZeroExitWithoutError(t *testing.T) {
	tm, err := Run(DefaultShell(), "exit 3", nil, Discard)
	if err != nil {
		t.Fatalf("a non-zero exit should be a measurement, not an error: %v", err)
	}
	if tm.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", tm.ExitCode)
	}
}

func TestRunWithoutShellSplitsTheCommand(t *testing.T) {
	cmd := "echo hi"
	if runtime.GOOS == "windows" {
		cmd = "cmd.exe /c exit 0" // Windows has no standalone echo binary
	}
	if _, err := Run(NoShell, cmd, nil, Discard); err != nil {
		t.Fatalf("Run without a shell failed: %v", err)
	}
}

func TestRunMissingBinaryIsAnError(t *testing.T) {
	if _, err := Run(NoShell, "definitely-not-a-real-binary-xyz", nil, Discard); err == nil {
		t.Error("expected an error when the binary cannot be started")
	}
}
