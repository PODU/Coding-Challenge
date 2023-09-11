package diff

import (
	"strings"
	"testing"
)

func TestIdentical(t *testing.T) {
	if out := Unified("a", "b", "x\ny\n", "x\ny\n"); out != "" {
		t.Fatalf("expected empty diff, got:\n%s", out)
	}
}

func TestInsert(t *testing.T) {
	out := Unified("a/f", "b/f", "line1\nline2\n", "line1\ninserted\nline2\n")
	if !strings.Contains(out, "+inserted") {
		t.Fatalf("missing inserted line:\n%s", out)
	}
	if !strings.Contains(out, "@@ -1,2 +1,3 @@") {
		t.Fatalf("wrong hunk header:\n%s", out)
	}
}

func TestDelete(t *testing.T) {
	out := Unified("a/f", "b/f", "a\nb\nc\n", "a\nc\n")
	if !strings.Contains(out, "-b") {
		t.Fatalf("missing deleted line:\n%s", out)
	}
}

func TestReplace(t *testing.T) {
	out := Unified("a/f", "b/f", "one\ntwo\nthree\n", "one\nTWO\nthree\n")
	if !strings.Contains(out, "-two") || !strings.Contains(out, "+TWO") {
		t.Fatalf("replace not represented:\n%s", out)
	}
}

func TestEmptySides(t *testing.T) {
	out := Unified("a/f", "/dev/null", "gone\n", "")
	if !strings.Contains(out, "-gone") {
		t.Fatalf("deletion of whole file:\n%s", out)
	}
	out2 := Unified("/dev/null", "b/f", "", "added\n")
	if !strings.Contains(out2, "+added") {
		t.Fatalf("addition of whole file:\n%s", out2)
	}
}

func TestNoFinalNewline(t *testing.T) {
	out := Unified("a/f", "b/f", "x\n", "x")
	if !strings.Contains(out, "\\ No newline at end of file") {
		t.Fatalf("expected no-newline marker:\n%s", out)
	}
}

func TestHeaderPresent(t *testing.T) {
	out := Unified("a/f", "b/f", "p\n", "q\n")
	if !strings.HasPrefix(out, "--- a/f\n+++ b/f\n") {
		t.Fatalf("missing file headers:\n%s", out)
	}
}
