package bot

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sitleman/challenger/internal/backup"
	"github.com/Sitleman/challenger/internal/config"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	makeBackupCommand = "make_backup"
	backupHour        = 4
	snapshotDirPrefix = ".backup-"
	backupFileLayout  = "20060102-1504"
	captionTimeLayout = "02.01.2006 15:04"
	sendAttempts      = 3
	sendRetryDelay    = 5 * time.Second
)

type backupKind string

const (
	dailyBackup     backupKind = "ежедневный"
	requestedBackup backupKind = "по запросу"
)

type backuper struct {
	bot        *telego.Bot
	conn       *sql.DB
	dir        string
	loc        *time.Location
	recipients []int64
	inFlight   sync.WaitGroup
}

func newBackuper(b *telego.Bot, conn *sql.DB, cfg config.Config) *backuper {
	return &backuper{
		bot:        b,
		conn:       conn,
		dir:        filepath.Dir(cfg.DBPath),
		loc:        cfg.Location,
		recipients: cfg.AdminUserIDs,
	}
}

func isBackupRequest(msg *telego.Message, botUsername string, admins map[int64]bool) bool {
	return msg.Chat.Type == telego.ChatTypePrivate &&
		msg.From != nil && admins[msg.From.ID] &&
		commandName(msg.Text, botUsername) == makeBackupCommand
}

func (b *backuper) start(ctx context.Context) {
	b.removeStaleSnapshots()
	if len(b.recipients) == 0 {
		log.Println("backups disabled: ADMIN_USER_IDS is not set")
		return
	}
	b.inFlight.Add(1)
	go func() {
		defer b.inFlight.Done()
		b.backupDaily(ctx)
	}()
}

func (b *backuper) removeStaleSnapshots() {
	dirs, err := filepath.Glob(filepath.Join(b.dir, snapshotDirPrefix+"*"))
	if err != nil {
		log.Printf("find stale snapshots: %v", err)
		return
	}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("remove stale snapshot %s: %v", dir, err)
		}
	}
}

func (b *backuper) backupDaily(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(untilNext(time.Now().In(b.loc), backupHour)):
		}
		b.send(ctx, b.recipients, dailyBackup)
	}
}

func (b *backuper) request(ctx context.Context, userID int64) {
	b.inFlight.Add(1)
	go func() {
		defer b.inFlight.Done()
		b.send(ctx, []int64{userID}, requestedBackup)
	}()
}

func (b *backuper) send(ctx context.Context, recipients []int64, kind backupKind) {
	now := time.Now().In(b.loc)
	path, cleanup, err := b.snapshot(ctx, now)
	if err != nil {
		b.fail(ctx, recipients, err)
		return
	}
	defer cleanup()
	info, err := os.Stat(path)
	if err != nil {
		b.fail(ctx, recipients, err)
		return
	}

	caption := backupCaption(now, info.Size(), kind)
	for _, id := range recipients {
		err := retry(ctx, sendAttempts, sendRetryDelay, func() error {
			return b.sendFile(ctx, id, path, caption)
		})
		if err != nil {
			log.Printf("send backup to %d: %v", id, err)
			continue
		}
		log.Printf("backup %s sent to %d", filepath.Base(path), id)
	}
}

func (b *backuper) snapshot(ctx context.Context, now time.Time) (string, func(), error) {
	dir, err := os.MkdirTemp(b.dir, snapshotDirPrefix)
	if err != nil {
		return "", nil, fmt.Errorf("create backup dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, backupFileName(now))
	if err := backup.Snapshot(ctx, b.conn, path); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func backupFileName(t time.Time) string {
	return "challenger-" + t.Format(backupFileLayout) + ".db"
}

func (b *backuper) fail(ctx context.Context, recipients []int64, cause error) {
	log.Printf("backup: %v", cause)
	notifyAdmins(ctx, b.bot, recipients, "Бэкап не получился: "+cause.Error())
}

func backupCaption(t time.Time, size int64, kind backupKind) string {
	kb := (size + 1023) / 1024
	return fmt.Sprintf("Бэкап базы (%s) · %s · %d КБ", kind, t.Format(captionTimeLayout), kb)
}

func (b *backuper) sendFile(ctx context.Context, chatID int64, path, caption string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = b.bot.SendDocument(ctx, &telego.SendDocumentParams{
		ChatID:   telego.ChatID{ID: chatID},
		Document: tu.File(tu.NameReader(f, filepath.Base(path))),
		Caption:  caption,
	})
	return err
}

func (b *backuper) wait() {
	b.inFlight.Wait()
}
