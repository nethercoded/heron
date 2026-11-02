package update

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrUpToDate = errors.New("heron is already up to date")
	ErrBusy     = errors.New("an update is already being installed")
)

func StagedPath(dataDir string) string {
	return filepath.Join(dataDir, "bin", "heron")
}

func Staged(dataDir string) (string, string, bool) {
	path := StagedPath(dataDir)
	raw, err := os.ReadFile(path + ".version")
	if err != nil {
		return "", "", false
	}
	if _, err := os.Stat(path); err != nil {
		return "", "", false
	}
	return path, strings.TrimSpace(string(raw)), true
}

func (u *Updater) Stage(ctx context.Context, rel Release, dataDir string) (string, error) {
	path := StagedPath(dataDir)
	if err := u.Download(ctx, rel, path); err != nil {
		return "", err
	}
	if err := os.WriteFile(path+".version", []byte(rel.Version()+"\n"), 0o640); err != nil {
		return "", err
	}
	return path, nil
}

type Restart struct {
	Path string
}

func (r *Restart) Error() string {
	return "restart into " + r.Path
}

func Handoff(dataDir, current string) error {
	path, version, ok := Staged(dataDir)
	if !ok {
		return nil
	}
	if Newer(current, version) {
		return errors.Join(os.Remove(path), os.Remove(path+".version"))
	}
	if !Newer(version, current) {
		return nil
	}
	return Exec(path)
}

type Status struct {
	Current   string
	Latest    *Release
	Available bool
	CheckedAt time.Time
	Err       error
	Ready     error
	Busy      bool
	Failed    error
}

type Service struct {
	Updater *Updater
	DataDir string
	Every   time.Duration
	Auto    func(context.Context) (bool, error)
	Restart func(path string)
	Log     *slog.Logger

	mu      sync.Mutex
	latest  *Release
	checked time.Time
	err     error
	busy    bool
	failed  error
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{
		Current:   s.Updater.Current,
		Latest:    s.latest,
		CheckedAt: s.checked,
		Err:       s.err,
		Ready:     s.Updater.Ready(),
		Busy:      s.busy,
		Failed:    s.failed,
	}
	st.Available = s.latest != nil && s.Updater.Newer(*s.latest)
	return st
}

func (s *Service) Check(ctx context.Context) Status {
	rel, err := s.Updater.Latest(ctx)
	s.mu.Lock()
	s.checked = time.Now()
	s.err = err
	if err == nil {
		s.latest = &rel
	}
	s.mu.Unlock()
	return s.Status()
}

func (s *Service) Install(ctx context.Context) (Release, error) {
	latest, err := s.claim()
	if err != nil {
		return Release{}, err
	}
	return s.finish(ctx, latest)
}

func (s *Service) claim() (*Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return nil, ErrBusy
	}
	s.busy = true
	s.failed = nil
	return s.latest, nil
}

func (s *Service) finish(ctx context.Context, latest *Release) (Release, error) {
	rel, err := s.install(ctx, latest)
	s.mu.Lock()
	s.busy = false
	if err != nil && !errors.Is(err, ErrUpToDate) {
		s.failed = err
	}
	s.mu.Unlock()
	return rel, err
}

func (s *Service) install(ctx context.Context, latest *Release) (Release, error) {
	if latest == nil {
		st := s.Check(ctx)
		if st.Err != nil {
			return Release{}, st.Err
		}
		latest = st.Latest
	}
	if !s.Updater.Newer(*latest) {
		return *latest, ErrUpToDate
	}
	path, err := s.Updater.Stage(ctx, *latest, s.DataDir)
	if err != nil {
		return *latest, err
	}
	s.Log.Info("update downloaded, restarting", "from", s.Updater.Current, "to", latest.Version())
	if s.Restart != nil {
		s.Restart(path)
	}
	return *latest, nil
}

func (s *Service) Run(ctx context.Context) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		s.tick(ctx)
		timer.Reset(s.Every)
	}
}

func (s *Service) tick(ctx context.Context) {
	st := s.Check(ctx)
	if st.Err != nil {
		s.Log.Warn("could not check for updates", "err", st.Err)
		return
	}
	if !st.Available {
		return
	}
	s.Log.Info("a new version of heron is available", "current", st.Current, "latest", st.Latest.Version(), "url", st.Latest.URL)
	if st.Ready != nil || s.Auto == nil {
		return
	}
	auto, err := s.Auto(ctx)
	if err != nil || !auto {
		return
	}
	if _, err := s.Install(ctx); err != nil {
		s.Log.Error("automatic update failed", "version", st.Latest.Version(), "err", err)
	}
}

func (s *Service) Begin(ctx context.Context) (Release, error) {
	st := s.Status()
	if st.Latest == nil {
		if st = s.Check(ctx); st.Err != nil {
			return Release{}, st.Err
		}
	}
	if !st.Available {
		return *st.Latest, ErrUpToDate
	}
	latest, err := s.claim()
	if err != nil {
		return Release{}, err
	}
	go func() {
		if _, err := s.finish(context.WithoutCancel(ctx), latest); err != nil {
			s.Log.Error("update failed", "version", st.Latest.Version(), "err", err)
		}
	}()
	return *st.Latest, nil
}
