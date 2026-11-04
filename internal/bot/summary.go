package bot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Sitleman/challenger/internal/challenge"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
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
	if !c.FinishDay.Valid {
		month, err := time.ParseInLocation(monthLayout, c.Month, h.loc)
		if err != nil {
			log.Printf("parse month %q: %v", c.Month, err)
			return
		}
		finishDay := challenge.NewBoard(month, int(c.StartDay), nil, time.Unix(msg.Date, 0)).Shown
		c, err = h.q.StopChallenge(ctx, db.StopChallengeParams{
			ID: c.ID, FinishDay: sql.NullInt64{Int64: int64(finishDay), Valid: true},
		})
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
	text, err := h.summaryText(ctx, c)
	if err != nil {
		return err
	}
	if c.TgApiMessageID != 0 {
		_, err := h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID: telego.ChatID{ID: c.ChatID}, MessageID: int(c.TgApiMessageID),
			Text: text, ParseMode: telego.ModeHTML,
			LinkPreviewOptions: &telego.LinkPreviewOptions{IsDisabled: true},
		})
		if err == nil || strings.Contains(err.Error(), "message is not modified") {
			return nil
		}
		var apiErr *telegoapi.Error
		if !errors.As(err, &apiErr) || apiErr.ErrorCode != 400 || apiErr.Description != "Bad Request: message to edit not found" {
			return fmt.Errorf("edit summary: %w", err)
		}
	}
	sent, err := h.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: c.ChatID}, MessageThreadID: int(c.ThreadID),
		Text: text, ParseMode: telego.ModeHTML,
		LinkPreviewOptions: &telego.LinkPreviewOptions{IsDisabled: true},
	})
	if err != nil {
		return fmt.Errorf("send summary: %w", err)
	}
	if err := h.q.SetSummaryMessage(ctx, db.SetSummaryMessageParams{ID: c.ID, TgApiMessageID: int64(sent.MessageID)}); err != nil {
		h.deleteMessage(ctx, c.ChatID, sent.MessageID)
		return fmt.Errorf("save summary message: %w", err)
	}
	return nil
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
	board := challenge.NewFinalBoard(month, int(c.StartDay), int(c.FinishDay.Int64), proofs)
	return (challenge.Summary{
		Board: board, Proofs: proofs, ChatID: c.ChatID, StatsMessageID: c.StatsMessageID,
	}).Render()
}
