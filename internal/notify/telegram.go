package notify

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/v2"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
)

const defaultTelegramAPI = "https://api.telegram.org"

type Telegram struct {
	client *http.Client
	api    string
	config Config
}

type telegramRequest struct {
	ChatID      string `json:"chat_id"`
	ThreadID    int64  `json:"message_thread_id,omitzero"`
	Text        string `json:"text"`
	ParseMode   string `json:"parse_mode"`
	LinkPreview struct {
		Disabled bool `json:"is_disabled"`
	} `json:"link_preview_options"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (t *Telegram) Notify(ctx context.Context, msg Message) error {
	payload := telegramRequest{
		ChatID:    t.config.ChatID,
		ThreadID:  t.config.ThreadID,
		Text:      telegramText(msg),
		ParseMode: "HTML",
	}
	payload.LinkPreview.Disabled = true

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", cmp.Or(t.api, defaultTelegramAPI), t.config.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", redact(err, t.config.BotToken))
	}
	defer resp.Body.Close()

	var out telegramResponse
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err := json.Unmarshal(raw, &out); err != nil || !out.OK {
		return fmt.Errorf("telegram: %w", &StatusError{Code: resp.StatusCode, Body: cmp.Or(out.Description, strings.TrimSpace(string(raw)))})
	}
	return nil
}

func telegramText(msg Message) string {
	var b strings.Builder
	esc := html.EscapeString

	switch msg.Event {
	case EventDown:
		fmt.Fprintf(&b, "🔴 <b>%s is down</b>\n", esc(msg.Monitor.Name))
	case EventRecovered:
		fmt.Fprintf(&b, "🟢 <b>%s is back up</b>\n", esc(msg.Monitor.Name))
	case EventFlapping:
		fmt.Fprintf(&b, "🟡 <b>%s keeps going up and down</b>\n", esc(msg.Monitor.Name))
	case EventStable:
		fmt.Fprintf(&b, "🔵 <b>%s is stable again</b>\n", esc(msg.Monitor.Name))
	case EventExpiring:
		fmt.Fprintf(&b, "⏳ <b>%s</b>\n", esc(msg.Title))
	case EventWarning:
		fmt.Fprintf(&b, "🟠 <b>%s</b>\n", esc(msg.Title))
	default:
		fmt.Fprintf(&b, "🔔 <b>Test notification from heron</b>\n")
	}

	if msg.Monitor != nil && msg.Monitor.DisplayTarget() != "" {
		fmt.Fprintf(&b, "<code>%s</code>\n", esc(msg.Monitor.DisplayTarget()))
	}
	if msg.Reason != "" {
		fmt.Fprintf(&b, "\n%s\n", esc(msg.Reason))
	}
	if msg.Downtime > 0 {
		fmt.Fprintf(&b, "Downtime: %s\n", humanize(msg.Downtime))
	}
	if msg.Link != "" {
		fmt.Fprintf(&b, "\n<a href=\"%s\">Open status page</a>", esc(msg.Link))
	}
	return strings.TrimSpace(b.String())
}

type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

func redact(err error, secret string) error {
	if secret == "" {
		return err
	}
	return &redactedError{msg: strings.ReplaceAll(err.Error(), secret, "***"), err: err}
}
