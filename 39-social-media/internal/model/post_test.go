package model

import (
	"errors"
	"testing"
	"time"
)

func TestPostValidate(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	base := func() Post {
		return Post{Author: "acme", Body: "hello", Target: "mock", Visibility: VisibilityPublic}
	}

	tests := []struct {
		name    string
		mutate  func(*Post)
		wantErr bool
	}{
		{"valid immediate", func(*Post) {}, false},
		{"valid scheduled future", func(p *Post) { p.ScheduledAt = &future }, false},
		{"missing author", func(p *Post) { p.Author = "  " }, true},
		{"missing body", func(p *Post) { p.Body = "" }, true},
		{"missing target", func(p *Post) { p.Target = "" }, true},
		{"bad visibility", func(p *Post) { p.Visibility = "loud" }, true},
		{"scheduled in past", func(p *Post) { p.ScheduledAt = &past }, true},
		{"scheduled exactly now", func(p *Post) { p.ScheduledAt = &now }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := base()
			tc.mutate(&p)
			err := p.Validate(now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("error not wrapped with ErrValidation: %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestStatusEditable(t *testing.T) {
	editable := []Status{StatusDraft, StatusScheduled}
	notEditable := []Status{StatusQueued, StatusPublishing, StatusPublished, StatusFailed, StatusCanceled}
	for _, s := range editable {
		if !s.Editable() {
			t.Errorf("status %q should be editable", s)
		}
	}
	for _, s := range notEditable {
		if s.Editable() {
			t.Errorf("status %q should not be editable", s)
		}
	}
}
