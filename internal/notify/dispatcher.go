package notify

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Dispatcher struct {
	factory  Factory
	log      *slog.Logger
	attempts int
	backoff  time.Duration

	wg   sync.WaitGroup
	ctx  context.Context
	stop context.CancelFunc
}

func NewDispatcher(factory Factory, log *slog.Logger) *Dispatcher {
	ctx, stop := context.WithCancel(context.Background())
	return &Dispatcher{
		factory:  factory,
		log:      log,
		attempts: 4,
		backoff:  2 * time.Second,
		ctx:      ctx,
		stop:     stop,
	}
}

func (d *Dispatcher) Dispatch(channels []Channel, msg Message) {
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		d.wg.Go(func() {
			log := d.log.With("channel", ch.Name, "kind", ch.Kind, "event", msg.Event)
			if err := d.deliver(d.ctx, ch, msg); err != nil {
				log.Warn("notification failed", "err", err)
				return
			}
			log.Debug("notification sent")
		})
	}
}

func (d *Dispatcher) Send(ctx context.Context, ch Channel, msg Message) error {
	n, err := d.factory.New(ch)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return n.Notify(ctx, msg)
}

func (d *Dispatcher) deliver(ctx context.Context, ch Channel, msg Message) error {
	wait := d.backoff
	var err error
	for attempt := range d.attempts {
		if attempt > 0 {
			select {
			case <-time.After(wait):
				wait *= 3
			case <-ctx.Done():
				return errors.Join(err, ctx.Err())
			}
		}
		if err = d.Send(ctx, ch, msg); err == nil || !retryable(err) {
			return err
		}
	}
	return err
}

func (d *Dispatcher) Close(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		d.stop()
		<-done
		return ctx.Err()
	}
}

func retryable(err error) bool {
	if se, ok := errors.AsType[*StatusError](err); ok {
		return se.Temporary()
	}
	return true
}
