package bot

import (
	"context"
	"errors"
	"fmt"
	"testing"

	ta "github.com/mymmrac/telego/telegoapi"
)

func TestRetryStopsOnSuccess(t *testing.T) {
	calls := 0
	err := retry(context.Background(), 3, 0, func() error {
		calls++
		if calls < 2 {
			return errors.New("timeout")
		}
		return nil
	})

	if err != nil || calls != 2 {
		t.Fatalf("retry = %v after %d calls, want nil after 2", err, calls)
	}
}

func TestRetryGivesUpAfterAttempts(t *testing.T) {
	calls := 0
	err := retry(context.Background(), 3, 0, func() error {
		calls++
		return errors.New("timeout")
	})

	if err == nil || calls != 3 {
		t.Fatalf("retry = %v after %d calls, want error after 3", err, calls)
	}
}

func TestRetryStopsWhenContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := retry(ctx, 3, 0, func() error {
		calls++
		cancel()
		return errors.New("timeout")
	})

	if err == nil || calls != 1 {
		t.Fatalf("retry = %v after %d calls, want error after 1", err, calls)
	}
}

func TestRetryDoesNotRepeatTelegramAPIErrors(t *testing.T) {
	calls := 0
	err := retry(context.Background(), 3, 0, func() error {
		calls++
		return fmt.Errorf("edit stats: %w", &ta.Error{ErrorCode: 400, Description: "message to edit not found"})
	})

	if err == nil || calls != 1 {
		t.Fatalf("retry = %v after %d calls, want error after 1", err, calls)
	}
}
