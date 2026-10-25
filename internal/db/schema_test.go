package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSchemaVersionMatchesLatestMigrationAfterOpen(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = conn.Close() }()

	latest, err := LatestMigration()
	if err != nil {
		t.Fatalf("LatestMigration: %v", err)
	}
	if latest < 2 {
		t.Fatalf("LatestMigration = %d, want at least 2", latest)
	}
	version, err := SchemaVersion(ctx, conn)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != latest {
		t.Fatalf("SchemaVersion = %d, want %d", version, latest)
	}
}

func TestSchemaVersionErrorsWithoutGooseTable(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "foreign.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "CREATE TABLE notes (id INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if _, err := SchemaVersion(ctx, conn); err == nil {
		t.Fatal("SchemaVersion on foreign database: expected error, got nil")
	}
}
