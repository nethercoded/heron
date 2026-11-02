package httpapi

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/nethercode/heron/internal/auth"
	"github.com/nethercode/heron/internal/engine"
	"github.com/nethercode/heron/internal/monitor"
	"github.com/nethercode/heron/internal/notify"
	"github.com/nethercode/heron/internal/store"
)

type Scheduler interface {
	Sync(m *monitor.Monitor)
	Stop(id int64)
	Trigger(id int64) bool
	Push(id int64, res monitor.Result) bool
}

type Sender interface {
	Send(ctx context.Context, ch notify.Channel, msg notify.Message) error
}

type Options struct {
	Version        string
	SetupCode      string
	Location       *time.Location
	TrustedProxies []netip.Prefix
	Assets         fs.FS
}

type Server struct {
	store     *store.Store
	engine    *engine.Engine
	scheduler Scheduler
	sender    Sender
	sessions  *auth.Sessions
	limiter   *auth.Limiter
	log       *slog.Logger
	opts      Options
}

func New(s *store.Store, e *engine.Engine, sched Scheduler, sender Sender, sessions *auth.Sessions, log *slog.Logger, opts Options) *Server {
	if opts.Location == nil {
		opts.Location = time.UTC
	}
	return &Server{
		store:     s,
		engine:    e,
		scheduler: sched,
		sender:    sender,
		sessions:  sessions,
		limiter:   auth.NewLimiter(10, 15*time.Minute),
		log:       log,
		opts:      opts,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /healthz", s.handle(s.health))

	mux.Handle("GET /api/status", s.handle(s.status))
	mux.Handle("GET /api/summary", s.handle(s.summary))
	mux.Handle("GET /badge", s.handle(s.overallBadge))
	mux.Handle("GET /badge/{slug}", s.handle(s.statusBadge))
	mux.Handle("GET /badge/{slug}/uptime", s.handle(s.uptimeBadge))
	mux.Handle("GET /badge/{slug}/response", s.handle(s.responseBadge))
	mux.Handle("GET /api/monitors/{slug}", s.handle(s.monitorDetail))
	mux.Handle("GET /api/monitors/{slug}/series", s.handle(s.monitorSeries))
	mux.Handle("GET /api/monitors/{slug}/calendar", s.handle(s.monitorCalendar))
	mux.Handle("GET /api/incidents", s.handle(s.incidents))
	mux.Handle("GET /api/incidents/{id}", s.handle(s.incident))
	mux.Handle("GET /api/stream", s.handle(s.stream))
	mux.Handle("GET /api/push/{token}", s.handle(s.push))
	mux.Handle("POST /api/push/{token}", s.handle(s.push))

	mux.Handle("GET /api/auth/state", s.handle(s.authState))
	mux.Handle("POST /api/auth/setup", s.handle(s.setup))
	mux.Handle("POST /api/auth/login", s.handle(s.login))
	mux.Handle("POST /api/auth/logout", s.handle(s.logout))

	admin := http.NewServeMux()
	admin.Handle("PUT /api/admin/account/password", s.handle(s.changePassword))

	admin.Handle("GET /api/admin/monitors", s.handle(s.listMonitors))
	admin.Handle("POST /api/admin/monitors", s.handle(s.createMonitor))
	admin.Handle("PUT /api/admin/monitors/order", s.handle(s.reorderMonitors))
	admin.Handle("GET /api/admin/monitors/{id}", s.handle(s.getMonitor))
	admin.Handle("GET /api/admin/monitors/{id}/series", s.handle(s.adminSeries))
	admin.Handle("GET /api/admin/monitors/{id}/calendar", s.handle(s.adminCalendar))
	admin.Handle("PUT /api/admin/monitors/{id}", s.handle(s.updateMonitor))
	admin.Handle("DELETE /api/admin/monitors/{id}", s.handle(s.deleteMonitor))
	admin.Handle("POST /api/admin/monitors/{id}/pause", s.handle(s.pauseMonitor(true)))
	admin.Handle("POST /api/admin/monitors/{id}/resume", s.handle(s.pauseMonitor(false)))
	admin.Handle("POST /api/admin/monitors/{id}/check", s.handle(s.checkMonitor))
	admin.Handle("POST /api/admin/monitors/{id}/token", s.handle(s.rotateToken))

	admin.Handle("GET /api/admin/channels", s.handle(s.listChannels))
	admin.Handle("POST /api/admin/channels", s.handle(s.createChannel))
	admin.Handle("PUT /api/admin/channels/{id}", s.handle(s.updateChannel))
	admin.Handle("DELETE /api/admin/channels/{id}", s.handle(s.deleteChannel))
	admin.Handle("POST /api/admin/channels/{id}/test", s.handle(s.testChannel))

	admin.Handle("GET /api/admin/incidents", s.handle(s.listIncidents))
	admin.Handle("POST /api/admin/incidents", s.handle(s.createIncident))
	admin.Handle("GET /api/admin/incidents/{id}", s.handle(s.getIncident))
	admin.Handle("PUT /api/admin/incidents/{id}", s.handle(s.updateIncident))
	admin.Handle("DELETE /api/admin/incidents/{id}", s.handle(s.deleteIncident))
	admin.Handle("POST /api/admin/incidents/{id}/updates", s.handle(s.addIncidentUpdate))

	admin.Handle("GET /api/admin/maintenance", s.handle(s.listMaintenance))
	admin.Handle("POST /api/admin/maintenance", s.handle(s.createMaintenance))
	admin.Handle("GET /api/admin/maintenance/{id}", s.handle(s.getMaintenance))
	admin.Handle("PUT /api/admin/maintenance/{id}", s.handle(s.updateMaintenance))
	admin.Handle("DELETE /api/admin/maintenance/{id}", s.handle(s.deleteMaintenance))

	admin.Handle("GET /api/admin/page", s.handle(s.getPage))
	admin.Handle("PUT /api/admin/page", s.handle(s.savePage))

	mux.Handle("/api/admin/", s.requireAdmin(admin))
	mux.Handle("/api/", s.handle(func(http.ResponseWriter, *http.Request) error {
		return notFound("endpoint")
	}))

	if s.opts.Assets != nil {
		mux.Handle("/", spa(s.opts.Assets))
	}

	csrf := http.NewCrossOriginProtection()
	csrf.AddInsecureBypassPattern("POST /api/push/{token}")
	csrf.SetDenyHandler(s.handle(func(http.ResponseWriter, *http.Request) error {
		return errorf(http.StatusForbidden, "cross_origin", "cross-origin request rejected")
	}))

	return s.observe(secureHeaders(csrf.Handler(mux)))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) error {
	if err := s.store.Ping(r.Context()); err != nil {
		return errorf(http.StatusServiceUnavailable, "unavailable", "database unavailable")
	}
	return ok(w, map[string]string{"status": "ok", "version": s.opts.Version})
}
