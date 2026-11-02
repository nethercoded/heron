package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/nethercoded/heron/internal/auth"
	"github.com/nethercoded/heron/internal/config"
	"github.com/nethercoded/heron/internal/kuma"
	"github.com/nethercoded/heron/internal/store"
)

const autoBackup = "before-update-"

func backupDir(cfg config.Config) string {
	return filepath.Join(cfg.DataDir, "backups")
}

func backup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	out := fs.String("o", "", "where to write the backup (default: <data>/backups/heron-<time>.db)")
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	path := *out
	if path == "" {
		path = filepath.Join(backupDir(cfg), store.BackupName("heron-", time.Now(), ""))
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := db.Backup(ctx, path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	fmt.Printf("saved %s (%s)\n", path, size(info.Size()))
	return nil
}

func restore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	force := fs.Bool("force", false, "restore even if heron seems to be running")
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: heron restore [-config path] backup.db")
	}
	src := fs.Arg(0)

	if !*force && running(cfg.Listen) {
		return fmt.Errorf("heron is answering on %s, stop it first (systemctl stop heron) or pass -force", cfg.Listen)
	}
	saved, err := store.Restore(context.Background(), cfg.DatabasePath(), src)
	if err != nil {
		return err
	}
	fmt.Printf("restored %s from %s\n", cfg.DatabasePath(), src)
	if saved != "" {
		fmt.Printf("the previous database is kept at %s\n", saved)
	}
	return nil
}

func running(listen string) bool {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func importKuma(args []string) error {
	if len(args) == 0 || args[0] != "kuma" {
		return errors.New("usage: heron import kuma [-config path] [-dry-run] backup.json|kuma.db")
	}
	fs := flag.NewFlagSet("import kuma", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "only show what would be imported")
	cfg, err := loadConfig(fs, args[1:])
	if err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: heron import kuma [-config path] [-dry-run] backup.json|kuma.db")
	}

	ctx := context.Background()
	src, err := kuma.Read(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	plan := src.Plan()

	for _, ch := range plan.Channels {
		fmt.Printf("  + channel  %-28s %s\n", ch.Name, ch.Kind)
	}
	for _, m := range plan.Monitors {
		fmt.Printf("  + monitor  %-28s %-9s %s\n", m.Name, m.Kind, m.DisplayTarget())
		for _, note := range plan.Notes[m.Name] {
			fmt.Printf("             ! %s\n", note)
		}
	}
	for _, s := range plan.Skipped {
		fmt.Printf("  - skipped  %-28s %s\n", s.Name, s.Reason)
	}
	if *dry {
		fmt.Printf("%d monitors and %d channels would be imported, %d skipped\n", len(plan.Monitors), len(plan.Channels), len(plan.Skipped))
		return nil
	}

	db, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := plan.Apply(ctx, db); err != nil {
		return err
	}
	fmt.Printf("imported %d monitors and %d channels, skipped %d\n", len(plan.Monitors), len(plan.Channels), len(plan.Skipped))
	if running(cfg.Listen) {
		fmt.Println("restart heron to start checking them (systemctl restart heron)")
	}
	return nil
}

func size(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func ensureDemo(ctx context.Context, db *store.Store, cfg config.Config) error {
	name, password := cfg.DemoLogin()
	hash, err := auth.Hash(password)
	if err != nil {
		return err
	}
	u, err := db.UserByName(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		_, err = db.CreateUser(ctx, name, hash, store.RoleViewer)
		return err
	}
	if err != nil {
		return err
	}
	if u.Role == store.RoleOwner {
		return fmt.Errorf("demo user %q is the owner, pick another name", name)
	}
	if err := db.SetPassword(ctx, u.ID, hash); err != nil {
		return err
	}
	return db.SetRole(ctx, u.ID, store.RoleViewer)
}
