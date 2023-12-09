package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"socialmedia/internal/model"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// timeFormat is how times are stored: RFC 3339 with nanoseconds, in UTC.
const timeFormat = "2006-01-02T15:04:05.999999999Z07:00"

const columns = `id, author, body, visibility, target, status,
	scheduled_at, published_at, external_id, error, attempts, created_at, updated_at`

type SQLite struct {
	db *sql.DB
}

// Open opens or creates the database at dsn and applies the schema.
//
// Only one connection is allowed. That avoids "database is locked" errors
// between writers, and with ":memory:" (used by the tests) it means every
// query sees the same database instead of a fresh empty one.
func Open(ctx context.Context, dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
	}
	s := &SQLite{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Schema is applied on startup. Keep it in sync with migrations/0001_init.sql.
const Schema = `
CREATE TABLE IF NOT EXISTS posts (
	id           TEXT PRIMARY KEY,
	author       TEXT NOT NULL,
	body         TEXT NOT NULL,
	visibility   TEXT NOT NULL,
	target       TEXT NOT NULL,
	status       TEXT NOT NULL,
	scheduled_at TEXT,
	published_at TEXT,
	external_id  TEXT NOT NULL DEFAULT '',
	error        TEXT NOT NULL DEFAULT '',
	attempts     INTEGER NOT NULL DEFAULT 0,
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_posts_due ON posts(status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_posts_target_external ON posts(target, external_id);
`

func (s *SQLite) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, Schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

func (s *SQLite) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *SQLite) Close() error { return s.db.Close() }

func timeToDB(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(timeFormat)
}

func dbToTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := time.Parse(timeFormat, ns.String)
	if err != nil {
		return nil, fmt.Errorf("parse time %q: %w", ns.String, err)
	}
	t = t.UTC()
	return &t, nil
}

func (s *SQLite) Create(ctx context.Context, p *model.Post) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO posts (`+columns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Author, p.Body, string(p.Visibility), p.Target, string(p.Status),
		timeToDB(p.ScheduledAt), timeToDB(p.PublishedAt), p.ExternalID, p.Error,
		p.Attempts, p.CreatedAt.UTC().Format(timeFormat), p.UpdatedAt.UTC().Format(timeFormat),
	)
	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanPost(sc scanner) (*model.Post, error) {
	var (
		p           model.Post
		visibility  string
		status      string
		scheduledAt sql.NullString
		publishedAt sql.NullString
		createdAt   string
		updatedAt   string
	)
	if err := sc.Scan(
		&p.ID, &p.Author, &p.Body, &visibility, &p.Target, &status,
		&scheduledAt, &publishedAt, &p.ExternalID, &p.Error, &p.Attempts,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	p.Visibility = model.Visibility(visibility)
	p.Status = model.Status(status)

	var err error
	if p.ScheduledAt, err = dbToTime(scheduledAt); err != nil {
		return nil, err
	}
	if p.PublishedAt, err = dbToTime(publishedAt); err != nil {
		return nil, err
	}
	if p.CreatedAt, err = time.Parse(timeFormat, createdAt); err != nil {
		return nil, err
	}
	if p.UpdatedAt, err = time.Parse(timeFormat, updatedAt); err != nil {
		return nil, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return &p, nil
}

func (s *SQLite) Get(ctx context.Context, id string) (*model.Post, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+columns+` FROM posts WHERE id = ?`, id)
	p, err := scanPost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get post: %w", err)
	}
	return p, nil
}

func (s *SQLite) List(ctx context.Context, f ListFilter) ([]*model.Post, int, error) {
	var where []string
	var args []any
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Target != "" {
		where = append(where, "target = ?")
		args = append(args, f.Target)
	}
	if f.Author != "" {
		where = append(where, "author = ?")
		args = append(args, f.Author)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts`+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT ` + columns + ` FROM posts` + clause + ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, append(args, limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	var out []*model.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// inTx runs fn inside a transaction, rolling back on error.
func (s *SQLite) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func getForUpdate(ctx context.Context, tx *sql.Tx, id string) (*model.Post, error) {
	row := tx.QueryRowContext(ctx, `SELECT `+columns+` FROM posts WHERE id = ?`, id)
	p, err := scanPost(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func savePost(ctx context.Context, tx *sql.Tx, p *model.Post) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE posts SET author=?, body=?, visibility=?, target=?, status=?,
			scheduled_at=?, published_at=?, external_id=?, error=?, attempts=?, updated_at=?
		 WHERE id=?`,
		p.Author, p.Body, string(p.Visibility), p.Target, string(p.Status),
		timeToDB(p.ScheduledAt), timeToDB(p.PublishedAt), p.ExternalID, p.Error,
		p.Attempts, p.UpdatedAt.UTC().Format(timeFormat), p.ID,
	)
	return err
}

func (s *SQLite) ClaimDue(ctx context.Context, now time.Time, limit int) ([]*model.Post, error) {
	if limit <= 0 {
		limit = 100
	}
	now = now.UTC()
	var claimed []*model.Post
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT `+columns+` FROM posts
			 WHERE status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?
			 ORDER BY scheduled_at ASC LIMIT ?`,
			string(model.StatusScheduled), now.Format(timeFormat), limit)
		if err != nil {
			return err
		}
		var due []*model.Post
		for rows.Next() {
			p, err := scanPost(rows)
			if err != nil {
				rows.Close()
				return err
			}
			due = append(due, p)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		for _, p := range due {
			res, err := tx.ExecContext(ctx,
				`UPDATE posts SET status=?, updated_at=? WHERE id=? AND status=?`,
				string(model.StatusQueued), now.Format(timeFormat), p.ID, string(model.StatusScheduled))
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n == 1 {
				p.Status = model.StatusQueued
				p.UpdatedAt = now
				claimed = append(claimed, p)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim due: %w", err)
	}
	return claimed, nil
}

// transition loads a post, checks fromOK, applies mutate, and saves it.
func (s *SQLite) transition(ctx context.Context, id string, fromOK func(model.Status) bool, notOKErr error, mutate func(*model.Post)) (*model.Post, error) {
	var out *model.Post
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		p, err := getForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !fromOK(p.Status) {
			return notOKErr
		}
		mutate(p)
		if err := savePost(ctx, tx, p); err != nil {
			return err
		}
		out = p
		return nil
	})
	return out, err
}

func is(s model.Status) func(model.Status) bool {
	return func(got model.Status) bool { return got == s }
}

func (s *SQLite) PublishNow(ctx context.Context, id string, now time.Time) (*model.Post, error) {
	now = now.UTC()
	return s.transition(ctx, id,
		func(st model.Status) bool { return st.Editable() }, ErrNotEditable,
		func(p *model.Post) {
			p.Status = model.StatusQueued
			p.ScheduledAt = nil
			p.UpdatedAt = now
		})
}

func (s *SQLite) BeginPublish(ctx context.Context, id string, now time.Time) (*model.Post, error) {
	now = now.UTC()
	return s.transition(ctx, id, is(model.StatusQueued), ErrNotClaimable,
		func(p *model.Post) {
			p.Status = model.StatusPublishing
			p.Attempts++
			p.UpdatedAt = now
		})
}

func (s *SQLite) FinishPublish(ctx context.Context, id, externalID string, now time.Time) (*model.Post, error) {
	now = now.UTC()
	return s.transition(ctx, id, is(model.StatusPublishing), ErrNotClaimable,
		func(p *model.Post) {
			p.Status = model.StatusPublished
			p.ExternalID = externalID
			p.PublishedAt = &now
			p.Error = ""
			p.UpdatedAt = now
		})
}

func (s *SQLite) RetryPublish(ctx context.Context, id, errMsg string, now time.Time) (*model.Post, error) {
	now = now.UTC()
	return s.transition(ctx, id, is(model.StatusPublishing), ErrNotClaimable,
		func(p *model.Post) {
			p.Status = model.StatusQueued
			p.Error = errMsg
			p.UpdatedAt = now
		})
}

func (s *SQLite) FailPublish(ctx context.Context, id, errMsg string, now time.Time) (*model.Post, error) {
	now = now.UTC()
	return s.transition(ctx, id, is(model.StatusPublishing), ErrNotClaimable,
		func(p *model.Post) {
			p.Status = model.StatusFailed
			p.Error = errMsg
			p.UpdatedAt = now
		})
}

func (s *SQLite) Cancel(ctx context.Context, id string, now time.Time) (*model.Post, error) {
	now = now.UTC()
	return s.transition(ctx, id,
		func(st model.Status) bool { return st.Editable() }, ErrNotEditable,
		func(p *model.Post) {
			p.Status = model.StatusCanceled
			p.UpdatedAt = now
		})
}

func (s *SQLite) Edit(ctx context.Context, id string, f EditFields, now time.Time) (*model.Post, error) {
	now = now.UTC()
	return s.transition(ctx, id,
		func(st model.Status) bool { return st.Editable() }, ErrNotEditable,
		func(p *model.Post) {
			if f.Body != nil {
				p.Body = *f.Body
			}
			if f.Visibility != nil {
				p.Visibility = *f.Visibility
			}
			if f.ScheduledAt != nil {
				t := f.ScheduledAt.UTC()
				p.ScheduledAt = &t
			}
			// Re-derive status from the (possibly new) schedule.
			if p.ScheduledAt == nil {
				p.Status = model.StatusDraft
			} else {
				p.Status = model.StatusScheduled
			}
			p.UpdatedAt = now
		})
}
