package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/Sitleman/challenger/migrations"
	"github.com/pressly/goose/v3"
)

func SchemaVersion(ctx context.Context, conn *sql.DB) (int64, error) {
	var version sql.NullInt64
	err := conn.QueryRowContext(ctx, "SELECT MAX(version_id) FROM goose_db_version").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version.Int64, nil
}

func LatestMigration() (int64, error) {
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return 0, err
	}
	var latest int64
	for _, name := range names {
		version, err := goose.NumericComponent(name)
		if err != nil {
			return 0, fmt.Errorf("migration %s: %w", name, err)
		}
		latest = max(latest, version)
	}
	return latest, nil
}
