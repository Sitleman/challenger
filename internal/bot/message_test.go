package bot

import (
	"testing"
	"time"

	"github.com/mymmrac/telego"
)

func hashtag(offset, length int) telego.MessageEntity {
	return telego.MessageEntity{Type: telego.EntityTypeHashtag, Offset: offset, Length: length}
}

func TestHasHashtag(t *testing.T) {
	tests := []struct {
		name string
		msg  telego.Message
		want bool
	}{
		{
			name: "text after cyrillic and emoji",
			msg:  telego.Message{Text: "Пробежка 🏃 #task", Entities: []telego.MessageEntity{hashtag(12, 5)}},
			want: true,
		},
		{
			name: "caption is case insensitive",
			msg:  telego.Message{Caption: "#TASK done", CaptionEntities: []telego.MessageEntity{hashtag(0, 5)}},
			want: true,
		},
		{
			name: "longer hashtag does not match",
			msg:  telego.Message{Text: "#tasks", Entities: []telego.MessageEntity{hashtag(0, 6)}},
			want: false,
		},
		{
			name: "plain text without entity does not match",
			msg:  telego.Message{Text: "#task"},
			want: false,
		},
		{
			name: "entity out of range is ignored",
			msg:  telego.Message{Text: "#t", Entities: []telego.MessageEntity{hashtag(0, 5)}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasHashtag(&tt.msg, "#task"); got != tt.want {
				t.Fatalf("hasHashtag = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandName(t *testing.T) {
	tests := map[string]string{
		"/start_challenge":                    "start_challenge",
		"/start_challenge@challenger_bot":     "start_challenge",
		"/start_challenge@Challenger_Bot now": "start_challenge",
		"/start_challenge@other_bot":          "",
		"hi /start_challenge":                 "",
	}
	for text, want := range tests {
		if got := commandName(text, "challenger_bot"); got != want {
			t.Errorf("commandName(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestTopicOf(t *testing.T) {
	inTopic := &telego.Message{Chat: telego.Chat{ID: -100}, MessageThreadID: 7, IsTopicMessage: true}
	reply := &telego.Message{Chat: telego.Chat{ID: -100}, MessageThreadID: 7}

	if got := topicOf(inTopic); got != (topic{chatID: -100, threadID: 7}) {
		t.Fatalf("topicOf(topic message) = %+v", got)
	}
	if got := topicOf(reply); got != (topic{chatID: -100, threadID: 0}) {
		t.Fatalf("topicOf(non-topic message) = %+v", got)
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		user telego.User
		want string
	}{
		{telego.User{ID: 1, FirstName: "Anna", LastName: "K", Username: "anna"}, "Anna (@anna)"},
		{telego.User{ID: 1, FirstName: "Anna"}, "Anna"},
		{telego.User{ID: 1, Username: "anna"}, "@anna"},
		{telego.User{ID: 42}, "42"},
	}
	for _, tt := range tests {
		if got := displayName(&tt.user); got != tt.want {
			t.Errorf("displayName(%+v) = %q, want %q", tt.user, got, tt.want)
		}
	}
}

func TestConfirmationReaction(t *testing.T) {
	tests := map[int64]string{0: trackedReaction, 1: trackedReaction, 2: confirmedReaction, 5: confirmedReaction}
	for count, want := range tests {
		if got := confirmationReaction(count); got != want {
			t.Errorf("confirmationReaction(%d) = %q, want %q", count, got, want)
		}
	}
}

func TestMonthStart(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	sentAt := time.Date(2026, time.October, 31, 22, 30, 0, 0, time.UTC)

	got := monthStart(sentAt, loc)

	if want := time.Date(2026, time.November, 1, 0, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("monthStart = %v, want %v", got, want)
	}
	if monthKey(got) != "2026-11" {
		t.Fatalf("monthKey = %q, want 2026-11", monthKey(got))
	}
}

func TestEditedSameDay(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	at := func(month time.Month, day, hour, minute int) int64 {
		return time.Date(2026, month, day, hour, minute, 0, 0, time.UTC).Unix()
	}
	tests := []struct {
		name     string
		sent     int64
		editedAt int64
		want     bool
	}{
		{name: "edited before day start", sent: at(time.October, 25, 6, 0), editedAt: at(time.October, 25, 23, 59), want: true},
		{name: "edited after day start", sent: at(time.October, 25, 20, 0), editedAt: at(time.October, 26, 0, 1), want: false},
		{name: "edited across local midnight", sent: at(time.October, 25, 20, 0), editedAt: at(time.October, 25, 21, 1), want: true},
		{name: "sent before day start edited after", sent: at(time.October, 24, 23, 30), editedAt: at(time.October, 25, 0, 30), want: false},
		{name: "same day number next month", sent: at(time.October, 25, 6, 0), editedAt: at(time.November, 25, 6, 0), want: false},
		{name: "not edited", sent: at(time.October, 25, 6, 0), editedAt: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &telego.Message{Date: tt.sent, EditDate: tt.editedAt}
			if got := editedSameDay(msg, loc); got != tt.want {
				t.Fatalf("editedSameDay = %v, want %v", got, tt.want)
			}
		})
	}
}
