package notify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nethercode/heron/internal/monitor"
)

type Kind string

const (
	KindTelegram Kind = "telegram"
	KindWebhook  Kind = "webhook"
)

type Channel struct {
	ID        int64
	Name      string
	Kind      Kind
	Enabled   bool
	Config    Config
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Config struct {
	BotToken string            `json:"bot_token,omitzero"`
	ChatID   string            `json:"chat_id,omitzero"`
	ThreadID int64             `json:"thread_id,omitzero"`
	URL      string            `json:"url,omitzero"`
	Secret   string            `json:"secret,omitzero"`
	Headers  map[string]string `json:"headers,omitzero"`
}

func (c *Channel) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, &monitor.FieldError{Field: "name", Message: "is required"})
	}
	switch c.Kind {
	case KindTelegram:
		if c.Config.BotToken == "" {
			errs = append(errs, &monitor.FieldError{Field: "bot_token", Message: "is required"})
		}
		if c.Config.ChatID == "" {
			errs = append(errs, &monitor.FieldError{Field: "chat_id", Message: "is required"})
		}
	case KindWebhook:
		u, err := url.Parse(c.Config.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, &monitor.FieldError{Field: "url", Message: "must be an http or https url"})
		}
	default:
		errs = append(errs, &monitor.FieldError{Field: "kind", Message: fmt.Sprintf("unknown kind %q", c.Kind)})
	}
	return errors.Join(errs...)
}

type Event string

const (
	EventDown      Event = "monitor.down"
	EventRecovered Event = "monitor.recovered"
	EventTest      Event = "test"
)

type Message struct {
	Event    Event
	Monitor  *monitor.Monitor
	Status   monitor.Status
	Reason   string
	At       time.Time
	Downtime time.Duration
	Link     string
}

type Notifier interface {
	Notify(ctx context.Context, msg Message) error
}

type Factory struct {
	Client      *http.Client
	TelegramAPI string
	UserAgent   string
}

func (f Factory) New(c Channel) (Notifier, error) {
	switch c.Kind {
	case KindTelegram:
		return &Telegram{client: f.Client, api: f.TelegramAPI, config: c.Config}, nil
	case KindWebhook:
		return &Webhook{client: f.Client, userAgent: f.UserAgent, config: c.Config}, nil
	}
	return nil, fmt.Errorf("unsupported channel kind %q", c.Kind)
}

type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("unexpected status %d", e.Code)
	}
	return fmt.Sprintf("unexpected status %d: %s", e.Code, e.Body)
}

func (e *StatusError) Temporary() bool {
	return e.Code == http.StatusTooManyRequests || e.Code >= 500
}

func humanize(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
}
