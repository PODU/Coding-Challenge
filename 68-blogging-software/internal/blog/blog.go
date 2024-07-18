// Package blog has the post and comment types, their validation, and slug
// generation.
package blog

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

// Status is a post's publication state.
type Status string

const (
	// Draft posts are visible only in the editor.
	Draft Status = "draft"
	// Published posts become public at PublishedAt.
	Published Status = "published"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	return s == Draft || s == Published
}

// Length limits, mostly to stop abuse.
const (
	MaxTitleLen   = 200
	MaxSummaryLen = 500
	MaxContentLen = 100_000
	MaxNameLen    = 80
	MaxEmailLen   = 254
	MaxBodyLen    = 4000
)

// Post is a blog post. HTMLCached is the sanitized rendering of MarkdownRaw and
// is regenerated on every save.
type Post struct {
	ID          int64      `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	MarkdownRaw string     `json:"markdown"`
	HTMLCached  string     `json:"-"` // derived, not persisted in JSON
	Status      Status     `json:"status"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsPublic reports whether the post is published and its publish time has
// come. A future PublishedAt is how posts are scheduled.
func (p *Post) IsPublic(now time.Time) bool {
	if p.Status != Published {
		return false
	}
	if p.PublishedAt == nil {
		return false
	}
	return !p.PublishedAt.After(now)
}

// Comment is a visitor's comment on a post. Email is stored but never shown.
// Body is plain text.
type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrTitleRequired = errors.New("title is required")
	ErrTitleTooLong  = errors.New("title is too long")
	ErrSummaryLong   = errors.New("summary is too long")
	ErrContentLong   = errors.New("content is too long")
	ErrBadStatus     = errors.New("invalid status")

	ErrNameRequired  = errors.New("name is required")
	ErrEmailRequired = errors.New("email is required")
	ErrEmailInvalid  = errors.New("email is invalid")
	ErrBodyRequired  = errors.New("comment body is required")
	ErrFieldTooLong  = errors.New("field is too long")
)

// Validate returns the first problem found, if any.
func (p *Post) Validate() error {
	t := strings.TrimSpace(p.Title)
	if t == "" {
		return ErrTitleRequired
	}
	if len([]rune(p.Title)) > MaxTitleLen {
		return ErrTitleTooLong
	}
	if len([]rune(p.Summary)) > MaxSummaryLen {
		return ErrSummaryLong
	}
	if len(p.MarkdownRaw) > MaxContentLen {
		return ErrContentLong
	}
	if !p.Status.Valid() {
		return ErrBadStatus
	}
	return nil
}

// Validate returns the first problem found, if any.
func (c *Comment) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return ErrNameRequired
	}
	if len([]rune(c.Name)) > MaxNameLen {
		return ErrFieldTooLong
	}
	if strings.TrimSpace(c.Email) == "" {
		return ErrEmailRequired
	}
	if len(c.Email) > MaxEmailLen {
		return ErrFieldTooLong
	}
	if !validEmail(c.Email) {
		return ErrEmailInvalid
	}
	if strings.TrimSpace(c.Body) == "" {
		return ErrBodyRequired
	}
	if len([]rune(c.Body)) > MaxBodyLen {
		return ErrFieldTooLong
	}
	return nil
}

// validEmail is a loose check: something@something.tld. The address is never
// displayed or mailed, so there's no point being stricter.
func validEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.IndexByte(s[at+1:], '@') >= 0 {
		return false
	}
	domain := s[at+1:]
	dot := strings.IndexByte(domain, '.')
	if dot <= 0 || dot == len(domain)-1 {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// Slugify turns a title into lowercase ASCII words joined by hyphens. Common
// accented letters are folded to ASCII and anything else is dropped. The result
// can be "", in which case callers fall back to the post id.
func Slugify(title string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		default:
			if folded := fold(r); folded != 0 {
				b.WriteRune(folded)
				prevHyphen = false
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// fold maps common accented Latin letters to ASCII, or returns 0.
func fold(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'ö', 'õ':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ñ':
		return 'n'
	case 'ç':
		return 'c'
	}
	return 0
}
