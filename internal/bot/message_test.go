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
		{telego.User{ID: 1, FirstName: "Anna", LastName: "K", Username: "anna"}, "Anna"},
		{telego.User{ID: 1, Username: "anna"}, "anna"},
		{telego.User{ID: 42}, "42"},
	}
	for _, tt := range tests {
		if got := displayName(&tt.user); got != tt.want {
			t.Errorf("displayName(%+v) = %q, want %q", tt.user, got, tt.want)
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
