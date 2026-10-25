package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLockIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	first, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}

	if _, err := AcquireLock(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second AcquireLock err = %v, want ErrLocked", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	again, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("AcquireLock after release: %v", err)
	}
	_ = again.Release()
}
