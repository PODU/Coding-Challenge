package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"blog/internal/blog"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

const schema = `
CREATE TABLE IF NOT EXISTS posts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    slug         TEXT NOT NULL UNIQUE,
    title        TEXT NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    markdown     TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft',
    published_at TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
CREATE INDEX IF NOT EXISTS idx_posts_published_at ON posts(published_at);

CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id);
`

// Timestamps are stored as UTC text in this layout.
const timeLayout = time.RFC3339Nano

// SQLiteStore uses the pure-Go modernc.org/sqlite driver, so no cgo is needed.
type SQLiteStore struct {
	db     *sql.DB
	render RenderFunc
}

// OpenSQLite opens or creates the database at path (":memory:" works) and
// sets up the schema.
func OpenSQLite(path string, render RenderFunc) (*SQLiteStore, error) {
	if render == nil {
		render = func(s string) string { return s }
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer at a time anyway, and a single connection keeps
	// ":memory:" databases from splitting into several.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteStore{db: db, render: render}, nil
}

func nullableTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(timeLayout), Valid: true}
}

func parseTime(s string) time.Time {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		// Accept plain RFC3339 too, in case a row was written by hand.
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t.UTC()
}

func parseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t := parseTime(ns.String)
	return &t
}

func (s *SQLiteStore) Create(ctx context.Context, p *blog.Post) (int64, error) {
	if p.Slug == "" {
		p.Slug = blog.Slugify(p.Title)
	}
	p.HTMLCached = s.render(p.MarkdownRaw)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// The slug may need the id, so insert with a placeholder first.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO posts (slug, title, summary, markdown, status, published_at, created_at, updated_at)
		 VALUES ('', ?, ?, ?, ?, ?, ?, ?)`,
		p.Title, p.Summary, p.MarkdownRaw, string(p.Status),
		nullableTime(p.PublishedAt), p.CreatedAt.UTC().Format(timeLayout), p.UpdatedAt.UTC().Format(timeLayout))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	p.ID = id

	base := p.Slug
	if base == "" {
		base = fmt.Sprintf("post-%d", id)
	}
	slug, err := uniqueSlugTx(ctx, tx, base, id)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE posts SET slug = ? WHERE id = ?`, slug, id); err != nil {
		return 0, err
	}
	p.Slug = slug

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// uniqueSlugTx returns the first of base, base-2, ... not used by another row.
func uniqueSlugTx(ctx context.Context, tx *sql.Tx, base string, ignoreID int64) (string, error) {
	candidate := base
	for i := 2; ; i++ {
		var owner int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM posts WHERE slug = ?`, candidate).Scan(&owner)
		if err == sql.ErrNoRows || (err == nil && owner == ignoreID) {
			return candidate, nil
		}
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

func (s *SQLiteStore) Update(ctx context.Context, p *blog.Post) error {
	p.HTMLCached = s.render(p.MarkdownRaw)
	// The slug is deliberately not updated, so links keep working.
	res, err := s.db.ExecContext(ctx,
		`UPDATE posts SET title=?, summary=?, markdown=?, status=?, published_at=?, updated_at=?
		 WHERE id=?`,
		p.Title, p.Summary, p.MarkdownRaw, string(p.Status),
		nullableTime(p.PublishedAt), p.UpdatedAt.UTC().Format(timeLayout), p.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	// The caller may have changed p.Slug; put the stored one back.
	return s.db.QueryRowContext(ctx, `SELECT slug FROM posts WHERE id=?`, p.ID).Scan(&p.Slug)
}

const postCols = `id, slug, title, summary, markdown, status, published_at, created_at, updated_at`

func (s *SQLiteStore) scanPost(row interface {
	Scan(dest ...any) error
}) (*blog.Post, error) {
	var (
		p           blog.Post
		status      string
		publishedAt sql.NullString
		createdAt   string
		updatedAt   string
	)
	if err := row.Scan(&p.ID, &p.Slug, &p.Title, &p.Summary, &p.MarkdownRaw,
		&status, &publishedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	p.Status = blog.Status(status)
	p.PublishedAt = parseNullTime(publishedAt)
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	p.HTMLCached = s.render(p.MarkdownRaw)
	return &p, nil
}

func (s *SQLiteStore) Get(ctx context.Context, id int64) (*blog.Post, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+postCols+` FROM posts WHERE id=?`, id)
	p, err := s.scanPost(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return p, err
}

func (s *SQLiteStore) GetBySlug(ctx context.Context, slug string) (*blog.Post, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+postCols+` FROM posts WHERE slug=?`, slug)
	p, err := s.scanPost(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return p, err
}

func (s *SQLiteStore) List(ctx context.Context, opts ListOpts) ([]*blog.Post, error) {
	var (
		where []string
		args  []any
	)
	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(opts.Status))
	}
	if opts.PublishedBefore != nil {
		where = append(where, "published_at IS NOT NULL AND published_at <= ?")
		args = append(args, opts.PublishedBefore.UTC().Format(timeLayout))
	}

	q := `SELECT ` + postCols + ` FROM posts`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	// Drafts have no published_at, so they sort by created_at.
	dir := "DESC"
	if opts.Order == OldestFirst {
		dir = "ASC"
	}
	q += fmt.Sprintf(" ORDER BY COALESCE(published_at, created_at) %s, id %s", dir, dir)
	if opts.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*blog.Post
	for rows.Next() {
		p, err := s.scanPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) AddComment(ctx context.Context, c *blog.Comment) error {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM posts WHERE id=?`, c.PostID).Scan(&exists)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO comments (post_id, name, email, body, created_at) VALUES (?, ?, ?, ?, ?)`,
		c.PostID, c.Name, c.Email, c.Body, c.CreatedAt.UTC().Format(timeLayout))
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) ListComments(ctx context.Context, postID int64) ([]*blog.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, post_id, name, email, body, created_at FROM comments
		 WHERE post_id=? ORDER BY created_at DESC, id DESC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*blog.Comment
	for rows.Next() {
		var (
			c         blog.Comment
			createdAt string
		)
		if err := rows.Scan(&c.ID, &c.PostID, &c.Name, &c.Email, &c.Body, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = parseTime(createdAt)
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Close() error { return s.db.Close() }
