package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"golang.org/x/term"

	"github.com/nethercode/heron/internal/auth"
	"github.com/nethercode/heron/internal/config"
	"github.com/nethercode/heron/internal/engine"
	"github.com/nethercode/heron/internal/httpapi"
	"github.com/nethercode/heron/internal/monitor"
	"github.com/nethercode/heron/internal/notify"
	"github.com/nethercode/heron/internal/scheduler"
	"github.com/nethercode/heron/internal/store"
	"github.com/nethercode/heron/web"
)

var version = "dev"

const usage = `heron is a self-hosted status page and uptime monitor.

Usage:
  heron [serve] [-config path]          start the server
  heron user create -u name [-config]   create an administrator
  heron user passwd -u name [-config]   change a password
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
		return serve(args)
	case "user":
		return user(args)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	ver := buildVersion()
	userAgent := "heron/" + ver + " (+https://github.com/nethercode/heron)"

	dispatcher := notify.NewDispatcher(notify.Factory{
		Client:    &http.Client{Timeout: 15 * time.Second},
		UserAgent: userAgent,
	}, log.With("component", "notify"))

	eng := engine.New(db, dispatcher, log.With("component", "engine"), cfg.PublicURL)
	sched := scheduler.New(monitor.NewRegistry(userAgent, cfg.ICMPPrivileged), eng, log.With("component", "scheduler"))

	monitors, err := db.Monitors(ctx)
	if err != nil {
		return err
	}

	runCtx, cancelRun := context.WithCancel(context.Background())
	sched.Start(runCtx, monitors)
	go eng.Housekeep(runCtx, time.Duration(cfg.RetentionDays)*24*time.Hour)

	api := httpapi.New(db, eng, sched, dispatcher, auth.NewSessions(db, cfg.SecureCookies()), log.With("component", "http"), httpapi.Options{
		Version:        ver,
		Location:       cfg.Location,
		TrustedProxies: cfg.Proxies,
		Assets:         web.Assets(),
	})

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelDebug),
		BaseContext:       func(net.Listener) context.Context { return runCtx },
	}

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		cancelRun()
		return err
	}

	users, _ := db.CountUsers(ctx)
	log.Info("heron started", "version", ver, "listen", ln.Addr().String(), "monitors", len(monitors), "data", cfg.DataDir)
	if users == 0 {
		log.Warn("no administrator yet, open the admin panel to finish setup", "url", setupURL(cfg, ln.Addr()))
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		cancelRun()
		sched.Wait()
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cancelRun()
	errs := []error{srv.Shutdown(shutdownCtx)}
	sched.Wait()
	errs = append(errs, dispatcher.Close(shutdownCtx))
	if err := errors.Join(errs...); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("bye")
	return nil
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
		return errors.New("usage: heron user create|passwd -u name")
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
		if _, err := db.CreateUser(ctx, *username, hash); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return fmt.Errorf("user %q already exists", *username)
			}
			return err
		}
		fmt.Printf("user %s created\n", *username)
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
