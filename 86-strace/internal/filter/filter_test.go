package filter

import "testing"

func TestParseEmptyTracesEverything(t *testing.T) {
	f, err := Parse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.ShouldTrace("write") || !f.ShouldTrace("anything") {
		t.Error("empty filter should trace everything")
	}
}

func TestParseTraceEquals(t *testing.T) {
	f, err := Parse([]string{"trace=open,read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.ShouldTrace("open") || !f.ShouldTrace("read") {
		t.Error("open/read should be traced")
	}
	if f.ShouldTrace("write") {
		t.Error("write should not be traced")
	}
}

func TestParseBareShorthand(t *testing.T) {
	f, err := Parse([]string{"write,close"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.ShouldTrace("write") || !f.ShouldTrace("close") {
		t.Error("write/close should be traced")
	}
	if f.ShouldTrace("read") {
		t.Error("read should not be traced")
	}
}

func TestParseNegation(t *testing.T) {
	f, err := Parse([]string{"trace=!close"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ShouldTrace("close") {
		t.Error("close should be excluded")
	}
	if !f.ShouldTrace("read") || !f.ShouldTrace("write") {
		t.Error("everything except close should be traced")
	}
}

func TestParseClass(t *testing.T) {
	f, err := Parse([]string{"trace=memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"mmap", "munmap", "mprotect", "brk"} {
		if !f.ShouldTrace(name) {
			t.Errorf("%s should be in memory class", name)
		}
	}
	if f.ShouldTrace("read") {
		t.Error("read is not in memory class")
	}
}

func TestParseUnioned(t *testing.T) {
	f, err := Parse([]string{"trace=open", "read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.ShouldTrace("open") || !f.ShouldTrace("read") {
		t.Error("repeated -e should union")
	}
}

func TestParseUnknownClass(t *testing.T) {
	if _, err := Parse([]string{"trace=bogusclass"}); err == nil {
		t.Error("expected error for unknown class/syscall")
	}
}

func TestParseEmptyExpr(t *testing.T) {
	if _, err := Parse([]string{"trace="}); err == nil {
		t.Error("expected error for empty expression")
	}
}
