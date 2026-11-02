package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		candidate, current string
		want               bool
	}{
		{"0.2.1", "0.2.0", true},
		{"v0.10.0", "0.9.9", true},
		{"1.0.0", "v1.0.0", false},
		{"0.2.0", "0.3.0", false},
		{"0.3.0", "dev", false},
		{"0.3.0-rc1", "0.2.0", true},
		{"garbage", "0.1.0", false},
	} {
		if got := Newer(tc.candidate, tc.current); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.candidate, tc.current, got, tc.want)
		}
	}
}

type fakeGitHub struct {
	*httptest.Server
	archive []byte
	sums    string
}

func newFakeGitHub(t *testing.T, version string, binary []byte) *fakeGitHub {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range map[string][]byte{"README.md": []byte("hi"), "heron": binary} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write(body)
	}
	_ = tw.Close()
	_ = gz.Close()

	name := fmt.Sprintf("heron_%s_linux_amd64.tar.gz", version)
	sum := sha256.Sum256(buf.Bytes())
	gh := &fakeGitHub{archive: buf.Bytes(), sums: hex.EncodeToString(sum[:]) + "  " + name + "\n"}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/nethercoded/heron/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		_ = json.MarshalWrite(w, map[string]any{
			"tag_name":     "v" + version,
			"html_url":     base + "/release",
			"body":         "notes",
			"published_at": time.Now().UTC().Format(time.RFC3339),
			"assets": []map[string]string{
				{"name": name, "browser_download_url": base + "/dl/" + name},
				{"name": "checksums.txt", "browser_download_url": base + "/dl/checksums.txt"},
			},
		})
	})
	mux.HandleFunc("GET /dl/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, gh.sums)
	})
	mux.HandleFunc("GET /dl/"+name, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(gh.archive)
	})
	gh.Server = httptest.NewServer(mux)
	t.Cleanup(gh.Close)
	return gh
}

func testUpdater(gh *fakeGitHub, current string) *Updater {
	u := New(current, "heron-test")
	u.API = gh.URL
	u.OS, u.Arch = "linux", "amd64"
	return u
}

func skipProbe(t *testing.T) {
	saved := probe
	probe = func(context.Context, string, string) error { return nil }
	t.Cleanup(func() { probe = saved })
}

func TestDownload(t *testing.T) {
	skipProbe(t)
	gh := newFakeGitHub(t, "0.3.0", []byte("new binary"))
	u := testUpdater(gh, "0.2.0")

	rel, err := u.Latest(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version() != "0.3.0" || !u.Newer(rel) {
		t.Fatalf("release = %+v", rel)
	}

	dst := filepath.Join(t.TempDir(), "heron")
	if err := u.Download(t.Context(), rel, dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "new binary" {
		t.Fatalf("binary = %q", got)
	}

	gh.sums = "0000  heron_0.3.0_linux_amd64.tar.gz\n"
	if err := u.Download(t.Context(), rel, dst); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want checksum mismatch", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "new binary" {
		t.Fatal("a failed download replaced the binary")
	}

	u.Arch = "riscv64"
	if err := u.Download(t.Context(), rel, dst); !errors.Is(err, ErrNoAsset) {
		t.Fatalf("err = %v, want missing asset", err)
	}

	if err := testUpdater(gh, "dev").Download(t.Context(), rel, dst); !errors.Is(err, ErrDevBuild) {
		t.Fatalf("err = %v, want dev build", err)
	}
}

func TestServiceInstall(t *testing.T) {
	skipProbe(t)
	gh := newFakeGitHub(t, "0.3.0", []byte("new binary"))
	dir := t.TempDir()
	restarted := make(chan string, 1)
	s := &Service{
		Updater: testUpdater(gh, "0.2.0"),
		DataDir: dir,
		Restart: func(path string) { restarted <- path },
		Log:     slog.New(slog.DiscardHandler),
	}

	st := s.Check(t.Context())
	if st.Err != nil || !st.Available || st.Latest.Version() != "0.3.0" {
		t.Fatalf("status = %+v", st)
	}
	if _, err := s.Begin(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case path := <-restarted:
		if path != StagedPath(dir) {
			t.Fatalf("restart path = %s", path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no restart after install")
	}
	if _, version, ok := Staged(dir); !ok || version != "0.3.0" {
		t.Fatalf("staged = %v %q", ok, version)
	}

	if err := Handoff(dir, "0.3.0", nil, s.Log); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := Staged(dir); !ok {
		t.Fatal("handoff removed the binary it is running from")
	}
	if err := Handoff(dir, "0.4.0", nil, s.Log); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := Staged(dir); ok {
		t.Fatal("handoff kept an outdated staged binary")
	}

	up := &Service{Updater: testUpdater(gh, "0.3.0"), DataDir: dir, Log: slog.New(slog.DiscardHandler)}
	if _, err := up.Begin(t.Context()); !errors.Is(err, ErrUpToDate) {
		t.Fatalf("err = %v, want up to date", err)
	}
}

func fakeExec(t *testing.T) *[]string {
	var calls []string
	old := execute
	execute = func(path string) error {
		calls = append(calls, path)
		return nil
	}
	t.Cleanup(func() { execute = old })
	return &calls
}

func TestRollbackAfterCrashes(t *testing.T) {
	skipProbe(t)
	calls := fakeExec(t)
	gh := newFakeGitHub(t, "0.3.0", []byte("new binary"))
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	s := &Service{
		Updater:  testUpdater(gh, "0.2.0"),
		DataDir:  dir,
		Auto:     func(context.Context) (bool, error) { return true, nil },
		Snapshot: func(context.Context, string) (string, error) { return "snap.db", nil },
		Restart:  func(string) {},
		Log:      log,
	}
	if _, err := s.Install(t.Context()); err != nil {
		t.Fatal(err)
	}

	for range maxAttempts {
		if err := Handoff(dir, "0.2.0", nil, log); err != nil {
			t.Fatal(err)
		}
	}
	if len(*calls) != maxAttempts {
		t.Fatalf("launched %d times, want %d", len(*calls), maxAttempts)
	}

	var restored string
	restore := func(_ context.Context, backup string) (bool, error) {
		restored = backup
		return true, nil
	}
	if err := Handoff(dir, "0.2.0", restore, log); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != maxAttempts {
		t.Fatal("handoff launched a version that keeps failing")
	}
	if restored != "snap.db" {
		t.Errorf("restored %q, want the snapshot taken before the update", restored)
	}
	if _, _, ok := Staged(dir); ok {
		t.Error("the failed binary is still staged")
	}
	r, ok := LastRejected(dir)
	if !ok || r.Version != "0.3.0" {
		t.Fatalf("rejected = %+v %v", r, ok)
	}
	if st := s.Status(); !errors.As(st.Failed, &Rejected{}) {
		t.Errorf("failed = %v", st.Failed)
	}

	s.tick(t.Context())
	if _, _, ok := Staged(dir); ok {
		t.Error("automatic updates installed the rejected version again")
	}
}

func TestRollbackToPreviousStaged(t *testing.T) {
	skipProbe(t)
	calls := fakeExec(t)
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	staged := StagedPath(dir)
	os.MkdirAll(filepath.Dir(staged), 0o750)
	os.WriteFile(staged, []byte("old binary"), 0o755)
	os.WriteFile(staged+".version", []byte("0.2.0\n"), 0o640)

	gh := newFakeGitHub(t, "0.3.0", []byte("new binary"))
	u := testUpdater(gh, "0.2.0")
	rel, err := u.Latest(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.Stage(t.Context(), rel, dir, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(prevPath(dir)); string(got) != "old binary" {
		t.Fatalf("previous binary = %q", got)
	}

	ConfirmWithin, SettleFor, ProbeEvery = 30*time.Millisecond, 0, 5*time.Millisecond
	t.Cleanup(func() { ConfirmWithin, SettleFor, ProbeEvery = 2*time.Minute, 20*time.Second, 5*time.Second })
	sick := func(context.Context) error { return errors.New("database unavailable") }
	if err := Confirm(t.Context(), dir, "0.3.0", sick, log); err == nil {
		t.Fatal("confirm passed a sick version")
	}

	if err := Handoff(dir, "0.3.0", nil, log); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0] != staged {
		t.Fatalf("exec calls = %v, want the restored binary", *calls)
	}
	if got, _ := os.ReadFile(staged); string(got) != "old binary" {
		t.Fatalf("staged = %q after rollback", got)
	}
	if r, _ := LastRejected(dir); r.Reason != "health check failed: database unavailable" {
		t.Errorf("reason = %q", r.Reason)
	}
}

func TestConfirmKeepsHealthyUpdate(t *testing.T) {
	dir := t.TempDir()
	staged := StagedPath(dir)
	os.MkdirAll(filepath.Dir(staged), 0o750)
	writeJSON(trialPath(staged), trial{Version: "0.3.0"})
	writeJSON(rejectedPath(dir), Rejected{Version: "0.2.9"})

	SettleFor, ProbeEvery = 0, 5*time.Millisecond
	t.Cleanup(func() { SettleFor, ProbeEvery = 20*time.Second, 5*time.Second })
	if err := Confirm(t.Context(), dir, "0.3.0", func(context.Context) error { return nil }, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(trialPath(staged)); !errors.Is(err, os.ErrNotExist) {
		t.Error("trial survived a healthy start")
	}
	if _, ok := LastRejected(dir); ok {
		t.Error("an old rejection survived a healthy update")
	}
}
