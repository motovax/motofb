# motofb

Go port of [fbchat-muqit](https://github.com/togashigreat/fbchat-muqit) v1.2.2 — unofficial Facebook + Messenger client.

**Module:** `github.com/motovax/motofb`

## Features

| Area | Status |
|---|---|
| Cookie session + HTML token extraction | Done |
| GraphQL batch + single mutations | Done |
| MQTT WebSocket (`edge-chat.facebook.com`) | Done |
| Realtime WebSocket (`gateway.facebook.com`) | Done |
| Delta/event parser (`/t_ms`, typing, presence, legacy_web) | Done |
| Messenger: send, react, unsend, threads, users, files | Done |
| Facebook: posts, photos, friend requests, reactions | Done |
| `ClientManager` multi-account + SQLite cookie storage | Done |

## Requirements

- Go 1.25+
- Facebook cookies exported from your browser (see **[docs/COOKIES.md](docs/COOKIES.md)**)
- **Group/page chats only** for bots — 1:1 Messenger DMs use E2E encryption

## Install

```bash
go get github.com/motovax/motofb
```

## Cookies and SQLite

Cookie data is stored in **`sessions.db`** (SQLite), not loose JSON files after the first import.

1. Export cookies from the browser — full guide: **[docs/COOKIES.md](docs/COOKIES.md)**
2. Import once per account:

```bash
go run ./cmd/importcookies shop-a shop-a-cookies.json
```

3. Run multi-account bot (loads cookies from SQLite):

```bash
cp accounts.json.example accounts.json
go run ./cmd/multibot
```

## Quick start (single account)

```go
client, err := motofb.NewFromCookieFile(ctx, "cookies.json")
defer client.Close()

client.On(events.Message, func(ctx context.Context, args ...any) error {
    msg := args[0].(models.Message)
    if msg.SenderID != client.UID() {
        _, _ = client.SendMessage(ctx, "hi", msg.ThreadID)
    }
    return nil
})

client.Run(ctx) // blocks until SIGINT
```

Echo bot (uses `cookies.json` in the working directory):

```bash
go run ./cmd/echobot
```

## Multi-account

```go
mgr, _ := motofb.NewManagerWithSQLite("sessions.db", nil)

// First time only — import browser exports into SQLite:
_ = mgr.ImportCookies(ctx, "shop-a", "shop-a-cookies.json")

// Register accounts (cookies loaded from SQLite when restore:true):
_ = mgr.AddAccountsFromFile(ctx, "accounts.json")

mgr.On(motofb.AllClients, events.Message, func(ctx context.Context, clientID string, args ...any) error {
    client, _ := mgr.GetClient(clientID)
    msg := args[0].(models.Message)
    // handle per-account ...
    return nil
})

_ = mgr.Run(ctx)
defer mgr.Close(ctx, true) // persist refreshed cookies to SQLite
```

`accounts.json` after cookies are imported:

```json
{
  "accounts": [
    {"id": "shop-a", "restore": true},
    {"id": "shop-b", "restore": true}
  ]
}
```

### Logging

```go
client.EnableInfoLogging()   // message, reactions, friend requests, notifications
client.EnableDefaultLogging() // debug log for every event type
```

See `docs/plans/fbchat-muqit-go-api-mapping.md` in the MotoVax monorepo for the full Python → Go mapping.

## Development

```bash
go test ./...
go build ./...
```

## License

Dual-licensed like upstream: GPL-3.0 (new Go code) + BSD-3-Clause (fbchat-derived portions).