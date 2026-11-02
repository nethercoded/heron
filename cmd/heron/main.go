package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"golang.org/x/term"

	"github.com/nethercoded/heron/internal/auth"
	"github.com/nethercoded/heron/internal/config"
	"github.com/nethercoded/heron/internal/engine"
	"github.com/nethercoded/heron/internal/httpapi"
	"github.com/nethercoded/heron/internal/maintenance"
	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/scheduler"
	"github.com/nethercoded/heron/internal/site"
	"github.com/nethercoded/heron/internal/store"
	"github.com/nethercoded/heron/internal/update"
	"github.com/nethercoded/heron/web"
)

var version = "dev"

const usage = `heron is a self-hosted status page and uptime monitor.

Usage:
  heron [serve] [-config path]          start the server
  heron user create -u name [-config]   create an administrator
  heron user passwd -u name [-config]   change a password
  heron user reset-2fa -u name          turn off two-factor sign in
  heron backup [-o file]                save a copy of the database
  heron restore [-force] file           replace the database with a backup
  heron import kuma [-dry-run] file     bring monitors over from Uptime Kuma
  heron update [-check]                 download the latest release in place
  heron version                         print the version

The config file defaults to $HERON_CONFIG or ./heron.toml.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "heron:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	switch cmd {
	case "serve":
		err := serve(args)
		if next, ok := errors.AsType[*update.Restart](err); ok {
			return update.Launch(next.Path)
		}
		return err
	case "user":
		return user(args)
	case "backup":
		return backup(args)
	case "restore":
		return restore(args)
	case "import":
		return importKuma(args)
	case "update":
		return selfUpdate(args)
	case "version":
		fmt.Println("heron", buildVersion())
		return nil
	case "help":
		fmt.Print(usage)
		return nil
	}
	fmt.Fprint(os.Stderr, usage)
	return fmt.Errorf("unknown command %q", cmd)
}

func loadConfig(fs *flag.FlagSet, args []string) (config.Config, error) {
	path := fs.String("config", defaultConfigPath(), "path to the config file")
	if err := fs.Parse(args); err != nil {
		return config.Config{}, err
	}
	return config.Load(*path)
}

func defaultConfigPath() string {
	if p := os.Getenv("HERON_CONFIG"); p != "" {
		return p
	}
	return "heron.toml"
}

func newLogger(cfg config.Config) *slog.Logger {
	level, _ := cfg.Level()
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

func openStore(ctx context.Context, cfg config.Config) (*store.Store, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return store.Open(ctx, cfg.DatabasePath())
}

func serve(args []string) error {
	cfg, err := loadConfig(flag.NewFlagSet("serve", flag.ExitOnError), args)
	if err != nil {
		return err
	}
	log := newLogger(cfg)
	slog.SetDefault(log)

	ver := buildVersion()
	rollback := func(ctx context.Context, backup string) (bool, error) {
		return store.RestoreIfNewer(ctx, cfg.DatabasePath(), backup)
	}
	if err := update.Handoff(cfg.DataDir, ver, rollback, log); err != nil {
		log.Warn("could not start the downloaded update, staying on the current version", "err", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	userAgent := "heron/" + ver + " (+https://github.com/nethercoded/heron)"

	dispatcher := notify.NewDispatcher(notify.Factory{
		Client:    &http.Client{Timeout: 15 * time.Second},
		UserAgent: userAgent,
	}, log.With("component", "notify"))

	eng := engine.New(db, dispatcher, maintenance.NewCalendar(cfg.Location), log.With("component", "engine"), cfg.PublicURL)
	if err := eng.ReloadMaintenance(ctx); err != nil {
		return fmt.Errorf("load maintenance windows: %w", err)
	}
	checkers := monitor.NewRegistry(userAgent, cfg.ICMPPrivileged)
	sched := scheduler.New(checkers, eng, log.With("component", "scheduler"))

	monitors, err := db.Monitors(ctx)
	if err != nil {
		return err
	}

	runCtx, cancelRun := context.WithCancel(context.Background())
	sched.Start(runCtx, monitors)
	go eng.Housekeep(runCtx, time.Duration(cfg.RetentionDays)*24*time.Hour)
	go eng.WatchMaintenance(runCtx)

	restart := make(chan string, 1)
	updates := &update.Service{
		Updater: update.New(ver, userAgent),
		DataDir: cfg.DataDir,
		Every:   6 * time.Hour,
		Auto:    db.AutoUpdate,
		Restart: func(path string) {
			select {
			case restart <- path:
			default:
			}
		},
		Snapshot: func(ctx context.Context, version string) (string, error) {
			dir := backupDir(cfg)
			path := filepath.Join(dir, store.BackupName(autoBackup, time.Now(), version))
			if err := db.Backup(ctx, path); err != nil {
				return "", err
			}
			return path, store.Prune(dir, autoBackup, 5)
		},
		Log: log.With("component", "update"),
	}
	if cfg.CheckUpdates {
		go updates.Run(runCtx)
	}

	users, err := db.CountUsers(ctx)
	if err != nil {
		cancelRun()
		return err
	}
	var setupCode string
	if users == 0 {
		setupCode = rand.Text()[:12]
	}
	demoUser, demoPassword := cfg.DemoLogin()
	switch {
	case cfg.Demo == "":
	case users == 0:
		log.Warn("the demo account is created once heron is set up, restart after the installer")
		demoUser, demoPassword = "", ""
	default:
		if err := ensureDemo(ctx, db, cfg); err != nil {
			cancelRun()
			return fmt.Errorf("demo account: %w", err)
		}
		log.Info("demo mode, anyone can look around the admin panel", "user", demoUser)
	}

	domains := site.New(db, filepath.Join(cfg.DataDir, "certs"), cfg.PublicURL, log.With("component", "site"))
	domains.OnChange = eng.SetPublicURL

	api := httpapi.New(db, eng, sched, dispatcher, auth.NewSessions(db, cfg.SecureCookies()), log.With("component", "http"), httpapi.Options{
		Version:        ver,
		SetupCode:      setupCode,
		Site:           domains,
		Location:       cfg.Location,
		TrustedProxies: cfg.Proxies,
		Assets:         web.Assets(),
		Updates:        updates,
		Checker:        checkers,
		DemoUser:       demoUser,
		DemoPassword:   demoPassword,
	})
	handler := api.Handler()
	domains.Handler = handler
	if err := domains.Start(ctx); err != nil {
		log.Error("could not start https for the saved domain, serving plain http only", "err", err)
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelDebug),
		BaseContext:       func(net.Listener) context.Context { return runCtx },
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.Listen)
	if err != nil {
		cancelRun()
		return err
	}

	log.Info("heron started", "version", ver, "listen", ln.Addr().String(), "monitors", len(monitors), "data", cfg.DataDir)
	if setupCode != "" {
		log.Warn("no administrator yet, open the admin panel and enter the setup code", "url", setupURL(cfg, ln.Addr()), "setup_code", setupCode)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	sick := make(chan error, 1)
	go func() {
		if err := update.Confirm(runCtx, cfg.DataDir, ver, selfCheck(ln.Addr(), ver), log.With("component", "update")); err != nil {
			sick <- err
		}
	}()

	var next string
	select {
	case err := <-serveErr:
		cancelRun()
		sched.Wait()
		return err
	case <-ctx.Done():
	case next = <-restart:
	case err := <-sick:
		log.Error("this version is not healthy, going back to the previous one", "err", err)
		if next, err = os.Executable(); err != nil {
			cancelRun()
			return err
		}
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cancelRun()
	domains.Close(shutdownCtx)
	errs := []error{srv.Shutdown(shutdownCtx)}
	sched.Wait()
	errs = append(errs, dispatcher.Close(shutdownCtx))
	err = errors.Join(errs...)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		if next == "" {
			return err
		}
		log.Warn("unclean shutdown before the update", "err", err)
	}
	if next != "" {
		log.Info("starting the new version", "path", next)
		return &update.Restart{Path: next}
	}
	log.Info("bye")
	return nil
}

func selfUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	check := fs.Bool("check", false, "only report whether a newer release exists")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ver := buildVersion()
	u := update.New(ver, "heron/"+ver+" (+https://github.com/nethercoded/heron)")
	ctx := context.Background()
	rel, err := u.Latest(ctx)
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}
	if !u.Newer(rel) {
		fmt.Printf("heron %s is up to date (latest release %s)\n", ver, rel.Version())
		return nil
	}
	fmt.Printf("heron %s is available, you run %s\n%s\n", rel.Version(), ver, rel.URL)
	if *check {
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	if err := u.Download(ctx, rel, exe); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("%w, run it as root: sudo heron update", err)
		}
		return err
	}
	fmt.Printf("updated %s to %s, restart heron to use it (systemctl restart heron)\n", exe, rel.Version())
	return nil
}

func selfCheck(addr net.Addr, ver string) func(context.Context) error {
	host, port, _ := net.SplitHostPort(addr.String())
	if ip := net.ParseIP(host); ip == nil || ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	url := "http://" + net.JoinHostPort(host, port) + "/healthz"
	client := &http.Client{Timeout: 5 * time.Second}
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var health struct {
			Version string `json:"version"`
		}
		if err := json.UnmarshalRead(io.LimitReader(resp.Body, 4<<10), &health); err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("/healthz answered %s", resp.Status)
		}
		if health.Version != ver {
			return fmt.Errorf("/healthz reports version %q", health.Version)
		}
		return nil
	}
}

func setupURL(cfg config.Config, addr net.Addr) string {
	if cfg.PublicURL != "" {
		return cfg.PublicURL + "/admin"
	}
	_, port, _ := net.SplitHostPort(addr.String())
	return "http://localhost:" + port + "/admin"
}

func user(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: heron user create|passwd|reset-2fa -u name")
	}
	action := args[0]

	fs := flag.NewFlagSet("user "+action, flag.ExitOnError)
	username := fs.String("u", "", "username")
	cfg, err := loadConfig(fs, args[1:])
	if err != nil {
		return err
	}
	if *username == "" {
		return errors.New("-u is required")
	}

	ctx := context.Background()
	db, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	if action == "reset-2fa" {
		u, err := db.UserByName(ctx, *username)
		if err != nil {
			return fmt.Errorf("user %q: %w", *username, err)
		}
		if err := db.SetTOTP(ctx, u.ID, ""); err != nil {
			return err
		}
		fmt.Printf("two-factor sign in for %s is off\n", *username)
		return nil
	}

	password, err := readPassword(os.Stdin)
	if err != nil {
		return err
	}
	if err := auth.CheckStrength(password); err != nil {
		return err
	}
	hash, err := auth.Hash(password)
	if err != nil {
		return err
	}

	switch action {
	case "create":
		count, err := db.CountUsers(ctx)
		if err != nil {
			return err
		}
		role := store.RoleAdmin
		if count == 0 {
			role = store.RoleOwner
		}
		if _, err := db.CreateUser(ctx, *username, hash, role); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return fmt.Errorf("user %q already exists", *username)
			}
			return err
		}
		fmt.Printf("user %s created (%s)\n", *username, role)
	case "passwd":
		u, err := db.UserByName(ctx, *username)
		if err != nil {
			return fmt.Errorf("user %q: %w", *username, err)
		}
		if err := db.SetPassword(ctx, u.ID, hash); err != nil {
			return err
		}
		if err := db.DeleteUserSessions(ctx, u.ID, nil); err != nil {
			return err
		}
		fmt.Printf("password for %s updated\n", *username)
	default:
		return fmt.Errorf("unknown user command %q", action)
	}
	return nil
}

func readPassword(in *os.File) (string, error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Repeat password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}
