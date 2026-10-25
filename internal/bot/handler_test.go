package bot

import (
	"context"
	"testing"
	"time"

	"github.com/Sitleman/challenger/internal/config"
	"github.com/mymmrac/telego"
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

func TestEditedMessagesThatCannotBeProofsAreDroppedBeforeStorage(t *testing.T) {
	const chatID = -1003436893374
	loc := time.FixedZone("MSK", 3*60*60)
	h := newHandler(context.Background(), nil, "bot", nil, config.Config{AllowedChatIDs: []int64{chatID}, Location: loc})
	sent := time.Date(2026, time.October, 25, 9, 0, 0, 0, loc)
	edit := func(mutate func(*telego.Message)) *telego.Message {
		msg := &telego.Message{
			Chat:     telego.Chat{ID: chatID},
			From:     &telego.User{ID: 7, FirstName: "Anna"},
			Text:     "#task",
			Entities: []telego.MessageEntity{hashtag(0, 5)},
			Date:     sent.Unix(),
			EditDate: sent.Add(time.Hour).Unix(),
		}
		mutate(msg)
		return msg
	}
	tests := map[string]*telego.Message{
		"bot author":          edit(func(m *telego.Message) { m.From.IsBot = true }),
		"no author":           edit(func(m *telego.Message) { m.From = nil }),
		"no hashtag":          edit(func(m *telego.Message) { m.Entities = nil }),
		"edited next day":     edit(func(m *telego.Message) { m.EditDate = sent.AddDate(0, 0, 1).Unix() }),
		"chat is not allowed": edit(func(m *telego.Message) { m.Chat.ID = -100 }),
	}
	for name, msg := range tests {
		t.Run(name, func(t *testing.T) {
			h.handle(context.Background(), telego.Update{EditedMessage: msg})
		})
	}
}
