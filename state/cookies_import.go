package state

import (
	"encoding/json"
	"os"

	fberr "github.com/motovax/motofb/errors"
)

const cookieSnapshotVersion = 1

// CookieSnapshotFromFile reads a browser cookie export and builds a storage snapshot.
// The snapshot is stored in SQLite and used for session restore on later runs.
func CookieSnapshotFromFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fberr.Wrap("CookieSnapshotFromFile", "read file", err)
	}
	return CookieSnapshotFromJSON(data)
}

// CookieSnapshotFromJSON validates cookie JSON and returns a storage-ready snapshot.
func CookieSnapshotFromJSON(data []byte) (map[string]any, error) {
	jar, err := LoadCookiesFromJSON(data)
	if err != nil {
		return nil, err
	}
	if _, err := UserIDFromJar(jar); err != nil {
		return nil, fberr.Wrap("CookieSnapshotFromJSON", "validate c_user cookie", err)
	}
	return map[string]any{
		"version": cookieSnapshotVersion,
		"cookies": DumpCookies(jar),
	}, nil
}

// CookieSnapshotFromRecords builds a snapshot from in-memory cookie records.
func CookieSnapshotFromRecords(records []CookieRecord) (map[string]any, error) {
	b, err := json.Marshal(records)
	if err != nil {
		return nil, err
	}
	return CookieSnapshotFromJSON(b)
}