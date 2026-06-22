# Archive Retention — Design Spec

**Date:** 2026-06-22
**Status:** Approved
**Branch:** 1-auto-cleanup-of-archived-items

---

## Overview

Extend the existing age-based retention to a two-step lifecycle: articles are first archived in Instapaper (current behaviour), then permanently deleted from Instapaper after a configurable `archive_retention_days` period. Prevents the Instapaper archive from growing unboundedly.

**Current lifecycle:**
```
sent → (max_age_days) → archived in Instapaper + deleted from SQLite
```

**New lifecycle:**
```
sent → (max_age_days) → archived in Instapaper + archived_at set in SQLite
                      → (archive_retention_days) → deleted from Instapaper + deleted from SQLite
```

When `archive_retention_days` is `0` (the default), the delete pass is skipped and behaviour is identical to before.

---

## Architecture

### Execution flow (updated)

```
start
  → load config (max_age_days, archive_retention_days)
  → open SQLite (run migrations)
  → xAuth: exchange credentials for access token
  → feed sync loop (unchanged)
  → archive pass:
      → query sent_items WHERE sent_at < now - max_age_days AND archived_at IS NULL
      → for each item:
          → if bookmark_id NOT NULL: POST /api/1.1/bookmarks/archive → log error on failure
          → MarkArchived(guid) — sets archived_at; always runs regardless of archive result
          → exception: if bookmark_id IS NULL, DeleteItem(guid) instead (pre-migration rows)
  → delete pass (only if archive_retention_days > 0):
      → query sent_items WHERE archived_at IS NOT NULL AND archived_at < now - archive_retention_days
      → for each item:
          → POST /api/1.1/bookmarks/delete → log error on failure
          → DeleteItem(guid) — always runs regardless of delete result
  → log "sync complete: %d added, %d archived, %d deleted"
  → exit 0
```

### Migration note

The `OldItems` query must be updated to exclude already-archived rows (`WHERE archived_at IS NULL`) so items don't cycle through the archive pass a second time.

---

## File Changes

### `internal/config/config.go`

Add `ArchiveRetentionDays int` with yaml tag `archive_retention_days`. Default `0` (disabled — no Instapaper deletes).

```go
type Config struct {
    MaxAgeDays           int    `yaml:"max_age_days"`
    ArchiveRetentionDays int    `yaml:"archive_retention_days"`
    SortByDate           *bool  `yaml:"sort_by_date"`
    Feeds                []Feed `yaml:"feeds"`
}
// No default needed — 0 means "disabled"
```

### `internal/state/db.go`

**Schema migration:** `ALTER TABLE sent_items ADD COLUMN archived_at DATETIME` — ignore "duplicate column name" error (same pattern as `bookmark_id`).

**Update `OldItems`:** add `AND archived_at IS NULL` so already-archived rows don't re-enter the archive pass.

**New methods:**

```go
// MarkArchived sets archived_at to now for the given guid.
func (db *DB) MarkArchived(guid string) error

// ArchivedItems returns rows where archived_at is set and older than retentionDays.
func (db *DB) ArchivedItems(retentionDays int) ([]SentItem, error)
```

### `internal/instapaper/client.go`

New method:

```go
// Delete permanently removes a bookmark from Instapaper.
func (c *Client) Delete(bookmarkID int64) error
```

POST `/api/1.1/bookmarks/delete` with `bookmark_id=N`. Success: HTTP 200.

### `cmd/syncer/main.go`

**Archive pass (modified):**
```go
for _, item := range aged {
    if item.BookmarkID != nil {
        log.Printf("archiving %q (bookmark %d)", item.GUID, *item.BookmarkID)
        if err := client.Archive(*item.BookmarkID); err != nil {
            log.Printf("ERROR archive bookmark %d (%s): %v", *item.BookmarkID, item.GUID, err)
        }
        if err := db.MarkArchived(item.GUID); err != nil {
            log.Printf("ERROR mark archived %s: %v", item.GUID, err)
        }
    } else {
        if err := db.DeleteItem(item.GUID); err != nil {
            log.Printf("ERROR delete item %s: %v", item.GUID, err)
        }
    }
    archived++
}
```

**Delete pass (new):**
```go
var deleted int
if cfg.ArchiveRetentionDays > 0 {
    toDelete, err := db.ArchivedItems(cfg.ArchiveRetentionDays)
    if err != nil {
        log.Printf("ERROR query archived items: %v", err)
    } else {
        for _, item := range toDelete {
            if item.BookmarkID != nil {
                log.Printf("deleting %q (bookmark %d)", item.GUID, *item.BookmarkID)
                if err := client.Delete(*item.BookmarkID); err != nil {
                    log.Printf("ERROR delete bookmark %d (%s): %v", *item.BookmarkID, item.GUID, err)
                }
            }
            if err := db.DeleteItem(item.GUID); err != nil {
                log.Printf("ERROR delete item %s: %v", item.GUID, err)
            }
            deleted++
        }
    }
}
log.Printf("sync complete: %d added, %d archived, %d deleted", added, archived, deleted)
```

---

## Configuration

### `config.yaml` / `config.example.yaml`

```yaml
max_age_days: 1
archive_retention_days: 30  # optional — 0 or omitted = disabled (no Instapaper deletes)

feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
```

---

## Error Handling

| Scenario | Behaviour |
|---|---|
| `Archive` fails for aged item | Log error, still call `MarkArchived` (item enters delete pass next run) |
| `Delete` fails for archived item | Log error, still call `DeleteItem` (removed from SQLite, won't retry) |
| `archive_retention_days` = 0 or omitted | Delete pass skipped entirely |
| Schema migration fails (not duplicate) | Fatal exit |

---

## Out of Scope

- Per-feed `archive_retention_days`
- Retry logic for failed delete calls
- Items previously processed by the old archive pass were already deleted from SQLite; they don't exist in the DB and are unaffected by this change
