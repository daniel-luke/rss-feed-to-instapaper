package state

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
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

func (db *DB) Close() error {
	return db.conn.Close()
}
