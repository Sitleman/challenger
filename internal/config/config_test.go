package config

import "testing"

func TestLoadReturnsTokenFromEnv(t *testing.T) {
	t.Setenv("BOT_TOKEN", "secret-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BotToken != "secret-token" {
		t.Fatalf("BotToken = %q, want %q", cfg.BotToken, "secret-token")
	}
}

func TestLoadErrorsWhenTokenMissing(t *testing.T) {
	t.Setenv("BOT_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when BOT_TOKEN is empty, got nil")
	}
}
