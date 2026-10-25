package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Sitleman/challenger/internal/config"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
	"github.com/valyala/fasthttp"
)

const statsRefreshDelay = 3 * time.Second

func Run(ctx context.Context, cfg config.Config, q *db.Queries) error {
	startedAt := time.Now()
	b, err := newBot(cfg.BotToken)
	if err != nil {
		return err
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("getMe: %w", err)
	}
	log.Printf("bot @%s (id=%d) started", me.Username, me.ID)

	workCtx, stopWork := context.WithCancel(ctx)
	defer stopWork()
	h := newHandler(workCtx, b, me.Username, q, cfg)
	go h.refreshAtMidnight(workCtx)

	updates := make(chan telego.Update)
	polled := make(chan error, 1)
	go func() {
		defer close(updates)
		polled <- poll(workCtx, b.GetUpdates, telego.GetUpdatesParams{
			AllowedUpdates: []string{"message", "message_reaction"},
			Timeout:        30,
		}, pollRetryDelay, updates)
	}()

	consume(workCtx, h, updates)
	stopWork()
	h.stats.wait()

	var dup *duplicateInstanceError
	if errors.As(<-polled, &dup) {
		standBy(ctx, b, cfg, me.Username, startedAt, dup)
	}
	log.Println("shutting down")
	return nil
}

func newBot(token string) (*telego.Bot, error) {
	// api.telegram.org resolves to IPv6-only in this environment; fasthttp needs a dual-stack dialer.
	httpClient := &fasthttp.Client{Dial: fasthttp.DialDualStack}
	return telego.NewBot(token,
		telego.WithDefaultDebugLogger(),
		telego.WithFastHTTPClient(httpClient),
	)
}

func consume(ctx context.Context, h *handler, updates <-chan telego.Update) {
	for update := range updates {
		h.handle(ctx, update)
	}
}
