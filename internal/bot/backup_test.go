package bot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sitleman/challenger/internal/backup"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
)

func TestIsBackupRequest(t *testing.T) {
	admins := map[int64]bool{7: true}
	private := telego.Chat{ID: 7, Type: telego.ChatTypePrivate}
	group := telego.Chat{ID: -100, Type: telego.ChatTypeSupergroup}
	admin := &telego.User{ID: 7}
	stranger := &telego.User{ID: 8}

	cases := []struct {
		name string
		msg  telego.Message
		want bool
	}{
		{"admin in private", telego.Message{Chat: private, From: admin, Text: "/make_backup"}, true},
		{"admin mentions this bot", telego.Message{Chat: private, From: admin, Text: "/make_backup@ChallengerBot"}, true},
		{"admin mentions other bot", telego.Message{Chat: private, From: admin, Text: "/make_backup@OtherBot"}, false},
		{"admin in group", telego.Message{Chat: group, From: admin, Text: "/make_backup"}, false},
		{"stranger in private", telego.Message{Chat: telego.Chat{ID: 8, Type: telego.ChatTypePrivate}, From: stranger, Text: "/make_backup"}, false},
		{"admin other command", telego.Message{Chat: private, From: admin, Text: "/start"}, false},
		{"no sender", telego.Message{Chat: private, Text: "/make_backup"}, false},
	}
	for _, c := range cases {
		if got := isBackupRequest(&c.msg, "challengerbot", admins); got != c.want {
			t.Errorf("%s: isBackupRequest = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestUntilNextBackupHour(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	cases := []struct {
		now  time.Time
		want time.Duration
	}{
		{time.Date(2026, 10, 25, 3, 0, 0, 0, loc), time.Hour + wakeLag},
		{time.Date(2026, 10, 25, 4, 0, 0, 0, loc), 24*time.Hour + wakeLag},
		{time.Date(2026, 10, 25, 4, 0, 4, 0, loc), 24*time.Hour + time.Second},
		{time.Date(2026, 10, 25, 23, 30, 0, 0, loc), 4*time.Hour + 30*time.Minute + wakeLag},
	}
	for _, c := range cases {
		if got := untilNext(c.now, backupHour); got != c.want {
			t.Errorf("untilNext(%s) = %s, want %s", c.now.Format(time.TimeOnly), got, c.want)
		}
	}
}

func TestCleanupStaleSnapshots(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, ".backup-123")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	keep := filepath.Join(dir, "challenger.db")
	if err := os.WriteFile(keep, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	(&backuper{dir: dir}).removeStaleSnapshots()

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale snapshot dir: stat err = %v, want not exist", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("database removed: %v", err)
	}
}

func TestBackupFileNameAndCaption(t *testing.T) {
	at := time.Date(2026, 10, 25, 4, 0, 0, 0, time.UTC)

	if got, want := backupFileName(at), "challenger-20261025-0400.db"; got != want {
		t.Fatalf("backupFileName = %q, want %q", got, want)
	}
	if got, want := backupCaption(at, 24*1024-100, dailyBackup), "Бэкап базы (ежедневный) · 25.10.2026 04:00 · 24 КБ"; got != want {
		t.Fatalf("backupCaption = %q, want %q", got, want)
	}
}

func TestSnapshotLandsInTempDirNextToDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "challenger.db")
	conn, err := db.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = conn.Close() }()
	b := &backuper{conn: conn, dir: filepath.Dir(dbPath)}
	at := time.Date(2026, 10, 25, 4, 0, 0, 0, time.UTC)

	path, cleanup, err := b.snapshot(ctx, at)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if filepath.Base(path) != backupFileName(at) || filepath.Dir(filepath.Dir(path)) != filepath.Dir(dbPath) {
		t.Fatalf("snapshot path = %s, want %s in a temp dir next to %s", path, backupFileName(at), dbPath)
	}
	if err := backup.Check(ctx, path); err != nil {
		t.Fatalf("Check snapshot: %v", err)
	}
	cleanup()
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("temp dir after cleanup: stat err = %v, want not exist", err)
	}
}
