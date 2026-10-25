package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Config struct {
	BotToken       string
	AllowedChatIDs []int64
	AdminUserIDs   []int64
	DBPath         string
	Location       *time.Location
}

func Load() (Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return Config{}, errors.New("BOT_TOKEN is not set")
	}
	chatIDs, err := parseIDs("ALLOWED_CHAT_IDS")
	if err != nil {
		return Config{}, err
	}
	if len(chatIDs) == 0 {
		return Config{}, errors.New("ALLOWED_CHAT_IDS is not set")
	}
	adminIDs, err := parseIDs("ADMIN_USER_IDS")
	if err != nil {
		return Config{}, err
	}
	tz := envOr("CHALLENGE_TZ", "Europe/Moscow")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Config{}, fmt.Errorf("CHALLENGE_TZ %q: %w", tz, err)
	}
	return Config{
		BotToken:       token,
		AllowedChatIDs: chatIDs,
		AdminUserIDs:   adminIDs,
		DBPath:         envOr("DB_PATH", "challenger.db"),
		Location:       loc,
	}, nil
}

func parseIDs(key string) ([]int64, error) {
	var ids []int64
	for _, part := range strings.Split(os.Getenv(key), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not an id", key, part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
