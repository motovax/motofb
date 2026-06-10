package state

import "github.com/motovax/motofb/internal"

// NewMQTTClientID generates a fresh MQTT device id on reconnect.
func NewMQTTClientID() string {
	return internal.GenerateUUID()
}