package auth

import (
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	windows map[string]*window
}

type window struct {
	start time.Time
	count int
}

func NewLimiter(limit int, per time.Duration) *Limiter {
	return &Limiter{limit: limit, window: per, now: time.Now, windows: map[string]*window{}}
}

func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if len(l.windows) > 4096 {
		l.sweep(now)
	}

	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= l.window {
		l.windows[key] = &window{start: now, count: 1}
		return true, 0
	}
	if w.count >= l.limit {
		return false, w.start.Add(l.window).Sub(now)
	}
	w.count++
	return true, 0
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.windows, key)
	l.mu.Unlock()
}

func (l *Limiter) sweep(now time.Time) {
	for key, w := range l.windows {
		if now.Sub(w.start) >= l.window {
			delete(l.windows, key)
		}
	}
}
