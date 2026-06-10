package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteStoreRoundtrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sessions.db")
	store, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	snap := map[string]any{
		"version": 1,
		"cookies": []any{map[string]any{"name": "c_user", "value": "42", "path": "/"}},
	}
	if err := store.Save(ctx, "shop-a", snap); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(ctx, "shop-a")
	if err != nil {
		t.Fatal(err)
	}
	if loaded["version"] != float64(1) {
		t.Fatalf("unexpected snapshot: %+v", loaded)
	}
	ids, err := store.List(ctx)
	if err != nil || len(ids) != 1 || ids[0] != "shop-a" {
		t.Fatalf("list: %v %v", ids, err)
	}
	if err := store.Delete(ctx, "shop-a"); err != nil {
		t.Fatal(err)
	}
	missing, err := store.Load(ctx, "shop-a")
	if err != nil || missing != nil {
		t.Fatalf("expected nil after delete, got %v err=%v", missing, err)
	}
}