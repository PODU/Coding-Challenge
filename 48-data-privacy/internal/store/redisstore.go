package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Store backed by Redis, for deployments that already run it.
type RedisStore struct {
	client *redis.Client
	prefix string
}

// NewRedisStore connects to Redis and pings it. password may be empty.
func NewRedisStore(addr, password string, db int) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("store: redis ping: %w", err)
	}
	return &RedisStore{client: client, prefix: "vault:"}, nil
}

func (s *RedisStore) recKey(token string) string  { return s.prefix + "rec:" + token }
func (s *RedisStore) dedupKey(hmac string) string { return s.prefix + "dedup:" + hmac }

func (s *RedisStore) Put(ctx context.Context, rec Record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("store: marshal record: %w", err)
	}
	if err := s.client.Set(ctx, s.recKey(rec.Token), data, 0).Err(); err != nil {
		return fmt.Errorf("store: redis set: %w", err)
	}
	return nil
}

func (s *RedisStore) Get(ctx context.Context, token string) (Record, bool, error) {
	data, err := s.client.Get(ctx, s.recKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("store: redis get: %w", err)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, false, fmt.Errorf("store: unmarshal record: %w", err)
	}
	return rec, true, nil
}

func (s *RedisStore) PutDedup(ctx context.Context, hmac, token string) error {
	if err := s.client.Set(ctx, s.dedupKey(hmac), token, 0).Err(); err != nil {
		return fmt.Errorf("store: redis set dedup: %w", err)
	}
	return nil
}

func (s *RedisStore) GetDedup(ctx context.Context, hmac string) (string, bool, error) {
	token, err := s.client.Get(ctx, s.dedupKey(hmac)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: redis get dedup: %w", err)
	}
	return token, true, nil
}

func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}
