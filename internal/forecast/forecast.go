package forecast

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/pelican"
	"github.com/nethercoded/heron/internal/store"
)

const (
	KindSlow   = "slow"
	KindHidden = "hidden_failures"
	KindMemory = "memory"
	KindDisk   = "disk"
)

type Panel interface {
	NodeUtilization(ctx context.Context, target string) (pelican.Utilization, error)
	ServerMemory(target string) (used, limit int64, ok bool)
}

type Sender interface {
	Dispatch(channels []notify.Channel, msg notify.Message)
}

type Service struct {
	Store  *store.Store
	Panel  Panel
	Sender Sender
	Link   func(m *monitor.Monitor) string
	Every  time.Duration
	Log    *slog.Logger
	Now    func() time.Time

	sampled time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) Run(ctx context.Context) {
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := s.Check(ctx); err != nil {
			s.Log.Warn("look for early warnings", "err", err)
		}
		timer.Reset(s.Every)
	}
}

type finding struct {
	kind    string
	title   string
	message string
}

func (s *Service) Check(ctx context.Context) error {
	now := s.now()
	monitors, err := s.Store.Monitors(ctx)
	if err != nil {
		return err
	}
	trend, err := s.Store.LatencyTrend(ctx, now, 6*time.Hour, 7*24*time.Hour)
	if err != nil {
		return err
	}
	if s.Panel != nil && now.Sub(s.sampled) >= 55*time.Minute {
		s.sample(ctx, monitors, now)
		s.sampled = now
	}

	saved, err := s.Store.Warnings(ctx)
	if err != nil {
		return err
	}
	active := map[string]store.Warning{}
	for _, w := range saved {
		active[fmt.Sprint(w.MonitorID, w.Kind)] = w
	}

	for _, m := range monitors {
		var found []finding
		if !m.Paused {
			found = s.inspect(ctx, m, trend[m.ID], now)
		}
		seen := map[string]bool{}
		for _, f := range found {
			key := fmt.Sprint(m.ID, f.kind)
			seen[f.kind] = true
			w, ok := active[key]
			if !ok {
				w = store.Warning{MonitorID: m.ID, Kind: f.kind, Since: now}
			}
			w.Message = f.message
			fresh := w.NotifiedAt.IsZero() || now.Sub(w.NotifiedAt) >= 24*time.Hour
			if fresh {
				w.NotifiedAt = now
			}
			if err := s.Store.SaveWarning(ctx, w); err != nil {
				return err
			}
			if fresh {
				s.alert(ctx, m, f, now)
			}
		}
		for _, kind := range []string{KindSlow, KindHidden, KindMemory, KindDisk} {
			if _, was := active[fmt.Sprint(m.ID, kind)]; was && !seen[kind] {
				if err := s.Store.ClearWarning(ctx, m.ID, kind); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Service) inspect(ctx context.Context, m *monitor.Monitor, l store.Latency, now time.Time) []finding {
	var out []finding
	if l.UsualN >= 100 && l.RecentN >= 10 && l.Usual > 0 {
		ratio := float64(l.Recent) / float64(l.Usual)
		if ratio >= 2 && l.Recent-l.Usual >= 150*time.Millisecond {
			out = append(out, finding{
				kind:    KindSlow,
				title:   fmt.Sprintf("%s answers %.1fx slower than usual", m.Name, ratio),
				message: fmt.Sprintf("%s over the last 6 hours, usually %s.", round(l.Recent), round(l.Usual)),
			})
		}
	}
	if l.HiddenFails >= 5 && float64(l.HiddenFails) >= 0.02*float64(l.Checks) {
		out = append(out, finding{
			kind:    KindHidden,
			title:   m.Name + " keeps failing checks without going down",
			message: fmt.Sprintf("%d of %d checks in the last day failed and passed on the retry. Something is wearing thin.", l.HiddenFails, l.Checks),
		})
	}
	if s.Panel == nil {
		return out
	}
	switch m.Kind {
	case monitor.KindPelicanServer:
		if used, limit, ok := s.Panel.ServerMemory(m.Target); ok && float64(used) >= 0.9*float64(limit) {
			out = append(out, finding{
				kind:    KindMemory,
				title:   m.Name + " is close to its memory limit",
				message: fmt.Sprintf("It uses %s of %s (%.0f%%). Game servers usually crash when they hit the limit.", gb(float64(used)), gb(float64(limit)), 100*float64(used)/float64(limit)),
			})
		}
	case monitor.KindPelicanNode:
		if f, ok := s.disk(ctx, m, now); ok {
			out = append(out, f)
		}
	}
	return out
}

func (s *Service) sample(ctx context.Context, monitors []*monitor.Monitor, now time.Time) {
	for _, m := range monitors {
		if m.Kind != monitor.KindPelicanNode || m.Paused {
			continue
		}
		u, err := s.Panel.NodeUtilization(ctx, m.Target)
		if err != nil || u.DiskTotal == 0 {
			continue
		}
		sm := store.Sample{At: now.Truncate(time.Hour), Value: float64(u.DiskUsed), Total: float64(u.DiskTotal)}
		if err := s.Store.AddSample(ctx, m.ID, KindDisk, sm); err != nil {
			s.Log.Warn("save disk sample", "monitor", m.Slug, "err", err)
		}
	}
}

func (s *Service) disk(ctx context.Context, m *monitor.Monitor, now time.Time) (finding, bool) {
	list, err := s.Store.Samples(ctx, m.ID, KindDisk, now.AddDate(0, 0, -7))
	if err != nil || len(list) == 0 {
		return finding{}, false
	}
	last := list[len(list)-1]
	if last.Total <= 0 {
		return finding{}, false
	}
	share := last.Value / last.Total
	days, growing := Until(list)
	switch {
	case growing && days <= 14:
		return finding{
			kind:    KindDisk,
			title:   fmt.Sprintf("The disk on %s fills up in about %s", m.Name, span(days)),
			message: fmt.Sprintf("%s of %s used and growing by %s a day. At this pace it is full around %s.", gb(last.Value), gb(last.Total), gb(Slope(list)*86400), now.Add(time.Duration(days*24*float64(time.Hour))).Format("Monday 2 January")),
		}, true
	case share >= 0.9:
		return finding{
			kind:    KindDisk,
			title:   fmt.Sprintf("The disk on %s is %.0f%% full", m.Name, share*100),
			message: fmt.Sprintf("%s of %s used.", gb(last.Value), gb(last.Total)),
		}, true
	}
	return finding{}, false
}

func Slope(list []Sample) float64 {
	if len(list) < 2 {
		return 0
	}
	t0 := list[0].At
	var sx, sy, sxx, sxy float64
	for _, sm := range list {
		x := sm.At.Sub(t0).Seconds()
		sx += x
		sy += sm.Value
		sxx += x * x
		sxy += x * sm.Value
	}
	n := float64(len(list))
	den := n*sxx - sx*sx
	if den == 0 {
		return 0
	}
	return (n*sxy - sx*sy) / den
}

type Sample = store.Sample

func Until(list []Sample) (float64, bool) {
	if len(list) < 12 || list[len(list)-1].At.Sub(list[0].At) < 24*time.Hour {
		return 0, false
	}
	slope := Slope(list)
	if slope <= 0 {
		return 0, false
	}
	last := list[len(list)-1]
	return (last.Total - last.Value) / slope / 86400, true
}

func (s *Service) alert(ctx context.Context, m *monitor.Monitor, f finding, now time.Time) {
	s.Log.Warn(f.title, "monitor", m.Slug)
	list, err := s.Store.ChannelsForMonitor(ctx, m.ID)
	if err != nil || len(list) == 0 {
		return
	}
	channels := make([]notify.Channel, 0, len(list))
	for _, c := range list {
		channels = append(channels, *c)
	}
	msg := notify.Message{Event: notify.EventWarning, Title: f.title, Monitor: m, Status: m.State.Status, Reason: f.message, At: now}
	if s.Link != nil {
		msg.Link = s.Link(m)
	}
	s.Sender.Dispatch(channels, msg)
}

func round(d time.Duration) string {
	return d.Round(time.Millisecond).String()
}

func gb(bytes float64) string {
	switch {
	case math.Abs(bytes) >= 1<<30:
		return fmt.Sprintf("%.1f GB", bytes/(1<<30))
	case math.Abs(bytes) >= 1<<20:
		return fmt.Sprintf("%.0f MB", bytes/(1<<20))
	}
	return fmt.Sprintf("%.0f KB", bytes/(1<<10))
}

func span(days float64) string {
	switch {
	case days < 1:
		return fmt.Sprintf("%.0f hours", math.Max(1, days*24))
	case days < 1.5:
		return "a day"
	}
	return fmt.Sprintf("%.0f days", days)
}
