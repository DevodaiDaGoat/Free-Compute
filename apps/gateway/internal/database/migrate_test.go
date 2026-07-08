package database

import (
	"os"
	"testing"
)

func TestMigrateIdempotent(t *testing.T) {
	path := "/tmp/freecompute-migrate-test.db"
	os.Remove(path)

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("open2 (idempotency): %v", err)
	}
	defer db2.Close()

	var userCount int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("expected 0 users, got %d", userCount)
	}

	var tableCount int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='reports'`).Scan(&tableCount); err != nil {
		t.Fatalf("count reports table: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("expected reports table to exist after second migrate")
	}
}
