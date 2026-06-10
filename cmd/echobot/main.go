// Echo bot example — replies in Messenger groups (not 1:1 DMs).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/motovax/motofb"
	"github.com/motovax/motofb/events"
	"github.com/motovax/motofb/models"
)

func main() {
	cookiePath := "cookies.json"
	if p := os.Getenv("MOTOFB_COOKIES_FILE"); p != "" {
		cookiePath = p
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	client, err := motofb.NewFromCookieFile(ctx, cookiePath)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

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