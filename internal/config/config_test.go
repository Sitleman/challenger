package config

import (
	"reflect"
	"testing"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("BOT_TOKEN", "secret-token")
	t.Setenv("ALLOWED_CHAT_IDS", "-100")
}

func TestLoadReturnsTokenFromEnv(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BotToken != "secret-token" {
		t.Fatalf("BotToken = %q, want %q", cfg.BotToken, "secret-token")
	}
}

func TestLoadErrorsWhenTokenMissing(t *testing.T) {
	setRequired(t)
	t.Setenv("BOT_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when BOT_TOKEN is empty, got nil")
	}
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)
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
	setRequired(t)
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
	setRequired(t)
	t.Setenv("CHALLENGE_TZ", "Mars/Olympus")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for unknown CHALLENGE_TZ, got nil")
	}
}

func TestLoadParsesAllowedChatIDs(t *testing.T) {
	setRequired(t)
	t.Setenv("ALLOWED_CHAT_IDS", " -1004391088785, -100200 ,")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int64{-1004391088785, -100200}; !reflect.DeepEqual(cfg.AllowedChatIDs, want) {
		t.Fatalf("AllowedChatIDs = %v, want %v", cfg.AllowedChatIDs, want)
	}
}

func TestLoadErrorsOnBadAllowedChatIDs(t *testing.T) {
	for _, value := range []string{"", " , ", "-100,abc"} {
		setRequired(t)
		t.Setenv("ALLOWED_CHAT_IDS", value)

		if _, err := Load(); err == nil {
			t.Errorf("ALLOWED_CHAT_IDS=%q: expected error, got nil", value)
		}
	}
}

func TestLoadParsesAdminUserIDs(t *testing.T) {
	setRequired(t)
	t.Setenv("ADMIN_USER_IDS", "532029049, 42")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int64{532029049, 42}; !reflect.DeepEqual(cfg.AdminUserIDs, want) {
		t.Fatalf("AdminUserIDs = %v, want %v", cfg.AdminUserIDs, want)
	}
}

func TestLoadAllowsEmptyAdminUserIDs(t *testing.T) {
	setRequired(t)
	t.Setenv("ADMIN_USER_IDS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.AdminUserIDs) != 0 {
		t.Fatalf("AdminUserIDs = %v, want empty", cfg.AdminUserIDs)
	}
}

func TestLoadErrorsOnBadAdminUserIDs(t *testing.T) {
	setRequired(t)
	t.Setenv("ADMIN_USER_IDS", "532029049,abc")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for bad ADMIN_USER_IDS, got nil")
	}
}
