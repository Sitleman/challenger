package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Sitleman/challenger/internal/config"
	"github.com/mymmrac/telego"
)

type duplicateInstanceError struct {
	description string
}

func (e *duplicateInstanceError) Error() string {
	return "another bot instance is polling updates: " + e.description
}

func standBy(ctx context.Context, b *telego.Bot, cfg config.Config, username string, startedAt time.Time, dup *duplicateInstanceError) {
	log.Printf("stop handling updates until shutdown: %v", dup)
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	notifyAdmins(ctx, b, cfg.AdminUserIDs, duplicateInstanceText(username, host, startedAt.In(cfg.Location), dup))
	<-ctx.Done()
}

func duplicateInstanceText(username, host string, startedAt time.Time, dup *duplicateInstanceError) string {
	return fmt.Sprintf(
		"@%s: Telegram ответил 409 на getUpdates — вероятно, запущен второй экземпляр бота.\n\n"+
			"Этот экземпляр (host %s, запущен %s) перестал обрабатывать сообщения и ждёт остановки. "+
			"Остановите лишний экземпляр, а если остался не тот, перезапустите нужный.\n\n"+
			"Telegram: %s",
		username, host, startedAt.Format("02.01.2006 15:04:05 MST"), dup.description,
	)
}

func notifyAdmins(ctx context.Context, b *telego.Bot, adminIDs []int64, text string) {
	if len(adminIDs) == 0 {
		log.Print("ADMIN_USER_IDS is not set, nobody to notify")
		return
	}
	for _, id := range adminIDs {
		err := retry(ctx, renderAttempts, renderRetryDelay, func() error {
			_, err := b.SendMessage(ctx, &telego.SendMessageParams{ChatID: telego.ChatID{ID: id}, Text: text})
			return err
		})
		if err != nil {
			log.Printf("notify admin %d: %v", id, err)
		}
	}
}
