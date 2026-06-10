// Import browser cookie JSON from stdin into SQLite session storage.
//
// Usage:
//
//	go run ./cmd/importcookies <client-id> [sessions.db] < cookie-json
//
// Example:
//
//	cat cookies-export.json | go run ./cmd/importcookies shop-a
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/motovax/motofb"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: importcookies <client-id> [sessions.db] < cookie-json\n")
		os.Exit(2)
	}
	clientID := os.Args[1]
	dbPath := "sessions.db"
	if len(os.Args) > 2 {
		dbPath = os.Args[2]
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	if len(data) == 0 {
		log.Fatal("no cookie JSON on stdin")
	}

	mgr, err := motofb.NewManagerWithSQLite(dbPath, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = mgr.Close(context.Background(), false)
	}()

	if err := mgr.ImportCookies(context.Background(), clientID, data); err != nil {
		log.Fatal(err)
	}
	log.Printf("imported cookies for %q into %s", clientID, dbPath)
}