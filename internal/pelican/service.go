package pelican

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

type Status struct {
	SyncedAt time.Time
	Nodes    int
	Servers  int
	Err      error
	Syncing  bool
}

type wingsNode struct {
	node  Node
	wings Wings
}

type snapshot struct {
	at     time.Time
	states map[string]ServerState
	err    error
}

type Service struct {
	Store    *store.Store
	Client   *http.Client
	Every    time.Duration
	Log      *slog.Logger
	OnChange func(m *monitor.Monitor, removed bool)

	mu      sync.Mutex
	nodes   map[int64]wingsNode
	servers map[string]int64
	cache   map[int64]*snapshot
	fetch   map[int64]*sync.Mutex
	status  Status
	running sync.Mutex
}

func New(s *store.Store, log *slog.Logger) *Service {
	return &Service{
		Store:   s,
		Client:  &http.Client{Timeout: 15 * time.Second},
		Every:   5 * time.Minute,
		Log:     log,
		nodes:   map[int64]wingsNode{},
		servers: map[string]int64{},
		cache:   map[int64]*snapshot{},
		fetch:   map[int64]*sync.Mutex{},
	}
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Service) Run(ctx context.Context) {
	for {
		if err := s.Sync(ctx); err != nil && !errors.Is(err, errDisabled) {
			s.Log.Warn("sync with pelican", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.Every):
		}
	}
}

var errDisabled = errors.New("the pelican integration is off")

func (s *Service) Begin() {
	go func() {
		if err := s.Sync(context.Background()); err != nil && !errors.Is(err, errDisabled) {
			s.Log.Warn("sync with pelican", "err", err)
		}
	}()
}

func (s *Service) Sync(ctx context.Context) error {
	s.running.Lock()
	defer s.running.Unlock()
	s.mu.Lock()
	s.status.Syncing = true
	s.mu.Unlock()

	nodes, servers, err := s.sync(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.Syncing = false
	if errors.Is(err, errDisabled) {
		s.status = Status{}
		return err
	}
	s.status.Err = err
	if err == nil {
		s.status.SyncedAt, s.status.Nodes, s.status.Servers = time.Now(), nodes, servers
	}
	return err
}

func (s *Service) sync(ctx context.Context) (int, int, error) {
	cfg, err := s.Store.Pelican(ctx)
	if err != nil {
		return 0, 0, err
	}
	if !cfg.Enabled || cfg.URL == "" || cfg.Key == "" {
		s.forget()
		return 0, 0, errDisabled
	}
	panel := &Panel{URL: cfg.URL, Key: cfg.Key, Client: s.Client}

	nodes, err := panel.Nodes(ctx)
	if err != nil {
		return 0, 0, err
	}
	servers, err := panel.Servers(ctx)
	if err != nil {
		return 0, 0, err
	}

	wired := map[int64]wingsNode{}
	for _, n := range nodes {
		token, err := panel.NodeToken(ctx, n.ID)
		if err != nil {
			return 0, 0, fmt.Errorf("node %s: %w", n.Name, err)
		}
		wired[n.ID] = wingsNode{node: n, wings: Wings{Address: n.Address, Token: token, Client: s.Client}}
	}
	owners := map[string]int64{}
	for _, srv := range servers {
		owners[srv.UUID] = srv.Node
	}
	s.mu.Lock()
	s.nodes, s.servers = wired, owners
	s.mu.Unlock()

	if err := s.reconcile(ctx, cfg, nodes, servers); err != nil {
		return 0, 0, err
	}
	return len(nodes), len(servers), nil
}

func (s *Service) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes, s.servers, s.cache = map[int64]wingsNode{}, map[string]int64{}, map[int64]*snapshot{}
}

func (s *Service) reconcile(ctx context.Context, cfg store.Pelican, nodes []Node, servers []Server) error {
	existing, err := s.Store.Monitors(ctx)
	if err != nil {
		return err
	}
	have := map[string]*monitor.Monitor{}
	for _, m := range existing {
		if m.Kind == monitor.KindPelicanNode || m.Kind == monitor.KindPelicanServer {
			have[string(m.Kind)+" "+m.Target] = m
		}
	}
	seen := map[string]bool{}
	nodeMonitor := map[int64]int64{}
	nodeName := map[int64]string{}

	for _, n := range nodes {
		key := string(monitor.KindPelicanNode) + " " + strconv.FormatInt(n.ID, 10)
		seen[key] = true
		nodeName[n.ID] = n.Name
		m, ok := have[key]
		if !ok {
			m = &monitor.Monitor{
				Name:   n.Name,
				Slug:   "node-" + monitor.Slugify(n.Name),
				Group:  n.Name,
				Kind:   monitor.KindPelicanNode,
				Target: strconv.FormatInt(n.ID, 10),
			}
			if err := s.create(ctx, cfg, m); err != nil {
				return fmt.Errorf("node %s: %w", n.Name, err)
			}
		}
		nodeMonitor[n.ID] = m.ID
	}

	for _, srv := range servers {
		key := string(monitor.KindPelicanServer) + " " + srv.UUID
		seen[key] = true
		parent := nodeMonitor[srv.Node]
		group := nodeName[srv.Node]
		if group == "" {
			s.mu.Lock()
			if n, ok := s.nodes[srv.Node]; ok {
				group = n.node.Name
			}
			s.mu.Unlock()
		}
		m, ok := have[key]
		if !ok {
			m = &monitor.Monitor{
				Name:     srv.Name,
				Slug:     monitor.Slugify(srv.Name),
				Group:    group,
				Kind:     monitor.KindPelicanServer,
				Target:   srv.UUID,
				Paused:   srv.Suspended,
				Settings: monitor.Settings{DependsOn: parent},
			}
			if err := s.create(ctx, cfg, m); err != nil {
				return fmt.Errorf("server %s: %w", srv.Name, err)
			}
			continue
		}
		changed := false
		if m.Settings.DependsOn != parent {
			m.Settings.DependsOn = parent
			changed = true
		}
		if m.Paused != srv.Suspended {
			m.Paused = srv.Suspended
			changed = true
		}
		if m.Group != group {
			m.Group = group
			changed = true
		}
		if changed {
			if err := s.Store.UpdateMonitor(ctx, m); err != nil {
				return err
			}
			s.changed(m, false)
		}
	}

	for key, m := range have {
		if seen[key] {
			continue
		}
		if err := s.Store.DeleteMonitor(ctx, m.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		s.Log.Info("removed a monitor that is gone from pelican", "monitor", m.Slug)
		s.changed(m, true)
	}
	return nil
}

func (s *Service) create(ctx context.Context, cfg store.Pelican, m *monitor.Monitor) error {
	m.Public = cfg.Public
	m.ChannelIDs = cfg.ChannelIDs
	m.Retries = 1
	m.Normalize()
	if m.Slug == "" || m.Slug == "node-" {
		m.Slug = "pelican-" + m.Target
	}
	base := m.Slug
	for i := 2; ; i++ {
		taken, err := s.Store.SlugTaken(ctx, m.Slug)
		if err != nil {
			return err
		}
		if !taken {
			break
		}
		m.Slug = base + "-" + strconv.Itoa(i)
	}
	if err := m.Validate(); err != nil {
		return err
	}
	if err := s.Store.CreateMonitor(ctx, m); err != nil {
		return err
	}
	s.Log.Info("added a monitor from pelican", "monitor", m.Slug, "kind", m.Kind)
	s.changed(m, false)
	return nil
}

func (s *Service) changed(m *monitor.Monitor, removed bool) {
	if s.OnChange != nil {
		s.OnChange(m, removed)
	}
}

func (s *Service) Check(ctx context.Context, m *monitor.Monitor) monitor.Result {
	switch m.Kind {
	case monitor.KindPelicanNode:
		return s.checkNode(ctx, m)
	case monitor.KindPelicanServer:
		return s.checkServer(ctx, m)
	}
	return monitor.Down(0, "not a pelican monitor")
}

func (s *Service) node(id int64) (wingsNode, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[id]
	return n, ok
}

func (s *Service) checkNode(ctx context.Context, m *monitor.Monitor) monitor.Result {
	id, _ := strconv.ParseInt(m.Target, 10, 64)
	n, ok := s.node(id)
	if !ok {
		return monitor.Down(0, "waiting for the first sync with pelican")
	}
	latency, err := n.wings.System(ctx)
	if err != nil {
		return monitor.Down(latency, "wings is not answering: %s", err)
	}
	if n.node.Maintenance {
		return monitor.Degraded(latency, "the node is in maintenance mode")
	}
	return monitor.Up(latency, "wings is running")
}

func (s *Service) checkServer(ctx context.Context, m *monitor.Monitor) monitor.Result {
	s.mu.Lock()
	nodeID, ok := s.servers[m.Target]
	s.mu.Unlock()
	if !ok {
		return monitor.Down(0, "waiting for the first sync with pelican")
	}
	n, ok := s.node(nodeID)
	if !ok {
		return monitor.Down(0, "the server's node is unknown")
	}
	started := time.Now()
	states, err := s.states(ctx, nodeID, n.wings)
	latency := time.Since(started)
	if err != nil {
		return monitor.Down(latency, "cannot reach its node: %s", err)
	}
	st, ok := states[m.Target]
	switch {
	case !ok:
		return monitor.Down(latency, "the node does not know this server")
	case st.Suspended:
		return monitor.Degraded(latency, "suspended in the panel")
	}
	switch st.State {
	case "running":
		return monitor.Up(latency, "running%s", usage(st))
	case "starting":
		return monitor.Degraded(latency, "starting")
	case "stopping":
		return monitor.Degraded(latency, "stopping")
	}
	return monitor.Down(latency, "the server is %s", st.State)
}

func usage(st ServerState) string {
	if st.Usage.MemoryLimit <= 0 {
		return ""
	}
	return fmt.Sprintf(", %d of %d MB memory, %.0f%% CPU", st.Usage.Memory>>20, st.Usage.MemoryLimit>>20, st.Usage.CPU)
}

func (s *Service) states(ctx context.Context, nodeID int64, w Wings) (map[string]ServerState, error) {
	s.mu.Lock()
	gate := s.fetch[nodeID]
	if gate == nil {
		gate = &sync.Mutex{}
		s.fetch[nodeID] = gate
	}
	s.mu.Unlock()
	gate.Lock()
	defer gate.Unlock()

	s.mu.Lock()
	snap := s.cache[nodeID]
	if snap != nil && time.Since(snap.at) < 10*time.Second {
		s.mu.Unlock()
		return snap.states, snap.err
	}
	s.mu.Unlock()

	list, err := w.Servers(ctx)
	states := make(map[string]ServerState, len(list))
	for _, st := range list {
		states[st.Config.UUID] = st
	}
	s.mu.Lock()
	s.cache[nodeID] = &snapshot{at: time.Now(), states: states, err: err}
	s.mu.Unlock()
	return states, err
}
