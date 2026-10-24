package bot

import (
	"sync"
	"time"
)

type refresher struct {
	delay    time.Duration
	render   func(id int64)
	mu       sync.Mutex
	pending  map[int64]bool
	renderMu sync.Mutex
	inFlight sync.WaitGroup
}

func newRefresher(delay time.Duration, render func(id int64)) *refresher {
	return &refresher{delay: delay, render: render, pending: map[int64]bool{}}
}

func (r *refresher) schedule(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending[id] {
		return
	}
	r.pending[id] = true
	r.inFlight.Add(1)
	time.AfterFunc(r.delay, func() {
		defer r.inFlight.Done()
		r.mu.Lock()
		delete(r.pending, id)
		r.mu.Unlock()
		r.renderMu.Lock()
		defer r.renderMu.Unlock()
		r.render(id)
	})
}

func (r *refresher) wait() {
	r.inFlight.Wait()
}
