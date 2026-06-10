// Import browser cookie exports into SQLite session storage.
//
// Usage:
//
//	go run ./cmd/importcookies <client-id> <browser-export.json> [sessions.db]
//
// Example:
//
//	go run ./cmd/importcookies shop-a shop-a-cookies.json
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/motovax/motofb"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: importcookies <client-id> <browser-export.json> [sessions.db]\n")
		os.Exit(2)
	}
	clientID := os.Args[1]
	cookieFile := os.Args[2]
	dbPath := "sessions.db"
	if len(os.Args) > 3 {
		dbPath = os.Args[3]
	}

	mgr, err := motofb.NewManagerWithSQLite(dbPath, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = mgr.Close(context.Background(), false)
	}()

	if err := mgr.ImportCookies(context.Background(), clientID, cookieFile); err != nil {
		log.Fatal(err)
	}
	log.Printf("imported cookies for %q into %s", clientID, dbPath)
}