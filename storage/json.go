package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// JSONStore saves one file per client id under Directory.
type JSONStore struct {
	Directory string
}

func safeClientID(id string) string {
	var b strings.Builder
	for _, c := range id {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func (s *JSONStore) path(clientID string) string {
	return filepath.Join(s.Directory, safeClientID(clientID)+".json")
}

// Save writes snapshot JSON.
func (s *JSONStore) Save(ctx context.Context, clientID string, snapshot map[string]any) error {
	_ = ctx
	if err := os.MkdirAll(s.Directory, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(clientID), b, 0o600)
}

// Load reads snapshot JSON.
func (s *JSONStore) Load(ctx context.Context, clientID string) (map[string]any, error) {
	_ = ctx
	b, err := os.ReadFile(s.path(clientID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete removes stored session.
func (s *JSONStore) Delete(ctx context.Context, clientID string) error {
	_ = ctx
	err := os.Remove(s.path(clientID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}