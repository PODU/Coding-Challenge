package store

import (
	"context"

	"blog/internal/blog"
)

// ImportPost inserts a post keeping its id and slug. Only used by Migrate.
func (s *SQLiteStore) ImportPost(ctx context.Context, p *blog.Post) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO posts (id, slug, title, summary, markdown, status, published_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Title, p.Summary, p.MarkdownRaw, string(p.Status),
		nullableTime(p.PublishedAt), p.CreatedAt.UTC().Format(timeLayout), p.UpdatedAt.UTC().Format(timeLayout))
	return err
}

// ImportComment inserts a comment keeping its id.
func (s *SQLiteStore) ImportComment(ctx context.Context, c *blog.Comment) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO comments (id, post_id, name, email, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.PostID, c.Name, c.Email, c.Body, c.CreatedAt.UTC().Format(timeLayout))
	return err
}

// Migrate copies all posts and comments from src to dst, keeping ids and
// slugs.
func Migrate(ctx context.Context, dst *SQLiteStore, src Store) (posts, comments int, err error) {
	all, err := src.List(ctx, ListOpts{Order: OldestFirst})
	if err != nil {
		return 0, 0, err
	}
	for _, p := range all {
		if err := dst.ImportPost(ctx, p); err != nil {
			return posts, comments, err
		}
		posts++
		// ListComments is newest-first; insert oldest-first instead.
		cs, err := src.ListComments(ctx, p.ID)
		if err != nil {
			return posts, comments, err
		}
		for i := len(cs) - 1; i >= 0; i-- {
			if err := dst.ImportComment(ctx, cs[i]); err != nil {
				return posts, comments, err
			}
			comments++
		}
	}
	return posts, comments, nil
}
