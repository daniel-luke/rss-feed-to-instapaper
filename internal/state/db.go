package state

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type SentItem struct {
	GUID       string
	BookmarkID *int64
	SentAt     time.Time
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
	// Migrate: add bookmark_id column absent from pre-retention DBs.
	if _, err := conn.Exec(`ALTER TABLE sent_items ADD COLUMN bookmark_id INTEGER`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			conn.Close()
			return nil, fmt.Errorf("migrate schema: %w", err)
		}
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

func (db *DB) MarkSentWithID(guid string, bookmarkID int64) error {
	_, err := db.conn.Exec(
		`INSERT INTO sent_items (guid, bookmark_id) VALUES (?, ?)
     ON CONFLICT(guid) DO UPDATE SET bookmark_id = excluded.bookmark_id`,
		guid, bookmarkID,
	)
	if err != nil {
		return fmt.Errorf("mark sent with id: %w", err)
	}
	return nil
}

func (db *DB) OldItems(maxAgeDays int) ([]SentItem, error) {
	rows, err := db.conn.Query(
		`SELECT guid, bookmark_id, sent_at FROM sent_items WHERE sent_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", maxAgeDays),
	)
	if err != nil {
		return nil, fmt.Errorf("query old items: %w", err)
	}
	defer rows.Close()

	var items []SentItem
	for rows.Next() {
		var item SentItem
		var sentAt string
		if err := rows.Scan(&item.GUID, &item.BookmarkID, &sentAt); err != nil {
			return nil, fmt.Errorf("scan old item: %w", err)
		}
		t, err := time.Parse("2006-01-02 15:04:05", sentAt)
		if err != nil {
			t, err = time.Parse(time.RFC3339, sentAt)
		}
		if err != nil {
			return nil, fmt.Errorf("parse sent_at %q: %w", sentAt, err)
		}
		item.SentAt = t
		items = append(items, item)
	}
	return items, rows.Err()
}

func (db *DB) DeleteItem(guid string) error {
	_, err := db.conn.Exec(`DELETE FROM sent_items WHERE guid = ?`, guid)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}
