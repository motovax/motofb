package mqtt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"regexp"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/motovax/motofb/graphql"
	"github.com/motovax/motofb/state"
)

var (
	reFirstSeq = regexp.MustCompile(`"firstDeltaSeqId":\s*(\d+)`)
	reLastSeq  = regexp.MustCompile(`"lastIssuedSeqId":\s*(\d+)`)
	reSyncTok  = regexp.MustCompile(`"syncToken":\s*"([^"]+)"`)
)

// Handler receives MQTT messages.
type Handler func(topic string, payload []byte)

// Client manages Facebook Messenger MQTT over WebSocket.
type Client struct {
	st      *state.State
	handler Handler
	log     *slog.Logger

	client      pahomqtt.Client
	sequenceID  int
	syncToken   string
	chatOn      bool
	foreground  bool
	running     bool
	reconnecting bool

	stopOnce sync.Once
	wg       sync.WaitGroup
}

// Connect establishes MQTT session.
func Connect(ctx context.Context, st *state.State, chatOn, foreground bool, handler Handler, log *slog.Logger) (*Client, error) {
	if log == nil {
		log = slog.Default()
	}
	seq, err := fetchSequenceID(ctx, st)
	if err != nil {
		return nil, err
	}
	c := &Client{
		st:            st,
		handler:       handler,
		log:           log,
		sequenceID:    seq,
		chatOn:        chatOn,
		foreground:    foreground,
	}
	if err := c.dial(ctx); err != nil {
		return nil, err
	}
	c.running = true
	c.wg.Add(1)
	go c.presenceLoop()
	c.wg.Add(1)
	go c.reconnectScheduler()
	return c, nil
}

func fetchSequenceID(ctx context.Context, st *state.State) (int, error) {
	results, err := st.GraphQLBatch(ctx, graphql.FromDocID(graphql.DocSyncSequence, map[string]any{
		"limit":                   1,
		"tags":                    []string{"INBOX"},
		"before":                  nil,
		"includeDeliveryReceipts": false,
		"includeSeqID":            true,
	}))
	if err != nil || len(results) == 0 {
		return 0, err
	}
	viewer, _ := results[0]["viewer"].(map[string]any)
	mt, _ := viewer["message_threads"].(map[string]any)
	return int(jsonNumber(mt["sync_sequence_id"])), nil
}

func jsonNumber(v any) float64 {
	f, _ := v.(float64)
	return f
}

func (c *Client) dial(ctx context.Context) error {
	sid := rand.Int63n(1 << 53)
	cookie, err := state.CookieHeader(c.st.Jar, "https://edge-chat.facebook.com/chat")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/chat?region=%s&sid=%d&cid=%s", c.st.Region, sid, c.st.MQTTClientID)
	username, _ := json.Marshal(map[string]any{
		"u": c.st.UserID, "s": sid, "chat_on": c.chatOn, "fg": c.foreground,
		"d": c.st.MQTTClientID, "aid": c.st.MQTTAppID, "st": SubscribeTopics,
		"pm": []any{}, "cp": 3, "ecp": 10, "ct": "websocket", "mqtt_sid": "",
		"dc": "", "no_auto_fg": true, "gas": nil, "pack": []any{}, "p": nil,
		"aids": nil, "a": c.st.UserAgent,
	})

	broker := fmt.Sprintf("wss://%s:443%s", Host, path)
	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("mqttwsclient")
	opts.SetUsername(string(username))
	opts.SetCleanSession(true)
	opts.SetProtocolVersion(3)
	opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
	opts.SetHTTPHeaders(map[string][]string{
		"Cookie":       {cookie},
		"User-Agent":   {c.st.UserAgent},
		"Origin":       {"https://www.facebook.com"},
		"Host":         {Host},
	})
	opts.SetAutoReconnect(false)
	opts.SetConnectTimeout(30 * time.Second)
	opts.SetDefaultPublishHandler(c.onMessage)
	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		c.log.Error("mqtt connection lost", "error", err)
	})

	c.client = pahomqtt.NewClient(opts)
	if t := c.client.Connect(); t.Wait() && t.Error() != nil {
		return t.Error()
	}
	if err := c.publishSyncQueue(); err != nil {
		return err
	}
	_ = ctx
	return nil
}

func (c *Client) onMessage(_ pahomqtt.Client, msg pahomqtt.Message) {
	payload := msg.Payload()
	c.extractMeta(payload)
	if c.handler != nil {
		c.handler(msg.Topic(), payload)
	}
}

func (c *Client) extractMeta(raw []byte) {
	s := string(raw)
	if m := reLastSeq.FindStringSubmatch(s); len(m) == 2 {
		c.sequenceID = atoi(m[1])
	} else if m := reFirstSeq.FindStringSubmatch(s); len(m) == 2 {
		c.sequenceID = atoi(m[1])
	}
	if m := reSyncTok.FindStringSubmatch(s); len(m) == 2 {
		c.syncToken = m[1]
	}
}

func (c *Client) publishSyncQueue() error {
	payload := map[string]any{
		"sync_api_version":          10,
		"max_deltas_able_to_process": 1000,
		"delta_batch_size":          500,
		"encoding":                  "JSON",
		"entity_fbid":               c.st.UserID,
	}
	topic := "/messenger_sync_create_queue"
	if c.syncToken != "" {
		topic = "/messenger_sync_get_diffs"
		payload["last_seq_id"] = fmt.Sprintf("%d", c.sequenceID)
		payload["sync_token"] = c.syncToken
	} else {
		payload["initial_titan_sequence_id"] = fmt.Sprintf("%d", c.sequenceID)
		payload["device_params"] = nil
	}
	b, _ := json.Marshal(payload)
	return c.Publish(topic, b, 1)
}

// Publish sends MQTT message.
func (c *Client) Publish(topic string, payload []byte, qos byte) error {
	if c.client == nil || !c.client.IsConnected() {
		return fmt.Errorf("mqtt: not connected")
	}
	t := c.client.Publish(topic, qos, false, payload)
	t.Wait()
	return t.Error()
}

// SetForeground publishes foreground state.
func (c *Client) SetForeground(v bool) error {
	b, _ := json.Marshal(map[string]any{"foreground": v})
	c.foreground = v
	return c.Publish("/foreground_state", b, 1)
}

// SetChatOn publishes client settings.
func (c *Client) SetChatOn(v bool) error {
	b, _ := json.Marshal(map[string]any{"make_user_available_when_in_foreground": v})
	c.chatOn = v
	return c.Publish("/set_client_settings", b, 1)
}

func (c *Client) presenceLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(50 * time.Second)
	defer ticker.Stop()
	for c.running {
		select {
		case <-ticker.C:
			if !c.running {
				return
			}
			if !c.running || c.client == nil || !c.client.IsConnected() {
				continue
			}
			p := map[string]any{
				"p": map[string]any{
					"user_id":     c.st.UserID,
					"last_active": time.Now().UnixMilli(),
					"active":      true,
				},
			}
			b, _ := json.Marshal(p)
			_ = c.Publish("/orca_presence", b, 1)
		}
	}
}

func (c *Client) reconnectScheduler() {
	defer c.wg.Done()
	for c.running {
		min := 26 * time.Minute
		max := 60 * time.Minute
		wait := min + time.Duration(rand.Int63n(int64(max-min)))
		deadline := time.After(wait)
		select {
		case <-deadline:
			if !c.running {
				return
			}
			_ = c.Reconnect(context.Background())
		}
	}
}

// Reconnect rotates client id and re-establishes session.
func (c *Client) Reconnect(ctx context.Context) error {
	c.reconnecting = true
	defer func() { c.reconnecting = false }()
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
	c.st.MQTTClientID = state.NewMQTTClientID()
	seq, err := fetchSequenceID(ctx, c.st)
	if err != nil {
		return err
	}
	c.sequenceID = seq
	c.syncToken = ""
	return c.dial(ctx)
}

// Stop disconnects MQTT.
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		c.running = false
		if c.client != nil && c.client.IsConnected() {
			c.client.Disconnect(250)
		}
		c.wg.Wait()
	})
}

func atoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}