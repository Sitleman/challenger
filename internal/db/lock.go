package db

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var ErrLocked = errors.New("database is in use by another process")

type Lock struct {
	file *os.File
}

func AcquireLock(dbPath string) (*Lock, error) {
	path := dbPath + ".lock"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%s: %w", path, ErrLocked)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &Lock{file: f}, nil
}

func (l *Lock) Release() error {
	return l.file.Close()
}
