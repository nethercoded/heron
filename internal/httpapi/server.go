package httpapi

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/nethercoded/heron/internal/auth"
	"github.com/nethercoded/heron/internal/engine"
	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/site"
	"github.com/nethercoded/heron/internal/store"
	"github.com/nethercoded/heron/internal/update"
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
	Site           Site
	Location       *time.Location
	TrustedProxies []netip.Prefix
	Assets         fs.FS
	Updates        *update.Service
}

type Server struct {
	store      *store.Store
	engine     *engine.Engine
	scheduler  Scheduler
	sender     Sender
	sessions   *auth.Sessions
	limiter    *auth.Limiter
	secondStep *auth.Tickets[int64]
	ceremonies *auth.Tickets[ceremony]
	log        *slog.Logger
	opts       Options
}

func New(s *store.Store, e *engine.Engine, sched Scheduler, sender Sender, sessions *auth.Sessions, log *slog.Logger, opts Options) *Server {
	if opts.Location == nil {
		opts.Location = time.UTC
	}
	return &Server{
		store:      s,
		engine:     e,
		scheduler:  sched,
		sender:     sender,
		sessions:   sessions,
		limiter:    auth.NewLimiter(10, 15*time.Minute),
		secondStep: auth.NewTickets[int64](5 * time.Minute),
		ceremonies: auth.NewTickets[ceremony](5 * time.Minute),
		log:        log,
		opts:       opts,
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
	mux.Handle("POST /api/install/domain", s.handle(s.installDomain))
	mux.HandleFunc("GET "+site.ProbePath+"{nonce}", s.probe)
	mux.Handle("POST /api/auth/login", s.handle(s.login))
	mux.Handle("POST /api/auth/login/code", s.handle(s.loginWithCode))
	mux.Handle("POST /api/auth/passkey/begin", s.handle(s.beginPasskeyLogin))
	mux.Handle("POST /api/auth/passkey/finish", s.handle(s.finishPasskeyLogin))
	mux.Handle("POST /api/auth/invite", s.handle(s.inviteInfo))
	mux.Handle("POST /api/auth/join", s.handle(s.join))
	mux.Handle("POST /api/auth/logout", s.handle(s.logout))
	mux.Handle("GET /metrics", s.handle(s.metrics))

	admin := http.NewServeMux()
	do := func(action string, h handlerFunc) http.Handler {
		return s.handle(s.audited(action, h))
	}
	owner := func(action string, h handlerFunc) http.Handler {
		return s.handle(s.role(store.RoleOwner, s.audited(action, h)))
	}

	admin.Handle("PUT /api/admin/account/password", do("account.password", s.changePassword))
	admin.Handle("POST /api/admin/account/totp", s.handle(s.startTOTP))
	admin.Handle("PUT /api/admin/account/totp", do("account.2fa_on", s.enableTOTP))
	admin.Handle("POST /api/admin/account/totp/disable", do("account.2fa_off", s.disableTOTP))
	admin.Handle("GET /api/admin/account/passkeys", s.handle(s.listPasskeys))
	admin.Handle("POST /api/admin/account/passkeys/begin", s.handle(s.beginPasskey))
	admin.Handle("POST /api/admin/account/passkeys/finish", do("account.passkey_add", s.finishPasskey))
	admin.Handle("DELETE /api/admin/account/passkeys/{id}", do("account.passkey_remove", s.deletePasskey))

	admin.Handle("GET /api/admin/monitors", s.handle(s.listMonitors))
	admin.Handle("POST /api/admin/monitors", do("monitor.create", s.createMonitor))
	admin.Handle("PUT /api/admin/monitors/order", do("monitor.reorder", s.reorderMonitors))
	admin.Handle("GET /api/admin/monitors/{id}", s.handle(s.getMonitor))
	admin.Handle("GET /api/admin/monitors/{id}/series", s.handle(s.adminSeries))
	admin.Handle("GET /api/admin/monitors/{id}/calendar", s.handle(s.adminCalendar))
	admin.Handle("PUT /api/admin/monitors/{id}", do("monitor.update", s.updateMonitor))
	admin.Handle("DELETE /api/admin/monitors/{id}", do("monitor.delete", s.deleteMonitor))
	admin.Handle("POST /api/admin/monitors/{id}/pause", do("monitor.pause", s.pauseMonitor(true)))
	admin.Handle("POST /api/admin/monitors/{id}/resume", do("monitor.resume", s.pauseMonitor(false)))
	admin.Handle("POST /api/admin/monitors/{id}/check", s.handle(s.checkMonitor))
	admin.Handle("POST /api/admin/monitors/{id}/token", do("monitor.token", s.rotateToken))

	admin.Handle("GET /api/admin/channels", s.handle(s.role(store.RoleAdmin, s.listChannels)))
	admin.Handle("POST /api/admin/channels", do("channel.create", s.createChannel))
	admin.Handle("PUT /api/admin/channels/{id}", do("channel.update", s.updateChannel))
	admin.Handle("DELETE /api/admin/channels/{id}", do("channel.delete", s.deleteChannel))
	admin.Handle("POST /api/admin/channels/{id}/test", s.handle(s.testChannel))

	admin.Handle("GET /api/admin/incidents", s.handle(s.listIncidents))
	admin.Handle("POST /api/admin/incidents", do("incident.create", s.createIncident))
	admin.Handle("GET /api/admin/incidents/{id}", s.handle(s.getIncident))
	admin.Handle("PUT /api/admin/incidents/{id}", do("incident.update", s.updateIncident))
	admin.Handle("DELETE /api/admin/incidents/{id}", do("incident.delete", s.deleteIncident))
	admin.Handle("POST /api/admin/incidents/{id}/updates", do("incident.post", s.addIncidentUpdate))

	admin.Handle("GET /api/admin/maintenance", s.handle(s.listMaintenance))
	admin.Handle("POST /api/admin/maintenance", do("maintenance.create", s.createMaintenance))
	admin.Handle("GET /api/admin/maintenance/{id}", s.handle(s.getMaintenance))
	admin.Handle("PUT /api/admin/maintenance/{id}", do("maintenance.update", s.updateMaintenance))
	admin.Handle("DELETE /api/admin/maintenance/{id}", do("maintenance.delete", s.deleteMaintenance))

	admin.Handle("GET /api/admin/site", s.handle(s.getSite))
	admin.Handle("PUT /api/admin/site", owner("settings.domain", s.saveSite))

	admin.Handle("GET /api/admin/page", s.handle(s.getPage))
	admin.Handle("PUT /api/admin/page", do("settings.page", s.savePage))

	admin.Handle("GET /api/admin/metrics", s.handle(s.role(store.RoleAdmin, s.getMetricsToken)))
	admin.Handle("POST /api/admin/metrics", do("settings.metrics_token", s.rotateMetricsToken))
	admin.Handle("DELETE /api/admin/metrics", do("settings.metrics_off", s.disableMetrics))

	admin.Handle("GET /api/admin/users", s.handle(s.listUsers))
	admin.Handle("POST /api/admin/invites", owner("user.invite", s.createInvite))
	admin.Handle("DELETE /api/admin/invites/{id}", owner("user.invite_revoke", s.deleteInvite))
	admin.Handle("PUT /api/admin/users/{id}", owner("user.role", s.updateUser))
	admin.Handle("DELETE /api/admin/users/{id}", owner("user.delete", s.deleteUser))
	admin.Handle("DELETE /api/admin/users/{id}/sessions", owner("user.sign_out", s.revokeUserSessions))
	admin.Handle("DELETE /api/admin/users/{id}/two-factor", owner("user.2fa_reset", s.resetTwoFactor))

	admin.Handle("GET /api/admin/audit", s.handle(s.listAudit))

	admin.Handle("GET /api/admin/backup", owner("backup.download", s.downloadBackup))
	admin.Handle("POST /api/admin/import/kuma/preview", s.handle(s.role(store.RoleAdmin, s.previewKuma)))
	admin.Handle("POST /api/admin/import/kuma", do("monitor.import", s.importKuma))

	admin.Handle("GET /api/admin/updates", s.handle(s.role(store.RoleOwner, s.getUpdates)))
	admin.Handle("POST /api/admin/updates/check", s.handle(s.role(store.RoleOwner, s.checkUpdates)))
	admin.Handle("PUT /api/admin/updates", owner("settings.auto_update", s.saveUpdates))
	admin.Handle("POST /api/admin/updates/install", owner("update.install", s.installUpdate))

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
