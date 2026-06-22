# Archive Retention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a two-step archive retention lifecycle — articles are archived in Instapaper after `max_age_days`, then permanently deleted from Instapaper after `archive_retention_days` more days.

**Architecture:** Add `archive_retention_days` config field (0 = disabled). Add `archived_at` column to SQLite — the archive pass now sets this instead of deleting the row. A new delete pass queries items with an old `archived_at` and calls `/api/1.1/bookmarks/delete` before removing from SQLite.

**Tech Stack:** Go 1.23, `database/sql`, `github.com/mattn/go-sqlite3`, `net/http`, `encoding/json`

## Global Constraints

- Module: `github.com/danielgroothuis/rss-feed-to-instapaper`
- CGO required — never `CGO_ENABLED=0` (mattn/go-sqlite3)
- Test command: `go test ./...` from repo root
- No new dependencies
- `client_test.go`: `package instapaper` (same-package; injects test server via `c.baseURL`)
- `db_test.go`: `package state_test` (external — do not change its package)
- `db_internal_test.go`: `package state` (internal — needed for direct `db.conn` access)
- Bookmark delete endpoint: `POST /api/1.1/bookmarks/delete` → success = HTTP 200
- `archive_retention_days` default: 0 (disabled — delete pass skipped)

---

## File Map

| File | Change |
|---|---|
| `internal/config/config.go` | Add `ArchiveRetentionDays int` field |
| `internal/config/config_test.go` | Tests for `archive_retention_days` |
| `internal/state/db.go` | Add `archived_at` migration, update `OldItems`, add `MarkArchived` + `ArchivedItems` |
| `internal/state/db_test.go` | External tests for `MarkArchived` |
| `internal/state/db_internal_test.go` | Tests for `ArchivedItems` + `OldItems` skips archived items |
| `internal/instapaper/client.go` | Add `Delete` method |
| `internal/instapaper/client_test.go` | Tests for `Delete` |
| `cmd/syncer/main.go` | Modify archive pass, add delete pass, update summary log |
| `config.example.yaml` | Add commented `archive_retention_days` |

---

### Task 1: Config — `ArchiveRetentionDays` field

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config.ArchiveRetentionDays int` — consumed by Task 4
- `0` means disabled (delete pass skipped); no non-zero default

- [ ] **Step 1: Write failing tests**

Add to `internal/config/config_test.go` (after `TestLoad_sort_by_date_explicit_true`):

```go
func TestLoad_archive_retention_days_default_zero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("feeds:\n  - url: https://example.com/feed.xml\n    label: Example\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ArchiveRetentionDays != 0 {
		t.Errorf("ArchiveRetentionDays: got %d, want 0", cfg.ArchiveRetentionDays)
	}
}

func TestLoad_archive_retention_days_explicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("archive_retention_days: 30\nfeeds:\n  - url: https://example.com/feed.xml\n    label: Example\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ArchiveRetentionDays != 30 {
		t.Errorf("ArchiveRetentionDays: got %d, want 30", cfg.ArchiveRetentionDays)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/config/... -run TestLoad_archive_retention -v
```

Expected: compile error — `cfg.ArchiveRetentionDays undefined`

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add `ArchiveRetentionDays` to the `Config` struct:

```go
type Config struct {
	MaxAgeDays           int    `yaml:"max_age_days"`
	ArchiveRetentionDays int    `yaml:"archive_retention_days"`
	SortByDate           *bool  `yaml:"sort_by_date"`
	Feeds                []Feed `yaml:"feeds"`
}
```

No default needed — zero value (0) is the correct default (disabled).

- [ ] **Step 4: Run all config tests**

```bash
go test ./internal/config/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config)[1]: add ArchiveRetentionDays field"
```

---

### Task 2: State — `archived_at` column, `MarkArchived`, `ArchivedItems`, update `OldItems`

**Files:**
- Modify: `internal/state/db.go`
- Modify: `internal/state/db_test.go`
- Modify: `internal/state/db_internal_test.go`

**Interfaces:**
- Produces:
  - `func (db *DB) MarkArchived(guid string) error`
  - `func (db *DB) ArchivedItems(retentionDays int) ([]SentItem, error)` — returns `SentItem` (same type as `OldItems`)
- Modifies: `OldItems` — adds `AND archived_at IS NULL` filter so already-archived rows are excluded
- Existing `SentItem`, `IsSent`, `MarkSent`, `MarkSentWithID`, `DeleteItem`, `Close` — unchanged

- [ ] **Step 1: Write failing tests**

Add to `internal/state/db_test.go` (after `TestDB_DeleteItem_nonexistent_is_ok`):

```go
func TestDB_MarkArchived_sets_archived_at(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.MarkSentWithID("to-archive", 99); err != nil {
		t.Fatalf("MarkSentWithID: %v", err)
	}
	if err := db.MarkArchived("to-archive"); err != nil {
		t.Fatalf("MarkArchived: %v", err)
	}

	// Item still exists in state.
	sent, err := db.IsSent("to-archive")
	if err != nil {
		t.Fatalf("IsSent: %v", err)
	}
	if !sent {
		t.Error("expected item still present after MarkArchived")
	}
}

func TestDB_MarkArchived_nonexistent_is_ok(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.MarkArchived("ghost"); err != nil {
		t.Fatalf("MarkArchived on nonexistent guid: %v", err)
	}
}
```

Add to `internal/state/db_internal_test.go` (after `TestDB_Open_migrates_existing_db`):

```go
func TestDB_OldItems_skips_archived_items(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Insert an old item that has already been archived.
	if _, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id, sent_at, archived_at) VALUES (?, ?, ?, ?)`,
		"already-archived", int64(10), "2020-01-01 00:00:00", "2020-01-02 00:00:00",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	items, err := db.OldItems(1)
	if err != nil {
		t.Fatalf("OldItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (archived item must be excluded)", len(items))
	}
}

func TestDB_ArchivedItems_returns_old_archived_items(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	bookmarkID := int64(77)
	// Insert an item that was archived long ago.
	if _, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id, sent_at, archived_at) VALUES (?, ?, ?, ?)`,
		"old-archived", bookmarkID, "2020-01-01 00:00:00", "2020-01-02 00:00:00",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	items, err := db.ArchivedItems(30)
	if err != nil {
		t.Fatalf("ArchivedItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].GUID != "old-archived" {
		t.Errorf("GUID: got %q", items[0].GUID)
	}
	if items[0].BookmarkID == nil || *items[0].BookmarkID != bookmarkID {
		t.Errorf("BookmarkID: got %v, want %d", items[0].BookmarkID, bookmarkID)
	}
}

func TestDB_ArchivedItems_skips_recently_archived(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// MarkArchived uses CURRENT_TIMESTAMP — item is too new to appear.
	if _, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id, sent_at) VALUES (?, ?, ?)`,
		"fresh", int64(1), "2020-01-01 00:00:00",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.MarkArchived("fresh"); err != nil {
		t.Fatalf("MarkArchived: %v", err)
	}

	items, err := db.ArchivedItems(1)
	if err != nil {
		t.Fatalf("ArchivedItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (recently archived must not be returned)", len(items))
	}
}

func TestDB_ArchivedItems_skips_non_archived_items(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Old item with no archived_at — should not appear in ArchivedItems.
	if _, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id, sent_at) VALUES (?, ?, ?)`,
		"not-archived", int64(2), "2020-01-01 00:00:00",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	items, err := db.ArchivedItems(1)
	if err != nil {
		t.Fatalf("ArchivedItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (non-archived item must not appear)", len(items))
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/state/... -v
```

Expected: compile errors — `MarkArchived` and `ArchivedItems` undefined; `TestDB_OldItems_skips_archived_items` may fail at runtime once methods exist

- [ ] **Step 3: Implement**

Replace `internal/state/db.go` with:

```go
package state

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type SentItem struct {
	GUID       string
	BookmarkID *int64
	SentAt     time.Time
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS sent_items (
		guid    TEXT PRIMARY KEY,
		sent_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	if _, err := conn.Exec(`ALTER TABLE sent_items ADD COLUMN bookmark_id INTEGER`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			conn.Close()
			return nil, fmt.Errorf("migrate schema (bookmark_id): %w", err)
		}
	}
	if _, err := conn.Exec(`ALTER TABLE sent_items ADD COLUMN archived_at DATETIME`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			conn.Close()
			return nil, fmt.Errorf("migrate schema (archived_at): %w", err)
		}
	}
	return &DB{conn: conn}, nil
}

func (db *DB) IsSent(guid string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM sent_items WHERE guid = ?`, guid).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query sent: %w", err)
	}
	return count > 0, nil
}

func (db *DB) MarkSent(guid string) error {
	_, err := db.conn.Exec(`INSERT OR IGNORE INTO sent_items (guid) VALUES (?)`, guid)
	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}
	return nil
}

func (db *DB) MarkSentWithID(guid string, bookmarkID int64) error {
	_, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id) VALUES (?, ?)
     ON CONFLICT(guid) DO UPDATE SET bookmark_id = excluded.bookmark_id`,
		guid, bookmarkID,
	)
	if err != nil {
		return fmt.Errorf("mark sent with id: %w", err)
	}
	return nil
}

func (db *DB) MarkArchived(guid string) error {
	_, err := db.conn.Exec(
		`UPDATE sent_items SET archived_at = CURRENT_TIMESTAMP WHERE guid = ?`,
		guid,
	)
	if err != nil {
		return fmt.Errorf("mark archived: %w", err)
	}
	return nil
}

func (db *DB) OldItems(maxAgeDays int) ([]SentItem, error) {
	rows, err := db.conn.Query(
		`SELECT guid, bookmark_id, sent_at FROM sent_items
		 WHERE sent_at < datetime('now', ?) AND archived_at IS NULL`,
		fmt.Sprintf("-%d days", maxAgeDays),
	)
	if err != nil {
		return nil, fmt.Errorf("query old items: %w", err)
	}
	defer rows.Close()

	var items []SentItem
	for rows.Next() {
		var item SentItem
		var sentAt string
		if err := rows.Scan(&item.GUID, &item.BookmarkID, &sentAt); err != nil {
			return nil, fmt.Errorf("scan old item: %w", err)
		}
		t, err := time.Parse("2006-01-02 15:04:05", sentAt)
		if err != nil {
			t, err = time.Parse(time.RFC3339, sentAt)
		}
		if err != nil {
			return nil, fmt.Errorf("parse sent_at %q: %w", sentAt, err)
		}
		item.SentAt = t
		items = append(items, item)
	}
	return items, rows.Err()
}

func (db *DB) ArchivedItems(retentionDays int) ([]SentItem, error) {
	rows, err := db.conn.Query(
		`SELECT guid, bookmark_id, sent_at FROM sent_items
		 WHERE archived_at IS NOT NULL AND archived_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", retentionDays),
	)
	if err != nil {
		return nil, fmt.Errorf("query archived items: %w", err)
	}
	defer rows.Close()

	var items []SentItem
	for rows.Next() {
		var item SentItem
		var sentAt string
		if err := rows.Scan(&item.GUID, &item.BookmarkID, &sentAt); err != nil {
			return nil, fmt.Errorf("scan archived item: %w", err)
		}
		t, err := time.Parse("2006-01-02 15:04:05", sentAt)
		if err != nil {
			t, err = time.Parse(time.RFC3339, sentAt)
		}
		if err != nil {
			return nil, fmt.Errorf("parse sent_at %q: %w", sentAt, err)
		}
		item.SentAt = t
		items = append(items, item)
	}
	return items, rows.Err()
}

func (db *DB) DeleteItem(guid string) error {
	_, err := db.conn.Exec(`DELETE FROM sent_items WHERE guid = ?`, guid)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}
```

- [ ] **Step 4: Run all state tests**

```bash
go test ./internal/state/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/db.go internal/state/db_test.go internal/state/db_internal_test.go
git commit -m "feat(state)[1]: add archived_at column, MarkArchived, ArchivedItems; filter OldItems"
```

---

### Task 3: Instapaper client — `Delete` method

**Files:**
- Modify: `internal/instapaper/client.go`
- Modify: `internal/instapaper/client_test.go`

**Interfaces:**
- Produces: `func (c *Client) Delete(bookmarkID int64) error`
- Endpoint: `POST /api/1.1/bookmarks/delete`, body param `bookmark_id=<id>`, success = HTTP 200
- All existing methods (`Authenticate`, `Add`, `Archive`) — unchanged

- [ ] **Step 1: Write failing tests**

Add to `internal/instapaper/client_test.go` (after `TestClient_Archive_non_200_returns_error`):

```go
func TestClient_Delete_sends_bookmark_id(t *testing.T) {
	var gotBookmarkID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1.1/bookmarks/delete" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_ = r.ParseForm()
		gotBookmarkID = r.FormValue("bookmark_id")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]map[string]interface{}{{"result": "success"}})
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Delete(123); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotBookmarkID != "123" {
		t.Errorf("bookmark_id: got %q, want \"123\"", gotBookmarkID)
	}
}

func TestClient_Delete_non_200_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Delete(1); err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/instapaper/... -run TestClient_Delete -v
```

Expected: compile error — `Delete` undefined

- [ ] **Step 3: Implement**

Add `Delete` to `internal/instapaper/client.go` (after the `Archive` method, before `signedPost`):

```go
func (c *Client) Delete(bookmarkID int64) error {
	params := url.Values{"bookmark_id": {strconv.FormatInt(bookmarkID, 10)}}
	resp, err := c.signedPost("/api/1.1/bookmarks/delete", params)
	if err != nil {
		return fmt.Errorf("delete bookmark %d: %w", bookmarkID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete bookmark: instapaper returned %d for id %d", resp.StatusCode, bookmarkID)
	}
	return nil
}
```

- [ ] **Step 4: Run all instapaper tests**

```bash
go test ./internal/instapaper/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/instapaper/client.go internal/instapaper/client_test.go
git commit -m "feat(instapaper)[1]: add Delete method"
```

---

### Task 4: Main orchestration + example config

**Files:**
- Modify: `cmd/syncer/main.go`
- Modify: `config.example.yaml`

**Interfaces:**
- Consumes:
  - `config.Config.ArchiveRetentionDays int` — Task 1
  - `state.DB.MarkArchived(guid string) error` — Task 2
  - `state.DB.ArchivedItems(retentionDays int) ([]SentItem, error)` — Task 2
  - `instapaper.Client.Delete(bookmarkID int64) error` — Task 3
- Existing interfaces (`OldItems`, `Archive`, `DeleteItem`, etc.) — unchanged signatures

No new tests needed for `main.go` — logic is covered by unit tests in Tasks 1–3.

- [ ] **Step 1: Baseline check**

```bash
go test ./...
```

Expected: all PASS before any changes

- [ ] **Step 2: Modify `cmd/syncer/main.go`**

Replace the archive pass and summary log. The full updated file:

```go
package main

import (
	"log"
	"os"
	"sort"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/config"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/feed"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/instapaper"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/state"
)

type pendingItem struct {
	item      feed.Item
	feedLabel string
}

func sortPending(items []pendingItem) {
	sort.Slice(items, func(i, j int) bool {
		pi, pj := items[i].item.PublishedAt, items[j].item.PublishedAt
		if pi == nil && pj == nil {
			return false
		}
		if pi == nil {
			return true
		}
		if pj == nil {
			return false
		}
		return pi.Before(*pj)
	})
}

func main() {
	configPath := envOr("CONFIG_PATH", "/config/config.yaml")
	statePath := envOr("STATE_PATH", "/data/state.db")
	username := requireEnv("INSTAPAPER_USERNAME")
	password := requireEnv("INSTAPAPER_PASSWORD")
	consumerKey := requireEnv("INSTAPAPER_CONSUMER_KEY")
	consumerSecret := requireEnv("INSTAPAPER_CONSUMER_SECRET")

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := state.Open(statePath)
	if err != nil {
		log.Fatalf("open state db: %v", err)
	}
	defer db.Close()

	client := instapaper.NewClient(consumerKey, consumerSecret, username, password)
	if err := client.Authenticate(); err != nil {
		log.Fatalf("authenticate with instapaper: %v", err)
	}

	var pending []pendingItem
	for _, f := range cfg.Feeds {
		before := len(pending)
		items, err := feed.FetchItems(f.URL)
		if err != nil {
			log.Printf("ERROR fetch feed %s: %v", f.URL, err)
			continue
		}
		for _, item := range items {
			sent, err := db.IsSent(item.GUID)
			if err != nil {
				log.Printf("ERROR check state for %s: %v", item.GUID, err)
				continue
			}
			if sent {
				continue
			}
			pending = append(pending, pendingItem{item: item, feedLabel: f.Label})
		}
		log.Printf("[%s] %d new articles", f.Label, len(pending)-before)
	}

	if *cfg.SortByDate {
		sortPending(pending)
	}

	var added int
	for _, p := range pending {
		log.Printf("adding [%s] %q", p.feedLabel, p.item.Title)
		bookmarkID, err := client.Add(p.item.URL, p.item.Title)
		if err != nil {
			log.Printf("ERROR add to instapaper %s: %v", p.item.URL, err)
			continue
		}
		if err := db.MarkSentWithID(p.item.GUID, bookmarkID); err != nil {
			log.Printf("ERROR mark sent %s: %v", p.item.GUID, err)
		}
		added++
	}

	var archived int
	aged, err := db.OldItems(cfg.MaxAgeDays)
	if err != nil {
		log.Printf("ERROR query old items: %v", err)
	} else {
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
	}

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
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: Update `config.example.yaml`**

Replace with:

```yaml
max_age_days: 1
# archive_retention_days: 30  # uncomment to delete archived articles from Instapaper after N days

feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
  - url: "https://another-blog.com/rss"
    label: "Another Blog"
```

- [ ] **Step 4: Build**

```bash
go build ./cmd/syncer/
```

Expected: no output, binary `syncer` created in repo root

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/syncer/main.go config.example.yaml
git commit -m "feat(syncer)[1]: add archive retention pass, delete from Instapaper after archive_retention_days"
```

---

## Self-Review

**Spec coverage:**

| Requirement | Task |
|---|---|
| `archive_retention_days` config field, default 0 | Task 1 ✓ |
| `archived_at DATETIME` column migration | Task 2 ✓ |
| `MarkArchived(guid) error` | Task 2 ✓ |
| `ArchivedItems(retentionDays int) ([]SentItem, error)` | Task 2 ✓ |
| `OldItems` excludes already-archived rows | Task 2 ✓ |
| `Delete(bookmarkID int64) error` on instapaper client | Task 3 ✓ |
| Archive pass: items with bookmark_id → Archive + MarkArchived | Task 4 ✓ |
| Archive pass: items without bookmark_id → DeleteItem (pre-migration) | Task 4 ✓ |
| Delete pass: only runs if `archive_retention_days > 0` | Task 4 ✓ |
| Delete pass: calls `Delete` (log error), then `DeleteItem` always | Task 4 ✓ |
| Summary log: `%d added, %d archived, %d deleted` | Task 4 ✓ |
| `config.example.yaml` updated | Task 4 ✓ |

**Placeholder scan:** None.

**Type consistency:**
- `ArchivedItems` returns `[]SentItem` — same type as `OldItems` — used identically in Task 4 ✓
- `MarkArchived(guid string) error` — defined Task 2, called Task 4 ✓
- `Delete(bookmarkID int64) error` — defined Task 3, called with `*item.BookmarkID` (int64) in Task 4 ✓
- `cfg.ArchiveRetentionDays int` — defined Task 1, read in Task 4 ✓
