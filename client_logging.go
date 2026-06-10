package motofb

import (
	"context"

	"github.com/motovax/motofb/events"
)

// EnableDefaultLogging registers debug handlers that log every known event type.
func (c *Client) EnableDefaultLogging() {
	for _, event := range events.All() {
		ev := event
		c.On(ev, func(ctx context.Context, args ...any) error {
			c.log.Debug("event received", "type", ev, "args", len(args))
			return nil
		})
	}
}

// EnableInfoLogging registers info-level handlers for high-signal events only.
func (c *Client) EnableInfoLogging() {
	highSignal := []events.Type{
		events.Message,
		events.MessageBump,
		events.MessageReaction,
		events.FriendRequestChange,
		events.FriendRequestListUpdate,
		events.PokeNotification,
		events.Notification,
		events.Error,
	}
	for _, ev := range highSignal {
		event := ev
		c.On(event, func(ctx context.Context, args ...any) error {
			c.log.Info("event received", "type", event)
			return nil
		})
	}
}

// logParseIssue logs parser/MQTT issues without failing the listen loop.
func (c *Client) logParseIssue(topic, detail string, err error) {
	attrs := []any{"topic", topic, "detail", detail}
	if err != nil {
		attrs = append(attrs, "error", err)
	}
	c.log.Warn("mqtt parse issue", attrs...)
}