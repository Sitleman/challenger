package backup

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sitleman/challenger/internal/db"
)

func Snapshot(ctx context.Context, conn *sql.DB, path string) error {
	if _, err := conn.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return fmt.Errorf("vacuum into %s: %w", path, err)
	}
	return Check(ctx, path)
}

func Check(ctx context.Context, path string) error {
	conn, err := db.OpenReadOnly(path)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var result string
	if err := conn.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check %s: %w", path, err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check %s: %s", path, result)
	}
	return nil
}
