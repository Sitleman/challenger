package bot

import (
	"sync"
	"testing"
	"time"
)

func TestRefresherCoalescesBurst(t *testing.T) {
	var mu sync.Mutex
	calls := map[int64]int{}
	done := make(chan int64, 10)
	r := newRefresher(20*time.Millisecond, func(id int64) {
		mu.Lock()
		calls[id]++
		mu.Unlock()
		done <- id
	})

	r.schedule(1)
	r.schedule(1)
	r.schedule(2)
	r.schedule(1)
	waitCalls(t, done, 2)
	time.Sleep(40 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls[1] != 1 || calls[2] != 1 {
		t.Fatalf("calls = %v, want one per id", calls)
	}
}

func TestRefresherRunsAgainAfterFire(t *testing.T) {
	done := make(chan int64, 10)
	r := newRefresher(10*time.Millisecond, func(id int64) { done <- id })

	r.schedule(1)
	waitCalls(t, done, 1)
	r.schedule(1)
	waitCalls(t, done, 1)
}

func TestRefresherSerializesRenders(t *testing.T) {
	var mu sync.Mutex
	running, maxRunning := 0, 0
	r := newRefresher(time.Millisecond, func(int64) {
		mu.Lock()
		running++
		maxRunning = max(maxRunning, running)
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
	})

	r.schedule(1)
	time.Sleep(10 * time.Millisecond)
	r.schedule(1)
	r.schedule(2)
	r.wait()

	if maxRunning != 1 {
		t.Fatalf("max concurrent renders = %d, want 1", maxRunning)
	}
}

func waitCalls(t *testing.T, done <-chan int64, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("render was called %d times, want %d", i, n)
		}
	}
}

func TestUntilNextDay(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	now := time.Date(2026, time.October, 31, 23, 59, 0, 0, loc)

	if got, want := untilNextDay(now), time.Minute+midnightLag; got != want {
		t.Fatalf("untilNextDay = %v, want %v", got, want)
	}
}
