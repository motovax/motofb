package motofb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"github.com/motovax/motofb/events"
	fberr "github.com/motovax/motofb/errors"
	"github.com/motovax/motofb/state"
	"github.com/motovax/motofb/storage"
)

// AllClients is the wildcard client id for manager handlers that receive every account's events.
const AllClients = "*"

// ManagerHandler processes an event for one managed account.
type ManagerHandler func(ctx context.Context, clientID string, args ...any) error

type registeredManagerHandler struct {
	id uintptr
	fn ManagerHandler
}

// Manager orchestrates multiple Client instances with isolated sessions and MQTT connections.
type Manager struct {
	Clients map[string]*Client
	storage storage.Store
	log     *slog.Logger

	mu        sync.RWMutex
	listeners map[managerKey][]registeredManagerHandler
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
		listeners: make(map[managerKey][]registeredManagerHandler),
		sem:       make(chan struct{}, 25),
	}
}

// NewManagerWithSQLite uses SQLite session storage (recommended for multi-account).
func NewManagerWithSQLite(dbPath string, log *slog.Logger) (*Manager, error) {
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		return nil, err
	}
	return NewManager(store, log), nil
}

// NewManagerWithDir uses JSON file storage under dir (one snapshot per client id).
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

// On registers a handler for one account. Use AllClients to receive events from every account.
func (m *Manager) On(clientID string, event events.Type, handler ManagerHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := managerKey{clientID: clientID, event: event}
	m.listeners[key] = append(m.listeners[key], registeredManagerHandler{
		id: handlerID(handler),
		fn: handler,
	})
}

// Off removes a previously registered manager handler.
func (m *Manager) Off(clientID string, event events.Type, handler ManagerHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := managerKey{clientID: clientID, event: event}
	list := m.listeners[key]
	id := handlerID(handler)
	for i, h := range list {
		if h.id == id {
			m.listeners[key] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// RegisterMethods registers On* methods from hooks for a specific account.
// Method signatures must be func(ctx context.Context, clientID string, ...) error.
func (m *Manager) RegisterMethods(clientID string, hooks any) error {
	val := reflect.ValueOf(hooks)
	typ := val.Type()
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !method.IsExported() || len(method.Name) < 3 || method.Name[:2] != "On" {
			continue
		}
		eventName := camelToSnake(method.Name[2:])
		event := events.Type(eventName)
		mval := val.Method(i)
		cid := clientID
		m.On(cid, event, func(ctx context.Context, clientID string, args ...any) error {
			in := make([]reflect.Value, 2+len(args))
			in[0] = reflect.ValueOf(ctx)
			in[1] = reflect.ValueOf(clientID)
			for j, arg := range args {
				in[j+2] = reflect.ValueOf(arg)
			}
			out := mval.Call(in)
			if len(out) == 1 && !out[0].IsNil() {
				if err, ok := out[0].Interface().(error); ok {
					return err
				}
			}
			return nil
		})
	}
	return nil
}

// ClientIDs returns registered account ids.
func (m *Manager) ClientIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.Clients))
	for id := range m.Clients {
		ids = append(ids, id)
	}
	return ids
}

// GetClient returns a managed client by id.
func (m *Manager) GetClient(clientID string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.Clients[clientID]
	if c == nil {
		return nil, fberr.New("GetClient", "unknown client id: "+clientID)
	}
	return c, nil
}

// AddClient registers a new account from a cookies file.
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

// RestoreClient loads a saved session snapshot when available, then registers the account.
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

// AddAccounts registers multiple accounts. Continues on individual failures and returns a joined error.
func (m *Manager) AddAccounts(ctx context.Context, accounts ...AccountSpec) error {
	var errs []error
	for _, spec := range accounts {
		var c *Client
		var err error
		if spec.Restore {
			c, err = m.RestoreClient(ctx, spec.ID, spec.CookiesPath, spec.options()...)
		} else {
			c, err = m.AddClient(ctx, spec.ID, spec.CookiesPath, spec.options()...)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", spec.ID, err))
			continue
		}
		m.log.Info("account registered", "client_id", spec.ID, "uid", c.UID(), "name", c.Name())
	}
	return errors.Join(errs...)
}

// AddAccountsFromFile loads account specs from JSON and registers them.
func (m *Manager) AddAccountsFromFile(ctx context.Context, path string) error {
	specs, err := LoadAccountSpecs(path)
	if err != nil {
		return err
	}
	return m.AddAccounts(ctx, specs...)
}

// StoredClientIDs returns client ids with persisted sessions when storage supports listing.
func (m *Manager) StoredClientIDs(ctx context.Context) ([]string, error) {
	if m.storage == nil {
		return nil, nil
	}
	lister, ok := m.storage.(storage.Lister)
	if !ok {
		return nil, fberr.New("StoredClientIDs", "storage does not support listing sessions")
	}
	return lister.List(ctx)
}

// RestoreAll registers every client id found in storage. cookiesFallback is used when
// a snapshot cannot be restored (optional; pass "" when snapshots are self-contained).
func (m *Manager) RestoreAll(ctx context.Context, cookiesFallback string) error {
	ids, err := m.StoredClientIDs(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		m.mu.RLock()
		_, exists := m.Clients[id]
		m.mu.RUnlock()
		if exists {
			continue
		}
		if _, err := m.RestoreClient(ctx, id, cookiesFallback); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errors.Join(errs...)
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

// SaveAllSessions persists every active client session.
func (m *Manager) SaveAllSessions(ctx context.Context) error {
	if m.storage == nil {
		return fberr.New("SaveAllSessions", "no session storage configured")
	}
	var errs []error
	for _, id := range m.ClientIDs() {
		if err := m.SaveSession(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
			m.log.Error("broadcast failed", "client_id", c.ManagerID(), "uid", c.UID(), "error", err)
		}
	}
}

// StartAll connects and listens on all clients. Partial failures are aggregated.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	clients := make([]*Client, 0, len(m.Clients))
	for _, c := range m.Clients {
		clients = append(clients, c)
	}
	m.mu.RUnlock()
	var errs []error
	for _, c := range clients {
		if err := c.Connect(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s connect: %w", c.ManagerID(), err))
			continue
		}
		if err := c.StartListening(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s listen: %w", c.ManagerID(), err))
		}
	}
	return errors.Join(errs...)
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

// Close stops all clients and optionally persists sessions when storage is configured.
func (m *Manager) Close(ctx context.Context, persist bool) error {
	if persist && m.storage != nil {
		_ = m.SaveAllSessions(ctx)
	}
	m.StopAll()
	var errs []error
	for _, id := range m.ClientIDs() {
		if err := m.RemoveClient(ctx, id, false); err != nil {
			errs = append(errs, err)
		}
	}
	if closer, ok := m.storage.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Run starts every account and blocks until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) error {
	if err := m.StartAll(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	m.StopAll()
	return ctx.Err()
}

func (m *Manager) routeEvent(clientID string, event events.Type, args ...any) {
	m.mu.RLock()
	specific := append([]registeredManagerHandler(nil), m.listeners[managerKey{clientID: clientID, event: event}]...)
	global := append([]registeredManagerHandler(nil), m.listeners[managerKey{clientID: AllClients, event: event}]...)
	m.mu.RUnlock()

	ctx := context.Background()
	dispatch := func(list []registeredManagerHandler) {
		for _, h := range list {
			go func(handler ManagerHandler) {
				m.sem <- struct{}{}
				defer func() { <-m.sem }()
				if err := handler(ctx, clientID, args...); err != nil {
					m.log.Error("manager handler failed", "client_id", clientID, "event", event, "error", err)
				}
			}(h.fn)
		}
	}
	dispatch(specific)
	dispatch(global)
}

func handlerID(h ManagerHandler) uintptr {
	if h == nil {
		return 0
	}
	return reflect.ValueOf(h).Pointer()
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