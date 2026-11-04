package expiry

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/store"
)

var thresholds = []int{30, 7, 1, 0}

type Prober interface {
	Check(ctx context.Context, t Target) (Result, error)
}

type Sender interface {
	Dispatch(channels []notify.Channel, msg notify.Message)
}

type Service struct {
	Store  *store.Store
	Prober Prober
	Sender Sender
	Link   func(m *monitor.Monitor) string
	Every  time.Duration
	Log    *slog.Logger

	mu       sync.Mutex
	checking atomic.Bool
}

func (s *Service) Checking() bool {
	return s.checking.Load()
}

func (s *Service) Begin() bool {
	if !s.checking.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		defer s.checking.Store(false)
		if err := s.Refresh(context.Background(), true); err != nil {
			s.Log.Warn("check domain and certificate expiry", "err", err)
		}
	}()
	return true
}

func (s *Service) Run(ctx context.Context) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := s.Refresh(ctx, false); err != nil {
			s.Log.Warn("check domain and certificate expiry", "err", err)
		}
		timer.Reset(s.Every)
	}
}

func key(host, kind string) string { return kind + " " + host }

func (s *Service) Refresh(ctx context.Context, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	monitors, err := s.Store.Monitors(ctx)
	if err != nil {
		return err
	}
	byID := make(map[int64]*monitor.Monitor, len(monitors))
	for _, m := range monitors {
		byID[m.ID] = m
	}
	active := slices.DeleteFunc(slices.Clone(monitors), func(m *monitor.Monitor) bool { return m.Paused })
	targets := Targets(active)

	saved, err := s.Store.Expiries(ctx)
	if err != nil {
		return err
	}
	known := map[string]store.Expiry{}
	for _, e := range saved {
		known[key(e.Host, e.Kind)] = e
	}
	wanted := map[string]bool{}
	for _, t := range targets {
		wanted[key(t.Host, string(t.Kind))] = true
	}
	for _, e := range saved {
		if !wanted[key(e.Host, e.Kind)] {
			if err := s.Store.DeleteExpiry(ctx, e.Host, e.Kind); err != nil {
				return err
			}
		}
	}

	now := time.Now()
	for _, t := range targets {
		e, seen := known[key(t.Host, string(t.Kind))]
		if !seen {
			e = store.Expiry{Host: t.Host, Kind: string(t.Kind)}
		}
		if force || e.CheckedAt.IsZero() || now.Sub(e.CheckedAt) >= s.Every || e.Error != "" {
			res, err := s.Prober.Check(ctx, t)
			e.CheckedAt = now
			if err != nil {
				e.Error = err.Error()
				s.Log.Debug("expiry check failed", "host", t.Host, "kind", t.Kind, "err", err)
			} else {
				if res.ExpiresAt.After(e.ExpiresAt.Add(24 * time.Hour)) {
					e.Warned = 0
				}
				e.Error, e.ExpiresAt, e.Issuer = "", res.ExpiresAt, res.Issuer
			}
		}
		if e.Error == "" && !e.ExpiresAt.IsZero() {
			e.Warned = s.warn(ctx, t, e, byID, now)
		}
		if err := s.Store.SaveExpiry(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

func DaysLeft(expires, now time.Time) int {
	return int(math.Ceil(expires.Sub(now).Hours() / 24))
}

func (s *Service) warn(ctx context.Context, t Target, e store.Expiry, byID map[int64]*monitor.Monitor, now time.Time) int {
	days := DaysLeft(e.ExpiresAt, now)
	level := 0
	for i, limit := range thresholds {
		if days <= limit {
			level = i + 1
		}
	}
	if level == 0 || level <= e.Warned {
		return e.Warned
	}

	var first *monitor.Monitor
	var channels []notify.Channel
	seen := map[int64]bool{}
	for _, id := range t.Monitors {
		m := byID[id]
		if m == nil {
			continue
		}
		if first == nil {
			first = m
		}
		list, err := s.Store.ChannelsForMonitor(ctx, id)
		if err != nil {
			s.Log.Error("load channels", "monitor", m.Slug, "err", err)
			continue
		}
		for _, c := range list {
			if !seen[c.ID] {
				seen[c.ID] = true
				channels = append(channels, *c)
			}
		}
	}
	if first == nil {
		return level
	}

	what := "The certificate for " + t.HostName()
	if t.Kind == KindDomain {
		what = "The domain " + t.Host
	}
	title := fmt.Sprintf("%s expires in %d days", what, days)
	switch {
	case days <= 0:
		title = what + " has expired"
	case days == 1:
		title = what + " expires tomorrow"
	}
	reason := "Expiry date: " + e.ExpiresAt.Format("2 January 2006") + "."
	if e.Issuer != "" {
		reason += " " + map[string]string{string(KindDomain): "Registrar", string(KindCert): "Issued by"}[e.Kind] + ": " + e.Issuer + "."
	}
	s.Log.Warn(title, "expires", e.ExpiresAt)
	if len(channels) > 0 {
		msg := notify.Message{Event: notify.EventExpiring, Title: title, Monitor: first, Reason: reason, At: now}
		if s.Link != nil {
			msg.Link = s.Link(first)
		}
		s.Sender.Dispatch(channels, msg)
	}
	return level
}

func (t Target) HostName() string {
	if t.Kind != KindCert {
		return t.Host
	}
	if host, port, err := net.SplitHostPort(t.Host); err == nil && port == "443" {
		return host
	}
	return t.Host
}
