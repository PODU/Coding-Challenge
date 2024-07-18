// Package render turns Markdown into HTML that is safe to put in a page.
// gomarkdown does the rendering and bluemonday then strips scripts, event
// handlers and dangerous URLs.
package render

import (
	"sync"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
)

// Renderer is safe for concurrent use.
type Renderer struct {
	policy *bluemonday.Policy
	mu     sync.Mutex // gomarkdown parser/renderer are not concurrency-safe
}

// New starts from bluemonday's UGC policy and also allows GFM tables, task
// lists and code block classes.
func New() *Renderer {
	p := bluemonday.UGCPolicy()
	p.AllowElements("table", "thead", "tbody", "tr", "th", "td")
	p.AllowAttrs("align").OnElements("th", "td")
	// language-* classes, for client-side highlighters.
	p.AllowAttrs("class").OnElements("code", "pre", "span")
	// Task-list checkboxes (rendered disabled).
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	return &Renderer{policy: p}
}

// Render returns md as sanitized HTML.
func (r *Renderer) Render(md string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs |
		parser.Strikethrough | parser.Tables | parser.Footnotes
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(md))

	opts := html.RendererOptions{Flags: html.CommonFlags}
	renderer := html.NewRenderer(opts)
	unsafe := markdown.Render(doc, renderer)

	return string(r.policy.SanitizeBytes(unsafe))
}
