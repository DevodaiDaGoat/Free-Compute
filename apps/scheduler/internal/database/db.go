package database

import (
	"context"
	"fmt"
)

// DB abstracts database operations for the scheduler.
// In a real implementation this would hold a *sql.DB or *pgxpool.Pool.
type DB struct {
	dsn string
}

// New creates a new DB instance with the given connection string.
func New(dsn string) *DB {
	return &DB{dsn: dsn}
}

// Connect opens a database connection.
func (db *DB) Connect(ctx context.Context) error {
	// TODO: open a real connection pool (pgx or database/sql).
	if db.dsn == "" {
		return fmt.Errorf("database: empty DSN")
	}
	return nil
}

// Close shuts down the database connection pool.
func (db *DB) Close() error {
	// TODO: close the real connection pool.
	return nil
}
