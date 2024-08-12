package cli

import (
	"strings"
	"testing"

	"github.com/codingchallenges/ccmc/internal/protocol"
)

func TestFormatGetSingleHit(t *testing.T) {
	vals := []protocol.Value{{Key: "testkey", Data: []byte("testvalue")}}
	out := FormatGet([]string{"testkey"}, vals, false)
	if out.Stdout != "testkey: testvalue" {
		t.Fatalf("got %q", out.Stdout)
	}
	if len(out.Misses) != 0 {
		t.Fatalf("unexpected misses: %v", out.Misses)
	}
}

func TestFormatGetQuiet(t *testing.T) {
	vals := []protocol.Value{{Key: "k", Data: []byte("rawvalue")}}
	out := FormatGet([]string{"k"}, vals, true)
	if out.Stdout != "rawvalue" {
		t.Fatalf("quiet got %q", out.Stdout)
	}
}

func TestFormatGetMiss(t *testing.T) {
	out := FormatGet([]string{"gone"}, nil, false)
	if out.Stdout != "" {
		t.Fatalf("expected empty stdout, got %q", out.Stdout)
	}
	if len(out.Misses) != 1 || out.Misses[0] != "gone" {
		t.Fatalf("misses = %v", out.Misses)
	}
}

func TestFormatGetMultiMixed(t *testing.T) {
	vals := []protocol.Value{
		{Key: "a", Data: []byte("AA")},
		{Key: "c", Data: []byte("CC")},
	}
	out := FormatGet([]string{"a", "b", "c"}, vals, false)
	want := "a: AA\nc: CC"
	if out.Stdout != want {
		t.Fatalf("stdout = %q, want %q", out.Stdout, want)
	}
	if len(out.Misses) != 1 || out.Misses[0] != "b" {
		t.Fatalf("misses = %v", out.Misses)
	}
}

func TestFormatGetWithCAS(t *testing.T) {
	vals := []protocol.Value{{Key: "k", Flags: 3, Data: []byte("v"), CAS: 99, HasCAS: true}}
	out := FormatGet([]string{"k"}, vals, false)
	if !strings.Contains(out.Stdout, "cas=99") || !strings.Contains(out.Stdout, "flags=3") {
		t.Fatalf("expected cas/flags annotation, got %q", out.Stdout)
	}
}

func TestFormatStore(t *testing.T) {
	cases := map[protocol.StoreStatus]string{
		protocol.Stored:    "STORED",
		protocol.NotStored: "NOT_STORED",
		protocol.Exists:    "EXISTS",
		protocol.NotFound:  "NOT_FOUND",
	}
	for status, want := range cases {
		if got := FormatStore(status); got != want {
			t.Errorf("FormatStore(%v) = %q, want %q", status, got, want)
		}
	}
}

func TestFormatDeleteAndNumeric(t *testing.T) {
	if FormatDelete(true) != "DELETED" || FormatDelete(false) != "NOT_FOUND" {
		t.Fatal("FormatDelete wrong")
	}
	if FormatNumeric(15, true) != "15" || FormatNumeric(0, false) != "NOT_FOUND" {
		t.Fatal("FormatNumeric wrong")
	}
}

func TestFormatStatsGolden(t *testing.T) {
	stats := []protocol.Stat{
		{Name: "pid", Value: "1234"},
		{Name: "uptime", Value: "4200"},
		{Name: "curr_items", Value: "7"},
	}
	got := FormatStats(stats)
	// Sorted by name, aligned to widest name ("curr_items" = 10).
	want := "curr_items  7\n" +
		"pid         1234\n" +
		"uptime      4200"
	if got != want {
		t.Fatalf("FormatStats golden mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatStatsEmpty(t *testing.T) {
	if got := FormatStats(nil); got != "(no stats)" {
		t.Fatalf("empty stats = %q", got)
	}
}
