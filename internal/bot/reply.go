package bot

import "github.com/mymmrac/telego"

func replyText(msg *telego.Message) string {
	if msg.Text != "" {
		return msg.Text
	}
	return "got your message"
}
