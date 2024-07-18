package render

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBasics(t *testing.T) {
	r := New()
	cases := []struct {
		name     string
		md       string
		contains []string
	}{
		{"heading", "# Title", []string{"<h1", "Title</h1>"}},
		{"bold and italic", "This is **bold** and *italic*.", []string{"<strong>bold</strong>", "<em>italic</em>"}},
		{"unordered list", "- one\n- two", []string{"<ul>", "<li>one</li>", "<li>two</li>"}},
		{"ordered list", "1. a\n2. b", []string{"<ol>", "<li>a</li>"}},
		{"code fence", "```\ncode();\n```", []string{"<pre>", "<code>", "code();"}},
		{"inline code", "use `go test`", []string{"<code>go test</code>"}},
		{"link", "[CC](https://example.com)", []string{`href="https://example.com"`, ">CC</a>"}},
		{"blockquote", "> quoted", []string{"<blockquote>", "quoted"}},
		{"table", "| a | b |\n|---|---|\n| 1 | 2 |", []string{"<table>", "<td", "1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := r.Render(c.md)
			for _, want := range c.contains {
				if !strings.Contains(out, want) {
					t.Errorf("Render(%q) = %q\n  missing %q", c.md, out, want)
				}
			}
		})
	}
}

func TestRenderSanitizesMalicious(t *testing.T) {
	r := New()
	cases := []struct {
		name      string
		md        string
		mustNot   []string
		mustNotCI bool
	}{
		{"script tag", "Hello <script>alert('xss')</script> world", []string{"<script", "alert('xss')"}, true},
		{"javascript url", "[click](javascript:alert(1))", []string{"javascript:"}, true},
		{"event handler", `<img src=x onerror="alert(1)">`, []string{"onerror"}, true},
		{"iframe", `<iframe src="evil"></iframe>`, []string{"<iframe"}, true},
		{"inline style attack", `<div style="position:fixed">x</div>`, []string{"position:fixed"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := r.Render(c.md)
			hay := out
			if c.mustNotCI {
				hay = strings.ToLower(out)
			}
			for _, bad := range c.mustNot {
				needle := bad
				if c.mustNotCI {
					needle = strings.ToLower(bad)
				}
				if strings.Contains(hay, needle) {
					t.Errorf("sanitized output still contains %q: %q", bad, out)
				}
			}
		})
	}
}

func TestRenderDeterministic(t *testing.T) {
	r := New()
	md := "# H\n\nSome **text** with a [link](https://x.test)."
	a := r.Render(md)
	b := r.Render(md)
	if a != b {
		t.Errorf("render not deterministic:\n a=%q\n b=%q", a, b)
	}
}
