package events

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestDispatcherOff(t *testing.T) {
	d := NewDispatcher(nil)
	var count atomic.Int32
	h := func(ctx context.Context, args ...any) error {
		count.Add(1)
		return nil
	}
	d.On(Message, h)
	d.Dispatch(context.Background(), Message)
	if count.Load() != 1 {
		t.Fatalf("expected handler to run once, got %d", count.Load())
	}
	d.Off(Message, h)
	d.Dispatch(context.Background(), Message)
	if count.Load() != 1 {
		t.Fatalf("expected handler to be removed, got %d", count.Load())
	}
}

type hookTarget struct {
	called atomic.Int32
}

func (h *hookTarget) OnMessage(ctx context.Context, msg any) error {
	h.called.Add(1)
	return nil
}

func TestRegisterMethods(t *testing.T) {
	d := NewDispatcher(nil)
	target := &hookTarget{}
	if err := d.RegisterMethods(target); err != nil {
		t.Fatal(err)
	}
	d.Dispatch(context.Background(), Message, "hello")
	if target.called.Load() != 1 {
		t.Fatalf("expected method hook to run, got %d", target.called.Load())
	}
}