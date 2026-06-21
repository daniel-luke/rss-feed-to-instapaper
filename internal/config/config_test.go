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
