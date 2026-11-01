package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/Sitleman/challenger/internal/config"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
)

type summaryAPI struct {
	mu         sync.Mutex
	messages   []telego.SendMessageParams
	edits      []telego.EditMessageTextParams
	failSends  int
	failSendAt int
	sendCalls  int
	reactions  int
	groupAdmin bool
}

func (a *summaryAPI) Call(_ context.Context, url string, data *ta.RequestData) (*ta.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := `true`
	switch url[strings.LastIndex(url, "/")+1:] {
	case "sendMessage":
		a.sendCalls++
		if a.failSends > 0 || a.sendCalls == a.failSendAt {
			a.failSends--
			return &ta.Response{Error: &ta.Error{ErrorCode: 403, Description: "Forbidden"}}, nil
		}
		var params telego.SendMessageParams
		if err := json.Unmarshal(data.BodyRaw, &params); err != nil {
			return nil, err
		}
		a.messages = append(a.messages, params)
		result = fmt.Sprintf(`{"message_id":%d,"date":1,"chat":{"id":-100123,"type":"supergroup"}}`, 100+len(a.messages))
	case "editMessageText":
		var params telego.EditMessageTextParams
		if err := json.Unmarshal(data.BodyRaw, &params); err != nil {
			return nil, err
		}
		a.edits = append(a.edits, params)
		result = `{"message_id":1,"date":1,"chat":{"id":-100123,"type":"supergroup"}}`
	case "setMessageReaction":
		a.reactions++
	case "getChatMember":
		status := "member"
		if a.groupAdmin {
			status = "administrator"
		}
		result = fmt.Sprintf(`{"status":%q,"user":{"id":8,"is_bot":false,"first_name":"Member"}}`, status)
	default:
		return nil, fmt.Errorf("unexpected method: %s", url)
	}
	return &ta.Response{Ok: true, Result: []byte(result)}, nil
}

func summaryHandler(t *testing.T) (*handler, *summaryAPI) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	api := &summaryAPI{}
	b, err := telego.NewBot("123456:"+strings.Repeat("a", 35), telego.WithAPICaller(api), telego.WithDiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	h := newHandler(ctx, b, "challengerbot", conn, config.Config{
		AllowedChatIDs: []int64{-100123}, AdminUserIDs: []int64{7}, Location: time.UTC,
	})
	h.stats.delay = 0
	t.Cleanup(func() {
		h.stats.wait()
		_ = conn.Close()
	})
	return h, api
}

func summaryCommand(at time.Time) *telego.Message {
	return &telego.Message{
		MessageID: 50, From: &telego.User{ID: 7, FirstName: "Admin"},
		Chat:           telego.Chat{ID: -100123, Type: telego.ChatTypeSupergroup},
		IsTopicMessage: true, MessageThreadID: 5, Text: "/stop_challenge", Date: at.Unix(),
	}
}

func summaryChallenge(t *testing.T, h *handler) db.Challenge {
	t.Helper()
	c, err := h.q.CreateChallenge(context.Background(), db.CreateChallengeParams{
		ChatID: -100123, ThreadID: 5, Month: "2026-10", StartDay: 24, StatsMessageID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStopChallengePublishesOnceAndIgnoresLateEvents(t *testing.T) {
	ctx := context.Background()
	h, api := summaryHandler(t)
	c := summaryChallenge(t, h)
	if err := h.q.CreateProof(ctx, db.CreateProofParams{ChallengeID: c.ID, MessageID: 10, UserID: 1, UserName: "Anna", Day: 24}); err != nil {
		t.Fatal(err)
	}
	for _, user := range []int64{2, 3} {
		if err := h.q.AddReaction(ctx, db.AddReactionParams{ChallengeID: c.ID, MessageID: 10, UserID: user, UserName: "Supporter"}); err != nil {
			t.Fatal(err)
		}
	}
	cmd := summaryCommand(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	cmd.Text += "@ChallengerBot"
	h.handle(ctx, telego.Update{Message: cmd})
	c, err := h.q.GetChallenge(ctx, c.ID)
	if err != nil || c.StoppedAt != cmd.Date || c.SummaryMessageID == 0 {
		t.Fatalf("stopped challenge = %+v, %v", c, err)
	}
	h.handle(ctx, telego.Update{MessageReaction: &telego.MessageReactionUpdated{
		Chat: cmd.Chat, MessageID: 10, User: &telego.User{ID: 2},
	}})
	late := *cmd
	late.From = &telego.User{ID: 1, FirstName: "Anna"}
	late.Text, late.Entities = "#task", []telego.MessageEntity{hashtag(0, 5)}
	h.handle(ctx, telego.Update{Message: &late})
	late.EditDate = late.Date + 1
	h.handle(ctx, telego.Update{EditedMessage: &late})
	h.handle(ctx, telego.Update{Message: cmd})
	h.stats.wait()
	rows, err := h.q.ListProofStats(ctx, c.ID)
	if err != nil || len(rows) != 1 || rows[0].Reactors != 2 {
		t.Fatalf("late updates changed stats: %+v, %v", rows, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	summaries := 0
	for _, msg := range api.messages {
		if strings.Contains(msg.Text, "🏁") {
			summaries++
			if msg.MessageThreadID != 5 || msg.ParseMode != telego.ModeHTML || !strings.Contains(msg.Text, "Anna — 1/3") || !strings.Contains(msg.Text, "Главная поддержка") {
				t.Fatalf("summary = %+v", msg)
			}
		}
	}
	if summaries != 1 || api.reactions != 0 {
		t.Fatalf("summaries = %d, reactions = %d, want 1 and 0", summaries, api.reactions)
	}
	if len(api.edits) == 0 || !strings.Contains(api.edits[len(api.edits)-1].Text, "Завершён 26.10") {
		t.Fatalf("final stats not frozen: %+v", api.edits)
	}
}

func TestStopChallengeCanRetryFailedPublicationWithoutChangingEndDay(t *testing.T) {
	h, api := summaryHandler(t)
	c := summaryChallenge(t, h)
	api.failSends = 1
	cmd := summaryCommand(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	h.handle(context.Background(), telego.Update{Message: cmd})
	cmd.Date += 24 * 60 * 60
	h.handle(context.Background(), telego.Update{Message: cmd})
	h.stats.wait()
	got, err := h.q.GetChallenge(context.Background(), c.ID)
	if err != nil || got.StoppedAt != cmd.Date-24*60*60 || got.SummaryMessageID == 0 {
		t.Fatalf("retried challenge = %+v, %v", got, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	last := api.messages[len(api.messages)-1]
	if !strings.Contains(last.Text, "24.10–26.10.2026") {
		t.Fatalf("retry moved end day: %s", last.Text)
	}
}

func TestStopChallengePreviousMonthAndGroupAdmin(t *testing.T) {
	h, api := summaryHandler(t)
	c := summaryChallenge(t, h)
	api.groupAdmin = true
	cmd := summaryCommand(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	cmd.From.ID = 8
	h.handle(context.Background(), telego.Update{Message: cmd})
	h.stats.wait()
	got, err := h.q.GetChallenge(context.Background(), c.ID)
	if err != nil || got.SummaryMessageID == 0 {
		t.Fatalf("previous month not stopped: %+v, %v", got, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if !strings.Contains(api.messages[0].Text, "24.10–31.10.2026 · 8 дней") {
		t.Fatalf("wrong previous-month period: %s", api.messages[0].Text)
	}
}

func TestStopChallengeRejectsNonAdminsAndWrongTopics(t *testing.T) {
	for _, name := range []string{"non-admin", "wrong topic", "wrong chat", "other bot", "no challenge"} {
		t.Run(name, func(t *testing.T) {
			h, _ := summaryHandler(t)
			var c db.Challenge
			if name != "no challenge" {
				c = summaryChallenge(t, h)
			}
			cmd := summaryCommand(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
			switch name {
			case "non-admin":
				cmd.From.ID = 8
			case "wrong topic":
				cmd.MessageThreadID = 6
			case "wrong chat":
				cmd.Chat.ID = -100999
			case "other bot":
				cmd.Text += "@OtherBot"
			}
			h.handle(context.Background(), telego.Update{Message: cmd})
			if c.ID != 0 {
				got, err := h.q.GetChallenge(context.Background(), c.ID)
				if err != nil || got.StoppedAt != 0 {
					t.Fatalf("challenge unexpectedly stopped: %+v, %v", got, err)
				}
			}
		})
	}
}

func TestStopChallengeResumesLongSummaryAfterPartialSend(t *testing.T) {
	ctx := context.Background()
	h, api := summaryHandler(t)
	c := summaryChallenge(t, h)
	for id := int64(10); id < 100; id++ {
		if err := h.q.CreateProof(ctx, db.CreateProofParams{ChallengeID: c.ID, MessageID: id, UserID: 1, UserName: "Anna", Day: 24}); err != nil {
			t.Fatal(err)
		}
		for _, user := range []int64{2, 3} {
			if err := h.q.AddReaction(ctx, db.AddReactionParams{ChallengeID: c.ID, MessageID: id, UserID: user, UserName: "B"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	api.failSendAt = 2
	cmd := summaryCommand(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))
	h.handle(ctx, telego.Update{Message: cmd})
	partial, err := h.q.GetChallenge(ctx, c.ID)
	if err != nil || partial.SummaryPartsSent != 1 || partial.SummaryMessageID == 0 {
		t.Fatalf("partial publication = %+v, %v", partial, err)
	}
	parts := summaryParts(partial.SummaryText)
	if len(parts) < 2 {
		t.Fatal("expected multipart summary")
	}
	h.handle(ctx, telego.Update{Message: cmd})
	h.stats.wait()
	complete, err := h.q.GetChallenge(ctx, c.ID)
	if err != nil || int(complete.SummaryPartsSent) != len(parts) || complete.SummaryText != partial.SummaryText || complete.SummaryMessageID != partial.SummaryMessageID {
		t.Fatalf("retry did not complete saved summary: %+v, %v", complete, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.messages) != len(parts)+1 {
		t.Fatalf("successful sends = %d, want %d parts and one error reply", len(api.messages), len(parts))
	}
	for i, part := range parts {
		messageIndex := i
		if i > 0 {
			messageIndex++
		}
		if api.messages[messageIndex].Text != part {
			t.Fatalf("part %d repeated or skipped", i)
		}
	}
}

func TestSummaryPartsPreserveHTMLLinesAndUnicode(t *testing.T) {
	line := "<a href=\"https://t.me/c/123/10\">" + strings.Repeat("🦊", 40) + "</a>"
	text := strings.Repeat(line+"\n", 100) + "Готово"
	parts := summaryParts(text)
	if len(parts) < 2 || strings.Join(parts, "\n") != text {
		t.Fatal("parts do not preserve the original text")
	}
	for _, part := range parts {
		if n := len(utf16.Encode([]rune(part))); n > 4096 {
			t.Fatalf("part exceeds Telegram limit: %d", n)
		}
		if strings.Count(part, "<a ") != strings.Count(part, "</a>") {
			t.Fatalf("HTML tag split across parts: %s", part)
		}
	}
}
