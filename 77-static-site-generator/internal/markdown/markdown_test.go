package markdown

import (
	"strings"
	"testing"
)

func TestTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"present", "# Hello World\n\nbody", "Hello World"},
		{"first wins", "# First\n\n# Second", "First"},
		{"absent", "no heading here\n## sub", ""},
		{"not h1", "## Sub only", ""},
		{"crlf", "# Win\r\n\r\nbody", "Win"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Title(c.in); got != c.want {
				t.Errorf("Title(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRenderBlocks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"h1", "# Hi", "<h1>Hi</h1>\n"},
		{"h3", "### Deep", "<h3>Deep</h3>\n"},
		{"paragraph", "hello world", "<p>hello world</p>\n"},
		{"multiline paragraph", "line one\nline two", "<p>line one line two</p>\n"},
		{"hr", "---", "<hr>\n"},
		{"unordered list", "- a\n- b", "<ul>\n<li>a</li>\n<li>b</li>\n</ul>\n"},
		{"ordered list", "1. a\n2. b", "<ol>\n<li>a</li>\n<li>b</li>\n</ol>\n"},
		{"blockquote", "> quoted text", "<blockquote>quoted text</blockquote>\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Render(c.in); got != c.want {
				t.Errorf("Render(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRenderInline(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		contains string
	}{
		{"bold", "**strong**", "<strong>strong</strong>"},
		{"italic", "*em*", "<em>em</em>"},
		{"code", "use `x = 1`", "<code>x = 1</code>"},
		{"link", "[go](https://go.dev)", `<a href="https://go.dev">go</a>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Render(c.in); !strings.Contains(got, c.contains) {
				t.Errorf("Render(%q) = %q, want it to contain %q", c.in, got, c.contains)
			}
		})
	}
}

func TestEscaping(t *testing.T) {
	got := Render("text with <tag> & ampersand")
	if strings.Contains(got, "<tag>") {
		t.Errorf("raw angle brackets not escaped: %q", got)
	}
	if !strings.Contains(got, "&lt;tag&gt;") || !strings.Contains(got, "&amp;") {
		t.Errorf("expected escaped entities, got %q", got)
	}
}

func TestCodeBlockPassthrough(t *testing.T) {
	in := "```go\nfunc main() {}\n```"
	got := Render(in)
	if !strings.Contains(got, "<pre><code") {
		t.Errorf("expected pre/code, got %q", got)
	}
	if !strings.Contains(got, "func main() {}") {
		t.Errorf("expected code content preserved, got %q", got)
	}
	if !strings.Contains(got, `class="language-go"`) {
		t.Errorf("expected language class, got %q", got)
	}
}

func TestCodeBlockEscapes(t *testing.T) {
	got := Render("```\n<html>\n```")
	if !strings.Contains(got, "&lt;html&gt;") {
		t.Errorf("expected escaped content inside code block, got %q", got)
	}
}

func TestTitleCase(t *testing.T) {
	if got := TitleCase("my-first_post"); got != "My First Post" {
		t.Errorf("TitleCase = %q, want %q", got, "My First Post")
	}
}
