package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
	"github.com/valyala/fasthttp"
)

const statsRefreshDelay = 3 * time.Second

func Run(ctx context.Context, token string, q *db.Queries, loc *time.Location) error {
	b, err := newBot(token)
	if err != nil {
		return err
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("getMe: %w", err)
	}
	log.Printf("bot @%s (id=%d) started", me.Username, me.ID)

	updates, err := b.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		AllowedUpdates: []string{"message", "message_reaction"},
		Timeout:        30,
	})
	if err != nil {
		return err
	}

	h := newHandler(ctx, b, me.Username, q, loc)
	go h.refreshAtMidnight(ctx)

	consume(ctx, h, updates)
	log.Println("shutting down")
	h.stats.wait()
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
