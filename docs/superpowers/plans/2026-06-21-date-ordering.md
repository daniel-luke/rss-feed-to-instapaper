# Date-Ordered Instapaper Adds — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add articles to Instapaper in publication-date order so the newest article always appears at the top of the reading queue.

**Architecture:** Collect all unsent items across all feeds into a single list, sort ascending by `PublishedAt` (oldest first, nil dates first), then add in that order. Newest article lands last → appears at top of Instapaper queue. Controlled by `SortByDate *bool` in config (nil/absent → true; `sort_by_date: false` disables).

**Tech Stack:** Go 1.23, `github.com/mmcdole/gofeed` (already parses `PublishedParsed *time.Time`), `gopkg.in/yaml.v3`

## Global Constraints

- Module: `github.com/danielgroothuis/rss-feed-to-instapaper`
- Go 1.23, CGO_ENABLED=1 required (mattn/go-sqlite3 in other packages)
- No new dependencies
- Run all tests with: `go test ./...`
- Config file lives at `/config/config.yaml` in production; tests use `t.TempDir()`
- `SortByDate *bool`: nil → true (default on); pointer to false → disabled; pointer to true → explicitly on

---

## File Map

| File | Change |
|------|--------|
| `internal/feed/fetcher.go` | Add `PublishedAt *time.Time` to `Item`; populate from `fi.PublishedParsed` |
| `internal/feed/fetcher_test.go` | Add tests for `PublishedAt` populated and nil |
| `internal/config/config.go` | Add `SortByDate *bool` field; default nil→true in `Load()` |
| `internal/config/config_test.go` | Add tests for default, explicit false, explicit true |
| `cmd/syncer/main.go` | Add `pendingItem` type + `sortPending` func; refactor feed loop to collect-sort-add |
| `cmd/syncer/main_test.go` | New file; test `sortPending` ordering |
| `config.example.yaml` | Add `# sort_by_date: false` as commented-out opt-out line |

---

### Task 1: Feed — PublishedAt field

**Files:**
- Modify: `internal/feed/fetcher.go`
- Modify: `internal/feed/fetcher_test.go`

**Interfaces:**
- Produces: `feed.Item.PublishedAt *time.Time` — nil when feed item has no pub date, non-nil `*time.Time` (UTC) when present

- [ ] **Step 1: Write failing tests**

Add to `internal/feed/fetcher_test.go` (after the existing tests):

```go
const datedRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Dated Feed</title>
    <link>https://example.com</link>
    <item>
      <title>Dated Article</title>
      <link>https://example.com/dated</link>
      <guid>guid-dated</guid>
      <pubDate>Mon, 01 Jan 2024 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`

const undatedRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Undated Feed</title>
    <link>https://example.com</link>
    <item>
      <title>Undated Article</title>
      <link>https://example.com/undated</link>
      <guid>guid-undated</guid>
    </item>
  </channel>
</rss>`

func TestFetchItems_published_at(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(datedRSS))
	}))
	defer srv.Close()

	items, err := feed.FetchItems(srv.URL)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].PublishedAt == nil {
		t.Fatal("expected PublishedAt to be non-nil")
	}
	want := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if !items[0].PublishedAt.Equal(want) {
		t.Errorf("PublishedAt: got %v, want %v", *items[0].PublishedAt, want)
	}
}

func TestFetchItems_no_pubdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(undatedRSS))
	}))
	defer srv.Close()

	items, err := feed.FetchItems(srv.URL)
	if err != nil {
		t.Fatalf("FetchItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].PublishedAt != nil {
		t.Errorf("expected PublishedAt to be nil, got %v", *items[0].PublishedAt)
	}
}
```

Also add `"time"` to the import block in `fetcher_test.go`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/feed/... -run TestFetchItems_published_at -v
go test ./internal/feed/... -run TestFetchItems_no_pubdate -v
```

Expected: `FAIL` — `items[0].PublishedAt` undefined (field doesn't exist yet).

- [ ] **Step 3: Add `PublishedAt` to `Item` and populate it**

Replace the full `internal/feed/fetcher.go` with:

```go
package feed

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type Item struct {
	GUID        string
	URL         string
	Title       string
	PublishedAt *time.Time
}

func FetchItems(feedURL string) ([]Item, error) {
	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: 30 * time.Second}
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
		if guid == "" {
			// skip items with no guid and no link — can't deduplicate or submit
			continue
		}
		items = append(items, Item{
			GUID:        guid,
			URL:         fi.Link,
			Title:       fi.Title,
			PublishedAt: fi.PublishedParsed,
		})
	}
	return items, nil
}
```

- [ ] **Step 4: Run all feed tests**

```bash
go test ./internal/feed/... -v
```

Expected: all tests pass, including the two new ones.

- [ ] **Step 5: Commit**

```bash
git add internal/feed/fetcher.go internal/feed/fetcher_test.go
git commit -m "feat(feed): add PublishedAt field to Item"
```

---

### Task 2: Config — SortByDate field

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing new
- Produces: `cfg.SortByDate *bool` — guaranteed non-nil after `Load()` returns; `*cfg.SortByDate == true` means sort enabled

- [ ] **Step 1: Write failing tests**

Add to `internal/config/config_test.go` (after the existing tests):

```go
func TestLoad_sort_by_date_default(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("feeds:\n  - url: https://example.com/feed.xml\n    label: Example\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SortByDate == nil {
		t.Fatal("SortByDate should not be nil after Load")
	}
	if !*cfg.SortByDate {
		t.Errorf("SortByDate: got false, want true (default)")
	}
}

func TestLoad_sort_by_date_false(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("sort_by_date: false\nfeeds:\n  - url: https://example.com/feed.xml\n    label: Example\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SortByDate == nil {
		t.Fatal("SortByDate should not be nil after Load")
	}
	if *cfg.SortByDate {
		t.Errorf("SortByDate: got true, want false")
	}
}

func TestLoad_sort_by_date_explicit_true(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(path, []byte("sort_by_date: true\nfeeds:\n  - url: https://example.com/feed.xml\n    label: Example\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SortByDate == nil {
		t.Fatal("SortByDate should not be nil after Load")
	}
	if !*cfg.SortByDate {
		t.Errorf("SortByDate: got false, want true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/... -run TestLoad_sort_by_date -v
```

Expected: `FAIL` — `cfg.SortByDate` undefined.

- [ ] **Step 3: Add `SortByDate *bool` to Config and default it**

Replace the full `internal/config/config.go` with:

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
	SortByDate *bool  `yaml:"sort_by_date"`
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
	if cfg.SortByDate == nil {
		t := true
		cfg.SortByDate = &t
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Run all config tests**

```bash
go test ./internal/config/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add SortByDate field, default true"
```

---

### Task 3: Main — collect-sort-add orchestration

**Files:**
- Modify: `cmd/syncer/main.go`
- Create: `cmd/syncer/main_test.go`
- Modify: `config.example.yaml`

**Interfaces:**
- Consumes: `feed.Item.PublishedAt *time.Time` (Task 1), `cfg.SortByDate *bool` (Task 2)
- Produces: nothing (final wiring)

- [ ] **Step 1: Write failing test for `sortPending`**

Create `cmd/syncer/main_test.go`:

```go
package main

import (
	"testing"
	"time"

	"github.com/danielgroothuis/rss-feed-to-instapaper/internal/feed"
)

func TestSortPending_order(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	items := []pendingItem{
		{item: feed.Item{GUID: "newest", PublishedAt: &t3}},
		{item: feed.Item{GUID: "nodate", PublishedAt: nil}},
		{item: feed.Item{GUID: "oldest", PublishedAt: &t1}},
		{item: feed.Item{GUID: "middle", PublishedAt: &t2}},
	}

	sortPending(items)

	want := []string{"nodate", "oldest", "middle", "newest"}
	for i, w := range want {
		if items[i].item.GUID != w {
			t.Errorf("position %d: got %q, want %q", i, items[i].item.GUID, w)
		}
	}
}

func TestSortPending_all_nil(t *testing.T) {
	items := []pendingItem{
		{item: feed.Item{GUID: "a", PublishedAt: nil}},
		{item: feed.Item{GUID: "b", PublishedAt: nil}},
	}
	sortPending(items)
	// order between nil-date items is undefined; just verify no panic
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestSortPending_empty(t *testing.T) {
	sortPending(nil) // must not panic
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/syncer/... -run TestSortPending -v
```

Expected: `FAIL` — `pendingItem` and `sortPending` undefined.

- [ ] **Step 3: Implement collect-sort-add in `main.go`**

Replace the full `cmd/syncer/main.go` with:

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
			pending = append(pending, pendingItem{item: item, feedLabel: f.Label})
		}
	}

	if *cfg.SortByDate {
		sortPending(pending)
	}

	for _, p := range pending {
		bookmarkID, err := client.Add(p.item.URL, p.item.Title)
		if err != nil {
			log.Printf("ERROR add to instapaper %s: %v", p.item.URL, err)
			continue
		}
		if err := db.MarkSentWithID(p.item.GUID, bookmarkID); err != nil {
			log.Printf("ERROR mark sent %s: %v", p.item.GUID, err)
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

- [ ] **Step 4: Run the sort tests**

```bash
go test ./cmd/syncer/... -run TestSortPending -v
```

Expected: all three `TestSortPending_*` tests pass.

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Update `config.example.yaml`**

Replace the full `config.example.yaml` with:

```yaml
max_age_days: 1
# sort_by_date: false  # uncomment to add articles in feed order instead of newest-first

feeds:
  - url: "https://example.com/feed.xml"
    label: "Example Blog"
  - url: "https://another-blog.com/rss"
    label: "Another Blog"
```

- [ ] **Step 7: Commit**

```bash
git add cmd/syncer/main.go cmd/syncer/main_test.go config.example.yaml
git commit -m "feat(syncer): collect all items then sort by date before adding to Instapaper"
```
