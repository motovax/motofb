package realtime

import (
	"encoding/binary"
	"testing"
)

func TestSubscriptionWireFormat(t *testing.T) {
	payload := `{"x-dgw-app-XRSS-method":"Falco"}`
	prefix := make([]byte, 7)
	prefix[0] = 14
	prefix[1] = 0
	prefix[2] = 0
	binary.BigEndian.PutUint32(prefix[3:7], uint32(len(payload)))
	msg := append(prefix, []byte(payload)...)
	msg = append(msg, 0, 0)

	if msg[0] != 14 {
		t.Fatalf("expected prefix byte 14, got %d", msg[0])
	}
	if msg[1] != 0 {
		t.Fatalf("expected index 0, got %d", msg[1])
	}
	if int(binary.BigEndian.Uint32(msg[3:7])) != len(payload) {
		t.Fatalf("unexpected payload length prefix")
	}
	if string(msg[7:7+len(payload)]) != payload {
		t.Fatalf("payload mismatch")
	}
	if msg[len(msg)-2] != 0 || msg[len(msg)-1] != 0 {
		t.Fatalf("expected trailing 0,0 suffix")
	}
}

func TestExtractJSON(t *testing.T) {
	raw := append([]byte{0, 14, 0, 0}, []byte(`{"data":{"viewer":{}}}`)...)
	out := ExtractJSON(raw)
	if out == nil || string(out) != `{"data":{"viewer":{}}}` {
		t.Fatalf("unexpected extract: %s", out)
	}
}

func TestFormatNotification(t *testing.T) {
	payload := []byte(`{
		"data": {
			"viewer": {
				"notifications_page": {
					"edges": [
						{},
						{
							"node": {
								"notif": {
									"notif_id": "n1",
									"body": {"text": "hello"},
									"url": "https://facebook.com",
									"creation_time": {"timestamp": 99},
									"tracking": {"from_uids": {"123": true}},
									"seen_state": "NEW"
								}
							}
						}
					]
				}
			}
		}
	}`)
	n := FormatNotification(payload)
	if n == nil || n.NotifID != "n1" || n.Body != "hello" || n.SenderID != "123" || n.Timestamp != 99 {
		t.Fatalf("unexpected notification: %+v", n)
	}
}