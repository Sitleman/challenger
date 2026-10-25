package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sitleman/challenger/migrations"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	if err := migrate(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func dsn(path string) string {
	return "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

func migrate(ctx context.Context, conn *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func OpenReadOnly(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	conn.SetMaxOpenConns(1)
	return conn, nil
}

func Checkpoint(ctx context.Context, path string) error {
	return execPragma(ctx, path, "PRAGMA wal_checkpoint(TRUNCATE)")
}

func UseRollbackJournal(ctx context.Context, path string) error {
	return execPragma(ctx, path, "PRAGMA journal_mode=DELETE")
}

func execPragma(ctx context.Context, path, pragma string) error {
	conn, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return fmt.Errorf("open sqlite %s: %w", path, err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, pragma); err != nil {
		return fmt.Errorf("%s on %s: %w", pragma, path, err)
	}
	return nil
}
