package config

import (
	"errors"
	"os"
)

type Config struct {
	BotToken string
}

func Load() (Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return Config{}, errors.New("BOT_TOKEN is not set")
	}
	return Config{BotToken: token}, nil
}
