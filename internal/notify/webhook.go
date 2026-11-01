package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderEvent     = "X-Heron-Event"
	HeaderTimestamp = "X-Heron-Timestamp"
	HeaderSignature = "X-Heron-Signature"
)

type Webhook struct {
	client    *http.Client
	userAgent string
	config    Config
}

type webhookPayload struct {
	Event    Event          `json:"event"`
	At       time.Time      `json:"at"`
	Status   string         `json:"status,omitzero"`
	Reason   string         `json:"reason,omitzero"`
	Downtime int64          `json:"downtime_seconds,omitzero"`
	Link     string         `json:"link,omitzero"`
	Monitor  *webhookTarget `json:"monitor,omitzero"`
}

type webhookTarget struct {
	ID     int64  `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Target string `json:"target,omitzero"`
}

func (w *Webhook) Notify(ctx context.Context, msg Message) error {
	payload := webhookPayload{
		Event:    msg.Event,
		At:       msg.At.UTC(),
		Status:   string(msg.Status),
		Reason:   msg.Reason,
		Downtime: int64(msg.Downtime.Seconds()),
		Link:     msg.Link,
	}
	if m := msg.Monitor; m != nil {
		payload.Monitor = &webhookTarget{ID: m.ID, Slug: m.Slug, Name: m.Name, Kind: string(m.Kind), Target: m.DisplayTarget()}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range w.config.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", w.userAgent)
	req.Header.Set(HeaderEvent, string(msg.Event))

	if w.config.Secret != "" {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		req.Header.Set(HeaderTimestamp, ts)
		req.Header.Set(HeaderSignature, "sha256="+Sign(w.config.Secret, ts, body))
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("webhook: %w", &StatusError{Code: resp.StatusCode, Body: strings.TrimSpace(string(raw))})
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return nil
}

func Sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
