package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*sql.DB
}

func Connect(databaseURL string) (*DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return &DB{db}, nil
}

func (db *DB) RunMigrations() error {
	// Create migrations tracking table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Read migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort by filename
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Check if already applied
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", entry.Name()).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration %s: %w", entry.Name(), err)
		}
		if exists {
			continue
		}

		// Read and execute migration
		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", entry.Name(), err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", entry.Name()); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", entry.Name(), err)
		}

		log.Printf("Applied migration: %s", entry.Name())
	}

	return nil
}
