package bot

import (
	"context"
	"fmt"
	"log"

	"github.com/mymmrac/telego"
	"github.com/valyala/fasthttp"
)

func Run(ctx context.Context, token string) error {
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
		AllowedUpdates: []string{"message"},
		Timeout:        30,
	})
	if err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		consume(ctx, b, updates)
		close(done)
	}()

	select {
	case <-ctx.Done():
		log.Println("shutting down")
	case <-done:
	}
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

func consume(ctx context.Context, b *telego.Bot, updates <-chan telego.Update) {
	for update := range updates {
		if update.Message == nil {
			continue
		}
		handleMessage(ctx, b, update.Message)
	}
}

func handleMessage(ctx context.Context, b *telego.Bot, msg *telego.Message) {
	_, err := b.SendMessage(ctx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: msg.Chat.ID},
		Text:   replyText(msg),
	})
	if err != nil {
		log.Printf("send message: %v", err)
	}
}
