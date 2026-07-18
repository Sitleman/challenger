package bot

import (
	"testing"

	"github.com/mymmrac/telego"
)

func TestReplyTextEchoesTextMessage(t *testing.T) {
	msg := &telego.Message{Text: "hello"}

	if got := replyText(msg); got != "hello" {
		t.Fatalf("replyText = %q, want %q", got, "hello")
	}
}

func TestReplyTextStubForNonTextMessage(t *testing.T) {
	msg := &telego.Message{}

	if got := replyText(msg); got != "got your message" {
		t.Fatalf("replyText = %q, want %q", got, "got your message")
	}
}
