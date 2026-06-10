package state

import (
	"encoding/json"

	fberr "github.com/motovax/motofb/errors"
)

const cookieSnapshotVersion = 1

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