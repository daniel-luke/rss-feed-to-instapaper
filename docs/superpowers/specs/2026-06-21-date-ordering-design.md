# Date-Ordered Instapaper Adds — Design Spec

## Overview

Add articles to Instapaper in chronological order so the newest article always appears at the top of the reading queue. On by default; can be disabled via config.

## Key Insight

Instapaper shows the most recently *added* article at the top of the queue. To make the newest article appear first, it must be added last. So: sort all new items ascending by publication date (oldest first), add in that order — newest item is added last and lands at the top.

## Architecture

Collect all new items across all feeds into a single list before any Instapaper calls. When `sort_by_date` is enabled (default), sort that list ascending by `PublishedAt`. Add items in sorted order.

This is a change to `main.go` orchestration: instead of add-per-feed-inline, it's collect-all → sort → add-all.

## Config

```yaml
sort_by_date: false   # disable to restore feed-order behaviour
```

Default: on (enabled when key is absent). Uses `*bool` in the Go struct so nil (key absent) maps to `true`.

## Data Flow

1. For each feed: fetch items, filter already-sent → append new items to shared slice (with feed label for logging)
2. If `sort_by_date` enabled: sort slice ascending by `PublishedAt` (nil date → front of slice, added first, appears at bottom of Instapaper)
3. Add items in order, storing `bookmark_id` as before

## File Changes

### `internal/feed/fetcher.go`
- Add `PublishedAt *time.Time` field to `Item` struct
- Populate from `fi.PublishedParsed` (already parsed by gofeed, no new dependencies)

### `internal/config/config.go`
- Add `SortByDate *bool` field with yaml tag `sort_by_date`
- In `Load()`: if `cfg.SortByDate == nil`, set to pointer to `true`

### `cmd/syncer/main.go`
- Collect all new items across feeds into `[]feedItem` (embed feed label for logging)
- After collection: if sort enabled, `sort.Slice` by `PublishedAt` ascending, nil dates first
- Add items in sorted order

## Testing

- `fetcher_test.go`: assert `PublishedAt` populated when feed item has date; nil when absent
- `config_test.go`: assert `SortByDate` defaults to `true` when absent; respects `false`
- `main_test.go` or integration: sort order verified via mock Instapaper server call order

## Error Handling

Items with no `PublishedAt`: sort to front (added first, land at bottom of Instapaper queue). Same behaviour as current default for undated feeds.

## Constraints

- No new dependencies
- CGO_ENABLED=1 required (existing constraint, unchanged)
- `sort_by_date: false` in `config.example.yaml` must be shown as a comment or optional field
