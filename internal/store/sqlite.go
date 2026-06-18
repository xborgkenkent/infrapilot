package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists local metadata and scan results.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite database.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS resource_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS scan_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scan_type TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
	`)
	return err
}

// SaveSnapshot stores a JSON payload for a resource kind.
func (s *Store) SaveSnapshot(ctx context.Context, kind, payload string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO resource_snapshots (kind, payload, created_at) VALUES (?, ?, ?)`,
		kind, payload, time.Now().UTC(),
	)
	return err
}

// SaveScan stores a scan result payload.
func (s *Store) SaveScan(ctx context.Context, scanType, payload string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO scan_results (scan_type, payload, created_at) VALUES (?, ?, ?)`,
		scanType, payload, time.Now().UTC(),
	)
	return err
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}
