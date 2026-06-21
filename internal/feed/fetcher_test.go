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
