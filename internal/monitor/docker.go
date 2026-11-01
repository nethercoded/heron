package monitor

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DockerChecker struct{}

type containerInfo struct {
	Name  string `json:"Name"`
	State struct {
		Status     string `json:"Status"`
		Running    bool   `json:"Running"`
		Restarting bool   `json:"Restarting"`
		ExitCode   int    `json:"ExitCode"`
		StartedAt  string `json:"StartedAt"`
		Health     *struct {
			Status        string `json:"Status"`
			FailingStreak int    `json:"FailingStreak"`
			Log           []struct {
				Output string `json:"Output"`
			} `json:"Log"`
		} `json:"Health"`
	} `json:"State"`
	RestartCount int `json:"RestartCount"`
}

func (DockerChecker) Check(ctx context.Context, m *Monitor) Result {
	client, base, err := dockerClient(m.Settings.DockerHost)
	if err != nil {
		return Down(0, "%v", err)
	}

	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/containers/"+url.PathEscape(strings.TrimPrefix(m.Target, "/"))+"/json", nil)
	if err != nil {
		return Down(0, "%v", err)
	}
	resp, err := client.Do(req)
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "docker api: %s", describeNetError(err))
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return Down(latency, "container %s not found", m.Target)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Down(latency, "docker api: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var info containerInfo
	if err := json.UnmarshalRead(io.LimitReader(resp.Body, 4<<20), &info); err != nil {
		return Down(latency, "decode container: %v", err)
	}

	st := info.State
	switch {
	case st.Restarting:
		return Down(latency, "restarting, %d restarts so far", info.RestartCount)
	case !st.Running:
		return Down(latency, "%s, exit code %d", st.Status, st.ExitCode)
	case st.Health != nil && st.Health.Status == "unhealthy":
		reason := "healthcheck failing"
		if n := len(st.Health.Log); n > 0 {
			reason = strings.TrimSpace(st.Health.Log[n-1].Output)
		}
		return Down(latency, "unhealthy: %s", truncate(reason, 160))
	case st.Health != nil && st.Health.Status == "starting":
		return Degraded(latency, "healthcheck starting")
	case st.Health != nil:
		return Up(latency, "running, %s", st.Health.Status)
	}
	return Up(latency, "running")
}

func dockerClient(host string) (*http.Client, string, error) {
	scheme, address, ok := strings.Cut(host, "://")
	if !ok {
		return nil, "", fmt.Errorf("invalid docker host %q", host)
	}
	switch scheme {
	case "tcp":
		return &http.Client{}, "http://" + address, nil
	case "unix":
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", address)
			},
			DisableKeepAlives: true,
		}
		return &http.Client{Transport: transport}, "http://docker", nil
	}
	return nil, "", fmt.Errorf("unsupported docker host scheme %q", scheme)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
