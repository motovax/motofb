# Facebook cookie setup for motofb

motofb authenticates with **browser cookies**, not email/password. Cookie data is stored in **SQLite** (`sessions.db`) so you only export from the browser once per account.

## Required cookies

At minimum you need Facebook session cookies. The most important ones:

| Cookie | Purpose |
|--------|---------|
| `c_user` | Your Facebook user id (required) |
| `xs` | Session auth token |
| `datr` | Browser identifier |
| `sb` | Session binding |

motofb validates that `c_user` is present when importing. Missing or expired cookies cause login to fail.

## One-time export from Chrome / Edge

1. Log in to [facebook.com](https://www.facebook.com) in a normal browser session (the account you want the bot to use).
2. Install **[Cookie-Editor](https://cookie-editor.cgagnier.ca/)** (or any extension that exports JSON in fbstate/C3C format).
3. Open Cookie-Editor on `facebook.com`.
4. Click **Export** → choose **JSON** format.
5. Save the file (e.g. `shop-a-cookies.json`).

### Export format

motofb accepts the standard array format used by Cookie-Editor and fbstate:

```json
[
  {"name": "c_user", "value": "100001234567890", "path": "/"},
  {"name": "xs", "value": "…", "path": "/"},
  {"name": "datr", "value": "…", "path": "/"}
]
```

Some exporters use `"key"` instead of `"name"` — both work.

## Store cookies in SQLite

Import once per account. Cookies are written to `sessions.db` and reused on every subsequent run.

```bash
# Import browser export into SQLite (default db: sessions.db)
go run ./cmd/importcookies shop-a shop-a-cookies.json

# Multiple accounts
go run ./cmd/importcookies shop-b shop-b-cookies.json
```

Or from Go:

```go
mgr, _ := motofb.NewManagerWithSQLite("sessions.db", nil)
err := mgr.ImportCookies(ctx, "shop-a", "shop-a-cookies.json")
```

### What gets stored

SQLite table `sessions`:

| Column | Content |
|--------|---------|
| `client_id` | Account id (e.g. `shop-a`) |
| `snapshot` | JSON with `version` and `cookies` array |
| `updated_at` | Unix timestamp |

After the bot runs and refreshes tokens, `snapshot` is updated with the latest cookies from the live session.

## Multi-account workflow

**First time (import cookies):**

```bash
go run ./cmd/importcookies shop-a shop-a-cookies.json
go run ./cmd/importcookies shop-b shop-b-cookies.json
```

**`accounts.json`** (no cookie files needed after import):

```json
{
  "accounts": [
    {"id": "shop-a", "restore": true},
    {"id": "shop-b", "restore": true}
  ]
}
```

**Run:**

```bash
go run ./cmd/multibot
```

motofb loads cookies from `sessions.db`, fetches fresh tokens from Facebook HTML, and saves updated cookies back to SQLite on shutdown.

## Single-account (echobot)

For quick testing you can still point at a cookie file directly:

```bash
cp shop-a-cookies.json cookies.json
go run ./cmd/echobot
```

For production, prefer importing into SQLite and using the Manager.

## Refreshing cookies

Facebook sessions expire. When login fails:

1. Log in again in the browser (same account).
2. Re-export cookies with Cookie-Editor.
3. Re-import: `go run ./cmd/importcookies shop-a shop-a-cookies.json`

The import overwrites the previous row for that `client_id` in SQLite.

## Security

- Cookie files and `sessions.db` are **full account credentials**. Treat them like passwords.
- Do not commit cookies or `sessions.db` to git (add to `.gitignore`).
- Use a dedicated Facebook account for automation; unofficial API use can trigger bans.
- Restrict file permissions: `chmod 600 sessions.db` on shared servers.

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `c_user cookie not found` | Re-export while logged in to facebook.com |
| `authentication failed` / redirect loop | Cookies expired — re-export and re-import |
| Account works in browser but not bot | Export from `www.facebook.com`, not a mobile subdomain |
| 1:1 Messenger DMs don't work | Expected — E2E encryption; use **group chats** only |

## Firefox

1. Install Cookie-Editor for Firefox.
2. Same steps: log in → open extension on facebook.com → Export JSON → import with `importcookies`.