// Package feed builds the RSS 2.0 feed. encoding/xml takes care of escaping.
package feed

import (
	"encoding/xml"
	"strconv"
	"time"

	"blog/internal/blog"
)

// RSS is the feed document.
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

// Channel is the feed metadata and its items.
type Channel struct {
	Title         string `xml:"title"`
	Link          string `xml:"link"`
	Description   string `xml:"description"`
	LastBuildDate string `xml:"lastBuildDate,omitempty"`
	Items         []Item `xml:"item"`
}

// Item is one post in the feed.
type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        GUID   `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// GUID is the post's permalink.
type GUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink bool   `xml:"isPermaLink,attr"`
}

// Options configures the feed.
type Options struct {
	Title       string
	Description string
	// BaseURL has no trailing slash, e.g. "http://localhost:8080".
	BaseURL string
	// BuildTime sets lastBuildDate if non-zero. Tests leave it zero so the
	// output can be compared against a golden file.
	BuildTime time.Time
}

// permalink uses the post id, which unlike the slug never changes.
func permalink(baseURL string, p *blog.Post) string {
	return baseURL + "/posts/" + strconv.FormatInt(p.ID, 10)
}

// Build makes the feed from posts, which must already be filtered and sorted.
func Build(posts []*blog.Post, opts Options) *RSS {
	ch := Channel{
		Title:       opts.Title,
		Link:        opts.BaseURL + "/",
		Description: opts.Description,
	}
	if !opts.BuildTime.IsZero() {
		ch.LastBuildDate = opts.BuildTime.UTC().Format(time.RFC1123Z)
	}
	for _, p := range posts {
		link := permalink(opts.BaseURL, p)
		pub := ""
		if p.PublishedAt != nil {
			pub = p.PublishedAt.UTC().Format(time.RFC1123Z)
		}
		ch.Items = append(ch.Items, Item{
			Title:       p.Title,
			Link:        link,
			GUID:        GUID{Value: link, IsPermaLink: true},
			Description: p.Summary,
			PubDate:     pub,
		})
	}
	return &RSS{Version: "2.0", Channel: ch}
}

// Render returns the feed as XML.
func Render(posts []*blog.Post, opts Options) ([]byte, error) {
	doc := Build(posts, opts)
	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	out := append([]byte(xml.Header), body...)
	out = append(out, '\n')
	return out, nil
}
