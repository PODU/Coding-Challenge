package blog

import (
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Hello, World", "hello-world"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"Already-Hyphenated", "already-hyphenated"},
		{"Punctuation!@#$%^&*() here", "punctuation-here"},
		{"!@#$%^&*()", ""}, // no alphanumerics -> empty
		{"Café déjà vu", "cafe-deja-vu"},
		{"under_score_words", "under-score-words"},
		{"Mixed 123 Numbers", "mixed-123-numbers"},
		{"--leading-and-trailing--", "leading-and-trailing"},
		{"日本語", ""}, // no ASCII mapping -> empty, caller falls back to id
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPostIsPublic(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name string
		post Post
		want bool
	}{
		{"draft", Post{Status: Draft, PublishedAt: &past}, false},
		{"published past", Post{Status: Published, PublishedAt: &past}, true},
		{"published now", Post{Status: Published, PublishedAt: &now}, true},
		{"published future (scheduled)", Post{Status: Published, PublishedAt: &future}, false},
		{"published no date", Post{Status: Published, PublishedAt: nil}, false},
	}
	for _, c := range cases {
		if got := c.post.IsPublic(now); got != c.want {
			t.Errorf("%s: IsPublic = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPostValidate(t *testing.T) {
	long := make([]rune, MaxTitleLen+1)
	for i := range long {
		long[i] = 'a'
	}
	cases := []struct {
		name    string
		post    Post
		wantErr error
	}{
		{"ok", Post{Title: "Hi", Status: Published}, nil},
		{"empty title", Post{Title: "  ", Status: Draft}, ErrTitleRequired},
		{"long title", Post{Title: string(long), Status: Draft}, ErrTitleTooLong},
		{"bad status", Post{Title: "Hi", Status: "weird"}, ErrBadStatus},
	}
	for _, c := range cases {
		if got := c.post.Validate(); got != c.wantErr {
			t.Errorf("%s: Validate = %v, want %v", c.name, got, c.wantErr)
		}
	}
}

func TestCommentValidate(t *testing.T) {
	cases := []struct {
		name    string
		comment Comment
		wantErr error
	}{
		{"ok", Comment{Name: "Ann", Email: "a@b.com", Body: "Nice"}, nil},
		{"no name", Comment{Email: "a@b.com", Body: "Nice"}, ErrNameRequired},
		{"no email", Comment{Name: "Ann", Body: "Nice"}, ErrEmailRequired},
		{"bad email", Comment{Name: "Ann", Email: "nope", Body: "Nice"}, ErrEmailInvalid},
		{"no body", Comment{Name: "Ann", Email: "a@b.com"}, ErrBodyRequired},
	}
	for _, c := range cases {
		if got := c.comment.Validate(); got != c.wantErr {
			t.Errorf("%s: Validate = %v, want %v", c.name, got, c.wantErr)
		}
	}
}
