package bot

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
)

const pollRetryDelay = 8 * time.Second

type updatesFetcher func(ctx context.Context, params *telego.GetUpdatesParams) ([]telego.Update, error)

func poll(ctx context.Context, fetch updatesFetcher, params telego.GetUpdatesParams, retryDelay time.Duration, out chan<- telego.Update) error {
	for {
		updates, err := fetch(ctx, &params)
		if ctx.Err() != nil {
			return nil
		}
		if conflict, ok := asConflict(err); ok {
			return &duplicateInstanceError{description: conflict.Description}
		}
		if err != nil {
			log.Printf("get updates: %v", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryDelay):
			}
			continue
		}
		for _, update := range updates {
			if update.UpdateID < params.Offset {
				continue
			}
			params.Offset = update.UpdateID + 1
			select {
			case <-ctx.Done():
				return nil
			case out <- update:
			}
		}
	}
}

func asConflict(err error) (*ta.Error, bool) {
	var apiErr *ta.Error
	if errors.As(err, &apiErr) && apiErr.ErrorCode == http.StatusConflict {
		return apiErr, true
	}
	return nil, false
}
