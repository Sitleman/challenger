package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mymmrac/telego"
	"github.com/valyala/fasthttp"
)

func loadToken() string {
	if t := os.Getenv("BOT_TOKEN"); t != "" {
		return t
	}
	for _, p := range []string{".env", "../.env", "../../.env"} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "BOT_TOKEN=") {
				f.Close()
				return strings.TrimSpace(strings.TrimPrefix(line, "BOT_TOKEN="))
			}
		}
		f.Close()
	}
	return ""
}

func dump(label string, v any) {
	raw, _ := json.MarshalIndent(v, "", "  ")
	log.Printf("%s\n%s", label, raw)
}

func main() {
	token := loadToken()
	if token == "" {
		log.Fatal("BOT_TOKEN not found in env or .env")
	}

	// api.telegram.org resolves to IPv6-only in this environment; fasthttp needs a dual-stack dialer.
	httpClient := &fasthttp.Client{
		Dial: fasthttp.DialDualStack,
	}
	bot, err := telego.NewBot(token,
		telego.WithDefaultDebugLogger(),
		telego.WithFastHTTPClient(httpClient),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	me, err := bot.GetMe(ctx)
	if err != nil {
		log.Fatalf("getMe failed: %v", err)
	}
	log.Printf("bot @%s (id=%d) started; allowed_updates=[message, message_reaction]", me.Username, me.ID)

	updates, err := bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		AllowedUpdates: []string{"message", "message_reaction"},
		Timeout:        30,
	})
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for update := range updates {
			switch {
			case update.MessageReaction != nil:
				dump("🎯 MESSAGE_REACTION (per-user, has identity + delta):", update.MessageReaction)
			case update.MessageReactionCount != nil:
				dump("⚠️  MESSAGE_REACTION_COUNT (anonymous aggregate — no user, no delta):", update.MessageReactionCount)
			case update.Message != nil:
				dump("💬 MESSAGE:", update.Message)
			default:
				dump("… OTHER UPDATE:", update)
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	cancel()
}
