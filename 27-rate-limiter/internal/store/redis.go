package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore keeps state in Redis so every instance shares one limit.
//
// Updates use WATCH plus MULTI/EXEC on the JSON-encoded State and retry on
// conflict. That lets the same Go algorithm code run unchanged; a Lua script
// per algorithm would be faster (see README).
type RedisStore struct {
	client     redis.UniversalClient
	prefix     string
	ttl        time.Duration
	maxRetries int
}

// NewRedisStore wraps a go-redis client. prefix namespaces the keys and ttl is
// applied on every write so abandoned keys self-expire.
func NewRedisStore(client redis.UniversalClient, prefix string, ttl time.Duration) *RedisStore {
	if prefix == "" {
		prefix = "rl:"
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &RedisStore{
		client:     client,
		prefix:     prefix,
		ttl:        ttl,
		maxRetries: 50,
	}
}

// Do executes fn atomically against the shared state for key.
func (s *RedisStore) Do(ctx context.Context, key string, fn func(*State) Decision) (Decision, error) {
	rkey := s.prefix + key
	var dec Decision

	txf := func(tx *redis.Tx) error {
		st := State{}
		val, err := tx.Get(ctx, rkey).Result()
		switch {
		case err == redis.Nil:
			// A missing key leaves st zeroed, which the algorithms treat as new.
		case err != nil:
			return err
		default:
			if uerr := json.Unmarshal([]byte(val), &st); uerr != nil {
				return uerr
			}
		}

		dec = fn(&st)

		data, err := json.Marshal(&st)
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
			p.Set(ctx, rkey, data, s.ttl)
			return nil
		})
		return err
	}

	for i := 0; i < s.maxRetries; i++ {
		err := s.client.Watch(ctx, txf, rkey)
		if err == nil {
			return dec, nil
		}
		if err == redis.TxFailedErr {
			continue // another writer got there first; retry
		}
		return Decision{}, err
	}
	return Decision{}, ErrContention
}

// Close closes the underlying client.
func (s *RedisStore) Close() error { return s.client.Close() }
