// Multi-account echo bot — one process, many Facebook accounts.
//
// Configure accounts in accounts.json:
//
//	{
//	  "accounts": [
//	    {"id": "shop-a", "cookies": "cookies-a.json", "restore": true},
//	    {"id": "shop-b", "cookies": "cookies-b.json", "restore": true}
//	  ]
//	}
//
// Run:
//
//	MOTOFB_ACCOUNTS_FILE=accounts.json MOTOFB_SESSIONS_DB=sessions.db go run ./cmd/multibot
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
	accountsFile := os.Getenv("MOTOFB_ACCOUNTS_FILE")
	if accountsFile == "" {
		accountsFile = "accounts.json"
	}
	dbPath := os.Getenv("MOTOFB_SESSIONS_DB")
	if dbPath == "" {
		dbPath = "sessions.db"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mgr, err := motofb.NewManagerWithSQLite(dbPath, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := mgr.AddAccountsFromFile(ctx, accountsFile); err != nil {
		log.Fatal(err)
	}

	for _, id := range mgr.ClientIDs() {
		c, err := mgr.GetClient(id)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("registered %s as %s (%s)", id, c.Name(), c.UID())
	}

	mgr.On(motofb.AllClients, events.Message, func(ctx context.Context, clientID string, args ...any) error {
		msg, ok := args[0].(models.Message)
		if !ok {
			return nil
		}
		client, err := mgr.GetClient(clientID)
		if err != nil || msg.SenderID == client.UID() {
			return nil
		}
		_, err = client.SendMessage(ctx, "["+clientID+"] echo: "+msg.Text, msg.ThreadID)
		return err
	})

	defer func() {
		_ = mgr.Close(context.Background(), true)
	}()

	if err := mgr.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}