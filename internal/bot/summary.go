package bot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/Sitleman/challenger/internal/challenge"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
)

func (h *handler) stopChallenge(ctx context.Context, msg *telego.Message) {
	t := topicOf(msg)
	admin, err := h.isAdmin(ctx, t.chatID, msg.From.ID)
	if err != nil {
		log.Printf("check admin: %v", err)
		return
	}
	if !admin {
		h.reply(ctx, t, "Завершить челлендж может только админ группы")
		return
	}
	day := challenge.DayOf(time.Unix(msg.Date, 0), h.loc)
	c, err := h.q.GetLatestChallengeByTopic(ctx, db.GetLatestChallengeByTopicParams{
		ChatID: t.chatID, ThreadID: t.threadID, Month: monthKey(day),
	})
	if errors.Is(err, sql.ErrNoRows) {
		h.reply(ctx, t, "В этой теме ещё не запускали челлендж")
		return
	}
	if err != nil {
		log.Printf("get latest challenge: %v", err)
		h.reply(ctx, t, "Не получилось найти челлендж, попробуйте ещё раз")
		return
	}
	if c.SummaryText != "" && int(c.SummaryPartsSent) >= len(summaryParts(c.SummaryText)) {
		text := "Челлендж уже завершён, итоги опубликованы"
		if url := challenge.MessageURL(c.ChatID, c.SummaryMessageID); url != "" {
			text += ": " + url
		}
		h.reply(ctx, t, text)
		return
	}
	if c.StoppedAt == 0 {
		c, err = h.q.StopChallenge(ctx, db.StopChallengeParams{ID: c.ID, StoppedAt: msg.Date})
		if err != nil {
			log.Printf("stop challenge: %v", err)
			h.reply(ctx, t, "Не получилось завершить челлендж, попробуйте ещё раз")
			return
		}
	}
	h.stats.schedule(c.ID)
	if err := h.publishSummary(ctx, c); err != nil {
		log.Printf("publish summary for challenge %d: %v", c.ID, err)
		h.reply(ctx, t, "Челлендж завершён, но не получилось опубликовать итоги. Повторите /stop_challenge")
	}
}

func (h *handler) publishSummary(ctx context.Context, c db.Challenge) error {
	if c.SummaryText == "" {
		text, err := h.summaryText(ctx, c)
		if err != nil {
			return err
		}
		if err := h.q.SetSummaryText(ctx, db.SetSummaryTextParams{ID: c.ID, SummaryText: text}); err != nil {
			return fmt.Errorf("save summary text: %w", err)
		}
		c.SummaryText = text
	}
	parts := summaryParts(c.SummaryText)
	for i := int(c.SummaryPartsSent); i < len(parts); i++ {
		sent, err := h.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID: telego.ChatID{ID: c.ChatID}, MessageThreadID: int(c.ThreadID),
			Text: parts[i], ParseMode: telego.ModeHTML,
			LinkPreviewOptions: &telego.LinkPreviewOptions{IsDisabled: true},
		})
		if err != nil {
			return fmt.Errorf("send summary part %d: %w", i+1, err)
		}
		if c.SummaryMessageID == 0 {
			c.SummaryMessageID = int64(sent.MessageID)
		}
		if err := h.q.SetSummaryProgress(ctx, db.SetSummaryProgressParams{
			ID: c.ID, SummaryMessageID: c.SummaryMessageID, SummaryPartsSent: int64(i + 1),
		}); err != nil {
			h.deleteMessage(ctx, c.ChatID, sent.MessageID)
			return fmt.Errorf("save summary progress: %w", err)
		}
	}
	return nil
}

func summaryParts(text string) []string {
	const limit = 4000
	var parts []string
	var current []string
	size := 0
	for _, line := range strings.Split(text, "\n") {
		n := len(utf16.Encode([]rune(line)))
		if size+n+1 > limit && len(current) > 0 {
			parts = append(parts, strings.Join(current, "\n"))
			current, size = nil, 0
		}
		current = append(current, line)
		size += n + 1
	}
	return append(parts, strings.Join(current, "\n"))
}

func (h *handler) summaryText(ctx context.Context, c db.Challenge) (string, error) {
	month, err := time.ParseInLocation(monthLayout, c.Month, h.loc)
	if err != nil {
		return "", fmt.Errorf("parse month %q: %w", c.Month, err)
	}
	rows, err := h.q.ListProofStats(ctx, c.ID)
	if err != nil {
		return "", fmt.Errorf("list proofs: %w", err)
	}
	proofs := toProofs(rows)
	board := challenge.NewFinalBoard(month, int(c.StartDay), proofs, time.Unix(c.StoppedAt, 0))
	stats, err := h.q.ListSupportStats(ctx, db.ListSupportStatsParams{ChallengeID: c.ID, EndDay: int64(board.Shown)})
	if err != nil {
		return "", fmt.Errorf("list support: %w", err)
	}
	supporters := make([]challenge.Supporter, len(stats))
	for i, r := range stats {
		name := r.UserName
		if name == "" && r.Recipients == stats[0].Recipients {
			member, err := h.bot.GetChatMember(ctx, &telego.GetChatMemberParams{ChatID: telego.ChatID{ID: c.ChatID}, UserID: r.UserID})
			if err == nil {
				user := member.MemberUser()
				name = displayName(&user)
			}
		}
		supporters[i] = challenge.Supporter{UserID: r.UserID, Name: name, Recipients: int(r.Recipients)}
	}
	return (challenge.Summary{Board: board, Proofs: proofs, Supporters: supporters, ChatID: c.ChatID}).Render(), nil
}
