package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Sitleman/challenger/internal/challenge"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
)

const (
	wakeLag          = 5 * time.Second
	renderAttempts   = 3
	renderRetryDelay = 5 * time.Second
)

func (h *handler) refreshAtDayStart(ctx context.Context) {
	today := challenge.DayOf(time.Now(), h.loc)
	h.refreshMonth(ctx, today)
	h.refreshMonth(ctx, monthStart(today, h.loc).Add(-time.Hour))
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(untilNext(time.Now().In(h.loc), challenge.DayStartHour)):
		}
		h.refreshMonth(ctx, challenge.DayOf(time.Now(), h.loc).AddDate(0, 0, -1))
	}
}

func (h *handler) refreshMonth(ctx context.Context, day time.Time) {
	challenges, err := h.q.ListChallengesByMonth(ctx, monthKey(day))
	if err != nil {
		log.Printf("list challenges: %v", err)
		return
	}
	for _, c := range challenges {
		if h.allowed[c.ChatID] {
			h.stats.schedule(c.ID)
		}
	}
}

func untilNext(now time.Time, hour int) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, hour, 0, 0, 0, now.Location())
	}
	return next.Sub(now) + wakeLag
}

func retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i == attempts-1 || ctx.Err() != nil || !isTransient(err) {
			break
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(delay):
		}
	}
	return err
}

func isTransient(err error) bool {
	var apiErr *ta.Error
	return !errors.As(err, &apiErr)
}

func (h *handler) renderStats(ctx context.Context, challengeID int64) error {
	c, err := h.q.GetChallenge(ctx, challengeID)
	if err != nil {
		return fmt.Errorf("get challenge: %w", err)
	}
	rows, err := h.q.ListProofStats(ctx, challengeID)
	if err != nil {
		return fmt.Errorf("list proofs: %w", err)
	}
	month, err := time.ParseInLocation(monthLayout, c.Month, h.loc)
	if err != nil {
		return fmt.Errorf("parse month %q: %w", c.Month, err)
	}

	now := time.Now()
	_, err = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
		ChatID:    telego.ChatID{ID: c.ChatID},
		MessageID: int(c.StatsMessageID),
		Text:      challenge.NewBoard(month, int(c.StartDay), toProofs(rows), now).Render(now),
		ParseMode: telego.ModeHTML,
	})
	if err != nil && !strings.Contains(err.Error(), "message is not modified") {
		return fmt.Errorf("edit stats: %w", err)
	}
	return nil
}

func toProofs(rows []db.ListProofStatsRow) []challenge.Proof {
	proofs := make([]challenge.Proof, len(rows))
	for i, r := range rows {
		proofs[i] = challenge.Proof{
			UserID:   r.UserID,
			UserName: r.UserName,
			Day:      int(r.Day),
			Reactors: int(r.Reactors),
		}
	}
	return proofs
}
