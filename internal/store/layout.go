package store

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	BlockHero        = "hero"
	BlockNotice      = "notice"
	BlockIncidents   = "incidents"
	BlockMaintenance = "maintenance"
	BlockMonitors    = "monitors"
	BlockHistory     = "history"

	MaxBlocks = 30
)

var (
	blockTypes   = []string{BlockHero, BlockNotice, BlockIncidents, BlockMaintenance, BlockMonitors, BlockHistory}
	blockID      = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
	blockStyles  = map[string][]string{BlockHero: {"", "full", "compact"}, BlockMonitors: {"", "bars", "list"}}
	noticeTones  = []string{"", "info", "success", "warning", "danger"}
	historySpans = []int{0, 7, 14, 30}
	blockWidths  = []int{0, 4, 6, 8}
)

type Block struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Hidden bool   `json:"hidden,omitzero"`
	Title  string `json:"title,omitzero"`
	Body   string `json:"body,omitzero"`
	Tone   string `json:"tone,omitzero"`
	Group  string `json:"group,omitzero"`
	Style  string `json:"style,omitzero"`
	Days   int    `json:"days,omitzero"`
	Stats  string `json:"stats,omitzero"`
	Width  int    `json:"width,omitzero"`
}

func DefaultBlocks() []Block {
	return []Block{
		{ID: "hero", Type: BlockHero},
		{ID: "incidents", Type: BlockIncidents},
		{ID: "maintenance", Type: BlockMaintenance},
		{ID: "monitors", Type: BlockMonitors},
		{ID: "history", Type: BlockHistory, Days: 7},
	}
}

func NormalizeBlocks(blocks []Block) []Block {
	if len(blocks) == 0 {
		return DefaultBlocks()
	}
	out := make([]Block, len(blocks))
	for i, b := range blocks {
		b.ID = strings.TrimSpace(b.ID)
		b.Title = strings.TrimSpace(b.Title)
		b.Body = strings.TrimSpace(strings.ReplaceAll(b.Body, "\r\n", "\n"))
		b.Group = strings.TrimSpace(b.Group)
		if b.Type != BlockNotice {
			b.Body, b.Tone = "", ""
		}
		if b.Type != BlockMonitors {
			b.Group = ""
		}
		if b.Type != BlockHistory {
			b.Days = 0
		}
		if b.Type != BlockHero {
			b.Stats = ""
		}
		if _, ok := blockStyles[b.Type]; !ok {
			b.Style = ""
		}
		if b.Width == 12 || b.Type == BlockHero {
			b.Width = 0
		}
		out[i] = b
	}
	return out
}

func ValidateBlocks(blocks []Block) map[string]string {
	fields := map[string]string{}
	if len(blocks) > MaxBlocks {
		fields["blocks"] = fmt.Sprintf("at most %d blocks", MaxBlocks)
		return fields
	}
	seen := map[string]bool{}
	heroes := 0
	for i, b := range blocks {
		at := func(field string) string { return fmt.Sprintf("blocks.%d.%s", i, field) }
		switch {
		case !blockID.MatchString(b.ID):
			fields[at("id")] = "must be 1 to 32 lowercase letters, digits or dashes"
		case seen[b.ID]:
			fields[at("id")] = "is used twice"
		}
		seen[b.ID] = true
		if !slices.Contains(blockTypes, b.Type) {
			fields[at("type")] = fmt.Sprintf("unknown block %q", b.Type)
			continue
		}
		if b.Type == BlockHero {
			if heroes++; heroes > 1 {
				fields[at("type")] = "the page can have one status header"
			}
		}
		if len([]rune(b.Title)) > 80 {
			fields[at("title")] = "must be at most 80 characters"
		}
		if len([]rune(b.Body)) > 2000 {
			fields[at("body")] = "must be at most 2000 characters"
		}
		if b.Type == BlockNotice && b.Title == "" && b.Body == "" {
			fields[at("body")] = "write something for the notice"
		}
		if !slices.Contains(noticeTones, b.Tone) {
			fields[at("tone")] = "must be info, success, warning or danger"
		}
		if styles, ok := blockStyles[b.Type]; ok && !slices.Contains(styles, b.Style) {
			fields[at("style")] = "must be one of " + strings.Join(styles[1:], ", ")
		}
		if !slices.Contains(historySpans, b.Days) {
			fields[at("days")] = "must be 7, 14 or 30"
		}
		if !slices.Contains(blockWidths, b.Width) {
			fields[at("width")] = "must be a third, half, two thirds or the full width"
		}
		if b.Stats != "" && b.Stats != "hide" {
			fields[at("stats")] = "must be empty or hide"
		}
		if len(b.Group) > 80 {
			fields[at("group")] = "must be at most 80 characters"
		}
	}
	return fields
}
