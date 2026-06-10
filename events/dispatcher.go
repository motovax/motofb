package events

import (
	"context"
	"log/slog"
	"reflect"
	"runtime"
	"sync"
)

// Handler processes a single event. Return an error to log it; dispatch continues.
type Handler func(ctx context.Context, args ...any) error

type registeredHandler struct {
	id uintptr
	fn Handler
}

// Dispatcher routes events to registered handlers.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[Type][]registeredHandler
	sem      chan struct{}
	log      *slog.Logger
}

func NewDispatcher(log *slog.Logger) *Dispatcher {
	if log == nil {
		log = slog.Default()
	}
	return &Dispatcher{
		handlers: make(map[Type][]registeredHandler),
		sem:      make(chan struct{}, 25),
		log:      log,
	}
}

func (d *Dispatcher) On(event Type, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[event] = append(d.handlers[event], registeredHandler{
		id: handlerID(handler),
		fn: handler,
	})
}

func (d *Dispatcher) Off(event Type, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	list := d.handlers[event]
	id := handlerID(handler)
	for i, h := range list {
		if h.id == id {
			d.handlers[event] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// Dispatch invokes all handlers for an event concurrently.
func (d *Dispatcher) Dispatch(ctx context.Context, event Type, args ...any) {
	d.mu.RLock()
	list := append([]registeredHandler(nil), d.handlers[event]...)
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
		}(h.fn)
	}
	wg.Wait()
}

func handlerID(h Handler) uintptr {
	if h == nil {
		return 0
	}
	return reflect.ValueOf(h).Pointer()
}

// RegisterMethods scans target for On* methods and registers them as handlers.
// Method names follow Go convention: OnMessage -> message, OnAdminAdded -> admin_added.
func (d *Dispatcher) RegisterMethods(target any) error {
	val := reflect.ValueOf(target)
	typ := val.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !method.IsExported() || len(method.Name) < 3 || method.Name[:2] != "On" {
			continue
		}
		eventName := camelToSnake(method.Name[2:])
		event := Type(eventName)
		mval := val.Method(i)
		fn := func(ctx context.Context, args ...any) error {
			in := make([]reflect.Value, 1+len(args))
			in[0] = reflect.ValueOf(ctx)
			for j, arg := range args {
				in[j+1] = reflect.ValueOf(arg)
			}
			out := mval.Call(in)
			if len(out) == 1 && !out[0].IsNil() {
				if err, ok := out[0].Interface().(error); ok {
					return err
				}
			}
			return nil
		}
		d.On(event, fn)
	}
	return nil
}

func camelToSnake(s string) string {
	if s == "" {
		return ""
	}
	var out []byte
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out = append(out, '_')
			}
			out = append(out, byte(r+'a'-'A'))
		} else {
			out = append(out, byte(r))
		}
	}
	return string(out)
}

// HandlerName returns the runtime name of a handler for debugging.
func HandlerName(h Handler) string {
	if h == nil {
		return "<nil>"
	}
	return runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
}