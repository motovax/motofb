package parser

import (
	"testing"

	"github.com/motovax/motofb/events"
	"github.com/motovax/motofb/models"
)

func TestParseNotificationsFriendRequest(t *testing.T) {
	p := New()
	payload := []byte(`{"type":"friending_state_change","userid":"123","action":"confirm"}`)
	ev := p.parseNotifications(payload)
	if ev == nil || ev.EventType != events.FriendRequestChange {
		t.Fatalf("unexpected event: %+v", ev)
	}
	state, ok := ev.Args[0].(models.FriendRequestState)
	if !ok || state.UserID != "123" || state.Action != "confirm" {
		t.Fatalf("unexpected args: %+v", ev.Args)
	}
}

func TestParseNotificationsPoke(t *testing.T) {
	p := New()
	payload := []byte(`{"type":"live_poke","poke_source":"999","poke_time":12345}`)
	ev := p.parseNotifications(payload)
	if ev == nil || ev.EventType != events.PokeNotification {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestParseClientPayloadMessageBump(t *testing.T) {
	p := New()
	delta := map[string]any{
		"payload": []any{byte('{'), byte('"'), byte('d'), byte('"'), byte(':'), byte('{'), byte('"'), byte('d'), byte('"'), byte(':'), byte('['), byte('{'), byte('"'), byte('r'), byte('"'), byte(':'), byte('1'), byte(','), byte('"'), byte('d'), byte('"'), byte(':'), byte('{'), byte('"'), byte('m'), byte('"'), byte(':'), byte('{'), byte('"'), byte('b'), byte('"'), byte(':'), byte('"'), byte('h'), byte('"'), byte('i'), byte('"'), byte('}'), byte('}'), byte('}'), byte(']'), byte('}'), byte('}')},
	}
	// Use proper JSON instead
	inner := `{"deltas":[{"replyType":1,"deltaMessageReply":{"repliedToMessage":{"body":"old","messageMetadata":{"messageId":"1","actorFbId":"2","threadKey":"t1","timestamp":1}},"message":{"body":"bump","messageMetadata":{"messageId":"2","actorFbId":"2","threadKey":"t1","timestamp":2}}}}]}`
	bytesPayload := make([]any, len(inner))
	for i, b := range []byte(inner) {
		bytesPayload[i] = float64(b)
	}
	delta = map[string]any{"payload": bytesPayload}
	ev := p.parseClientPayload(delta)
	if ev == nil || ev.EventType != events.MessageBump {
		t.Fatalf("expected message_bump, got %+v", ev)
	}
}

func TestParseMessageReaction(t *testing.T) {
	raw := map[string]any{
		"messageId":        "mid",
		"threadKey":        "tid",
		"userId":           float64(1),
		"senderId":         float64(2),
		"reaction":         "❤️",
		"action":           float64(0),
		"reactionTimestamp": float64(99),
	}
	r := parseMessageReaction(raw)
	if r.ID != "mid" || r.Reaction != "❤️" || r.Reactor != 1 {
		t.Fatalf("unexpected reaction: %+v", r)
	}
}