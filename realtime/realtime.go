package realtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/motovax/motofb/models"
	"github.com/motovax/motofb/state"
)

const (
	wsHost = "wss://gateway.facebook.com/ws/realtime"
	appID  = "2220391788200892"
)

// Handler receives realtime WebSocket frames.
type Handler func(frameType string, data []byte)

// Client manages Facebook realtime gateway WebSocket.
type Client struct {
	st      *state.State
	handler Handler
	log     *slog.Logger
	conn    *websocket.Conn
	running bool
	wg      sync.WaitGroup
}

// Connect opens the realtime WebSocket and sends subscriptions.
func Connect(ctx context.Context, st *state.State, handler Handler, log *slog.Logger) (*Client, error) {
	if log == nil {
		log = slog.Default()
	}
	c := &Client{st: st, handler: handler, log: log}
	if err := c.connect(ctx); err != nil {
		return nil, err
	}
	c.running = true
	c.wg.Add(1)
	go c.listen(ctx)
	return c, nil
}

func (c *Client) connect(ctx context.Context) error {
	q := url.Values{}
	q.Set("x-dgw-appid", appID)
	q.Set("x-dgw-appversion", "0")
	q.Set("x-dgw-authtype", "1:0")
	q.Set("x-dgw-version", "5")
	q.Set("x-dgw-uuid", c.st.UserID)
	q.Set("x-dgw-tier", "prod")
	q.Set("x-dgw-deviceid", c.st.MQTTClientID)
	q.Set("x-dgw-app-stream-group", "group1")

	u := wsHost + "?" + q.Encode()
	cookie, err := state.CookieHeader(c.st.Jar, "https://www.facebook.com")
	if err != nil {
		return err
	}
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Cookie":          {cookie},
			"Origin":          {"https://www.facebook.com"},
			"User-Agent":      {c.st.UserAgent},
			"Referer":         {"https://www.facebook.com"},
			"Accept-Encoding": {"gzip, deflate, br"},
			"Accept-Language": {"en-US,en;q=0.9"},
		},
	}
	conn, _, err := websocket.Dial(ctx, u, opts)
	if err != nil {
		return err
	}
	c.conn = conn
	return c.sendSubscriptions(ctx)
}

func (c *Client) subscriptions() []string {
	uid := c.st.UserID
	return []string{
		`{"x-dgw-app-XRSS-method":"Falco","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:USER_ACTIVITY_UPDATE_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"9525970914181809","x-dgw-app-XRSS-routing_hint":"UserActivitySubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:ACTOR_GATEWAY_EXPERIENCE_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"24191710730466150","x-dgw-app-XRSS-routing_hint":"CometActorGatewayExperienceSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
		fmt.Sprintf(`{"x-dgw-app-XRSS-method":"FBLQ:comet_notifications_live_query_experimental","x-dgw-app-XRSS-doc_id":"9784489068321501","x-dgw-app-XRSS-actor_id":"%s","x-dgw-app-XRSS-page_id":"%s","x-dgw-app-XRSS-request_stream_retry":"false","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`, uid, uid),
		`{"x-dgw-app-XRSS-method":"FBGQLS:FRIEND_REQUEST_CONFIRM_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"9687616244672204","x-dgw-app-XRSS-routing_hint":"FriendingCometFriendRequestConfirmSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:FRIEND_REQUEST_RECEIVE_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"24047008371656912","x-dgw-app-XRSS-routing_hint":"FriendingCometFriendRequestReceiveSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:RTWEB_CALL_BLOCKED_SETTING_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"24429620016626810","x-dgw-app-XRSS-routing_hint":"RTWebCallBlockedSettingSubscription_CallBlockSettingSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
		`{"x-dgw-app-XRSS-method":"PresenceUnifiedJSON","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com/friends"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:MESSENGER_CHAT_TABS_NOTIFICATION_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"23885219097739619","x-dgw-app-XRSS-routing_hint":"MWChatTabsNotificationSubscription_MessengerChatTabsNotificationSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com/friends"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:BATCH_NOTIFICATION_STATE_CHANGE_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"30300156509571373","x-dgw-app-XRSS-routing_hint":"CometBatchNotificationsStateChangeSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com/friends"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:NOTIFICATION_STATE_CHANGE_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"23864641996495578","x-dgw-app-XRSS-routing_hint":"CometNotificationsStateChangeSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
		`{"x-dgw-app-XRSS-method":"FBGQLS:NOTIFICATION_STATE_CHANGE_SUBSCRIBE","x-dgw-app-XRSS-doc_id":"9754477301332178","x-dgw-app-XRSS-routing_hint":"CometFriendNotificationsStateChangeSubscription","x-dgw-app-xrs-body":"true","x-dgw-app-XRS-Accept-Ack":"RSAck","x-dgw-app-XRSS-http_referer":"https://www.facebook.com"}`,
	}
}

func (c *Client) sendSubscriptions(ctx context.Context) error {
	for i, payload := range c.subscriptions() {
		prefix := make([]byte, 7)
		prefix[0] = 14
		prefix[1] = byte(i)
		prefix[2] = 0
		binary.BigEndian.PutUint32(prefix[3:7], uint32(len(payload)))
		msg := append(prefix, []byte(payload)...)
		msg = append(msg, 0, 0)
		if err := c.conn.Write(ctx, websocket.MessageBinary, msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) listen(ctx context.Context) {
	defer c.wg.Done()
	for c.running {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if !c.running {
				return
			}
			c.log.Debug("realtime read ended, scheduling reconnect", "error", err)
			c.scheduleReconnect(ctx)
			return
		}
		if c.handler != nil {
			c.handler("binary", data)
		}
	}
}

func (c *Client) scheduleReconnect(ctx context.Context) {
	if !c.running {
		return
	}
	time.Sleep(time.Second)
	if !c.running {
		return
	}
	if err := c.reconnect(ctx); err != nil {
		c.log.Error("realtime reconnect failed", "error", err)
		if c.running {
			go c.scheduleReconnect(ctx)
		}
	}
}

func (c *Client) reconnect(ctx context.Context) error {
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusNormalClosure, "reconnect")
		c.conn = nil
	}
	return c.connect(ctx)
}

// ExtractJSON returns the JSON object slice from a realtime binary/text frame.
func ExtractJSON(data []byte) []byte {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	if data[0] == '{' || data[0] == '[' {
		return data
	}
	if idx := bytes.IndexByte(data, '{'); idx >= 0 {
		return data[idx:]
	}
	return nil
}

// FormatNotification extracts a notification from a realtime WebSocket payload.
func FormatNotification(data []byte) *models.FacebookNotification {
	jsonPayload := ExtractJSON(data)
	if jsonPayload == nil {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal(jsonPayload, &root); err != nil {
		return nil
	}
	viewer, _ := root["data"].(map[string]any)
	viewer, _ = viewer["viewer"].(map[string]any)
	if viewer == nil {
		return nil
	}
	page, _ := viewer["notifications_page"].(map[string]any)
	edges, _ := page["edges"].([]any)
	if len(edges) < 2 {
		return nil
	}
	edge, _ := edges[1].(map[string]any)
	node, _ := edge["node"].(map[string]any)
	notif, _ := node["notif"].(map[string]any)
	if notif == nil {
		return nil
	}
	tracking, _ := notif["tracking"].(map[string]any)
	fromUIDs, _ := tracking["from_uids"].(map[string]any)
	senderID := ""
	for k := range fromUIDs {
		senderID = k
		break
	}
	body := ""
	if b, ok := notif["body"].(map[string]any); ok {
		body = fmt.Sprint(b["text"])
	}
	var ts int64
	if ct, ok := notif["creation_time"].(map[string]any); ok {
		ts = int64(jsonNumber(ct["timestamp"]))
	}
	return &models.FacebookNotification{
		NotifID:   fmt.Sprint(notif["notif_id"]),
		Body:      body,
		SenderID:  senderID,
		URL:       fmt.Sprint(notif["url"]),
		Timestamp: ts,
		SeenState: notif["seen_state"],
	}
}

func jsonNumber(v any) float64 {
	f, _ := v.(float64)
	return f
}

// Stop closes the realtime connection.
func (c *Client) Stop() {
	c.running = false
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusNormalClosure, "shutdown")
	}
	c.wg.Wait()
}