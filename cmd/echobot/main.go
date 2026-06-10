// Echo bot example — replies in Messenger groups (not 1:1 DMs).
//
// First-time setup:
//
//	cat cookie-export.json | go run ./cmd/importcookies default
//
// Run:
//
//	go run ./cmd/echobot
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/motovax/motofb"
	"github.com/motovax/motofb/events"
	"github.com/motovax/motofb/models"
)

func main() {
	const (
		clientID = "default"
		dbPath   = "sessions.db"
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mgr, err := motofb.NewManagerWithSQLite(dbPath, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = mgr.Close(context.Background(), true)
	}()

	client, err := mgr.RestoreClient(ctx, clientID)
	if err != nil {
		log.Fatal(err)
	}

	client.On(events.Message, func(ctx context.Context, args ...any) error {
		msg, ok := args[0].(models.Message)
		if !ok || msg.SenderID == client.UID() {
			return nil
		}
		_, err := client.SendMessage(ctx, "echo: "+msg.Text, msg.ThreadID)
		return err
	})

	log.Printf("Logged in as %s (%s)", client.Name(), client.UID())
	if err := client.Run(ctx); err != nil {
		log.Fatal(err)
	}
}