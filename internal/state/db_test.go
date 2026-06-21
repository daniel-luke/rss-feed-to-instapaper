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
