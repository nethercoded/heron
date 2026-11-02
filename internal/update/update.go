package update

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepo = "nethercoded/heron"
	DefaultAPI  = "https://api.github.com"

	maxArchive = 200 << 20
)

var (
	ErrDevBuild    = errors.New("development builds do not update themselves")
	ErrNoAsset     = errors.New("the release has no build for this platform")
	ErrChecksum    = errors.New("checksum mismatch")
	ErrUnsupported = errors.New("updates are not supported on this platform")
)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type Release struct {
	Tag       string    `json:"tag_name"`
	Notes     string    `json:"body"`
	URL       string    `json:"html_url"`
	Published time.Time `json:"published_at"`
	Assets    []Asset   `json:"assets"`
}

func (r Release) Version() string {
	return strings.TrimPrefix(r.Tag, "v")
}

func (r Release) asset(name string) (Asset, bool) {
	i := slices.IndexFunc(r.Assets, func(a Asset) bool { return a.Name == name })
	if i < 0 {
		return Asset{}, false
	}
	return r.Assets[i], true
}

type Updater struct {
	Repo      string
	API       string
	Client    *http.Client
	UserAgent string
	Current   string
	OS        string
	Arch      string
}

func New(current, userAgent string) *Updater {
	return &Updater{
		Repo:      DefaultRepo,
		API:       DefaultAPI,
		Client:    &http.Client{Timeout: 5 * time.Minute},
		UserAgent: userAgent,
		Current:   current,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

func (u *Updater) Ready() error {
	if _, ok := parse(u.Current); !ok {
		return ErrDevBuild
	}
	if u.OS == "windows" {
		return ErrUnsupported
	}
	return nil
}

func (u *Updater) Latest(ctx context.Context) (Release, error) {
	var rel Release
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := u.get(ctx, u.API+"/repos/"+u.Repo+"/releases/latest", "application/vnd.github+json")
	if err != nil {
		return rel, err
	}
	defer resp.Body.Close()
	if err := json.UnmarshalRead(io.LimitReader(resp.Body, 1<<20), &rel); err != nil {
		return rel, fmt.Errorf("read release: %w", err)
	}
	if rel.Tag == "" {
		return rel, errors.New("github returned a release without a tag")
	}
	return rel, nil
}

func (u *Updater) Newer(rel Release) bool {
	return Newer(rel.Version(), u.Current)
}

func (u *Updater) Download(ctx context.Context, rel Release, dst string) error {
	if err := u.Ready(); err != nil {
		return err
	}
	name := fmt.Sprintf("heron_%s_%s_%s.tar.gz", rel.Version(), u.OS, u.Arch)
	archive, ok := rel.asset(name)
	if !ok {
		return fmt.Errorf("%w (%s/%s)", ErrNoAsset, u.OS, u.Arch)
	}
	sums, ok := rel.asset("checksums.txt")
	if !ok {
		return errors.New("the release has no checksums.txt")
	}

	want, err := u.checksum(ctx, sums.URL, name)
	if err != nil {
		return err
	}

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".heron-download-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer tmp.Close()

	resp, err := u.get(ctx, archive.URL, "application/octet-stream")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxArchive)); err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return fmt.Errorf("%w for %s", ErrChecksum, name)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}

	bin := dst + ".new"
	if err := extract(tmp, bin); err != nil {
		_ = os.Remove(bin)
		return err
	}
	if err := probe(ctx, bin, rel.Version()); err != nil {
		_ = os.Remove(bin)
		return err
	}
	return os.Rename(bin, dst)
}

func (u *Updater) checksum(ctx context.Context, url, name string) (string, error) {
	resp, err := u.get(ctx, url, "text/plain")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	lines := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for lines.Scan() {
		sum, file, ok := strings.Cut(strings.TrimSpace(lines.Text()), "  ")
		if ok && strings.TrimSpace(file) == name {
			return strings.ToLower(sum), nil
		}
	}
	if err := lines.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

func (u *Updater) get(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	if u.UserAgent != "" {
		req.Header.Set("User-Agent", u.UserAgent)
	}
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

func extract(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	archive := tar.NewReader(gz)
	for {
		h, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("the archive has no heron binary")
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		if h.Typeflag != tar.TypeReg || filepath.Base(h.Name) != "heron" || strings.Contains(h.Name, "/") {
			continue
		}
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, io.LimitReader(archive, maxArchive)); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
}

var probe = func(ctx context.Context, bin, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "version").Output()
	if err != nil {
		return fmt.Errorf("the new binary does not start: %w", err)
	}
	if got := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "heron")); strings.TrimPrefix(got, "v") != version {
		return fmt.Errorf("the new binary reports version %q, expected %s", got, version)
	}
	return nil
}

func Newer(candidate, current string) bool {
	a, ok := parse(candidate)
	if !ok {
		return false
	}
	b, ok := parse(current)
	if !ok {
		return false
	}
	return slices.Compare(a[:], b[:]) > 0
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
