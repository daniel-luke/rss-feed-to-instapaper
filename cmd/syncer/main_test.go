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
