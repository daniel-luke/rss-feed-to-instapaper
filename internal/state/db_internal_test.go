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
	// Insert a pre-migration row with a past timestamp directly.
	if _, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, sent_at) VALUES (?, ?)`,
		"pre-migration", "2020-01-01 00:00:00",
	); err != nil {
		t.Fatalf("insert pre-migration: %v", err)
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
