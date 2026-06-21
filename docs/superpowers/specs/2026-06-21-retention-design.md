# Retention — Design Spec

**Date:** 2026-06-21  
**Status:** Approved

---

## Overview

Add age-based retention to the RSS → Instapaper syncer. Articles older than `max_age_days` are archived in Instapaper (removed from reading queue, kept searchable) and purged from local SQLite state. Requires switching from the Instapaper Simple API to the full xAuth API to support bookmark management.

---

## Architecture

### Updated execution flow

```
start
  → load config (including max_age_days, default 1)
  → open SQLite (run migrations if needed)
  → xAuth: exchange consumer key/secret + username/password for access token
  → for each feed:
      → fetch + parse RSS
      → for each item: skip if GUID in SQLite
      → POST /api/1.1/bookmarks/add → returns bookmark_id
      → store GUID + bookmark_id in SQLite
  → retention pass:
      → query sent_items WHERE sent_at < now - max_age_days
      → for each aged item:
          → if bookmark_id NOT NULL: POST /api/1.1/bookmarks/archive
          → DELETE row from SQLite (regardless of archive result)
  → exit 0
```

Sequential. No concurrency.

### Migration note

Items in SQLite from before this change have `bookmark_id = NULL`. On retention they are deleted from local state only — no archive call is made. They remain in Instapaper unarchived.

---

## API: Instapaper Full API (xAuth)

### Authentication

```
POST https://www.instapaper.com/api/1/oauth/access_token
Content-Type: application/x-www-form-urlencoded

Signed with OAuth 1.0a HMAC-SHA1 using consumer key + secret.
Body params: x_auth_username, x_auth_password, x_auth_mode=client_auth
Response: oauth_token=...&oauth_token_secret=...
```

Token is fetched fresh each run. No persistence needed.

### Add bookmark

```
POST https://www.instapaper.com/api/1.1/bookmarks/add
Signed with OAuth 1.0a using consumer key/secret + access token/secret.
Body: url, title (optional)
Response: JSON array, first element is the bookmark object with "bookmark_id"
Success: HTTP 200
```

### Archive bookmark

```
POST https://www.instapaper.com/api/1.1/bookmarks/archive
Body: bookmark_id
Response: JSON array with the updated bookmark
Success: HTTP 200
```

### OAuth 1.0a signing (HMAC-SHA1)

Required parameters on every signed request:
- `oauth_consumer_key`
- `oauth_token` (empty string for the xAuth token exchange itself)
- `oauth_signature_method=HMAC-SHA1`
- `oauth_timestamp` (Unix epoch seconds)
- `oauth_nonce` (random hex string)
- `oauth_version=1.0`
- `oauth_signature` (computed)

Signing key: `percent_encode(consumer_secret) + "&" + percent_encode(token_secret)`  
Signature base: `METHOD&percent_encode(URL)&percent_encode(sorted_params)`

---

## File Changes

### Modified files

#### `internal/config/config.go`

Add `MaxAgeDays int` field with yaml tag `max_age_days`. If zero or omitted, default to `1` inside `Load()`.

```go
type Config struct {
    MaxAgeDays int    `yaml:"max_age_days"`
    Feeds      []Feed `yaml:"feeds"`
}
// In Load(): if cfg.MaxAgeDays == 0 { cfg.MaxAgeDays = 1 }
```

#### `internal/state/db.go`

- Schema migration: `ALTER TABLE sent_items ADD COLUMN bookmark_id INTEGER` (run only if column absent — use `PRAGMA table_info`)
- New type: `SentItem{ GUID string; BookmarkID *int64; SentAt time.Time }`
- New method: `MarkSentWithID(guid string, bookmarkID int64) error` — replaces `MarkSent` for new items
- Keep `MarkSent(guid string) error` for backward compatibility (inserts with NULL bookmark_id)
- New method: `OldItems(maxAgeDays int) ([]SentItem, error)` — returns rows where `sent_at < datetime('now', '-N days')`
- New method: `DeleteItem(guid string) error`

#### `internal/instapaper/client.go`

- `NewClient(consumerKey, consumerSecret, username, password string) *Client`
- `Authenticate() error` — exchanges credentials for access token via xAuth; stores token on client
- `Add(itemURL, title string) (int64, error)` — posts to `/api/1.1/bookmarks/add`; parses `bookmark_id` from JSON response
- `Archive(bookmarkID int64) error` — posts to `/api/1.1/bookmarks/archive`
- All requests signed via `oauth.go`

#### `cmd/syncer/main.go`

- Read `INSTAPAPER_CONSUMER_KEY` + `INSTAPAPER_CONSUMER_SECRET` (fatal if missing)
- Pass consumer key/secret to `instapaper.NewClient`
- Call `client.Authenticate()` after DB open (fatal on failure)
- Store `bookmarkID` from `Add()` via `db.MarkSentWithID()`
- Add retention pass after feed loop

### New files

#### `internal/instapaper/oauth.go`

Pure function: `sign(method, rawURL string, params url.Values, consumerKey, consumerSecret, token, tokenSecret, nonce, timestamp string) string`

Returns the `oauth_signature` value. Nonce and timestamp are passed in (not generated here) so the function is purely deterministic and testable with fixed inputs. All OAuth parameter assembly and nonce/timestamp generation happen in `client.go`; this file only computes the HMAC-SHA1 signature.

---

## Configuration

### `config.yaml`

```yaml
max_age_days: 1   # optional — defaults to 1 if omitted

feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
```

### Credentials (k8s Secret — add two new keys)

```
INSTAPAPER_CONSUMER_KEY=<from Instapaper>
INSTAPAPER_CONSUMER_SECRET=<from Instapaper>
INSTAPAPER_USERNAME=<email>
INSTAPAPER_PASSWORD=<password>
```

Update `manifests/secret.yaml` template to include the two new keys.

---

## Error Handling

| Scenario | Behaviour |
|---|---|
| xAuth fails | Fatal exit (code 1) — can't sync or archive without valid token |
| `Add` to full API fails | Log + skip item, don't store GUID (will retry next run) |
| `Archive` fails for aged item | Log error, still delete from SQLite (article stays in Instapaper; no retry) |
| SQLite migration fails | Fatal exit (code 1) |
| `max_age_days` missing from config | Default to `1` (not an error) |

---

## Out of Scope

- Per-feed retention periods
- Delete (as opposed to archive)
- Retry logic for failed archive calls
- Backfilling `bookmark_id` for items already in SQLite
