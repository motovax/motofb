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
	var gotID atomic.Value
	mgr.On("acc1", events.Message, func(ctx context.Context, clientID string, args ...any) error {
		gotID.Store(clientID)
		count.Add(1)
		return nil
	})
	mgr.routeEvent("acc1", events.Message)
	waitFor(t, func() bool { return count.Load() == 1 })
	if gotID.Load() != "acc1" {
		t.Fatalf("expected client id acc1, got %v", gotID.Load())
	}
	mgr.routeEvent("acc2", events.Message)
	time.Sleep(20 * time.Millisecond)
	if count.Load() != 1 {
		t.Fatalf("expected event isolation between client ids")
	}
}

func TestManagerOnAllClients(t *testing.T) {
	mgr := NewManager(nil, nil)
	var count atomic.Int32
	mgr.On(AllClients, events.Message, func(ctx context.Context, clientID string, args ...any) error {
		count.Add(1)
		return nil
	})
	mgr.routeEvent("acc1", events.Message)
	mgr.routeEvent("acc2", events.Message)
	waitFor(t, func() bool { return count.Load() == 2 })
}

func TestManagerOff(t *testing.T) {
	mgr := NewManager(nil, nil)
	var count atomic.Int32
	h := func(ctx context.Context, clientID string, args ...any) error {
		count.Add(1)
		return nil
	}
	mgr.On("acc1", events.Message, h)
	mgr.routeEvent("acc1", events.Message)
	waitFor(t, func() bool { return count.Load() == 1 })
	mgr.Off("acc1", events.Message, h)
	mgr.routeEvent("acc1", events.Message)
	time.Sleep(20 * time.Millisecond)
	if count.Load() != 1 {
		t.Fatalf("expected handler to be removed")
	}
}

func TestLoadAccountSpecs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	if err := os.WriteFile(path, []byte(`{
		"accounts": [
			{"id": "a", "cookies": "a.json", "restore": true},
			{"id": "b", "cookies": "b.json"}
		]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	specs, err := LoadAccountSpecs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 || specs[0].ID != "a" || !specs[0].Restore {
		t.Fatalf("unexpected specs: %+v", specs)
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