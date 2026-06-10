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
| `ClientManager` multi-account + SQLite session storage | Done |

## Requirements

- Go 1.25+
- Facebook cookies JSON (browser export)
- **Group/page chats only** for bots — 1:1 Messenger DMs use E2E encryption

## Install

```bash
go get github.com/motovax/motofb
```

## Quick start

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

Echo bot example:

```bash
go run ./cmd/echobot
```

## API surface

`Client` exposes Python `MessengerClient` + `FacebookClient` methods:

- **Messenger:** `SendMessage`, `React`, `FetchThreadList`, `CreateGroupThread`, `MuteThread`, …
- **Facebook:** `PublishPost`, `UploadPhoto`, `SendFriendRequest`, `ReactToPost`, …
- **Lifecycle:** `StartListening`, `Listen`, `Run`, `Close`
- **Multi-account:** `motofb.Manager` — isolated sessions, MQTT, and event routing per account

### Multi-account

```go
mgr, _ := motofb.NewManagerWithSQLite("sessions.db", nil)
_ = mgr.AddAccountsFromFile(ctx, "accounts.json") // or mgr.AddAccounts(ctx, specs...)

mgr.On(motofb.AllClients, events.Message, func(ctx context.Context, clientID string, args ...any) error {
    client, _ := mgr.GetClient(clientID)
    msg := args[0].(models.Message)
    // handle per-account ...
    return nil
})

_ = mgr.Run(ctx) // start all accounts, block until shutdown
defer mgr.Close(ctx, true) // persist cookie snapshots
```

`accounts.json`:

```json
{
  "accounts": [
    {"id": "shop-a", "cookies": "cookies-a.json", "restore": true},
    {"id": "shop-b", "cookies": "cookies-b.json", "restore": true}
  ]
}
```

Example: `go run ./cmd/multibot` (reads `accounts.json`, stores sessions in `sessions.db`)

SQLite stores one row per account (`client_id`, cookie snapshot JSON). JSON dir and Redis backends remain available via `NewManagerWithDir` / `NewManagerWithRedis`.

Copy `accounts.json.example` to `accounts.json` and add one cookies file per account.

### Logging

```go
client.EnableInfoLogging()  // message, reactions, friend requests, notifications
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