package feed

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"blog/internal/blog"
)

func tp(s string) *time.Time { t, _ := time.Parse(time.RFC3339, s); u := t.UTC(); return &u }

func sample() []*blog.Post {
	return []*blog.Post{
		{ID: 2, Title: "Second & Special <tag>", Summary: "Has <b>html</b> & ampersand",
			Status: blog.Published, PublishedAt: tp("2026-06-02T08:00:00Z")},
		{ID: 1, Title: "First Post", Summary: "Intro",
			Status: blog.Published, PublishedAt: tp("2026-06-01T08:00:00Z")},
	}
}

func TestFeedRenderValidXML(t *testing.T) {
	out, err := Render(sample(), Options{Title: "Blog", Description: "desc", BaseURL: "http://localhost:8080"})
	if err != nil {
		t.Fatal(err)
	}
	// Parses as well-formed XML.
	var doc RSS
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, out)
	}
	if doc.Version != "2.0" {
		t.Errorf("version = %q, want 2.0", doc.Version)
	}
	if len(doc.Channel.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(doc.Channel.Items))
	}
}

func TestFeedItemFields(t *testing.T) {
	doc := Build(sample(), Options{Title: "Blog", Description: "desc", BaseURL: "http://localhost:8080"})
	it := doc.Channel.Items[0]
	if it.Title != "Second & Special <tag>" {
		t.Errorf("title decoded = %q", it.Title)
	}
	if it.Link != "http://localhost:8080/posts/2" {
		t.Errorf("link = %q", it.Link)
	}
	if it.GUID.Value != "http://localhost:8080/posts/2" || !it.GUID.IsPermaLink {
		t.Errorf("guid = %+v", it.GUID)
	}
	want := tp("2026-06-02T08:00:00Z").Format(time.RFC1123Z)
	if it.PubDate != want {
		t.Errorf("pubDate = %q, want %q", it.PubDate, want)
	}
}

func TestFeedEscaping(t *testing.T) {
	out, _ := Render(sample(), Options{Title: "Blog", BaseURL: "http://x"})
	s := string(out)
	// Raw special chars must be entity-escaped in the serialized XML.
	if strings.Contains(s, "Second & Special <tag>") {
		t.Errorf("title not escaped in output:\n%s", s)
	}
	if !strings.Contains(s, "Second &amp; Special &lt;tag&gt;") {
		t.Errorf("expected escaped title in output:\n%s", s)
	}
}

func TestFeedEmpty(t *testing.T) {
	out, err := Render(nil, Options{Title: "Empty Blog", Description: "none", BaseURL: "http://x"})
	if err != nil {
		t.Fatal(err)
	}
	var doc RSS
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("empty feed not valid XML: %v", err)
	}
	if len(doc.Channel.Items) != 0 {
		t.Errorf("empty feed has %d items", len(doc.Channel.Items))
	}
	if doc.Channel.Title != "Empty Blog" {
		t.Errorf("channel title = %q", doc.Channel.Title)
	}
}
