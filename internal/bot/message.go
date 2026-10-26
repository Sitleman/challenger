package bot

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/Sitleman/challenger/internal/challenge"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const monthLayout = "2006-01"

type topic struct {
	chatID   int64
	threadID int64
}

func topicOf(msg *telego.Message) topic {
	t := topic{chatID: msg.Chat.ID}
	if msg.IsTopicMessage {
		t.threadID = int64(msg.MessageThreadID)
	}
	return t
}

func commandName(text, botUsername string) string {
	cmd, mention, _ := tu.ParseCommand(text)
	if mention != "" && !strings.EqualFold(mention, botUsername) {
		return ""
	}
	return cmd
}

func hasHashtag(msg *telego.Message, tag string) bool {
	return containsHashtag(msg.Text, msg.Entities, tag) || containsHashtag(msg.Caption, msg.CaptionEntities, tag)
}

func containsHashtag(text string, entities []telego.MessageEntity, tag string) bool {
	units := utf16.Encode([]rune(text))
	for _, e := range entities {
		if e.Type == telego.EntityTypeHashtag && strings.EqualFold(entityText(units, e), tag) {
			return true
		}
	}
	return false
}

func entityText(units []uint16, e telego.MessageEntity) string {
	if e.Offset < 0 || e.Length <= 0 || e.Offset+e.Length > len(units) {
		return ""
	}
	return string(utf16.Decode(units[e.Offset : e.Offset+e.Length]))
}

func editedSameDay(msg *telego.Message, loc *time.Location) bool {
	if msg.EditDate == 0 {
		return false
	}
	sent := challenge.DayOf(time.Unix(msg.Date, 0), loc)
	edited := challenge.DayOf(time.Unix(msg.EditDate, 0), loc)
	return sent.Format(time.DateOnly) == edited.Format(time.DateOnly)
}

func displayName(u *telego.User) string {
	switch {
	case u.FirstName != "" && u.Username != "":
		return u.FirstName + " (@" + u.Username + ")"
	case u.FirstName != "":
		return u.FirstName
	case u.Username != "":
		return "@" + u.Username
	default:
		return strconv.FormatInt(u.ID, 10)
	}
}

func monthStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

func monthKey(t time.Time) string {
	return t.Format(monthLayout)
}
