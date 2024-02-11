package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketRecords = []byte("records")
	bucketDedup   = []byte("dedup")
)

// BoltStore keeps records in a bbolt file. Put commits before returning, so a
// token is never handed out for a record that wasn't saved.
type BoltStore struct {
	db *bolt.DB
}

// NewBoltStore opens or creates the database at path.
func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("store: open bolt: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketRecords, bucketDedup} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: init buckets: %w", err)
	}
	return &BoltStore{db: db}, nil
}

func (s *BoltStore) Put(_ context.Context, rec Record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("store: marshal record: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketRecords).Put([]byte(rec.Token), data)
	})
}

func (s *BoltStore) Get(_ context.Context, token string) (Record, bool, error) {
	var (
		rec   Record
		found bool
	)
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketRecords).Get([]byte(token))
		if v == nil {
			return nil
		}
		found = true
		return json.Unmarshal(v, &rec)
	})
	if err != nil {
		return Record{}, false, fmt.Errorf("store: get record: %w", err)
	}
	return rec, found, nil
}

func (s *BoltStore) PutDedup(_ context.Context, hmac, token string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketDedup).Put([]byte(hmac), []byte(token))
	})
}

func (s *BoltStore) GetDedup(_ context.Context, hmac string) (string, bool, error) {
	var (
		token string
		found bool
	)
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketDedup).Get([]byte(hmac))
		if v == nil {
			return nil
		}
		found = true
		token = string(v)
		return nil
	})
	if err != nil {
		return "", false, fmt.Errorf("store: get dedup: %w", err)
	}
	return token, found, nil
}

func (s *BoltStore) Ping(_ context.Context) error {
	return s.db.View(func(tx *bolt.Tx) error {
		if tx.Bucket(bucketRecords) == nil {
			return fmt.Errorf("store: records bucket missing")
		}
		return nil
	})
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}
