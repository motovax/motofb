// Package motofb provides an unofficial Facebook Messenger client.
package motofb

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/motovax/motofb/events"
	"github.com/motovax/motofb/facebook"
	"github.com/motovax/motofb/messenger"
	mqttclient "github.com/motovax/motofb/mqtt"
	"github.com/motovax/motofb/parser"
	"github.com/motovax/motofb/realtime"
	"github.com/motovax/motofb/state"
)

// Client is the main entry point matching fbchat-muqit.Client.
type Client struct {
	state    *state.State
	events   *events.Dispatcher
	log      *slog.Logger
	parser   *parser.Parser
	messenger *messenger.Service
	facebook *facebook.Client
	mqtt     *mqttclient.Client
	realtime *realtime.Client
	ls       *messenger.LSRequester

	eventQueue chan parser.ParsedEvent
	listening  bool
	online     bool

	managerID    string
	managerRoute func(clientID string, event events.Type, args ...any)

	wg sync.WaitGroup
}

// Option configures Client construction.
type Option func(*clientConfig)

type clientConfig struct {
	userAgent     string
	proxyURL      string
	log           *slog.Logger
	online        bool
	initialState  *state.State
	cookiesPath   string
}

// WithUserAgent sets a custom browser user agent.
func WithUserAgent(ua string) Option {
	return func(c *clientConfig) { c.userAgent = ua }
}

// WithProxy sets an HTTP/SOCKS proxy URL.
func WithProxy(url string) Option {
	return func(c *clientConfig) { c.proxyURL = url }
}

// WithLogger sets the structured logger.
func WithLogger(log *slog.Logger) Option {
	return func(c *clientConfig) { c.log = log }
}

// WithOnline controls chat_on/foreground MQTT flags (default true).
func WithOnline(online bool) Option {
	return func(c *clientConfig) { c.online = online }
}

// WithInitialState restores a session without re-reading cookies.
func WithInitialState(st *state.State) Option {
	return func(c *clientConfig) { c.initialState = st }
}

// NewFromCookieFile authenticates using a browser cookie JSON export.
func NewFromCookieFile(ctx context.Context, cookiePath string, opts ...Option) (*Client, error) {
	cfg := clientConfig{log: slog.Default(), online: true, cookiesPath: cookiePath}
	for _, o := range opts {
		o(&cfg)
	}
	var st *state.State
	var err error
	if cfg.initialState != nil {
		st = cfg.initialState
	} else {
		st, err = state.FromCookieFile(ctx, cookiePath, state.Options{
			UserAgent: cfg.userAgent,
			ProxyURL:  cfg.proxyURL,
		})
		if err != nil {
			return nil, err
		}
	}
	return newClient(st, cfg), nil
}

func newClient(st *state.State, cfg clientConfig) *Client {
	p := parser.New()
	c := &Client{
		state:      st,
		events:     events.NewDispatcher(cfg.log),
		log:        cfg.log,
		parser:     p,
		online:     cfg.online,
		eventQueue: make(chan parser.ParsedEvent, 1000),
	}
	c.ls = messenger.NewLSRequester(c.publishMQTT)
	c.messenger = &messenger.Service{State: st, UID: st.UserID, LS: c.ls, Parser: p}
	c.facebook = facebook.New(st)
	return c
}

func (c *Client) publishMQTT(topic string, payload []byte) error {
	if c.mqtt == nil {
		return state.ErrNotLoggedIn()
	}
	return c.mqtt.Publish(topic, payload, 1)
}

// UID returns the authenticated Facebook user id.
func (c *Client) UID() string { return c.state.UserID }

// ManagerID returns the id assigned by a Manager, or empty for standalone clients.
func (c *Client) ManagerID() string { return c.managerID }

// Name returns the authenticated user's display name when available.
func (c *Client) Name() string { return c.state.UserName }

// State exposes the session for advanced use.
func (c *Client) State() *state.State { return c.state }

// Messenger returns the messenger API service.
func (c *Client) Messenger() *messenger.Service { return c.messenger }

// Facebook returns the facebook API service.
func (c *Client) Facebook() *facebook.Client { return c.facebook }

// On registers an event handler.
func (c *Client) On(event events.Type, handler events.Handler) {
	c.events.On(event, handler)
}

// Off removes a previously registered event handler.
func (c *Client) Off(event events.Type, handler events.Handler) {
	c.events.Off(event, handler)
}

// RegisterMethods registers On* methods from hooks as event handlers.
func (c *Client) RegisterMethods(hooks any) error {
	return c.events.RegisterMethods(hooks)
}

// EnableAutoRefresh enables periodic session token refresh on the underlying state.
func (c *Client) EnableAutoRefresh(interval time.Duration) {
	if c.state != nil {
		c.state.EnableAutoRefresh(interval)
	}
}

// Connect validates the session is ready.
func (c *Client) Connect(ctx context.Context) error {
	_ = ctx
	if c.state == nil || !c.state.LoggedIn {
		return state.ErrNotLoggedIn()
	}
	return nil
}

// StartListening connects MQTT and realtime and begins event dispatch.
func (c *Client) StartListening(ctx context.Context) error {
	if c.state == nil {
		return state.ErrNotLoggedIn()
	}
	if c.listening {
		return nil
	}
	var err error
	c.mqtt, err = mqttclient.Connect(ctx, c.state, c.online, c.online, c.handleMQTT, c.log)
	if err != nil {
		return err
	}
	c.realtime, err = realtime.Connect(ctx, c.state, c.handleRealtime, c.log)
	if err != nil {
		c.mqtt.Stop()
		c.mqtt = nil
		return err
	}
	if c.online {
		_ = c.mqtt.SetChatOn(true)
		_ = c.mqtt.SetForeground(true)
	}
	c.listening = true
	c.wg.Add(1)
	go c.dispatchLoop(ctx)
	return nil
}

// StopListening stops MQTT and realtime.
func (c *Client) StopListening() error {
	c.listening = false
	if c.mqtt != nil {
		c.mqtt.Stop()
		c.mqtt = nil
	}
	if c.realtime != nil {
		c.realtime.Stop()
		c.realtime = nil
	}
	return nil
}

// Listen blocks until ctx is cancelled after starting listeners.
func (c *Client) Listen(ctx context.Context) error {
	if err := c.StartListening(ctx); err != nil {
		return err
	}
	c.dispatchEvent(ctx, events.Listening)
	<-ctx.Done()
	return c.StopListening()
}

// Run is a blocking convenience: connect, listen until interrupt, close.
func (c *Client) Run(ctx context.Context) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}
	defer c.Close()
	return c.Listen(ctx)
}

// Close tears down the client session.
func (c *Client) Close() error {
	_ = c.StopListening()
	c.wg.Wait()
	return c.state.Close()
}

func (c *Client) handleMQTT(topic string, payload []byte) {
	if topic == "/ls_resp" {
		var resp struct {
			RequestID int    `json:"request_id"`
			Payload   string `json:"payload"`
		}
		if err := json.Unmarshal(payload, &resp); err == nil {
			c.ls.Resolve(messenger.LSResponse{RequestID: resp.RequestID, Payload: resp.Payload})
		}
		return
	}
	if topic == "/t_ms" && bytesContains(payload, []byte("deltas")) {
		for _, ev := range c.parser.ParseTMS(payload) {
			c.enqueue(ev)
		}
		return
	}
	if ev := c.parser.ParseAll(topic, payload); ev != nil {
		c.enqueue(*ev)
	}
}

func (c *Client) handleRealtime(_ string, _ []byte) {
	// Parity with Python: realtime handler is currently no-op.
}

func (c *Client) enqueue(ev parser.ParsedEvent) {
	select {
	case c.eventQueue <- ev:
	default:
		c.log.Warn("event queue full, dropping event", "type", ev.EventType)
	}
}

func (c *Client) dispatchLoop(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-c.eventQueue:
			if !ok || !c.listening {
				return
			}
			c.dispatchEvent(ctx, ev.EventType, ev.Args...)
		}
	}
}

func (c *Client) dispatchEvent(ctx context.Context, event events.Type, args ...any) {
	if c.managerRoute != nil && c.managerID != "" {
		c.managerRoute(c.managerID, event, args...)
	}
	c.events.Dispatch(ctx, event, args...)
}

func bytesContains(b, sub []byte) bool {
	return len(sub) == 0 || (len(b) >= len(sub) && indexOfBytes(b, sub) >= 0)
}

func indexOfBytes(b, sub []byte) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		if string(b[i:i+len(sub)]) == string(sub) {
			return i
		}
	}
	return -1
}