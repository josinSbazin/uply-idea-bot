package storage

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS ideas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_message_id INTEGER NOT NULL,
    telegram_chat_id INTEGER NOT NULL,
    telegram_user_id INTEGER NOT NULL,
    telegram_username TEXT DEFAULT '',
    telegram_first_name TEXT DEFAULT '',
    raw_text TEXT NOT NULL,
    enriched_json TEXT DEFAULT '',
    title TEXT DEFAULT '',
    category TEXT DEFAULT '',
    priority TEXT DEFAULT '',
    complexity TEXT DEFAULT '',
    affected_repos TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'new',
    admin_notes TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ideas_status ON ideas(status);
CREATE INDEX IF NOT EXISTS idx_ideas_category ON ideas(category);
CREATE INDEX IF NOT EXISTS idx_ideas_priority ON ideas(priority);
CREATE INDEX IF NOT EXISTS idx_ideas_created_at ON ideas(created_at);
CREATE INDEX IF NOT EXISTS idx_ideas_telegram_chat_id ON ideas(telegram_chat_id);
`

// Init initializes the SQLite database
func Init(dbPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("Warning: failed to enable WAL mode: %v", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		log.Printf("Warning: failed to enable foreign keys: %v", err)
	}

	// Run migrations
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	log.Printf("SQLite database initialized at %s", dbPath)
	return nil
}

// DB returns the database connection
func DB() *sql.DB {
	return db
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}
