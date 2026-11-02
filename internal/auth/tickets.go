package auth

import (
	"crypto/rand"
	"sync"
	"time"
)

type Tickets[T any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	pending map[string]ticket[T]
}

type ticket[T any] struct {
	value   T
	expires time.Time
}

func NewTickets[T any](ttl time.Duration) *Tickets[T] {
	return &Tickets[T]{ttl: ttl, now: time.Now, pending: map[string]ticket[T]{}}
}

func (t *Tickets[T]) Issue(value T) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	if len(t.pending) > 1024 {
		for id, p := range t.pending {
			if now.After(p.expires) {
				delete(t.pending, id)
			}
		}
	}
	id := rand.Text()
	t.pending[id] = ticket[T]{value: value, expires: now.Add(t.ttl)}
	return id
}

func (t *Tickets[T]) Peek(id string) (T, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lookup(id)
}

func (t *Tickets[T]) Take(id string) (T, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	value, ok := t.lookup(id)
	delete(t.pending, id)
	return value, ok
}

func (t *Tickets[T]) lookup(id string) (T, bool) {
	p, ok := t.pending[id]
	if !ok || t.now().After(p.expires) {
		var zero T
		return zero, false
	}
	return p.value, true
}
