package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"pastebin/internal/paste"

	_ "modernc.org/sqlite" // pure-Go, CGo-free SQLite driver
)

// SQLiteStore is the default store, using the pure-Go modernc.org/sqlite.
type SQLiteStore struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS pastes (
    id         TEXT PRIMARY KEY,
    content    TEXT NOT NULL,
    language   TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    expires_at INTEGER,
    delete_key TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pastes_expires_at ON pastes(expires_at);
`

// NewSQLiteStore opens or creates the database at dsn (":memory:" works too)
// and sets up the schema.
func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A shared in-memory database disappears when its last connection closes,
	// and concurrent connections see different databases. Pin to one connection
	// to keep ":memory:" stable and to serialize writes (SQLite is single-writer).
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Create implements Store.
func (s *SQLiteStore) Create(ctx context.Context, p paste.Paste) error {
	var expires *int64
	if p.ExpiresAt != nil {
		v := p.ExpiresAt.Unix()
		expires = &v
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pastes (id, content, language, created_at, expires_at, delete_key)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Content, p.Language, p.CreatedAt.Unix(), expires, p.DeleteKey)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("insert paste: %w", err)
	}
	return nil
}

// Get implements Store.
func (s *SQLiteStore) Get(ctx context.Context, id string) (paste.Paste, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, content, language, created_at, expires_at, delete_key
		 FROM pastes WHERE id = ?`, id)
	var (
		p         paste.Paste
		createdAt int64
		expiresAt sql.NullInt64
	)
	if err := row.Scan(&p.ID, &p.Content, &p.Language, &createdAt, &expiresAt, &p.DeleteKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return paste.Paste{}, ErrNotFound
		}
		return paste.Paste{}, fmt.Errorf("scan paste: %w", err)
	}
	p.CreatedAt = time.Unix(createdAt, 0).UTC()
	if expiresAt.Valid {
		t := time.Unix(expiresAt.Int64, 0).UTC()
		p.ExpiresAt = &t
	}
	return p, nil
}

// Delete implements Store.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pastes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete paste: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteExpired implements Store.
func (s *SQLiteStore) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM pastes WHERE expires_at IS NOT NULL AND expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return int(n), nil
}

// Ping implements Store.
func (s *SQLiteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close implements Store.
func (s *SQLiteStore) Close() error { return s.db.Close() }

// isUniqueViolation reports a UNIQUE or PRIMARY KEY conflict. It checks the
// message (and the extended result codes 2067/1555 within it) rather than
// depending on the driver's error type.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, sub := range []string{"UNIQUE constraint failed", "constraint failed: UNIQUE", "(2067)", "(1555)"} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
