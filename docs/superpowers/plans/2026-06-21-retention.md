# Retention Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add age-based article retention — archive old Instapaper bookmarks and purge them from SQLite state on every sync run.

**Architecture:** Switch from Simple API to full xAuth API (OAuth 1.0a HMAC-SHA1). On each run: xAuth authenticate, sync feeds (store bookmark_id), then archive+delete items older than `max_age_days`. Signing lives in a pure stdlib function in its own file; the client owns nonce/timestamp generation.

**Tech Stack:** Go 1.23, `crypto/hmac`, `crypto/sha1`, `encoding/base64`, `net/http`, `encoding/json`, `database/sql`, `github.com/mattn/go-sqlite3`

## Global Constraints

- Module: `github.com/danielgroothuis/rss-feed-to-instapaper`
- CGO required — never `CGO_ENABLED=0` (mattn/go-sqlite3)
- Test command: `go test ./...` from repo root
- No new dependencies — stdlib only for OAuth (no external OAuth library)
- `client.go` and `client_test.go`: `package instapaper` (same-package; tests inject test server via `c.baseURL`)
- `db_test.go`: `package state_test` (existing external test file — do not change its package)
- xAuth endpoint: `POST /api/1/oauth/access_token` → response: form-encoded `oauth_token=...&oauth_token_secret=...`
- Bookmark add endpoint: `POST /api/1.1/bookmarks/add` → response: JSON array, success = HTTP 200
- Bookmark archive endpoint: `POST /api/1.1/bookmarks/archive` → response: JSON array, success = HTTP 200
- **Not 201** — the old Simple API returned 201; the full API returns 200
- `max_age_days` default: 1 when zero or omitted

---

### Task 1: Config — MaxAgeDays field

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config.MaxAgeDays int` — consumed by Task 5

- [ ] **Step 1: Write failing tests**

Add to `internal/config/config_test.go` (after the existing tests):

```go
func TestLoad_max_age_days_default(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("feeds:\n  - url: https://example.com/feed.xml\n    label: Example\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxAgeDays != 1 {
		t.Errorf("MaxAgeDays: got %d, want 1", cfg.MaxAgeDays)
	}
}

func TestLoad_max_age_days_explicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("max_age_days: 7\nfeeds:\n  - url: https://example.com/feed.xml\n    label: Example\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxAgeDays != 7 {
		t.Errorf("MaxAgeDays: got %d, want 7", cfg.MaxAgeDays)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/config/... -run TestLoad_max_age -v
```

Expected: FAIL — `MaxAgeDays: got 0, want 1` (field undefined = compile error is also fine)

- [ ] **Step 3: Implement**

Replace `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Feed struct {
	URL   string `yaml:"url"`
	Label string `yaml:"label"`
}

type Config struct {
	MaxAgeDays int    `yaml:"max_age_days"`
	Feeds      []Feed `yaml:"feeds"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Feeds) == 0 {
		return nil, fmt.Errorf("config has no feeds")
	}
	for i, f := range cfg.Feeds {
		if f.URL == "" {
			return nil, fmt.Errorf("feed %d missing url", i)
		}
	}
	if cfg.MaxAgeDays == 0 {
		cfg.MaxAgeDays = 1
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Run all config tests**

```bash
go test ./internal/config/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add MaxAgeDays field, default 1"
```

---

### Task 2: State — bookmark_id column and new methods

**Files:**
- Modify: `internal/state/db.go`
- Modify: `internal/state/db_test.go` (add `MarkSentWithID` + `DeleteItem` external tests)
- Create: `internal/state/db_internal_test.go` (`package state` — needs direct DB access to set past timestamps for OldItems tests)

**Interfaces:**
- Produces:
  - `type SentItem struct { GUID string; BookmarkID *int64; SentAt time.Time }`
  - `func (db *DB) MarkSentWithID(guid string, bookmarkID int64) error`
  - `func (db *DB) OldItems(maxAgeDays int) ([]SentItem, error)`
  - `func (db *DB) DeleteItem(guid string) error`
- Existing `IsSent`, `MarkSent`, `Close` — unchanged

- [ ] **Step 1: Write failing tests**

Add to `internal/state/db_test.go` (after existing tests):

```go
func TestDB_MarkSentWithID_tracks_guid(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.MarkSentWithID("guid-with-id", 12345); err != nil {
		t.Fatalf("MarkSentWithID: %v", err)
	}

	sent, err := db.IsSent("guid-with-id")
	if err != nil {
		t.Fatalf("IsSent: %v", err)
	}
	if !sent {
		t.Error("expected guid tracked after MarkSentWithID")
	}
}

func TestDB_MarkSentWithID_idempotent(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.MarkSentWithID("g", 1); err != nil {
		t.Fatalf("first MarkSentWithID: %v", err)
	}
	if err := db.MarkSentWithID("g", 1); err != nil {
		t.Fatalf("second MarkSentWithID should be idempotent: %v", err)
	}
}

func TestDB_DeleteItem(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.MarkSent("to-delete"); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if err := db.DeleteItem("to-delete"); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}

	sent, err := db.IsSent("to-delete")
	if err != nil {
		t.Fatalf("IsSent: %v", err)
	}
	if sent {
		t.Error("expected item gone after DeleteItem")
	}
}

func TestDB_DeleteItem_nonexistent_is_ok(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.DeleteItem("does-not-exist"); err != nil {
		t.Fatalf("DeleteItem on nonexistent guid: %v", err)
	}
}
```

Create `internal/state/db_internal_test.go` (uses `package state` to access `db.conn` for inserting past timestamps):

```go
package state

import (
	"testing"
)

func TestDB_OldItems_returns_aged_items(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	bookmarkID := int64(42)
	// Insert directly with a past timestamp to bypass timing sensitivity.
	if _, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id, sent_at) VALUES (?, ?, ?)`,
		"old-guid", bookmarkID, "2020-01-01 00:00:00",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	items, err := db.OldItems(1)
	if err != nil {
		t.Fatalf("OldItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].GUID != "old-guid" {
		t.Errorf("GUID: got %q", items[0].GUID)
	}
	if items[0].BookmarkID == nil || *items[0].BookmarkID != bookmarkID {
		t.Errorf("BookmarkID: got %v, want %d", items[0].BookmarkID, bookmarkID)
	}
}

func TestDB_OldItems_nil_bookmark_id_for_pre_migration_rows(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Simulate a pre-retention row (no bookmark_id).
	if _, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, sent_at) VALUES (?, ?)`,
		"old-no-id", "2020-01-01 00:00:00",
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	items, err := db.OldItems(1)
	if err != nil {
		t.Fatalf("OldItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].BookmarkID != nil {
		t.Errorf("BookmarkID: expected nil, got %v", items[0].BookmarkID)
	}
}

func TestDB_OldItems_skips_recent_items(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.MarkSent("recent"); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	items, err := db.OldItems(1) // 1 day threshold — just-inserted item is not old
	if err != nil {
		t.Fatalf("OldItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 (recent item must not be returned)", len(items))
	}
}

func TestDB_Open_migrates_existing_db(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/state.db"

	// First open: create table without bookmark_id (simulate pre-retention DB).
	db, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := db.MarkSent("pre-migration"); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	db.Close()

	// Second open: migration should add bookmark_id column without error.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second open (migration): %v", err)
	}
	defer db2.Close()

	// Insert directly with past timestamp via the now-migrated column.
	if _, err := db2.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id, sent_at) VALUES (?, ?, ?)`,
		"post-migration", int64(99), "2020-01-01 00:00:00",
	); err != nil {
		t.Fatalf("insert after migration: %v", err)
	}

	items, err := db2.OldItems(1)
	if err != nil {
		t.Fatalf("OldItems after migration: %v", err)
	}
	// Should return both: the pre-migration row (no bookmark_id) and the new one.
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/state/... -v
```

Expected: compile errors — `MarkSentWithID`, `OldItems`, `DeleteItem`, `SentItem` undefined

- [ ] **Step 3: Implement**

Replace `internal/state/db.go`:

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
	// Migrate: add bookmark_id column absent from pre-retention DBs.
	if _, err := conn.Exec(`ALTER TABLE sent_items ADD COLUMN bookmark_id INTEGER`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			conn.Close()
			return nil, fmt.Errorf("migrate schema: %w", err)
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
		`INSERT OR IGNORE INTO sent_items (guid, bookmark_id) VALUES (?, ?)`,
		guid, bookmarkID,
	)
	if err != nil {
		return fmt.Errorf("mark sent with id: %w", err)
	}
	return nil
}

func (db *DB) OldItems(maxAgeDays int) ([]SentItem, error) {
	rows, err := db.conn.Query(
		`SELECT guid, bookmark_id, sent_at FROM sent_items WHERE sent_at < datetime('now', ?)`,
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
		item.SentAt, _ = time.Parse("2006-01-02 15:04:05", sentAt)
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

Expected: all PASS (including pre-existing IsSent/MarkSent/Open tests)

- [ ] **Step 5: Commit**

```bash
git add internal/state/db.go internal/state/db_test.go internal/state/db_internal_test.go
git commit -m "feat(state): add bookmark_id column, SentItem, OldItems, DeleteItem"
```

---

### Task 3: OAuth 1.0a HMAC-SHA1 signing

**Files:**
- Create: `internal/instapaper/oauth.go`
- Create: `internal/instapaper/oauth_test.go`

**Interfaces:**
- Produces (package-internal): `func sign(method, rawURL string, bodyParams url.Values, consumerKey, consumerSecret, token, tokenSecret, nonce, timestamp string) string`
- `bodyParams` = request-specific params only (url, title, x_auth_*, bookmark_id, etc.). OAuth protocol params (oauth_consumer_key, oauth_nonce, etc.) are assembled internally — must NOT be pre-set in bodyParams.
- `oauth_token` is omitted from the signature base string when `token` is empty string (xAuth initial call has no token yet).

- [ ] **Step 1: Write failing test**

Create `internal/instapaper/oauth_test.go`:

```go
package instapaper

import (
	"net/url"
	"testing"
)

// Test vector from OAuth Core 1.0 spec §A.5.
// Expected base string and signature are specified in the RFC.
func TestSign_known_vector(t *testing.T) {
	bodyParams := url.Values{
		"file": {"vacation.jpg"},
		"size": {"original"},
	}
	got := sign(
		"GET",
		"http://photos.example.net/photos",
		bodyParams,
		"dpf43f3p2l4k3l03", // consumerKey
		"kd94hf93k423kf44", // consumerSecret
		"nnch734d00sl2jdk", // token
		"pfkkdhi9sl3r4s00", // tokenSecret
		"kllo9940pd9333jh", // nonce
		"1191242096",       // timestamp
	)
	want := "tR3+Ty81lMeYAr/Fid0kMTYa/WM="
	if got != want {
		t.Errorf("sign: got %q, want %q", got, want)
	}
}

func TestSign_deterministic(t *testing.T) {
	params := url.Values{"url": {"https://example.com/article"}}
	a := sign("POST", "https://www.instapaper.com/api/1.1/bookmarks/add",
		params, "key", "secret", "tok", "toksecret", "nonce123", "1700000000")
	b := sign("POST", "https://www.instapaper.com/api/1.1/bookmarks/add",
		params, "key", "secret", "tok", "toksecret", "nonce123", "1700000000")
	if a != b {
		t.Errorf("sign not deterministic: %q != %q", a, b)
	}
}

func TestSign_different_secrets_differ(t *testing.T) {
	params := url.Values{"url": {"https://example.com"}}
	a := sign("POST", "https://example.com/api", params, "key", "secretA", "", "", "n", "1")
	b := sign("POST", "https://example.com/api", params, "key", "secretB", "", "", "n", "1")
	if a == b {
		t.Error("different consumer secrets must produce different signatures")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/instapaper/... -run TestSign -v
```

Expected: compile error — `sign` undefined

- [ ] **Step 3: Implement**

Create `internal/instapaper/oauth.go`:

```go
package instapaper

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
)

// sign computes an OAuth 1.0a HMAC-SHA1 signature.
// bodyParams holds request-specific params only (url, title, x_auth_*, etc.).
// OAuth params are assembled here from the explicit arguments.
// oauth_token is omitted when token is empty (xAuth initial exchange).
func sign(method, rawURL string, bodyParams url.Values, consumerKey, consumerSecret, token, tokenSecret, nonce, timestamp string) string {
	all := url.Values{}
	for k, vs := range bodyParams {
		all[k] = vs
	}
	all.Set("oauth_consumer_key", consumerKey)
	all.Set("oauth_nonce", nonce)
	all.Set("oauth_signature_method", "HMAC-SHA1")
	all.Set("oauth_timestamp", timestamp)
	all.Set("oauth_version", "1.0")
	if token != "" {
		all.Set("oauth_token", token)
	}

	var pairs []string
	for k, vs := range all {
		for _, v := range vs {
			pairs = append(pairs, pctEnc(k)+"="+pctEnc(v))
		}
	}
	sort.Strings(pairs)
	paramStr := strings.Join(pairs, "&")

	base := strings.ToUpper(method) + "&" + pctEnc(rawURL) + "&" + pctEnc(paramStr)
	key := pctEnc(consumerSecret) + "&" + pctEnc(tokenSecret)

	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(base))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// pctEnc percent-encodes per RFC 3986 §2.1 (spaces as %20, not +).
func pctEnc(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/instapaper/... -run TestSign -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/instapaper/oauth.go internal/instapaper/oauth_test.go
git commit -m "feat(instapaper): add OAuth 1.0a HMAC-SHA1 signing"
```

---

### Task 4: Instapaper full xAuth API client

**Files:**
- Modify: `internal/instapaper/client.go`
- Modify: `internal/instapaper/client_test.go`

**Interfaces:**
- Consumes: `sign(...)` from `oauth.go` (Task 3)
- Produces:
  - `func NewClient(consumerKey, consumerSecret, username, password string) *Client`
  - `func (c *Client) Authenticate() error` — fetches xAuth token, stores on client
  - `func (c *Client) Add(itemURL, title string) (int64, error)` — returns `bookmark_id`
  - `func (c *Client) Archive(bookmarkID int64) error`

Note: `client_test.go` stays in `package instapaper` (same-package). Tests set `c.baseURL = srv.URL` to inject the httptest server. The `c.accessToken` and `c.accessSecret` fields are also set directly in tests that skip authentication.

- [ ] **Step 1: Write failing tests**

Replace `internal/instapaper/client_test.go`:

```go
package instapaper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Authenticate_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1/oauth/access_token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_ = r.ParseForm()
		if r.FormValue("x_auth_mode") != "client_auth" {
			t.Errorf("x_auth_mode: got %q", r.FormValue("x_auth_mode"))
		}
		if r.FormValue("oauth_consumer_key") == "" {
			t.Error("oauth_consumer_key missing")
		}
		if r.FormValue("oauth_signature") == "" {
			t.Error("oauth_signature missing")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("oauth_token=testtoken&oauth_token_secret=testsecret"))
	}))
	defer srv.Close()

	c := NewClient("ckey", "csecret", "user@example.com", "pass")
	c.baseURL = srv.URL

	if err := c.Authenticate(); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if c.accessToken != "testtoken" {
		t.Errorf("accessToken: got %q, want testtoken", c.accessToken)
	}
	if c.accessSecret != "testsecret" {
		t.Errorf("accessSecret: got %q, want testsecret", c.accessSecret)
	}
}

func TestClient_Authenticate_non_200_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Authenticate(); err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}

func TestClient_Add_returns_bookmark_id(t *testing.T) {
	type bm struct {
		BookmarkID int64 `json:"bookmark_id"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1.1/bookmarks/add" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("url") == "" {
			t.Error("url param missing")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]bm{{BookmarkID: 99999}})
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL
	c.accessToken = "tok"
	c.accessSecret = "toksecret"

	id, err := c.Add("https://example.com/article", "Title")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != 99999 {
		t.Errorf("bookmark_id: got %d, want 99999", id)
	}
}

func TestClient_Add_empty_title_omits_param(t *testing.T) {
	type bm struct {
		BookmarkID int64 `json:"bookmark_id"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("title") != "" {
			t.Errorf("expected no title param, got %q", r.FormValue("title"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]bm{{BookmarkID: 1}})
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if _, err := c.Add("https://example.com/article", ""); err != nil {
		t.Fatalf("Add empty title: %v", err)
	}
}

func TestClient_Add_non_200_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	_, err := c.Add("https://example.com/article", "Title")
	if err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}

func TestClient_Archive_sends_bookmark_id(t *testing.T) {
	var gotBookmarkID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1.1/bookmarks/archive" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		gotBookmarkID = r.FormValue("bookmark_id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{{"bookmark_id": 42}})
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Archive(42); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if gotBookmarkID != "42" {
		t.Errorf("bookmark_id: got %q, want \"42\"", gotBookmarkID)
	}
}

func TestClient_Archive_non_200_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient("k", "s", "u", "p")
	c.baseURL = srv.URL

	if err := c.Archive(1); err == nil {
		t.Fatal("expected error for non-200, got nil")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/instapaper/... -run "TestClient_Authenticate|TestClient_Add_returns|TestClient_Archive" -v
```

Expected: compile errors — new `NewClient` signature, `Authenticate`/`Archive` undefined, `Add` return type mismatch

- [ ] **Step 3: Implement**

Replace `internal/instapaper/client.go`:

```go
package instapaper

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	consumerKey    string
	consumerSecret string
	username       string
	password       string
	accessToken    string
	accessSecret   string
	baseURL        string
	http           *http.Client
}

func NewClient(consumerKey, consumerSecret, username, password string) *Client {
	return &Client{
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		username:       username,
		password:       password,
		baseURL:        "https://www.instapaper.com",
		http:           &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Authenticate() error {
	params := url.Values{
		"x_auth_username": {c.username},
		"x_auth_password": {c.password},
		"x_auth_mode":     {"client_auth"},
	}
	resp, err := c.signedPost("/api/1/oauth/access_token", params)
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authenticate: instapaper returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("authenticate: read response: %w", err)
	}

	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return fmt.Errorf("authenticate: parse response: %w", err)
	}
	c.accessToken = vals.Get("oauth_token")
	c.accessSecret = vals.Get("oauth_token_secret")
	if c.accessToken == "" {
		return fmt.Errorf("authenticate: no oauth_token in response")
	}
	return nil
}

type bookmarkItem struct {
	BookmarkID int64 `json:"bookmark_id"`
}

func (c *Client) Add(itemURL, title string) (int64, error) {
	params := url.Values{"url": {itemURL}}
	if title != "" {
		params.Set("title", title)
	}
	resp, err := c.signedPost("/api/1.1/bookmarks/add", params)
	if err != nil {
		return 0, fmt.Errorf("add bookmark: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("add bookmark: instapaper returned %d for %s", resp.StatusCode, itemURL)
	}

	var items []bookmarkItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return 0, fmt.Errorf("add bookmark: decode response: %w", err)
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("add bookmark: empty response for %s", itemURL)
	}
	return items[0].BookmarkID, nil
}

func (c *Client) Archive(bookmarkID int64) error {
	params := url.Values{"bookmark_id": {strconv.FormatInt(bookmarkID, 10)}}
	resp, err := c.signedPost("/api/1.1/bookmarks/archive", params)
	if err != nil {
		return fmt.Errorf("archive bookmark %d: %w", bookmarkID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("archive bookmark: instapaper returned %d for id %d", resp.StatusCode, bookmarkID)
	}
	return nil
}

func (c *Client) signedPost(endpoint string, params url.Values) (*http.Response, error) {
	nonce := newNonce()
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	sig := sign("POST", c.baseURL+endpoint, params, c.consumerKey, c.consumerSecret, c.accessToken, c.accessSecret, nonce, ts)

	params.Set("oauth_consumer_key", c.consumerKey)
	params.Set("oauth_nonce", nonce)
	params.Set("oauth_signature", sig)
	params.Set("oauth_signature_method", "HMAC-SHA1")
	params.Set("oauth_timestamp", ts)
	params.Set("oauth_version", "1.0")
	if c.accessToken != "" {
		params.Set("oauth_token", c.accessToken)
	}

	return c.http.PostForm(c.baseURL+endpoint, params)
}

func newNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 4: Run all instapaper tests**

```bash
go test ./internal/instapaper/... -v
```

Expected: all PASS (including `TestSign_*` from Task 3)

- [ ] **Step 5: Commit**

```bash
git add internal/instapaper/client.go internal/instapaper/client_test.go
git commit -m "feat(instapaper): switch to full xAuth API, Add returns bookmark_id, add Archive"
```

---

### Task 5: Main orchestration + manifest update

**Files:**
- Modify: `cmd/syncer/main.go`
- Modify: `manifests/secret.yaml`

**Interfaces:**
- Consumes:
  - `config.Config.MaxAgeDays int` — Task 1
  - `state.SentItem` with `BookmarkID *int64` — Task 2
  - `state.DB.MarkSentWithID(guid string, bookmarkID int64) error` — Task 2
  - `state.DB.OldItems(maxAgeDays int) ([]state.SentItem, error)` — Task 2
  - `state.DB.DeleteItem(guid string) error` — Task 2
  - `instapaper.NewClient(consumerKey, consumerSecret, username, password string) *instapaper.Client` — Task 4
  - `instapaper.Client.Authenticate() error` — Task 4
  - `instapaper.Client.Add(itemURL, title string) (int64, error)` — Task 4
  - `instapaper.Client.Archive(bookmarkID int64) error` — Task 4

No new tests needed for main.go (integration tested by `go build`; the logic is covered by unit tests in tasks 1–4). The manifest change is a template update — credentials go in the cluster, not in this file.

- [ ] **Step 1: Baseline check**

```bash
go test ./...
```

Expected: all PASS before any changes

- [ ] **Step 2: Implement main.go**

Replace `cmd/syncer/main.go`:

```go
package main

import (
	"log"
	"os"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/config"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/feed"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/instapaper"
	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/state"
)

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

	for _, f := range cfg.Feeds {
		log.Printf("processing feed: %s (%s)", f.Label, f.URL)
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

			bookmarkID, err := client.Add(item.URL, item.Title)
			if err != nil {
				log.Printf("ERROR add to instapaper %s: %v", item.URL, err)
				continue
			}

			if err := db.MarkSentWithID(item.GUID, bookmarkID); err != nil {
				log.Printf("ERROR mark sent %s: %v", item.GUID, err)
			}
		}
	}

	aged, err := db.OldItems(cfg.MaxAgeDays)
	if err != nil {
		log.Printf("ERROR query old items: %v", err)
	} else {
		for _, item := range aged {
			if item.BookmarkID != nil {
				if err := client.Archive(*item.BookmarkID); err != nil {
					log.Printf("ERROR archive bookmark %d (%s): %v", *item.BookmarkID, item.GUID, err)
				}
			}
			if err := db.DeleteItem(item.GUID); err != nil {
				log.Printf("ERROR delete item %s: %v", item.GUID, err)
			}
		}
	}

	log.Printf("sync complete")
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

- [ ] **Step 3: Update secret.yaml template**

Replace `manifests/secret.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: instapaper-credentials
  namespace: rss-feed-to-instapaper
type: Opaque
stringData:
  INSTAPAPER_USERNAME: "your-instapaper-email@example.com"
  INSTAPAPER_PASSWORD: "your-instapaper-password"
  INSTAPAPER_CONSUMER_KEY: "your-consumer-key"
  INSTAPAPER_CONSUMER_SECRET: "your-consumer-secret"
```

- [ ] **Step 4: Build**

```bash
go build ./cmd/syncer/
```

Expected: no output, binary `syncer` created

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/syncer/main.go manifests/secret.yaml
git commit -m "feat(syncer): wire xAuth, store bookmark_id, add retention pass"
```

- [ ] **Step 7: Patch cluster Secret**

After implementation, the real cluster Secret needs the two new keys. Run this against the cluster (not automated — do it manually after the PR merges):

```bash
kubectl patch secret instapaper-credentials -n rss-feed-to-instapaper \
  --type=merge \
  -p '{"stringData":{"INSTAPAPER_CONSUMER_KEY":"<real-key>","INSTAPAPER_CONSUMER_SECRET":"<real-secret>"}}'
```

---

## Self-Review

**Spec coverage:**

| Requirement | Task |
|---|---|
| `MaxAgeDays int` in Config, default 1 | Task 1 ✓ |
| `ALTER TABLE ... ADD COLUMN bookmark_id INTEGER` migration | Task 2 ✓ |
| `SentItem` with `BookmarkID *int64` | Task 2 ✓ |
| `MarkSentWithID`, `OldItems`, `DeleteItem` | Task 2 ✓ |
| `MarkSent` kept unchanged | Task 2 ✓ |
| `sign()` pure HMAC-SHA1 in `oauth.go` | Task 3 ✓ |
| `NewClient(consumerKey, consumerSecret, username, password)` | Task 4 ✓ |
| `Authenticate()` → xAuth token exchange | Task 4 ✓ |
| `Add()` returns `(int64, error)` | Task 4 ✓ |
| `Archive(bookmarkID int64) error` | Task 4 ✓ |
| `INSTAPAPER_CONSUMER_KEY` + `INSTAPAPER_CONSUMER_SECRET` env vars | Task 5 ✓ |
| Authenticate failure = fatal | Task 5 ✓ |
| Add failure = log + skip (GUID not stored, retried next run) | Task 5 ✓ |
| Archive failure = log, delete from SQLite anyway | Task 5 ✓ |
| NULL bookmark_id = skip archive call, still delete | Task 5 ✓ |
| `manifests/secret.yaml` template updated | Task 5 ✓ |

**Placeholder scan:** None.

**Type consistency:**
- `SentItem.BookmarkID *int64` — defined Task 2, nil-checked in Task 5 ✓
- `MarkSentWithID(guid string, bookmarkID int64)` — same signature Task 2 and Task 5 ✓
- `OldItems(maxAgeDays int) ([]SentItem, error)` — same Task 2 and Task 5 ✓
- `NewClient(consumerKey, consumerSecret, username, password string)` — same Task 4 and Task 5 ✓
- `Add` returns `(int64, error)` — same Task 4 and Task 5 ✓
