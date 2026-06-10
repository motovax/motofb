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