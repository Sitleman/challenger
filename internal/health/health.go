package health

import (
	"errors"
	"os"
	"time"
)

const Path = "/tmp/challenger-ready"

func Check(path string, now time.Time) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	age := now.Sub(info.ModTime())
	if age < 0 || age > 75*time.Second {
		return errors.New("poll heartbeat expired")
	}
	return nil
}

func Record(path string, ready bool) error {
	if ready {
		return os.WriteFile(path, nil, 0600)
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
