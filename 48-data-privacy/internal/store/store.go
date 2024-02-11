// Package store persists vault records. Backends only ever see ciphertext.
package store

import (
	"context"
	"errors"
	"time"
)

// Record is a stored, encrypted value.
type Record struct {
	Token      string    `json:"token"`
	KeyVersion byte      `json:"key_version"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
	Format     string    `json:"format,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	RequestID  string    `json:"request_id,omitempty"`
}

// ErrClosed is returned after Close.
var ErrClosed = errors.New("store: closed")

// Store must be safe for concurrent use.
type Store interface {
	// Put stores rec, overwriting any record with the same token. It must not
	// return until the write is durable.
	Put(ctx context.Context, rec Record) error
	Get(ctx context.Context, token string) (rec Record, ok bool, err error)
	// PutDedup and GetDedup map an HMAC of a plaintext to its token, for
	// deterministic tokenization.
	PutDedup(ctx context.Context, hmac, token string) error
	GetDedup(ctx context.Context, hmac string) (token string, ok bool, err error)
	Ping(ctx context.Context) error
	Close() error
}
