package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Sitleman/challenger/internal/db"
)

const asideLayout = "20060102-150405"

var (
	ErrNewerSchema = errors.New("backup schema is newer than this binary")
	sidecars       = []string{"", "-wal", "-shm"}
)

func Restore(ctx context.Context, src, dbPath string, now time.Time) (string, error) {
	lock, err := db.AcquireLock(dbPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = lock.Release() }()

	staged := dbPath + ".restoring"
	if err := stage(ctx, src, staged); err != nil {
		removeDB(staged)
		return "", fmt.Errorf("backup %s: %w", src, err)
	}
	aside, err := setAside(ctx, dbPath, now)
	if err != nil {
		removeDB(staged)
		return "", err
	}
	if err := os.Rename(staged, dbPath); err != nil {
		removeDB(staged)
		return "", putBack(aside, dbPath, err)
	}
	return aside, syncDir(filepath.Dir(dbPath))
}

func stage(ctx context.Context, src, staged string) error {
	removeDB(staged)
	if err := copyFile(src, staged); err != nil {
		return err
	}
	if err := copyFile(src+"-wal", staged+"-wal"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := db.UseRollbackJournal(ctx, staged); err != nil {
		return err
	}
	if err := Check(ctx, staged); err != nil {
		return err
	}
	return checkSchema(ctx, staged)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy backup: %w", err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return fmt.Errorf("sync %s: %w", dst, err)
	}
	return out.Close()
}

func checkSchema(ctx context.Context, path string) error {
	conn, err := db.OpenReadOnly(path)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	version, err := db.SchemaVersion(ctx, conn)
	if err != nil {
		return fmt.Errorf("not a challenger database: %w", err)
	}
	latest, err := db.LatestMigration()
	if err != nil {
		return err
	}
	if version > latest {
		return fmt.Errorf("%w: backup %d, binary %d", ErrNewerSchema, version, latest)
	}
	return nil
}

func setAside(ctx context.Context, dbPath string, now time.Time) (string, error) {
	if _, err := os.Stat(dbPath); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	_ = db.Checkpoint(ctx, dbPath)
	aside := dbPath + ".before-restore-" + now.Format(asideLayout)
	if err := moveDB(dbPath, aside); err != nil {
		return "", fmt.Errorf("set aside: %w", err)
	}
	return aside, nil
}

func moveDB(from, to string) error {
	for _, suffix := range sidecars {
		err := os.Rename(from+suffix, to+suffix)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func putBack(aside, dbPath string, cause error) error {
	if aside == "" {
		return fmt.Errorf("move restored database into place: %w", cause)
	}
	if err := moveDB(aside, dbPath); err != nil {
		return fmt.Errorf("move restored database into place: %w; previous database left at %s: %v", cause, aside, err)
	}
	return fmt.Errorf("move restored database into place, previous database put back: %w", cause)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", dir, err)
	}
	return nil
}

func removeDB(path string) {
	for _, suffix := range sidecars {
		_ = os.Remove(path + suffix)
	}
}
