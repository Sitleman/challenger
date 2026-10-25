package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sitleman/challenger/internal/backup"
	"github.com/Sitleman/challenger/internal/bot"
	"github.com/Sitleman/challenger/internal/config"
	"github.com/Sitleman/challenger/internal/db"
)

const restoreCommand = "restore"

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(args) > 0 && args[0] == restoreCommand {
		return restore(ctx, args[1:])
	}
	return serve(ctx)
}

func restore(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: challenger restore <backup.db>")
	}
	dbPath := config.DBPath()
	aside, err := backup.Restore(ctx, args[0], dbPath, time.Now())
	if aside != "" {
		log.Printf("previous database moved to %s", aside)
	}
	if errors.Is(err, db.ErrLocked) {
		return fmt.Errorf("stop the bot before restoring: %w", err)
	}
	if err != nil {
		return err
	}
	log.Printf("restored %s into %s", args[0], dbPath)
	return nil
}

func serve(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	lock, err := db.AcquireLock(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	return bot.Run(ctx, cfg, conn)
}
