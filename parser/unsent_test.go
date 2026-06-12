package parser_test

import (
	"testing"

	"github.com/motovax/motofb/parser"
)

func TestParseMessageNotUnsentWhenTimestampZero(t *testing.T) {
	p := parser.New()
	msg := p.ParseMessageFromGraphQL(map[string]any{
		"message_id":                "mid.1",
		"timestamp_precise":         "1770655962781",
		"unsent_timestamp_precise":  "0",
		"message_sender":            map[string]any{"id": "123"},
		"message":                   map[string]any{"text": "hello"},
	}, "thread", 0)
	if msg.Unsent {
		t.Fatal("expected normal message not to be marked unsent")
	}
}

func TestParseMessageUnsentWhenTimestampSet(t *testing.T) {
	p := parser.New()
	msg := p.ParseMessageFromGraphQL(map[string]any{
		"message_id":               "mid.2",
		"timestamp_precise":        "1770655962781",
		"unsent_timestamp_precise": "1770655999999",
		"message_sender":           map[string]any{"id": "123"},
		"message":                  map[string]any{"text": "gone"},
	}, "thread", 0)
	if !msg.Unsent {
		t.Fatal("expected deleted message to be marked unsent")
	}
}