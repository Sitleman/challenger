package bot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
)

type fetchResult struct {
	updates []telego.Update
	err     error
}

type fakeFetcher struct {
	results []fetchResult
	offsets []int
	cancel  context.CancelFunc
}

func (f *fakeFetcher) fetch(ctx context.Context, params *telego.GetUpdatesParams) ([]telego.Update, error) {
	f.offsets = append(f.offsets, params.Offset)
	if len(f.results) == 0 {
		f.cancel()
		return nil, ctx.Err()
	}
	r := f.results[0]
	f.results = f.results[1:]
	return r.updates, r.err
}

func runPoll(t *testing.T, results ...fetchResult) ([]int, *fakeFetcher, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeFetcher{results: results, cancel: cancel}
	out := make(chan telego.Update)
	done := make(chan error, 1)
	go func() {
		defer close(out)
		done <- poll(ctx, f.fetch, telego.GetUpdatesParams{}, 0, out)
	}()
	var ids []int
	for u := range out {
		ids = append(ids, u.UpdateID)
	}
	return ids, f, <-done
}

func updates(ids ...int) []telego.Update {
	us := make([]telego.Update, len(ids))
	for i, id := range ids {
		us[i] = telego.Update{UpdateID: id}
	}
	return us
}

func TestPollStopsOnConflict(t *testing.T) {
	conflict := &ta.Error{ErrorCode: 409, Description: "Conflict: terminated by other getUpdates request"}

	ids, _, err := runPoll(t, fetchResult{updates: updates(1)}, fetchResult{err: conflict})

	var dup *duplicateInstanceError
	if !errors.As(err, &dup) {
		t.Fatalf("err = %v, want duplicateInstanceError", err)
	}
	if dup.description != conflict.Description {
		t.Fatalf("description = %q, want %q", dup.description, conflict.Description)
	}
	if want := []int{1}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("updates = %v, want %v", ids, want)
	}
}

func TestPollDetectsConflictFromBotAPI(t *testing.T) {
	const description = "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = fmt.Fprintf(w, `{"ok":false,"error_code":409,"description":%q}`, description)
	}))
	defer srv.Close()
	b, err := telego.NewBot("123456:"+strings.Repeat("a", 35), telego.WithAPIServer(srv.URL), telego.WithDiscardLogger())
	if err != nil {
		t.Fatalf("new bot: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = poll(ctx, b.GetUpdates, telego.GetUpdatesParams{}, 0, make(chan telego.Update))

	var dup *duplicateInstanceError
	if !errors.As(err, &dup) || dup.description != description {
		t.Fatalf("err = %v, want duplicateInstanceError with %q", err, description)
	}
}

func TestPollRetriesOtherErrors(t *testing.T) {
	ids, _, err := runPoll(t,
		fetchResult{err: errors.New("connection reset")},
		fetchResult{err: &ta.Error{ErrorCode: 502, Description: "Bad Gateway"}},
		fetchResult{updates: updates(7)},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{7}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("updates = %v, want %v", ids, want)
	}
}

func TestPollAdvancesOffsetAndSkipsSeenUpdates(t *testing.T) {
	ids, f, err := runPoll(t, fetchResult{updates: updates(1, 2)}, fetchResult{updates: updates(2, 3)})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("updates = %v, want %v", ids, want)
	}
	if want := []int{0, 3, 4}; !reflect.DeepEqual(f.offsets, want) {
		t.Fatalf("offsets = %v, want %v", f.offsets, want)
	}
}

func TestPollIgnoresConflictAfterShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fetch := func(context.Context, *telego.GetUpdatesParams) ([]telego.Update, error) {
		cancel()
		return nil, &ta.Error{ErrorCode: 409, Description: "Conflict"}
	}

	if err := poll(ctx, fetch, telego.GetUpdatesParams{}, 0, make(chan telego.Update)); err != nil {
		t.Fatalf("err = %v, want nil after shutdown", err)
	}
}
