package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/motovax/motofb/state"
)

func TestCookieSnapshotFromJSON(t *testing.T) {
	data := []byte(`[
		{"name":"c_user","value":"100001","path":"/"},
		{"name":"xs","value":"secret","path":"/"}
	]`)
	snap, err := state.CookieSnapshotFromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if snap["version"] != 1 {
		t.Fatalf("version = %v", snap["version"])
	}
	cookies, ok := snap["cookies"].([]map[string]any)
	if !ok || len(cookies) == 0 {
		t.Fatalf("expected cookies in snapshot: %+v", snap)
	}
}

func TestCookieSnapshotFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(path, []byte(`[
		{"name":"c_user","value":"42","path":"/"},
		{"name":"xs","value":"tok","path":"/"}
	]`), 0o600); err != nil {
		t.Fatal(err)
	}
	snap, err := state.CookieSnapshotFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap["cookies"]; !ok {
		t.Fatalf("missing cookies: %+v", snap)
	}
}