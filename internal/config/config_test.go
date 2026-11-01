package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesFileAndEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heron.toml")
	body := `
listen = ":9000"
public_url = "https://status.example.com/"
trusted_proxies = ["10.0.0.0/8", "127.0.0.1"]
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERON_RETENTION_DAYS", "14")
	t.Setenv("HERON_TIMEZONE", "Europe/Moscow")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Listen != ":9000" {
		t.Errorf("listen = %q", cfg.Listen)
	}
	if cfg.PublicURL != "https://status.example.com" {
		t.Errorf("public_url = %q", cfg.PublicURL)
	}
	if cfg.RetentionDays != 14 {
		t.Errorf("retention_days = %d", cfg.RetentionDays)
	}
	if cfg.Location.String() != "Europe/Moscow" {
		t.Errorf("location = %s", cfg.Location)
	}
	if len(cfg.Proxies) != 2 || cfg.Proxies[1].Bits() != 32 {
		t.Errorf("proxies = %v", cfg.Proxies)
	}
	if !cfg.SecureCookies() {
		t.Error("expected secure cookies for https public url")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("HERON_TIMEZONE", "Mars/Olympus")
	t.Setenv("HERON_LOG_LEVEL", "loud")
	t.Setenv("HERON_PUBLIC_URL", "status.example.com")

	if _, err := Load(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadMissingFileFallsBackToDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != ":8080" || cfg.RetentionDays != 7 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}
