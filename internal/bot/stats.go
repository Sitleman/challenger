package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Sitleman/challenger/internal/challenge"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
)

const midnightLag = 5 * time.Second

func (h *handler) refreshAtMidnight(ctx context.Context) {
	now := time.Now().In(h.loc)
	h.refreshMonth(ctx, now)
	h.refreshMonth(ctx, monthStart(now, h.loc).Add(-time.Hour))
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(untilNextDay(time.Now().In(h.loc))):
		}
		h.refreshMonth(ctx, time.Now().In(h.loc).Add(-time.Hour))
	}
}

func (h *handler) refreshMonth(ctx context.Context, day time.Time) {
	challenges, err := h.q.ListChallengesByMonth(ctx, monthKey(day))
	if err != nil {
		log.Printf("list challenges: %v", err)
		return
	}
	for _, c := range challenges {
		h.stats.schedule(c.ID)
	}
}

func untilNextDay(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return next.Sub(now) + midnightLag
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
		Text:      challenge.NewBoard(month, toProofs(rows), now).Render(now),
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
