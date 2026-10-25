package bot

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/Sitleman/challenger/internal/challenge"
	"github.com/Sitleman/challenger/internal/config"
	"github.com/Sitleman/challenger/internal/db"
	"github.com/mymmrac/telego"
)

const (
	startChallengeCommand = "start_challenge"
	trackedReaction       = "👀"
	confirmedReaction     = "🏆"
)

type handler struct {
	bot      *telego.Bot
	username string
	q        *db.Queries
	loc      *time.Location
	allowed  map[int64]bool
	admins   map[int64]bool
	stats    *refresher
}

func newHandler(ctx context.Context, b *telego.Bot, username string, q *db.Queries, cfg config.Config) *handler {
	h := &handler{
		bot:      b,
		username: username,
		q:        q,
		loc:      cfg.Location,
		allowed:  idSet(cfg.AllowedChatIDs),
		admins:   idSet(cfg.AdminUserIDs),
	}
	h.stats = newRefresher(statsRefreshDelay, func(challengeID int64) {
		err := retry(ctx, renderAttempts, renderRetryDelay, func() error {
			return h.renderStats(ctx, challengeID)
		})
		if err != nil {
			log.Printf("render stats for challenge %d: %v", challengeID, err)
		}
	})
	return h
}

func idSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func (h *handler) handle(ctx context.Context, update telego.Update) {
	switch {
	case update.Message != nil && h.isAllowed(update.Message.Chat.ID):
		h.handleMessage(ctx, update.Message)
	case update.MessageReaction != nil && h.isAllowed(update.MessageReaction.Chat.ID):
		h.handleReaction(ctx, update.MessageReaction)
	}
}

func (h *handler) isAllowed(chatID int64) bool {
	if h.allowed[chatID] {
		return true
	}
	log.Printf("ignore update from chat %d: not in ALLOWED_CHAT_IDS", chatID)
	return false
}

func (h *handler) handleMessage(ctx context.Context, msg *telego.Message) {
	if msg.From == nil || msg.From.IsBot {
		return
	}
	switch {
	case commandName(msg.Text, h.username) == startChallengeCommand:
		h.startChallenge(ctx, msg)
	case hasHashtag(msg, challenge.Hashtag):
		h.recordProof(ctx, msg)
	}
}

func (h *handler) startChallenge(ctx context.Context, msg *telego.Message) {
	t := topicOf(msg)
	admin, err := h.isAdmin(ctx, t.chatID, msg.From.ID)
	if err != nil {
		log.Printf("check admin: %v", err)
		return
	}
	if !admin {
		h.reply(ctx, t, "Запустить челлендж может только админ группы")
		return
	}

	now := time.Now()
	sentAt := time.Unix(msg.Date, 0).In(h.loc)
	month := monthStart(sentAt, h.loc)
	_, err = h.q.GetChallengeByTopic(ctx, db.GetChallengeByTopicParams{
		ChatID: t.chatID, ThreadID: t.threadID, Month: monthKey(month),
	})
	switch {
	case err == nil:
		h.reply(ctx, t, "Челлендж на этот месяц в этой теме уже запущен")
		return
	case !errors.Is(err, sql.ErrNoRows):
		log.Printf("get challenge: %v", err)
		return
	}

	stats, err := h.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:          telego.ChatID{ID: t.chatID},
		MessageThreadID: int(t.threadID),
		Text:            challenge.NewBoard(month, sentAt.Day(), nil, now).Render(now),
		ParseMode:       telego.ModeHTML,
	})
	if err != nil {
		log.Printf("send stats: %v", err)
		return
	}
	_, err = h.q.CreateChallenge(ctx, db.CreateChallengeParams{
		ChatID:         t.chatID,
		ThreadID:       t.threadID,
		Month:          monthKey(month),
		StatsMessageID: int64(stats.MessageID),
		StartDay:       int64(sentAt.Day()),
	})
	if err != nil {
		log.Printf("create challenge: %v", err)
		h.deleteMessage(ctx, t.chatID, stats.MessageID)
		h.reply(ctx, t, "Не получилось запустить челлендж, попробуйте ещё раз")
		return
	}
	err = h.bot.PinChatMessage(ctx, &telego.PinChatMessageParams{
		ChatID: telego.ChatID{ID: t.chatID}, MessageID: stats.MessageID, DisableNotification: true,
	})
	if err != nil {
		log.Printf("pin stats: %v", err)
		h.reply(ctx, t, "Не получилось закрепить статистику — дайте боту право закреплять сообщения")
	}
}

func (h *handler) isAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	if h.admins[userID] {
		return true, nil
	}
	member, err := h.bot.GetChatMember(ctx, &telego.GetChatMemberParams{
		ChatID: telego.ChatID{ID: chatID}, UserID: userID,
	})
	if err != nil {
		return false, err
	}
	status := member.MemberStatus()
	return status == telego.MemberStatusCreator || status == telego.MemberStatusAdministrator, nil
}

func (h *handler) reply(ctx context.Context, t topic, text string) {
	_, err := h.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:          telego.ChatID{ID: t.chatID},
		MessageThreadID: int(t.threadID),
		Text:            text,
	})
	if err != nil {
		log.Printf("reply: %v", err)
	}
}

func (h *handler) deleteMessage(ctx context.Context, chatID int64, messageID int) {
	err := h.bot.DeleteMessage(ctx, &telego.DeleteMessageParams{ChatID: telego.ChatID{ID: chatID}, MessageID: messageID})
	if err != nil {
		log.Printf("delete message: %v", err)
	}
}

func (h *handler) recordProof(ctx context.Context, msg *telego.Message) {
	t := topicOf(msg)
	sentAt := time.Unix(msg.Date, 0).In(h.loc)
	c, err := h.q.GetChallengeByTopic(ctx, db.GetChallengeByTopicParams{
		ChatID: t.chatID, ThreadID: t.threadID, Month: monthKey(sentAt),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		log.Printf("get challenge: %v", err)
		return
	}
	if int64(sentAt.Day()) < c.StartDay {
		return
	}
	err = h.q.CreateProof(ctx, db.CreateProofParams{
		ChallengeID: c.ID,
		MessageID:   int64(msg.MessageID),
		UserID:      msg.From.ID,
		UserName:    displayName(msg.From),
		Day:         int64(sentAt.Day()),
	})
	if err != nil {
		log.Printf("create proof: %v", err)
		return
	}
	h.react(ctx, t.chatID, msg.MessageID, trackedReaction)
	h.stats.schedule(c.ID)
}

func (h *handler) handleReaction(ctx context.Context, r *telego.MessageReactionUpdated) {
	if r.User == nil || r.User.IsBot {
		return
	}
	p, err := h.q.FindProof(ctx, db.FindProofParams{ChatID: r.Chat.ID, MessageID: int64(r.MessageID)})
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		log.Printf("find proof: %v", err)
		return
	}
	if len(r.NewReaction) > 0 {
		err = h.q.AddReaction(ctx, db.AddReactionParams{ChallengeID: p.ChallengeID, MessageID: p.MessageID, UserID: r.User.ID})
	} else {
		err = h.q.RemoveReaction(ctx, db.RemoveReactionParams{ChallengeID: p.ChallengeID, MessageID: p.MessageID, UserID: r.User.ID})
	}
	if err != nil {
		log.Printf("update reaction: %v", err)
		return
	}
	h.stats.schedule(p.ChallengeID)

	count, err := h.q.CountReactions(ctx, db.CountReactionsParams{ChallengeID: p.ChallengeID, MessageID: p.MessageID})
	if err != nil {
		log.Printf("count reactions: %v", err)
		return
	}
	h.react(ctx, r.Chat.ID, r.MessageID, confirmationReaction(count))
}

func confirmationReaction(count int64) string {
	if count >= challenge.MinReactors {
		return confirmedReaction
	}
	return trackedReaction
}

func (h *handler) react(ctx context.Context, chatID int64, messageID int, emoji string) {
	err := h.bot.SetMessageReaction(ctx, &telego.SetMessageReactionParams{
		ChatID:    telego.ChatID{ID: chatID},
		MessageID: messageID,
		Reaction:  []telego.ReactionType{&telego.ReactionTypeEmoji{Type: telego.ReactionEmoji, Emoji: emoji}},
	})
	if err != nil {
		log.Printf("set reaction: %v", err)
	}
}
