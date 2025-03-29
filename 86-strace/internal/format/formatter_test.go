package format

import "testing"

func TestLine(t *testing.T) {
	got := Line("write", []string{"1", `"hi\n"`, "3"}, "3")
	want := `write(1, "hi\n", 3) = 3`
	if got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
}

func TestLineNoArgs(t *testing.T) {
	got := Line("getpid", nil, "4242")
	want := "getpid() = 4242"
	if got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
}

func TestLineNoReturn(t *testing.T) {
	got := Line("exit_group", []string{"0"}, "?")
	want := "exit_group(0) = ?"
	if got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
}

func TestLineErrnoReturn(t *testing.T) {
	got := Line("openat", []string{"AT_FDCWD", `"/nope"`, "O_RDONLY"}, "-1 ENOENT (No such file or directory)")
	want := `openat(AT_FDCWD, "/nope", O_RDONLY) = -1 ENOENT (No such file or directory)`
	if got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
}

func TestExitLine(t *testing.T) {
	if got := ExitLine(0); got != "+++ exited with 0 +++" {
		t.Errorf("ExitLine(0) = %q", got)
	}
	if got := ExitLine(2); got != "+++ exited with 2 +++" {
		t.Errorf("ExitLine(2) = %q", got)
	}
}

func TestSignalLine(t *testing.T) {
	if got := SignalLine("SIGSEGV"); got != "--- SIGSEGV ---" {
		t.Errorf("SignalLine = %q", got)
	}
}
