package motofb

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/motovax/motofb/events"
	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/state"
	"github.com/motovax/motofb/storage"
)

// Manager orchestrates multiple Client instances.
type Manager struct {
	Clients map[string]*Client
	storage storage.Store
	log     *slog.Logger

	mu        sync.RWMutex
	listeners map[managerKey][]events.Handler
	sem       chan struct{}
}

type managerKey struct {
	clientID string
	event    events.Type
}

// NewManager creates a multi-account manager.
func NewManager(store storage.Store, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		Clients:   make(map[string]*Client),
		storage:   store,
		log:       log,
		listeners: make(map[managerKey][]events.Handler),
		sem:       make(chan struct{}, 25),
	}
}

// NewManagerWithDir uses JSON file storage under dir.
func NewManagerWithDir(dir string, log *slog.Logger) *Manager {
	return NewManager(&storage.JSONStore{Directory: dir}, log)
}

// NewManagerWithRedis uses Redis-backed session storage.
func NewManagerWithRedis(redisURL, keyPrefix string, log *slog.Logger) (*Manager, error) {
	store, err := storage.NewRedisStore(redisURL, keyPrefix)
	if err != nil {
		return nil, err
	}
	return NewManager(store, log), nil
}

// On registers a manager-scoped handler for a specific client id.
func (m *Manager) On(clientID string, event events.Type, handler events.Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := managerKey{clientID: clientID, event: event}
	m.listeners[key] = append(m.listeners[key], handler)
}

// AddClient registers a new account.
func (m *Manager) AddClient(ctx context.Context, clientID, cookiesPath string, opts ...Option) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Clients[clientID]; ok {
		return nil, fberr.New("AddClient", fmt.Sprintf("client id already registered: %s", clientID))
	}
	cfg := clientConfig{log: m.log, online: true, cookiesPath: cookiesPath}
	for _, o := range opts {
		o(&cfg)
	}
	var st *state.State
	var err error
	if cfg.initialState != nil {
		st = cfg.initialState
	} else {
		st, err = state.FromCookieFile(ctx, cookiesPath, state.Options{
			UserAgent: cfg.userAgent,
			ProxyURL:  cfg.proxyURL,
		})
		if err != nil {
			return nil, err
		}
	}
	c := newClient(st, cfg)
	c.managerID = clientID
	c.managerRoute = m.routeEvent
	m.Clients[clientID] = c
	return c, nil
}

// RestoreClient loads cookies from storage then adds client.
func (m *Manager) RestoreClient(ctx context.Context, clientID, cookiesPath string, opts ...Option) (*Client, error) {
	if m.storage == nil {
		return m.AddClient(ctx, clientID, cookiesPath, opts...)
	}
	snap, err := m.storage.Load(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if snap != nil {
		if cookies, ok := snap["cookies"].([]any); ok && len(cookies) > 0 {
			records := cookieRecordsFromSnap(cookies)
			st, err := state.FromCookieRecords(ctx, records, state.Options{})
			if err == nil {
				opts = append(opts, WithInitialState(st))
			}
		}
	}
	return m.AddClient(ctx, clientID, cookiesPath, opts...)
}

func cookieRecordsFromSnap(items []any) []state.CookieRecord {
	out := make([]state.CookieRecord, 0, len(items))
	for _, item := range items {
		m, _ := item.(map[string]any)
		out = append(out, state.CookieRecord{
			Name:  fmt.Sprint(m["name"]),
			Value: fmt.Sprint(m["value"]),
			Path:  fmt.Sprint(m["path"]),
		})
	}
	return out
}

// RemoveClient stops and removes a client.
func (m *Manager) RemoveClient(ctx context.Context, clientID string, persist bool) error {
	m.mu.Lock()
	c := m.Clients[clientID]
	delete(m.Clients, clientID)
	m.mu.Unlock()
	if c == nil {
		return nil
	}
	if persist && m.storage != nil && c.state != nil {
		_ = m.storage.Save(ctx, clientID, c.state.Snapshot())
	}
	return c.Close()
}

// SaveSession persists one client session.
func (m *Manager) SaveSession(ctx context.Context, clientID string) error {
	if m.storage == nil {
		return fberr.New("SaveSession", "no session storage configured")
	}
	m.mu.RLock()
	c := m.Clients[clientID]
	m.mu.RUnlock()
	if c == nil || c.state == nil {
		return fberr.New("SaveSession", "no active session for client id: "+clientID)
	}
	return m.storage.Save(ctx, clientID, c.state.Snapshot())
}

// Broadcast sends a message from every connected client.
func (m *Manager) Broadcast(ctx context.Context, message, threadID string) {
	m.mu.RLock()
	clients := make([]*Client, 0, len(m.Clients))
	for _, c := range m.Clients {
		clients = append(clients, c)
	}
	m.mu.RUnlock()
	for _, c := range clients {
		if _, err := c.SendMessage(ctx, message, threadID); err != nil {
			m.log.Error("broadcast failed", "uid", c.UID(), "error", err)
		}
	}
}

// StartAll connects and listens on all clients.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	clients := make([]*Client, 0, len(m.Clients))
	for _, c := range m.Clients {
		clients = append(clients, c)
	}
	m.mu.RUnlock()
	for _, c := range clients {
		if err := c.Connect(ctx); err != nil {
			return err
		}
		if err := c.StartListening(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all clients.
func (m *Manager) StopAll() {
	m.mu.RLock()
	clients := make([]*Client, 0, len(m.Clients))
	for _, c := range m.Clients {
		clients = append(clients, c)
	}
	m.mu.RUnlock()
	for _, c := range clients {
		_ = c.StopListening()
	}
}

func (m *Manager) routeEvent(clientID string, event events.Type, args ...any) {
	m.mu.RLock()
	list := append([]events.Handler(nil), m.listeners[managerKey{clientID: clientID, event: event}]...)
	m.mu.RUnlock()
	ctx := context.Background()
	for _, h := range list {
		go func(handler events.Handler) {
			m.sem <- struct{}{}
			defer func() { <-m.sem }()
			_ = handler(ctx, args...)
		}(h)
	}
}