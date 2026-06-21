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
