package notify

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	discordRed    = 0xe5484d
	discordGreen  = 0x30a46c
	discordYellow = 0xf5c53b
	discordGray   = 0x8b8d98
)

type Discord struct {
	client    *http.Client
	userAgent string
	config    Config
}

type discordRequest struct {
	Content  string          `json:"content,omitzero"`
	Embeds   []discordEmbed  `json:"embeds"`
	Mentions discordMentions `json:"allowed_mentions"`
}

type discordMentions struct {
	Parse []string `json:"parse"`
	Roles []string `json:"roles,omitzero"`
	Users []string `json:"users,omitzero"`
}

type discordEmbed struct {
	Title       string         `json:"title"`
	URL         string         `json:"url,omitzero"`
	Description string         `json:"description,omitzero"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields,omitzero"`
	Timestamp   string         `json:"timestamp,omitzero"`
	Footer      *discordFooter `json:"footer,omitzero"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordFooter struct {
	Text string `json:"text"`
}

func validDiscordURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.TrimPrefix(strings.TrimPrefix(u.Host, "ptb."), "canary.")
	return (host == "discord.com" || host == "discordapp.com") && strings.HasPrefix(u.Path, "/api/webhooks/")
}

func (d *Discord) Notify(ctx context.Context, msg Message) error {
	body, err := json.Marshal(discordPayload(msg, d.config.Mention))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.config.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: %w", redact(err, d.config.URL))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("discord: %w", &StatusError{Code: resp.StatusCode, Body: strings.TrimSpace(string(raw))})
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return nil
}

func discordPayload(msg Message, mention string) discordRequest {
	embed := discordEmbed{Color: discordGray, URL: msg.Link, Footer: &discordFooter{Text: "heron"}}
	switch msg.Event {
	case EventDown:
		embed.Title = msg.Monitor.Name + " is down"
		embed.Color = discordRed
	case EventRecovered:
		embed.Title = msg.Monitor.Name + " is back up"
		embed.Color = discordGreen
	case EventFlapping:
		embed.Title = msg.Monitor.Name + " keeps going up and down"
		embed.Color = discordYellow
	case EventStable:
		embed.Title = msg.Monitor.Name + " is stable again"
		embed.Color = discordGreen
	case EventExpiring, EventWarning:
		embed.Title = msg.Title
		embed.Color = discordYellow
	default:
		embed.Title = "Test notification from heron"
	}
	embed.Description = msg.Reason
	if msg.Monitor != nil && msg.Monitor.DisplayTarget() != "" {
		embed.Fields = append(embed.Fields, discordField{Name: "Target", Value: "`" + msg.Monitor.DisplayTarget() + "`", Inline: true})
	}
	if msg.Downtime > 0 {
		embed.Fields = append(embed.Fields, discordField{Name: "Downtime", Value: humanize(msg.Downtime), Inline: true})
	}
	if !msg.At.IsZero() {
		embed.Timestamp = msg.At.UTC().Format(time.RFC3339)
	}

	out := discordRequest{Embeds: []discordEmbed{embed}, Mentions: discordMentions{Parse: []string{}}}
	if msg.Event == EventDown || msg.Event == EventTest {
		out.Content, out.Mentions = discordMention(mention)
	}
	return out
}

func discordMention(raw string) (string, discordMentions) {
	m := discordMentions{Parse: []string{}}
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return "", m
	case raw == "@everyone" || raw == "@here":
		m.Parse = []string{"everyone"}
		return raw, m
	case strings.HasPrefix(raw, "<@&") && strings.HasSuffix(raw, ">"):
		m.Roles = []string{raw[3 : len(raw)-1]}
		return raw, m
	case strings.HasPrefix(raw, "<@") && strings.HasSuffix(raw, ">"):
		m.Users = []string{strings.TrimPrefix(raw[2:len(raw)-1], "!")}
		return raw, m
	case isDigits(raw):
		m.Roles = []string{raw}
		return "<@&" + raw + ">", m
	}
	return "", m
}

func isDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}
