# RSS → Instapaper Syncer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a containerized Go binary that reads RSS feeds from a YAML config, sends unseen items to Instapaper via the Simple API, and tracks sent state in SQLite — deployed as a Kubernetes CronJob.

**Architecture:** Single Go binary with four internal packages (config, state, feed, instapaper) wired together in `cmd/syncer/main.go`. SQLite on a PVC tracks sent GUIDs. k8s CronJob triggers the binary hourly.

**Tech Stack:** Go 1.23, `github.com/mmcdole/gofeed`, `github.com/mattn/go-sqlite3` (CGO), `gopkg.in/yaml.v3`, Docker multi-stage (alpine), Kubernetes CronJob + PVC.

## Global Constraints

- Module name: `github.com/danielgroothuis/rss-feed-to-instapaper`
- Go version: 1.23
- CGO required for go-sqlite3 — do not use `CGO_ENABLED=0`
- Config file path default: `/config/config.yaml` (overridable via `CONFIG_PATH` env var)
- State DB path default: `/data/state.db` (overridable via `STATE_PATH` env var)
- Instapaper credentials via env: `INSTAPAPER_USERNAME`, `INSTAPAPER_PASSWORD` (fatal if missing)
- Instapaper Simple API: `POST https://www.instapaper.com/api/add`, success = HTTP 201
- Exit 0 on successful run (even with partial errors); exit 1 only on fatal startup errors
- Never commit real credentials — `manifests/secret.yaml` contains placeholder values only

---

## File Map

| File | Responsibility |
|---|---|
| `go.mod` | Module definition + dependencies |
| `internal/config/config.go` | Load + validate YAML config |
| `internal/config/config_test.go` | Tests for config loading |
| `internal/state/db.go` | SQLite open, IsSent, MarkSent, Close |
| `internal/state/db_test.go` | Tests for state DB |
| `internal/feed/fetcher.go` | Fetch + parse RSS via gofeed |
| `internal/feed/fetcher_test.go` | Tests using httptest server |
| `internal/instapaper/client.go` | POST to Instapaper Simple API |
| `internal/instapaper/client_test.go` | Tests using httptest server (same package) |
| `cmd/syncer/main.go` | Entry point, wires all packages, orchestration loop |
| `Dockerfile` | Multi-stage alpine build with CGO |
| `config.example.yaml` | Example feed configuration |
| `manifests/cronjob.yaml` | k8s CronJob manifest |
| `manifests/pvc.yaml` | PersistentVolumeClaim for SQLite |
| `manifests/secret.yaml` | Secret template with placeholder values |

---

## Task 1: Go module scaffold

**Files:**
- Create: `go.mod`

**Interfaces:**
- Produces: module `github.com/danielgroothuis/rss-feed-to-instapaper` available for all internal packages

- [ ] **Step 1: Initialize Go module**

```bash
cd /path/to/rss-feed-to-instapaper
go mod init github.com/danielgroothuis/rss-feed-to-instapaper
```

Expected: `go.mod` created with `module github.com/danielgroothuis/rss-feed-to-instapaper` and `go 1.23`

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/mmcdole/gofeed@latest
go get github.com/mattn/go-sqlite3@latest
go get gopkg.in/yaml.v3@latest
```

Expected: `go.mod` and `go.sum` updated with all three packages.

- [ ] **Step 3: Verify tidy**

```bash
go mod tidy
```

Expected: no output, no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "feat(syncer): init go module with dependencies"
```

---

## Task 2: Config package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `type Feed struct { URL string; Label string }`
  - `type Config struct { Feeds []Feed }`
  - `func Load(path string) (*Config, error)` — returns error on missing file, parse failure, empty feeds list, or any feed missing URL

- [ ] **Step 1: Write the failing tests**

Create `internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/config"
)

func TestLoad_valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte(`feeds:
  - url: "https://example.com/feed.xml"
    label: "Test Blog"
`), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d", len(cfg.Feeds))
	}
	if cfg.Feeds[0].URL != "https://example.com/feed.xml" {
		t.Errorf("unexpected URL: %s", cfg.Feeds[0].URL)
	}
	if cfg.Feeds[0].Label != "Test Blog" {
		t.Errorf("unexpected label: %s", cfg.Feeds[0].Label)
	}
}

func TestLoad_missing_file(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_empty_feeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("feeds: []\n"), 0644)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for empty feeds, got nil")
	}
}

func TestLoad_feed_missing_url(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte(`feeds:
  - label: "No URL here"
`), 0644)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for feed missing URL, got nil")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/config/...
```

Expected: compile error `cannot find package "github.com/danielgroothuis/rss-feed-to-instapaper/internal/config"`

- [ ] **Step 3: Implement config package**

Create `internal/config/config.go`:

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
	Feeds []Feed `yaml:"feeds"`
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
	return &cfg, nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: 4 tests, all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add YAML config loader with validation"
```

---

## Task 3: State package (SQLite)

**Files:**
- Create: `internal/state/db.go`
- Create: `internal/state/db_test.go`

**Interfaces:**
- Consumes: `mattn/go-sqlite3` (imported via blank import in db.go)
- Produces:
  - `type DB struct` (opaque)
  - `func Open(path string) (*DB, error)` — opens SQLite file (or `:memory:`), creates `sent_items` table if absent
  - `func (db *DB) IsSent(guid string) (bool, error)`
  - `func (db *DB) MarkSent(guid string) error` — idempotent (INSERT OR IGNORE)
  - `func (db *DB) Close() error`

- [ ] **Step 1: Write the failing tests**

Create `internal/state/db_test.go`:

```go
package state_test

import (
	"testing"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/state"
)

func TestDB_IsSent_MarkSent(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	const guid = "test-guid-1"

	sent, err := db.IsSent(guid)
	if err != nil {
		t.Fatalf("IsSent: %v", err)
	}
	if sent {
		t.Fatal("expected not sent before MarkSent")
	}

	if err := db.MarkSent(guid); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}

	sent, err = db.IsSent(guid)
	if err != nil {
		t.Fatalf("IsSent after mark: %v", err)
	}
	if !sent {
		t.Fatal("expected sent after MarkSent")
	}
}

func TestDB_MarkSent_idempotent(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	const guid = "test-guid-2"
	if err := db.MarkSent(guid); err != nil {
		t.Fatalf("first MarkSent: %v", err)
	}
	if err := db.MarkSent(guid); err != nil {
		t.Fatalf("second MarkSent should be idempotent: %v", err)
	}
}

func TestDB_Open_creates_table(t *testing.T) {
	db, err := state.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// MarkSent on a fresh DB should not fail (table exists)
	if err := db.MarkSent("any-guid"); err != nil {
		t.Fatalf("MarkSent on fresh db: %v", err)
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/state/...
```

Expected: compile error `cannot find package "...internal/state"`

- [ ] **Step 3: Implement state package**

Create `internal/state/db.go`:

```go
package state

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
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

func (db *DB) Close() error {
	return db.conn.Close()
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/state/... -v
```

Expected: 3 tests, all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state/
git commit -m "feat(state): add SQLite state tracker for sent GUIDs"
```

---

## Task 4: Feed fetcher package

**Files:**
- Create: `internal/feed/fetcher.go`
- Create: `internal/feed/fetcher_test.go`

**Interfaces:**
- Consumes: `github.com/mmcdole/gofeed`
- Produces:
  - `type Item struct { GUID string; URL string; Title string }`
  - `func FetchItems(feedURL string) ([]Item, error)` — parses RSS/Atom; falls back to item Link as GUID when GUID field absent

- [ ] **Step 1: Write the failing tests**

Create `internal/feed/fetcher_test.go`:

```go
package feed_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/feed"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <item>
      <title>Article One</title>
      <link>https://example.com/1</link>
      <guid>guid-1</guid>
    </item>
    <item>
      <title>Article Two</title>
      <link>https://example.com/2</link>
      <guid>guid-2</guid>
    </item>
  </channel>
</rss>`

func TestFetchItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	items, err := feed.FetchItems(srv.URL)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].GUID != "guid-1" {
		t.Errorf("expected guid-1, got %s", items[0].GUID)
	}
	if items[0].URL != "https://example.com/1" {
		t.Errorf("expected https://example.com/1, got %s", items[0].URL)
	}
	if items[0].Title != "Article One" {
		t.Errorf("expected 'Article One', got %s", items[0].Title)
	}
}

func TestFetchItems_fallback_guid(t *testing.T) {
	const noGUIDRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <link>https://example.com</link>
    <item>
      <title>No GUID Item</title>
      <link>https://example.com/3</link>
    </item>
  </channel>
</rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(noGUIDRSS))
	}))
	defer srv.Close()

	items, err := feed.FetchItems(srv.URL)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].GUID != "https://example.com/3" {
		t.Errorf("expected link as fallback GUID, got %s", items[0].GUID)
	}
}

func TestFetchItems_bad_url(t *testing.T) {
	_, err := feed.FetchItems("http://127.0.0.1:0/no-server-here")
	if err == nil {
		t.Fatal("expected error for unreachable URL, got nil")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/feed/...
```

Expected: compile error `cannot find package "...internal/feed"`

- [ ] **Step 3: Implement feed fetcher**

Create `internal/feed/fetcher.go`:

```go
package feed

import (
	"fmt"

	"github.com/mmcdole/gofeed"
)

type Item struct {
	GUID  string
	URL   string
	Title string
}

func FetchItems(feedURL string) ([]Item, error) {
	fp := gofeed.NewParser()
	parsed, err := fp.ParseURL(feedURL)
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", feedURL, err)
	}
	items := make([]Item, 0, len(parsed.Items))
	for _, fi := range parsed.Items {
		guid := fi.GUID
		if guid == "" {
			guid = fi.Link
		}
		items = append(items, Item{
			GUID:  guid,
			URL:   fi.Link,
			Title: fi.Title,
		})
	}
	return items, nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/feed/... -v
```

Expected: 3 tests, all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/feed/
git commit -m "feat(feed): add RSS/Atom fetcher with GUID fallback to link"
```

---

## Task 5: Instapaper client package

**Files:**
- Create: `internal/instapaper/client.go`
- Create: `internal/instapaper/client_test.go`

**Interfaces:**
- Produces:
  - `type Client struct` (opaque)
  - `func NewClient(username, password string) *Client`
  - `func (c *Client) Add(itemURL, title string) error` — POSTs to `/api/add`, returns error if status != 201

Note: `client_test.go` uses `package instapaper` (same package, not `_test`) to access the unexported `baseURL` field for test server injection.

- [ ] **Step 1: Write the failing tests**

Create `internal/instapaper/client_test.go`:

```go
package instapaper

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClient_Add_success(t *testing.T) {
	var gotUsername, gotPassword, gotURL, gotTitle string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotUsername = r.FormValue("username")
		gotPassword = r.FormValue("password")
		gotURL = r.FormValue("url")
		gotTitle = r.FormValue("title")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("user@example.com", "secret")
	c.baseURL = srv.URL

	err := c.Add("https://example.com/article", "Article Title")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotUsername != "user@example.com" {
		t.Errorf("username: got %q", gotUsername)
	}
	if gotPassword != "secret" {
		t.Errorf("password: got %q", gotPassword)
	}
	if gotURL != "https://example.com/article" {
		t.Errorf("url: got %q", gotURL)
	}
	if gotTitle != "Article Title" {
		t.Errorf("title: got %q", gotTitle)
	}
}

func TestClient_Add_empty_title(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("title") != "" {
			t.Errorf("expected no title param, got %q", r.FormValue("title"))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("u", "p")
	c.baseURL = srv.URL

	if err := c.Add("https://example.com/article", ""); err != nil {
		t.Fatalf("Add with empty title: %v", err)
	}
}

func TestClient_Add_non_201_returns_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClient("u", "p")
	c.baseURL = srv.URL

	err := c.Add("https://example.com/article", "Title")
	if err == nil {
		t.Fatal("expected error for non-201 response, got nil")
	}
}

func TestClient_Add_encodes_url_in_form(t *testing.T) {
	var rawBody url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		rawBody = r.Form
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewClient("u", "p")
	c.baseURL = srv.URL

	articleURL := "https://example.com/path?q=hello world&a=1"
	_ = c.Add(articleURL, "")

	if rawBody.Get("url") != articleURL {
		t.Errorf("url not properly form-encoded: got %q", rawBody.Get("url"))
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
go test ./internal/instapaper/...
```

Expected: compile error `cannot find package "...internal/instapaper"`

- [ ] **Step 3: Implement Instapaper client**

Create `internal/instapaper/client.go`:

```go
package instapaper

import (
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	username string
	password string
	baseURL  string
	http     *http.Client
}

func NewClient(username, password string) *Client {
	return &Client{
		username: username,
		password: password,
		baseURL:  "https://www.instapaper.com",
		http:     &http.Client{},
	}
}

func (c *Client) Add(itemURL, title string) error {
	params := url.Values{
		"username": {c.username},
		"password": {c.password},
		"url":      {itemURL},
	}
	if title != "" {
		params.Set("title", title)
	}

	resp, err := c.http.PostForm(c.baseURL+"/api/add", params)
	if err != nil {
		return fmt.Errorf("post to instapaper: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("instapaper returned %d for %s", resp.StatusCode, itemURL)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
go test ./internal/instapaper/... -v
```

Expected: 4 tests, all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/instapaper/
git commit -m "feat(instapaper): add Simple API client with form POST"
```

---

## Task 6: Main orchestration

**Files:**
- Create: `cmd/syncer/main.go`

**Interfaces:**
- Consumes:
  - `config.Load(path string) (*Config, error)`
  - `config.Config.Feeds []config.Feed` (each `.URL`, `.Label`)
  - `state.Open(path string) (*DB, error)`, `(*DB).IsSent`, `(*DB).MarkSent`, `(*DB).Close`
  - `feed.FetchItems(feedURL string) ([]feed.Item, error)` (each `.GUID`, `.URL`, `.Title`)
  - `instapaper.NewClient(username, password string) *Client`, `(*Client).Add(url, title string) error`
- Produces: compiled binary `syncer`

- [ ] **Step 1: Create main.go**

Create `cmd/syncer/main.go`:

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

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := state.Open(statePath)
	if err != nil {
		log.Fatalf("open state db: %v", err)
	}
	defer db.Close()

	client := instapaper.NewClient(username, password)

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

			if err := client.Add(item.URL, item.Title); err != nil {
				log.Printf("ERROR add to instapaper %s: %v", item.URL, err)
				continue
			}

			if err := db.MarkSent(item.GUID); err != nil {
				log.Printf("ERROR mark sent %s: %v", item.GUID, err)
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

- [ ] **Step 2: Verify binary compiles**

```bash
go build ./cmd/syncer/
```

Expected: `syncer` binary in current directory, no errors.

- [ ] **Step 3: Run all tests to confirm nothing broken**

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 4: Clean up binary and commit**

```bash
rm -f syncer
git add cmd/syncer/main.go
git commit -m "feat(syncer): add main orchestration loop"
```

---

## Task 7: Dockerfile

**Files:**
- Create: `Dockerfile`
- Create: `config.example.yaml`

**Interfaces:**
- Produces: Docker image that runs `/syncer` binary, expects `/config/config.yaml` and `/data/` mounted at runtime

- [ ] **Step 1: Create Dockerfile**

Create `Dockerfile`:

```dockerfile
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /syncer ./cmd/syncer

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs

COPY --from=builder /syncer /syncer

ENTRYPOINT ["/syncer"]
```

- [ ] **Step 2: Create config.example.yaml**

Create `config.example.yaml`:

```yaml
feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
  - url: "https://another-blog.com/rss"
    label: "Another Blog"
```

- [ ] **Step 3: Create .dockerignore**

Create `.dockerignore`:

```
docs/
manifests/
*.md
.git/
.gitignore
```

- [ ] **Step 4: Build image locally to verify**

```bash
docker build -t rss-syncer:test .
```

Expected: image builds successfully, both stages complete, no errors.

- [ ] **Step 5: Verify binary runs (expect config error, not crash)**

```bash
docker run --rm \
  -e INSTAPAPER_USERNAME=test \
  -e INSTAPAPER_PASSWORD=test \
  -e CONFIG_PATH=/nonexistent.yaml \
  rss-syncer:test
```

Expected: exits with `load config: read config: open /nonexistent.yaml: no such file or directory` — confirms binary runs and config loading works.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile .dockerignore config.example.yaml
git commit -m "feat(docker): add multi-stage alpine build with CGO support"
```

---

## Task 8: Kubernetes manifests

**Files:**
- Create: `manifests/pvc.yaml`
- Create: `manifests/secret.yaml`
- Create: `manifests/cronjob.yaml`

**Interfaces:**
- Produces: three YAML files ready to apply with `kubectl apply -f manifests/`
- The CronJob mounts PVC at `/data` and expects a ConfigMap `rss-syncer-config` with key `config.yaml` mounted at `/config/config.yaml`

- [ ] **Step 1: Create PVC manifest**

Create `manifests/pvc.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: rss-syncer-state
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Mi
```

- [ ] **Step 2: Create Secret template**

Create `manifests/secret.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: instapaper-credentials
  namespace: default
type: Opaque
stringData:
  INSTAPAPER_USERNAME: "your-instapaper-email@example.com"
  INSTAPAPER_PASSWORD: "your-instapaper-password"
```

- [ ] **Step 3: Create CronJob manifest**

Create `manifests/cronjob.yaml`:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: rss-syncer
  namespace: default
spec:
  schedule: "0 * * * *"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: syncer
              image: your-registry/rss-syncer:latest
              imagePullPolicy: Always
              envFrom:
                - secretRef:
                    name: instapaper-credentials
              volumeMounts:
                - name: state
                  mountPath: /data
                - name: config
                  mountPath: /config
          volumes:
            - name: state
              persistentVolumeClaim:
                claimName: rss-syncer-state
            - name: config
              configMap:
                name: rss-syncer-config
                items:
                  - key: config.yaml
                    path: config.yaml
```

Note: Create the ConfigMap from your actual config file with:
```bash
kubectl create configmap rss-syncer-config --from-file=config.yaml=config.yaml
```

- [ ] **Step 4: Lint manifests (if kubectl available)**

```bash
kubectl apply --dry-run=client -f manifests/
```

Expected: `...configured (dry run)` for all three resources, no errors.

- [ ] **Step 5: Commit**

```bash
git add manifests/
git commit -m "feat(manifests): add k8s CronJob, PVC, and Secret template"
```

---

## Task 9: Final integration check + README

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Create .gitignore**

Create `.gitignore`:

```
syncer
*.db
*.db-shm
*.db-wal
config.yaml
```

Note: `config.yaml` (not `config.example.yaml`) is gitignored so real feed lists with potentially sensitive URLs aren't committed.

- [ ] **Step 2: Run full test suite**

```bash
go test ./... -v -count=1
```

Expected: all tests across all packages PASS.

- [ ] **Step 3: Verify build**

```bash
go build ./cmd/syncer/
rm syncer
```

Expected: compiles with no warnings or errors.

- [ ] **Step 4: Commit**

```bash
git add .gitignore
git commit -m "chore: add gitignore for binary, DB files, and local config"
```
