package events

import (
	"context"
	"log/slog"
	"sync"
)

// Handler processes a single event. Return an error to log it; dispatch continues.
type Handler func(ctx context.Context, args ...any) error

// Dispatcher routes events to registered handlers.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[Type][]Handler
	sem      chan struct{}
	log      *slog.Logger
}

func NewDispatcher(log *slog.Logger) *Dispatcher {
	if log == nil {
		log = slog.Default()
	}
	return &Dispatcher{
		handlers: make(map[Type][]Handler),
		sem:      make(chan struct{}, 25),
		log:      log,
	}
}

func (d *Dispatcher) On(event Type, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[event] = append(d.handlers[event], handler)
}

func (d *Dispatcher) Off(event Type, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	list := d.handlers[event]
	for i, h := range list {
		if &h == &handler {
			d.handlers[event] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// Dispatch invokes all handlers for an event concurrently.
func (d *Dispatcher) Dispatch(ctx context.Context, event Type, args ...any) {
	d.mu.RLock()
	list := append([]Handler(nil), d.handlers[event]...)
	d.mu.RUnlock()

	var wg sync.WaitGroup
	for _, h := range list {
		wg.Add(1)
		go func(handler Handler) {
			defer wg.Done()
			d.sem <- struct{}{}
			defer func() { <-d.sem }()
			if err := handler(ctx, args...); err != nil {
				d.log.Error("event handler failed", "event", event, "error", err)
			}
		}(h)
	}
	wg.Wait()
}