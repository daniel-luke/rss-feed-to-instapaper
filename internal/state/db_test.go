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
