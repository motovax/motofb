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

Cookie data lives in **`sessions.db`** (SQLite). Export from the browser once, pipe JSON into SQLite, then run bots.

1. Export cookies from the browser — full guide: **[docs/COOKIES.md](docs/COOKIES.md)**
2. Import once per account:

```bash
cat cookie-export.json | go run ./cmd/importcookies shop-a
```

3. Run multi-account bot:

```bash
cp accounts.json.example accounts.json
go run ./cmd/multibot
```

## Quick start (single account)

```go
mgr, _ := motofb.NewManagerWithSQLite("sessions.db", nil)
client, _ := mgr.RestoreClient(ctx, "default")
defer mgr.Close(ctx, true)

client.On(events.Message, func(ctx context.Context, args ...any) error {
    msg := args[0].(models.Message)
    if msg.SenderID != client.UID() {
        _, _ = client.SendMessage(ctx, "hi", msg.ThreadID)
    }
    return nil
})

client.Run(ctx) // blocks until SIGINT
```

Echo bot:

```bash
cat cookie-export.json | go run ./cmd/importcookies default
go run ./cmd/echobot
```

## Multi-account

```go
mgr, _ := motofb.NewManagerWithSQLite("sessions.db", nil)

// First time only — import browser JSON into SQLite:
_ = mgr.ImportCookies(ctx, "shop-a", cookieJSON)

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

`accounts.json`:

```json
{
  "accounts": [
    {"id": "shop-a"},
    {"id": "shop-b"}
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