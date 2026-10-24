package config

import (
	"errors"
	"fmt"
	"os"
	"time"
	_ "time/tzdata"
)

type Config struct {
	BotToken string
	DBPath   string
	Location *time.Location
}

func Load() (Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return Config{}, errors.New("BOT_TOKEN is not set")
	}
	tz := envOr("CHALLENGE_TZ", "Europe/Moscow")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Config{}, fmt.Errorf("CHALLENGE_TZ %q: %w", tz, err)
	}
	return Config{
		BotToken: token,
		DBPath:   envOr("DB_PATH", "challenger.db"),
		Location: loc,
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
