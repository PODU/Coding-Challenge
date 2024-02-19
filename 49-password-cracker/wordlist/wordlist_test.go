package wordlist

import (
	"context"
	"strings"
	"testing"
)

func collect(t *testing.T, input string) []string {
	t.Helper()
	out, errc := Stream(context.Background(), strings.NewReader(input))
	var got []string
	for w := range out {
		got = append(got, w)
	}
	if err := <-errc; err != nil {
		t.Fatalf("stream error: %v", err)
	}
	return got
}

func TestStreamTrimsAndSkipsBlanks(t *testing.T) {
	got := collect(t, "apple\n\n  password \n\thunter2\t\n\ncorrecthorse\n")
	want := []string{"apple", "password", "hunter2", "correcthorse"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("word %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStreamCRLF(t *testing.T) {
	got := collect(t, "foo\r\nbar\r\n")
	if len(got) != 2 || got[0] != "foo" || got[1] != "bar" {
		t.Fatalf("CRLF not trimmed: %q", got)
	}
}

func TestStreamCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out, _ := Stream(ctx, strings.NewReader("a\nb\nc\nd\n"))
	<-out // consume one
	cancel()
	// Drain; must terminate (no deadlock / goroutine leak).
	for range out {
	}
}
