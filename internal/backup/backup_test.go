package backup

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sitleman/challenger/internal/db"
)

var restoredAt = time.Date(2026, 10, 25, 21, 30, 0, 0, time.UTC)

func TestSnapshotIncludesWritesStillInWAL(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	conn := openSeeded(t, filepath.Join(dir, "live.db"), -100)
	defer func() { _ = conn.Close() }()
	if info, err := os.Stat(filepath.Join(dir, "live.db-wal")); err != nil || info.Size() == 0 {
		t.Fatalf("precondition: expected non-empty WAL, stat = %v, %v", info, err)
	}

	snapshot := filepath.Join(t.TempDir(), "snapshot.db")
	if err := Snapshot(ctx, conn, snapshot); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if got := chatIDs(t, snapshot); len(got) != 1 || got[0] != -100 {
		t.Fatalf("snapshot challenges = %v, want [-100]", got)
	}
}

func TestCheckRejectsCorruptFile(t *testing.T) {
	path := writeGarbage(t)

	if err := Check(context.Background(), path); err == nil {
		t.Fatal("Check on garbage file: expected error, got nil")
	}
}

func TestRestoreReplacesDatabaseAndSetsAsideCurrent(t *testing.T) {
	ctx := context.Background()
	src := seededFile(t, filepath.Join(t.TempDir(), "backup.db"), -1)
	dst := seededFile(t, filepath.Join(t.TempDir(), "challenger.db"), -2)

	aside, err := Restore(ctx, src, dst, restoredAt)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if want := dst + ".before-restore-20261025-213000"; aside != want {
		t.Fatalf("aside = %q, want %q", aside, want)
	}
	if got := chatIDs(t, dst); len(got) != 1 || got[0] != -1 {
		t.Fatalf("restored challenges = %v, want [-1]", got)
	}
	if got := chatIDs(t, aside); len(got) != 1 || got[0] != -2 {
		t.Fatalf("set-aside challenges = %v, want [-2]", got)
	}
	if leftovers, _ := filepath.Glob(dst + ".restoring*"); len(leftovers) != 0 {
		t.Fatalf("staging leftovers after restore: %v", leftovers)
	}
}

func TestRestoreSetsAsideWritesStillInWAL(t *testing.T) {
	ctx := context.Background()
	src := seededFile(t, filepath.Join(t.TempDir(), "backup.db"), -1)
	dst := crashedCopy(t, -2)

	aside, err := Restore(ctx, src, dst, restoredAt)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(aside + suffix); err == nil {
			_ = os.Remove(aside + suffix)
		}
	}
	if got := chatIDs(t, aside); len(got) != 1 || got[0] != -2 {
		t.Fatalf("set-aside challenges without WAL = %v, want [-2]", got)
	}
}

func TestRestoreFromSourceWithWritesStillInWAL(t *testing.T) {
	src := crashedCopy(t, -1)
	dst := seededFile(t, filepath.Join(t.TempDir(), "challenger.db"), -2)

	if _, err := Restore(context.Background(), src, dst, restoredAt); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if got := chatIDs(t, dst); len(got) != 1 || got[0] != -1 {
		t.Fatalf("restored challenges = %v, want [-1]", got)
	}
	if leftovers, _ := filepath.Glob(dst + ".restoring*"); len(leftovers) != 0 {
		t.Fatalf("staging leftovers after restore: %v", leftovers)
	}
}

func TestRestoreIntoMissingDatabase(t *testing.T) {
	src := seededFile(t, filepath.Join(t.TempDir(), "backup.db"), -1)
	dst := filepath.Join(t.TempDir(), "challenger.db")

	aside, err := Restore(context.Background(), src, dst, restoredAt)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if aside != "" {
		t.Fatalf("aside = %q, want empty", aside)
	}
	if got := chatIDs(t, dst); len(got) != 1 || got[0] != -1 {
		t.Fatalf("restored challenges = %v, want [-1]", got)
	}
}

func TestRestoreRefusesNewerSchema(t *testing.T) {
	ctx := context.Background()
	src := seededFile(t, filepath.Join(t.TempDir(), "backup.db"), -1)
	bumpSchema(t, src)
	dst := seededFile(t, filepath.Join(t.TempDir(), "challenger.db"), -2)

	_, err := Restore(ctx, src, dst, restoredAt)

	if !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("Restore err = %v, want ErrNewerSchema", err)
	}
	assertUntouched(t, dst, -2)
}

func TestRestoreRefusesCorruptBackup(t *testing.T) {
	dst := seededFile(t, filepath.Join(t.TempDir(), "challenger.db"), -2)

	if _, err := Restore(context.Background(), writeGarbage(t), dst, restoredAt); err == nil {
		t.Fatal("Restore from garbage: expected error, got nil")
	}
	assertUntouched(t, dst, -2)
}

func TestRestoreRefusesWhileDatabaseIsLocked(t *testing.T) {
	src := seededFile(t, filepath.Join(t.TempDir(), "backup.db"), -1)
	dst := seededFile(t, filepath.Join(t.TempDir(), "challenger.db"), -2)
	lock, err := db.AcquireLock(dst)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	_, err = Restore(context.Background(), src, dst, restoredAt)

	if !errors.Is(err, db.ErrLocked) {
		t.Fatalf("Restore err = %v, want db.ErrLocked", err)
	}
	assertUntouched(t, dst, -2)
}

func openSeeded(t *testing.T, path string, chatID int64) *sql.DB {
	t.Helper()
	conn, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = db.New(conn).CreateChallenge(context.Background(), db.CreateChallengeParams{
		ChatID: chatID, ThreadID: 5, Month: "2026-10", StatsMessageID: 1, StartDay: 24,
	})
	if err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	return conn
}

func seededFile(t *testing.T, path string, chatID int64) string {
	t.Helper()
	if err := openSeeded(t, path, chatID).Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return path
}

func crashedCopy(t *testing.T, chatID int64) string {
	t.Helper()
	live := filepath.Join(t.TempDir(), "live.db")
	conn := openSeeded(t, live, chatID)
	defer func() { _ = conn.Close() }()
	dst := filepath.Join(t.TempDir(), "challenger.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		copyTestFile(t, live+suffix, dst+suffix)
	}
	if info, err := os.Stat(dst + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("precondition: expected non-empty WAL copy, stat = %v, %v", info, err)
	}
	return dst
}

func copyTestFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func bumpSchema(t *testing.T, path string) {
	t.Helper()
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_, err = conn.Exec("INSERT INTO goose_db_version (version_id, is_applied) VALUES (9999, 1)")
	if err != nil {
		t.Fatalf("bump schema: %v", err)
	}
}

func writeGarbage(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "garbage.db")
	if err := os.WriteFile(path, []byte("definitely not an sqlite database file"), 0o600); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	return path
}

func assertUntouched(t *testing.T, dst string, chatID int64) {
	t.Helper()
	if got := chatIDs(t, dst); len(got) != 1 || got[0] != chatID {
		t.Fatalf("database after refused restore = %v, want [%d]", got, chatID)
	}
	matches, err := filepath.Glob(dst + ".*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, m := range matches {
		if m != dst+".lock" {
			t.Fatalf("unexpected leftover %s after refused restore", m)
		}
	}
}

func chatIDs(t *testing.T, path string) []int64 {
	t.Helper()
	conn, err := db.OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer func() { _ = conn.Close() }()
	rows, err := conn.Query("SELECT chat_id FROM challenges ORDER BY id")
	if err != nil {
		t.Fatalf("query %s: %v", path, err)
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return ids
}
