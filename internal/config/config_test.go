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

func TestLoadDefaults(t *testing.T) {
	t.Setenv("BOT_TOKEN", "secret-token")
	t.Setenv("DB_PATH", "")
	t.Setenv("CHALLENGE_TZ", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBPath != "challenger.db" {
		t.Fatalf("DBPath = %q, want %q", cfg.DBPath, "challenger.db")
	}
	if cfg.Location.String() != "Europe/Moscow" {
		t.Fatalf("Location = %q, want %q", cfg.Location, "Europe/Moscow")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("BOT_TOKEN", "secret-token")
	t.Setenv("DB_PATH", "/data/stage.db")
	t.Setenv("CHALLENGE_TZ", "Asia/Yekaterinburg")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBPath != "/data/stage.db" || cfg.Location.String() != "Asia/Yekaterinburg" {
		t.Fatalf("cfg = %q %q, want overrides", cfg.DBPath, cfg.Location)
	}
}

func TestLoadErrorsOnUnknownTimezone(t *testing.T) {
	t.Setenv("BOT_TOKEN", "secret-token")
	t.Setenv("CHALLENGE_TZ", "Mars/Olympus")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for unknown CHALLENGE_TZ, got nil")
	}
}
