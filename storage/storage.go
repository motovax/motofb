package storage

import "context"

// Store persists session cookie snapshots per client id.
type Store interface {
	Save(ctx context.Context, clientID string, snapshot map[string]any) error
	Load(ctx context.Context, clientID string) (map[string]any, error)
	Delete(ctx context.Context, clientID string) error
}