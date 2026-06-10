package motofb

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/motovax/motofb/events"
	"github.com/motovax/motofb/storage"
)

func TestManagerEventRouting(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManagerWithDir(dir, nil)
	var count atomic.Int32
	mgr.On("acc1", events.Message, func(ctx context.Context, args ...any) error {
		count.Add(1)
		return nil
	})
	mgr.routeEvent("acc1", events.Message)
	waitFor(t, func() bool { return count.Load() == 1 })
	mgr.routeEvent("acc2", events.Message)
	time.Sleep(20 * time.Millisecond)
	if count.Load() != 1 {
		t.Fatalf("expected event isolation between client ids")
	}
}

func TestJSONSessionStorageRoundtrip(t *testing.T) {
	dir := t.TempDir()
	store := &storage.JSONStore{Directory: dir}
	ctx := context.Background()
	snap := map[string]any{
		"version": 1,
		"cookies": []any{map[string]any{"name": "c_user", "value": "1", "path": "/"}},
	}
	if err := store.Save(ctx, "c1", snap); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded["version"] != float64(1) {
		t.Fatalf("unexpected snapshot: %+v", loaded)
	}
	path := filepath.Join(dir, "c1.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted json file: %v", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}