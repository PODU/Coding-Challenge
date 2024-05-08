package paste

import (
	"context"
	"strings"
	"time"

	"pastebin/internal/clock"
	"pastebin/internal/highlight"
)

// Store is the part of store.Store the service needs, declared here so this
// package doesn't depend on the store package.
type Store interface {
	Create(ctx context.Context, p Paste) error
	Get(ctx context.Context, id string) (Paste, error)
	Delete(ctx context.Context, id string) error
}

// CreateParams is the input to Create.
type CreateParams struct {
	Content   string
	Language  string // raw, will be normalized
	ExpiresIn string // Go duration, "", or "never"
}

// CreateResult is the new paste and its delete key.
type CreateResult struct {
	Paste     Paste
	DeleteKey string
}

// Config sets the service limits.
type Config struct {
	MaxBytes int64         // maximum content length in bytes
	MaxTTL   time.Duration // maximum user-requested TTL; 0 = unlimited
}

// Service holds the paste rules: validation, size and TTL limits, and expiry.
type Service struct {
	store Store
	clk   clock.Clock
	cfg   Config
}

// NewService returns a Service.
func NewService(store Store, clk clock.Clock, cfg Config) *Service {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Service{store: store, clk: clk, cfg: cfg}
}

const maxCreateAttempts = 5

// Create validates and stores a new paste. IDs are random, so on the off
// chance of a collision it retries with a new one.
func (s *Service) Create(ctx context.Context, params CreateParams) (CreateResult, error) {
	if params.Content == "" {
		return CreateResult{}, ErrInvalid
	}
	if s.cfg.MaxBytes > 0 && int64(len(params.Content)) > s.cfg.MaxBytes {
		return CreateResult{}, ErrTooLarge
	}

	lang, ok := highlight.Normalize(params.Language)
	if !ok && strings.TrimSpace(params.Language) != "" {
		return CreateResult{}, ErrInvalid
	}

	expiresAt, err := s.parseExpiry(params.ExpiresIn)
	if err != nil {
		return CreateResult{}, err
	}

	deleteKey, err := newDeleteKey()
	if err != nil {
		return CreateResult{}, err
	}

	now := s.clk.Now().UTC()
	var lastErr error
	for attempt := 0; attempt < maxCreateAttempts; attempt++ {
		id, err := newID()
		if err != nil {
			return CreateResult{}, err
		}
		p := Paste{
			ID:        id,
			Content:   params.Content,
			Language:  lang,
			CreatedAt: now,
			ExpiresAt: expiresAt,
			DeleteKey: deleteKey,
		}
		err = s.store.Create(ctx, p)
		if err == nil {
			return CreateResult{Paste: p, DeleteKey: deleteKey}, nil
		}
		if !isConflict(err) {
			return CreateResult{}, err
		}
		lastErr = err
	}
	return CreateResult{}, lastErr
}

// parseExpiry turns expires_in (a Go duration) into an expiry time. "",
// "never" and "0" mean no expiry.
func (s *Service) parseExpiry(in string) (*time.Time, error) {
	in = strings.TrimSpace(strings.ToLower(in))
	if in == "" || in == "never" || in == "0" {
		return nil, nil
	}
	d, err := time.ParseDuration(in)
	if err != nil || d < 0 {
		return nil, ErrInvalid
	}
	if d == 0 {
		return nil, nil
	}
	if s.cfg.MaxTTL > 0 && d > s.cfg.MaxTTL {
		return nil, ErrInvalid
	}
	t := s.clk.Now().UTC().Add(d)
	return &t, nil
}

// Get returns ErrExpired for an expired paste even if the sweeper hasn't
// removed it yet.
func (s *Service) Get(ctx context.Context, id string) (Paste, error) {
	p, err := s.store.Get(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return Paste{}, ErrNotFound
		}
		return Paste{}, err
	}
	if p.IsExpired(s.clk.Now()) {
		return Paste{}, ErrExpired
	}
	return p, nil
}

// Delete removes a paste if deleteKey matches. Unknown and expired pastes give
// ErrNotFound, a wrong key ErrForbidden.
func (s *Service) Delete(ctx context.Context, id, deleteKey string) error {
	p, err := s.store.Get(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if p.IsExpired(s.clk.Now()) {
		return ErrNotFound
	}
	if deleteKey == "" || deleteKey != p.DeleteKey {
		return ErrForbidden
	}
	if err := s.store.Delete(ctx, id); err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Now returns the current time from the service clock.
func (s *Service) Now() time.Time { return s.clk.Now() }
