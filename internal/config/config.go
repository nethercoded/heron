package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Listen         string   `toml:"listen"`
	DataDir        string   `toml:"data_dir"`
	PublicURL      string   `toml:"public_url"`
	LogLevel       string   `toml:"log_level"`
	LogFormat      string   `toml:"log_format"`
	Timezone       string   `toml:"timezone"`
	RetentionDays  int      `toml:"retention_days"`
	TrustedProxies []string `toml:"trusted_proxies"`
	ICMPPrivileged bool     `toml:"icmp_privileged"`

	Location *time.Location `toml:"-"`
	Proxies  []netip.Prefix `toml:"-"`
}

func Default() Config {
	return Config{
		Listen:        ":8080",
		DataDir:       "./data",
		LogLevel:      "info",
		LogFormat:     "text",
		Timezone:      "UTC",
		RetentionDays: 7,
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		if _, err := toml.DecodeFile(path, &cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("read config %s: %w", path, err)
		}
	}
	if err := cfg.applyEnv(os.LookupEnv); err != nil {
		return cfg, err
	}
	if err := cfg.resolve(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) applyEnv(lookup func(string) (string, bool)) error {
	text := map[string]*string{
		"HERON_LISTEN":     &c.Listen,
		"HERON_DATA_DIR":   &c.DataDir,
		"HERON_PUBLIC_URL": &c.PublicURL,
		"HERON_LOG_LEVEL":  &c.LogLevel,
		"HERON_LOG_FORMAT": &c.LogFormat,
		"HERON_TIMEZONE":   &c.Timezone,
	}
	for key, dst := range text {
		if v, ok := lookup(key); ok {
			*dst = v
		}
	}
	if v, ok := lookup("HERON_RETENTION_DAYS"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("HERON_RETENTION_DAYS: %w", err)
		}
		c.RetentionDays = n
	}
	if v, ok := lookup("HERON_ICMP_PRIVILEGED"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("HERON_ICMP_PRIVILEGED: %w", err)
		}
		c.ICMPPrivileged = b
	}
	if v, ok := lookup("HERON_TRUSTED_PROXIES"); ok {
		c.TrustedProxies = strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
	}
	return nil
}

func (c *Config) resolve() error {
	var errs []error

	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		errs = append(errs, fmt.Errorf("timezone: %w", err))
	}
	c.Location = loc

	if c.RetentionDays < 1 {
		errs = append(errs, errors.New("retention_days must be at least 1"))
	}

	if c.PublicURL != "" {
		u, err := url.Parse(c.PublicURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			errs = append(errs, fmt.Errorf("public_url %q is not an absolute url", c.PublicURL))
		}
		c.PublicURL = strings.TrimRight(c.PublicURL, "/")
	}

	c.Proxies = c.Proxies[:0]
	for _, raw := range c.TrustedProxies {
		p, err := parsePrefix(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("trusted_proxies: %w", err))
			continue
		}
		c.Proxies = append(c.Proxies, p)
	}

	if _, err := c.Level(); err != nil {
		errs = append(errs, err)
	}

	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		errs = append(errs, fmt.Errorf("data_dir: %w", err))
	}
	c.DataDir = abs

	return errors.Join(errs...)
}

func (c Config) Level() (slog.Level, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return l, fmt.Errorf("log_level %q: %w", c.LogLevel, err)
	}
	return l, nil
}

func (c Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "heron.db")
}

func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.PublicURL, "https://")
}

func parsePrefix(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		return netip.ParsePrefix(s)
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}
