package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withMaster runs run(args) with $CCPM_MASTER set, returning the exit code.
func withMaster(t *testing.T, pw string, args ...string) int {
	t.Helper()
	t.Setenv("CCPM_MASTER", pw)
	return run(args)
}

func TestCLIGoldenPath(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "test.ccpm")
	// --timeout 0 disables session caching so every command re-decrypts from
	// disk, proving persistence rather than reusing an in-memory key.
	v := []string{"--vault", vp, "--timeout", "0"}

	// init
	if code := withMaster(t, "hunter2", append([]string{"init", "test"}, v...)...); code != exitOK {
		t.Fatalf("init exit = %d, want 0", code)
	}
	if _, err := os.Stat(vp); err != nil {
		t.Fatalf("vault file not created: %v", err)
	}

	// re-init refuses
	if code := withMaster(t, "hunter2", append([]string{"init", "test"}, v...)...); code != exitUser {
		t.Fatalf("re-init exit = %d, want %d", code, exitUser)
	}

	// add three records, one generated
	if code := withMaster(t, "hunter2", append([]string{"add", "test", "--record", "github", "--username", "alice", "--password", "p1"}, v...)...); code != exitOK {
		t.Fatalf("add github exit = %d", code)
	}
	if code := withMaster(t, "hunter2", append([]string{"add", "test", "--record", "aws", "--username", "bob", "--password", "p2"}, v...)...); code != exitOK {
		t.Fatalf("add aws exit = %d", code)
	}
	if code := withMaster(t, "hunter2", append([]string{"add", "test", "--record", "gen", "--username", "carol", "--gen", "--length", "24"}, v...)...); code != exitOK {
		t.Fatalf("add gen exit = %d", code)
	}

	// duplicate add refused
	if code := withMaster(t, "hunter2", append([]string{"add", "test", "--record", "github"}, v...)...); code != exitUser {
		t.Fatalf("duplicate add exit = %d, want %d", code, exitUser)
	}

	// list
	if code := withMaster(t, "hunter2", append([]string{"list", "test"}, v...)...); code != exitOK {
		t.Fatalf("list exit = %d", code)
	}

	// get (persistence: this is a fresh decrypt from disk)
	if code := withMaster(t, "hunter2", append([]string{"get", "test", "--record", "github", "--show"}, v...)...); code != exitOK {
		t.Fatalf("get exit = %d", code)
	}

	// update then verify
	if code := withMaster(t, "hunter2", append([]string{"update", "test", "--record", "github", "--password", "newpass"}, v...)...); code != exitOK {
		t.Fatalf("update exit = %d", code)
	}

	// rm then get fails
	if code := withMaster(t, "hunter2", append([]string{"rm", "test", "--record", "aws"}, v...)...); code != exitOK {
		t.Fatalf("rm exit = %d", code)
	}
	if code := withMaster(t, "hunter2", append([]string{"get", "test", "--record", "aws"}, v...)...); code != exitUser {
		t.Fatalf("get of deleted record exit = %d, want %d", code, exitUser)
	}
}

func TestCLIWrongPassword(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "test.ccpm")
	v := []string{"--vault", vp, "--timeout", "0"}

	if code := withMaster(t, "correct", append([]string{"init", "test"}, v...)...); code != exitOK {
		t.Fatalf("init exit = %d", code)
	}
	// signin with wrong password -> exit 2
	if code := withMaster(t, "wrong", append([]string{"signin", "test"}, v...)...); code != exitAuth {
		t.Fatalf("wrong-password signin exit = %d, want %d", code, exitAuth)
	}
	// get with wrong password -> exit 2
	if code := withMaster(t, "wrong", append([]string{"get", "test", "--record", "x"}, v...)...); code != exitAuth {
		t.Fatalf("wrong-password get exit = %d, want %d", code, exitAuth)
	}
}

func TestCLITamperDetection(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "test.ccpm")
	v := []string{"--vault", vp, "--timeout", "0"}

	if code := withMaster(t, "pw", append([]string{"init", "test"}, v...)...); code != exitOK {
		t.Fatalf("init exit = %d", code)
	}
	// Flip a byte on disk.
	data, err := os.ReadFile(vp)
	if err != nil {
		t.Fatalf("read vault: %v", err)
	}
	data[len(data)-1] ^= 0x01
	if err := os.WriteFile(vp, data, 0o600); err != nil {
		t.Fatalf("write tampered: %v", err)
	}
	// Opening should fail with auth exit code 2.
	if code := withMaster(t, "pw", append([]string{"list", "test"}, v...)...); code != exitAuth {
		t.Fatalf("tampered open exit = %d, want %d", code, exitAuth)
	}
}

func TestCLISessionReuse(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "test.ccpm")
	// Long timeout: a session should let a later command run without the
	// password env present.
	v := []string{"--vault", vp, "--timeout", "10m"}

	if code := withMaster(t, "pw", append([]string{"init", "test"}, v...)...); code != exitOK {
		t.Fatalf("init exit = %d", code)
	}
	t.Cleanup(func() { run(append([]string{"signout", "test"}, "--vault", vp)) })

	// signin establishes the session.
	if code := withMaster(t, "pw", append([]string{"signin", "test"}, v...)...); code != exitOK {
		t.Fatalf("signin exit = %d", code)
	}
	// Now unset the password; the cached session key should still work.
	os.Unsetenv("CCPM_MASTER")
	if code := run(append([]string{"add", "test", "--record", "r1", "--password", "p"}, v...)); code != exitOK {
		t.Fatalf("add via session exit = %d (session not reused)", code)
	}
	if code := run(append([]string{"list", "test"}, v...)); code != exitOK {
		t.Fatalf("list via session exit = %d", code)
	}

	// signout ends the session; without password and without session, fail.
	if code := run(append([]string{"signout", "test"}, "--vault", vp)); code != exitOK {
		t.Fatalf("signout exit = %d", code)
	}
	if code := run(append([]string{"list", "test"}, v...)); code == exitOK {
		t.Fatal("list succeeded after signout without password; expected failure")
	}
}

func TestCLIGen(t *testing.T) {
	if code := run([]string{"gen", "--length", "32"}); code != exitOK {
		t.Fatalf("gen exit = %d", code)
	}
	if code := run([]string{"gen", "--no-upper", "--no-lower", "--no-digits", "--no-symbols"}); code == exitOK {
		t.Fatal("gen with all classes disabled should fail")
	}
}
