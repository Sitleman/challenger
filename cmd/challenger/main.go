package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/Sitleman/challenger/internal/bot"
	"github.com/Sitleman/challenger/internal/config"
	"github.com/Sitleman/challenger/internal/db"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	return bot.Run(ctx, cfg, db.New(conn))
}
