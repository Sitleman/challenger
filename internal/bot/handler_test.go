package bot

import (
	"context"
	"testing"

	"github.com/Sitleman/challenger/internal/config"
)

func TestIsAdminTrustsConfiguredAdminsWithoutTelegram(t *testing.T) {
	h := newHandler(context.Background(), nil, "bot", nil, config.Config{AdminUserIDs: []int64{532029049}})

	admin, err := h.isAdmin(context.Background(), -1003436893374, 532029049)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !admin {
		t.Fatal("isAdmin = false, want true for user from ADMIN_USER_IDS")
	}
}
