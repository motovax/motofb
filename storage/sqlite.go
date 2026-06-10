package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteSchema stores per-account cookie snapshots (JSON with version + cookies array).
const sqliteSchema = `
CREATE TABLE IF NOT EXISTS sessions (
	client_id TEXT PRIMARY KEY,
	snapshot  TEXT NOT NULL,
	updated_at INTEGER NOT NULL
);
`

// SQLiteStore persists session snapshots in a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLite opens (or creates) a SQLite database for session storage.
func OpenSQLite(path string) (*SQLiteStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("storage: create db directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(sqliteSchema)
	return err
}

// Close closes the database handle.
func (s *SQLiteStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Save upserts a session snapshot for clientID.
func (s *SQLiteStore) Save(ctx context.Context, clientID string, snapshot map[string]any) error {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions (client_id, snapshot, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(client_id) DO UPDATE SET
			snapshot = excluded.snapshot,
			updated_at = excluded.updated_at
	`, clientID, string(b), now)
	return err
}

// Load returns the stored snapshot or nil when not found.
func (s *SQLiteStore) Load(ctx context.Context, clientID string) (map[string]any, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT snapshot FROM sessions WHERE client_id = ?`, clientID).Scan(&raw)
	if err == sql.ErrNoRows {
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

// Delete removes a stored session.
func (s *SQLiteStore) Delete(ctx context.Context, clientID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE client_id = ?`, clientID)
	return err
}

// List returns all client ids with stored sessions.
func (s *SQLiteStore) List(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT client_id FROM sessions ORDER BY client_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}