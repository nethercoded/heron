package broker

import "sync"

type Broker[T any] struct {
	mu     sync.RWMutex
	subs   map[chan T]struct{}
	buffer int
}

func New[T any](buffer int) *Broker[T] {
	return &Broker[T]{subs: map[chan T]struct{}{}, buffer: buffer}
}

func (b *Broker[T]) Subscribe() (<-chan T, func()) {
	ch := make(chan T, b.buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			b.mu.Unlock()
			close(ch)
		})
	}
}

func (b *Broker[T]) Publish(v T) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- v:
		default:
		}
	}
}

func (b *Broker[T]) Subscribers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
