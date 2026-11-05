package health

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadinessRequiresRecentSuccessfulPoll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ready")
	now := time.Now()
	if Check(path, now) == nil {
		t.Fatal("missing heartbeat is healthy")
	}
	if err := Record(path, true); err != nil {
		t.Fatal(err)
	}
	if err := Check(path, time.Now()); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-2 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if Check(path, now) == nil {
		t.Fatal("stale heartbeat is healthy")
	}
	if err := Record(path, true); err != nil {
		t.Fatal(err)
	}
	if err := Record(path, false); err != nil {
		t.Fatal(err)
	}
	if Check(path, now) == nil {
		t.Fatal("failed poll is healthy")
	}
}
