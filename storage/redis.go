package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisStore persists session snapshots in Redis.
type RedisStore struct {
	Client *redis.Client
	Prefix string
}

// NewRedisStore connects to Redis at url with an optional key prefix.
func NewRedisStore(url string, prefix string) (*RedisStore, error) {
	if prefix == "" {
		prefix = "fbchat:session:"
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("storage: parse redis url: %w", err)
	}
	return &RedisStore{
		Client: redis.NewClient(opt),
		Prefix: prefix,
	}, nil
}

func (r *RedisStore) key(clientID string) string {
	return r.Prefix + clientID
}

// Save stores a session snapshot.
func (r *RedisStore) Save(ctx context.Context, clientID string, snapshot map[string]any) error {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return r.Client.Set(ctx, r.key(clientID), b, 0).Err()
}

// Load retrieves a session snapshot.
func (r *RedisStore) Load(ctx context.Context, clientID string) (map[string]any, error) {
	raw, err := r.Client.Get(ctx, r.key(clientID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes a session snapshot.
func (r *RedisStore) Delete(ctx context.Context, clientID string) error {
	return r.Client.Del(ctx, r.key(clientID)).Err()
}

// List returns client ids with stored sessions under the configured prefix.
func (r *RedisStore) List(ctx context.Context) ([]string, error) {
	var ids []string
	iter := r.Client.Scan(ctx, 0, r.Prefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if len(key) > len(r.Prefix) {
			ids = append(ids, key[len(r.Prefix):])
		}
	}
	return ids, iter.Err()
}