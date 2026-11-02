package httpapi

import (
	"time"

	"github.com/nethercode/heron/internal/monitor"
	"github.com/nethercode/heron/internal/notify"
	"github.com/nethercode/heron/internal/store"
)

type stamp time.Time

func (t stamp) MarshalText() ([]byte, error) {
	if time.Time(t).IsZero() {
		return []byte(""), nil
	}
	return time.Time(t).UTC().MarshalText()
}

func (t *stamp) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*t = stamp{}
		return nil
	}
	return (*time.Time)(t).UnmarshalText(b)
}

func (t stamp) IsZero() bool {
	return time.Time(t).IsZero()
}

type dayDTO struct {
	Date     string   `json:"date"`
	Uptime   *float64 `json:"uptime"`
	Checks   int64    `json:"checks"`
	Down     int64    `json:"down"`
	Degraded int64    `json:"degraded"`
}

type uptimeDTO struct {
	Day     float64 `json:"day"`
	Week    float64 `json:"week"`
	Month   float64 `json:"month"`
	History float64 `json:"history"`
}

type publicMonitorDTO struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitzero"`
	Group       string         `json:"group"`
	Kind        monitor.Kind   `json:"kind"`
	Status      monitor.Status `json:"status"`
	Since       stamp          `json:"since,omitzero"`
	CheckedAt   stamp          `json:"checked_at,omitzero"`
	LatencyMS   int64          `json:"latency_ms"`
	Uptime      uptimeDTO      `json:"uptime"`
	Days        []dayDTO       `json:"days"`
}

type groupDTO struct {
	Name     string             `json:"name"`
	Monitors []publicMonitorDTO `json:"monitors"`
}

type affectedDTO struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type updateDTO struct {
	ID        int64                `json:"id"`
	Status    store.IncidentStatus `json:"status"`
	Body      string               `json:"body"`
	CreatedAt stamp                `json:"created_at"`
}

type incidentDTO struct {
	ID         int64                `json:"id"`
	Title      string               `json:"title"`
	Impact     store.Impact         `json:"impact"`
	Status     store.IncidentStatus `json:"status"`
	Automatic  bool                 `json:"automatic"`
	StartedAt  stamp                `json:"started_at"`
	ResolvedAt stamp                `json:"resolved_at,omitzero"`
	UpdatedAt  stamp                `json:"updated_at"`
	Monitors   []affectedDTO        `json:"monitors"`
	MonitorIDs []int64              `json:"monitor_ids,omitzero"`
	Updates    []updateDTO          `json:"updates"`
}

func toIncident(inc *store.Incident, names map[int64]*monitor.Monitor, admin bool) incidentDTO {
	dto := incidentDTO{
		ID:         inc.ID,
		Title:      inc.Title,
		Impact:     inc.Impact,
		Status:     inc.Status,
		Automatic:  inc.Automatic,
		StartedAt:  stamp(inc.StartedAt),
		ResolvedAt: stamp(inc.ResolvedAt),
		UpdatedAt:  stamp(inc.UpdatedAt),
		Monitors:   []affectedDTO{},
		Updates:    make([]updateDTO, 0, len(inc.Updates)),
	}
	if admin {
		dto.MonitorIDs = inc.MonitorIDs
	}
	for _, id := range inc.MonitorIDs {
		if m, ok := names[id]; ok && (admin || m.Public) {
			dto.Monitors = append(dto.Monitors, affectedDTO{Slug: m.Slug, Name: m.Name})
		}
	}
	for _, u := range inc.Updates {
		dto.Updates = append(dto.Updates, updateDTO{ID: u.ID, Status: u.Status, Body: u.Body, CreatedAt: stamp(u.CreatedAt)})
	}
	return dto
}

type pointDTO struct {
	At     stamp   `json:"at"`
	AvgMS  float64 `json:"avg_ms"`
	MaxMS  int64   `json:"max_ms"`
	Checks int64   `json:"checks"`
	Down   int64   `json:"down"`
}

type beatDTO struct {
	At        stamp          `json:"at"`
	Status    monitor.Status `json:"status"`
	LatencyMS int64          `json:"latency_ms"`
	Message   string         `json:"message,omitzero"`
}

func toBeats(beats []store.Beat, withMessages bool) []beatDTO {
	out := make([]beatDTO, 0, len(beats))
	for _, b := range beats {
		dto := beatDTO{At: stamp(b.At), Status: b.Status, LatencyMS: b.Latency.Milliseconds()}
		if withMessages {
			dto.Message = b.Message
		}
		out = append(out, dto)
	}
	return out
}

type monitorInput struct {
	Name        string           `json:"name"`
	Slug        string           `json:"slug"`
	Description string           `json:"description"`
	Group       string           `json:"group"`
	Kind        monitor.Kind     `json:"kind"`
	Target      string           `json:"target"`
	IntervalS   int              `json:"interval_s"`
	TimeoutS    int              `json:"timeout_s"`
	Retries     int              `json:"retries"`
	Public      bool             `json:"public"`
	Paused      bool             `json:"paused"`
	Settings    monitor.Settings `json:"settings"`
	ChannelIDs  []int64          `json:"channel_ids"`
}

func (in monitorInput) apply(m *monitor.Monitor) {
	m.Name = in.Name
	m.Slug = in.Slug
	m.Description = in.Description
	m.Group = in.Group
	m.Kind = in.Kind
	m.Target = in.Target
	m.Interval = time.Duration(in.IntervalS) * time.Second
	m.Timeout = time.Duration(in.TimeoutS) * time.Second
	m.Retries = in.Retries
	m.Public = in.Public
	m.Settings = in.Settings
	m.ChannelIDs = in.ChannelIDs
}

type adminMonitorDTO struct {
	ID          int64            `json:"id"`
	Slug        string           `json:"slug"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Group       string           `json:"group"`
	Kind        monitor.Kind     `json:"kind"`
	Target      string           `json:"target"`
	IntervalS   int64            `json:"interval_s"`
	TimeoutS    int64            `json:"timeout_s"`
	Retries     int              `json:"retries"`
	Public      bool             `json:"public"`
	Paused      bool             `json:"paused"`
	Position    int              `json:"position"`
	Settings    monitor.Settings `json:"settings"`
	PushToken   string           `json:"push_token,omitzero"`
	ChannelIDs  []int64          `json:"channel_ids"`
	Status      monitor.Status   `json:"status"`
	Since       stamp            `json:"since,omitzero"`
	CheckedAt   stamp            `json:"checked_at,omitzero"`
	LatencyMS   int64            `json:"latency_ms"`
	Message     string           `json:"message"`
	Uptime      *float64         `json:"uptime_24h,omitzero"`
	CreatedAt   stamp            `json:"created_at"`
	UpdatedAt   stamp            `json:"updated_at"`
}

func toAdminMonitor(m *monitor.Monitor) adminMonitorDTO {
	return adminMonitorDTO{
		ID:          m.ID,
		Slug:        m.Slug,
		Name:        m.Name,
		Description: m.Description,
		Group:       m.Group,
		Kind:        m.Kind,
		Target:      m.Target,
		IntervalS:   int64(m.Interval.Seconds()),
		TimeoutS:    int64(m.Timeout.Seconds()),
		Retries:     m.Retries,
		Public:      m.Public,
		Paused:      m.Paused,
		Position:    m.Position,
		Settings:    m.Settings,
		PushToken:   m.PushToken,
		ChannelIDs:  m.ChannelIDs,
		Status:      m.State.Status,
		Since:       stamp(m.State.Since),
		CheckedAt:   stamp(m.State.CheckedAt),
		LatencyMS:   m.State.Latency.Milliseconds(),
		Message:     m.State.Message,
		CreatedAt:   stamp(m.CreatedAt),
		UpdatedAt:   stamp(m.UpdatedAt),
	}
}

type channelDTO struct {
	ID        int64         `json:"id"`
	Name      string        `json:"name"`
	Kind      notify.Kind   `json:"kind"`
	Enabled   bool          `json:"enabled"`
	Config    notify.Config `json:"config"`
	CreatedAt stamp         `json:"created_at"`
	UpdatedAt stamp         `json:"updated_at"`
}

func toChannel(c *notify.Channel) channelDTO {
	return channelDTO{ID: c.ID, Name: c.Name, Kind: c.Kind, Enabled: c.Enabled, Config: c.Config, CreatedAt: stamp(c.CreatedAt), UpdatedAt: stamp(c.UpdatedAt)}
}

type channelInput struct {
	Name    string        `json:"name"`
	Kind    notify.Kind   `json:"kind"`
	Enabled bool          `json:"enabled"`
	Config  notify.Config `json:"config"`
}

type incidentInput struct {
	Title      string               `json:"title"`
	Impact     store.Impact         `json:"impact"`
	Status     store.IncidentStatus `json:"status"`
	Body       string               `json:"body"`
	MonitorIDs []int64              `json:"monitor_ids"`
	StartedAt  time.Time            `json:"started_at,omitzero"`
}

type updateInput struct {
	Status store.IncidentStatus `json:"status"`
	Body   string               `json:"body"`
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type setupInput struct {
	credentials
	Code string `json:"code"`
}

type userDTO struct {
	Username string `json:"username"`
}

func toUser(u *store.User) *userDTO {
	if u == nil {
		return nil
	}
	return &userDTO{Username: u.Username}
}
