package update

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const maxAttempts = 2

var execute = Exec

var (
	ConfirmWithin = 2 * time.Minute
	SettleFor     = 20 * time.Second
	ProbeEvery    = 5 * time.Second
)

type trial struct {
	Version  string `json:"version"`
	Previous string `json:"previous"`
	From     string `json:"from"`
	Backup   string `json:"backup,omitzero"`
	Attempts int    `json:"attempts"`
	Failed   string `json:"failed,omitzero"`
}

func (t trial) exhausted() bool {
	return t.Failed != "" || t.Attempts >= maxAttempts
}

func (t trial) reason() string {
	if t.Failed != "" {
		return t.Failed
	}
	return fmt.Sprintf("it stopped %d times before it became healthy", t.Attempts)
}

type Rejected struct {
	Version string    `json:"version"`
	Reason  string    `json:"reason"`
	At      time.Time `json:"at"`
}

func (r Rejected) Error() string {
	return fmt.Sprintf("heron %s did not start (%s), so heron went back to the previous version", r.Version, r.Reason)
}

func trialPath(staged string) string     { return staged + ".trial" }
func prevPath(dataDir string) string     { return StagedPath(dataDir) + ".prev" }
func rejectedPath(dataDir string) string { return filepath.Join(dataDir, "bin", "rejected.json") }

func readJSON(path string, v any) bool {
	raw, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(raw, v) == nil
}

func writeJSON(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func remove(paths ...string) error {
	var errs []error
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func LastRejected(dataDir string) (Rejected, bool) {
	var r Rejected
	return r, readJSON(rejectedPath(dataDir), &r) && r.Version != ""
}

func ClearRejected(dataDir string) error {
	return remove(rejectedPath(dataDir))
}

func Launch(path string) error {
	var t trial
	if readJSON(trialPath(path), &t) {
		t.Attempts++
		if err := writeJSON(trialPath(path), t); err != nil {
			return err
		}
	}
	return execute(path)
}

type RestoreFunc func(ctx context.Context, backup string) (bool, error)

func Handoff(dataDir, current string, restore RestoreFunc, log *slog.Logger) error {
	staged := StagedPath(dataDir)
	var t trial
	if readJSON(trialPath(staged), &t) {
		_, version, ok := Staged(dataDir)
		switch {
		case !ok || version != t.Version:
			_ = remove(trialPath(staged))
		case t.exhausted():
			if err := rollback(dataDir, t, restore, log); err != nil {
				return err
			}
			if current == t.Version {
				return relaunch(dataDir, t)
			}
		}
	}

	path, version, ok := Staged(dataDir)
	if !ok {
		return nil
	}
	if Newer(current, version) {
		return remove(path, path+".version", trialPath(path))
	}
	if !Newer(version, current) {
		return nil
	}
	return Launch(path)
}

func relaunch(dataDir string, t trial) error {
	target := t.From
	if _, _, ok := Staged(dataDir); ok {
		target = StagedPath(dataDir)
	}
	self, _ := os.Executable()
	if target == "" || (target == self && target != StagedPath(dataDir)) {
		return fmt.Errorf("heron %s failed and there is no previous binary to go back to", t.Version)
	}
	return execute(target)
}

func rollback(dataDir string, t trial, restore RestoreFunc, log *slog.Logger) error {
	log.Error("the update did not become healthy, going back", "version", t.Version, "back_to", t.Previous, "reason", t.reason())
	if t.Backup != "" && restore != nil {
		restored, err := restore(context.Background(), t.Backup)
		switch {
		case err != nil:
			log.Error("could not restore the database from before the update", "backup", t.Backup, "err", err)
		case restored:
			log.Warn("restored the database from before the update", "backup", t.Backup)
		}
	}

	staged := StagedPath(dataDir)
	if err := remove(staged, staged+".version"); err != nil {
		return err
	}
	prev := prevPath(dataDir)
	if _, err := os.Stat(prev); err == nil {
		if err := os.Rename(prev, staged); err != nil {
			return err
		}
		if err := os.Rename(prev+".version", staged+".version"); err != nil {
			return errors.Join(err, remove(staged))
		}
	}
	err := writeJSON(rejectedPath(dataDir), Rejected{Version: t.Version, Reason: t.reason(), At: time.Now().UTC()})
	return errors.Join(err, remove(trialPath(staged)))
}

func keepPrevious(dataDir string) error {
	staged := StagedPath(dataDir)
	prev := prevPath(dataDir)
	if err := remove(prev, prev+".version"); err != nil {
		return err
	}
	if _, _, ok := Staged(dataDir); !ok {
		return nil
	}
	if err := os.Rename(staged, prev); err != nil {
		return err
	}
	return os.Rename(staged+".version", prev+".version")
}

func restorePrevious(dataDir string) {
	staged := StagedPath(dataDir)
	prev := prevPath(dataDir)
	if _, err := os.Stat(prev); err != nil {
		return
	}
	_ = os.Rename(prev, staged)
	_ = os.Rename(prev+".version", staged+".version")
}

func Confirm(ctx context.Context, dataDir, current string, healthy func(context.Context) error, log *slog.Logger) error {
	path := trialPath(StagedPath(dataDir))
	var t trial
	if !readJSON(path, &t) || t.Version != current {
		return nil
	}
	log.Info("checking the new version before keeping it", "version", current, "within", ConfirmWithin)

	start := time.Now()
	tick := time.NewTicker(ProbeEvery)
	defer tick.Stop()
	var last error
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
		last = healthy(ctx)
		if last == nil && time.Since(start) >= SettleFor {
			if err := remove(path); err != nil {
				return err
			}
			_ = ClearRejected(dataDir)
			log.Info("the update is healthy, keeping it", "version", current)
			return nil
		}
		if time.Since(start) >= ConfirmWithin {
			if last == nil {
				last = errors.New("health check did not pass in time")
			}
			t.Failed = "health check failed: " + last.Error()
			if err := writeJSON(path, t); err != nil {
				return errors.Join(last, err)
			}
			return last
		}
	}
}
