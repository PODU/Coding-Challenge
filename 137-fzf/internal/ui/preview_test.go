package ui

import (
	"runtime"
	"strings"
	"testing"

	"ccfzf/internal/cli"
	"ccfzf/internal/item"
)

func build(t *testing.T, raw string) *item.Item {
	t.Helper()
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	return b.Build(7, raw)
}

// unquote strips the platform's shell quoting so the tests can assert on what
// the command will actually see.
func unquote(s string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(s, `"`, "")
	}
	return strings.ReplaceAll(s, "'", "")
}

func TestPreviewPlaceholders(t *testing.T) {
	it := build(t, "alpha beta gamma")
	cases := map[string]string{
		"cat {}":        "cat alpha beta gamma",
		"echo {1}":      "echo alpha",
		"echo {2}":      "echo beta",
		"echo {-1}":     "echo gamma",
		"echo {2..3}":   "echo beta gamma",
		"echo {n}":      "echo 7",
		"echo {q}":      "echo my query",
		"echo {} | wc":  "echo alpha beta gamma | wc",
		"echo no specs": "echo no specs",
	}
	for tmpl, want := range cases {
		got := unquote(previewCommand(tmpl, it, nil, "my query", item.DefaultDelimiter()))
		if got != want {
			t.Errorf("%q: got %q, want %q", tmpl, got, want)
		}
	}
}

func TestPreviewPlusUsesTheSelection(t *testing.T) {
	it := build(t, "one")
	got := unquote(previewCommand("cat {+}", it, []string{"a", "b"}, "", item.DefaultDelimiter()))
	if got != "cat a b" {
		t.Errorf("got %q", got)
	}
	// With nothing marked, {+} is the current item.
	got = unquote(previewCommand("cat {+}", it, nil, "", item.DefaultDelimiter()))
	if got != "cat one" {
		t.Errorf("got %q", got)
	}
}

func TestPreviewQuotesArguments(t *testing.T) {
	it := build(t, "a file with spaces")
	got := previewCommand("cat {}", it, nil, "", item.DefaultDelimiter())
	if strings.Contains(got, "cat a file") {
		t.Errorf("the item should be quoted as one argument: %q", got)
	}
}

func TestPreviewWithNoCurrentItem(t *testing.T) {
	if got := previewCommand("cat {}", nil, nil, "", item.DefaultDelimiter()); got != "" {
		t.Errorf("nothing to preview, got %q", got)
	}
	// ... unless the command only depends on the query.
	if got := previewCommand("echo {q}", nil, nil, "q", item.DefaultDelimiter()); got == "" {
		t.Error("a query-only preview should still run")
	}
}

func TestPreviewLeavesUnknownPlaceholdersAlone(t *testing.T) {
	it := build(t, "one")
	got := previewCommand("echo {bogus}", it, nil, "", item.DefaultDelimiter())
	if !strings.Contains(got, "{bogus}") {
		t.Errorf("got %q", got)
	}
}

func TestPreviewPaneNarrowsTheList(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) {
		o.Preview = "echo hello"
		o.PreviewWin = cli.PreviewWindow{Position: cli.PreviewRight, Size: 50, Percent: true}
	}, "alpha")
	defer func() { s.send(key("esc")); <-done }()

	s.waitFor(t, "the preview separator", func(text string) bool {
		return strings.Contains(text, "│")
	})
	line := s.lines()[screenRows-3]
	if !strings.HasPrefix(line, "> alpha") {
		t.Errorf("list row = %q", line)
	}
	if i := strings.Index(line, "│"); i < 0 || i > screenCols/2 {
		t.Errorf("separator should sit at the half-way column: %q", line)
	}
}

func TestPreviewHiddenKeepsTheFullWidth(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) {
		o.Preview = "echo hello"
		o.PreviewWin = cli.PreviewWindow{Position: cli.PreviewRight, Size: 50, Percent: true, Hidden: true}
	}, "alpha")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "> alpha")
	if strings.Contains(s.text(), "│") {
		t.Error("a hidden preview should not draw a separator")
	}
}
