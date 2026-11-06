package pelican

import (
	"cmp"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Node struct {
	ID          int64
	Name        string
	Address     string
	Maintenance bool
}

type Server struct {
	UUID      string
	Name      string
	Node      int64
	Suspended bool
}

type Panel struct {
	URL    string
	Key    string
	Client *http.Client
}

type envelope struct {
	Data []struct {
		Attributes jsontext.Value `json:"attributes"`
	} `json:"data"`
	Meta struct {
		Pagination struct {
			Current int `json:"current_page"`
			Total   int `json:"total_pages"`
		} `json:"pagination"`
	} `json:"meta"`
}

type nodeAttributes struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FQDN        string `json:"fqdn"`
	Scheme      string `json:"scheme"`
	Listen      int    `json:"daemon_listen"`
	Connect     int    `json:"daemon_connect"`
	Maintenance bool   `json:"maintenance_mode"`
}

type serverAttributes struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Node      int64  `json:"node"`
	Suspended bool   `json:"suspended"`
	Status    any    `json:"status"`
}

func (p *Panel) Nodes(ctx context.Context) ([]Node, error) {
	var out []Node
	err := p.list(ctx, "/api/application/nodes", func(raw jsontext.Value) error {
		var a nodeAttributes
		if err := json.Unmarshal(raw, &a); err != nil {
			return err
		}
		port := cmp.Or(a.Connect, a.Listen, 8080)
		out = append(out, Node{
			ID:          a.ID,
			Name:        a.Name,
			Address:     fmt.Sprintf("%s://%s:%d", cmp.Or(a.Scheme, "https"), a.FQDN, port),
			Maintenance: a.Maintenance,
		})
		return nil
	})
	return out, err
}

func (p *Panel) Servers(ctx context.Context) ([]Server, error) {
	var out []Server
	err := p.list(ctx, "/api/application/servers", func(raw jsontext.Value) error {
		var a serverAttributes
		if err := json.Unmarshal(raw, &a); err != nil {
			return err
		}
		status, _ := a.Status.(string)
		out = append(out, Server{UUID: a.UUID, Name: a.Name, Node: a.Node, Suspended: a.Suspended || status == "suspended"})
		return nil
	})
	return out, err
}

func (p *Panel) NodeToken(ctx context.Context, id int64) (string, error) {
	var cfg struct {
		Token string `json:"token"`
	}
	if err := p.get(ctx, "/api/application/nodes/"+strconv.FormatInt(id, 10)+"/configuration", &cfg); err != nil {
		return "", err
	}
	if cfg.Token == "" {
		return "", errors.New("the panel did not share the node token, give the API key read access to nodes")
	}
	return cfg.Token, nil
}

func (p *Panel) list(ctx context.Context, path string, each func(jsontext.Value) error) error {
	for page := 1; page <= 200; page++ {
		var env envelope
		if err := p.get(ctx, fmt.Sprintf("%s?per_page=100&page=%d", path, page), &env); err != nil {
			return err
		}
		for _, item := range env.Data {
			if err := each(item.Attributes); err != nil {
				return err
			}
		}
		if env.Meta.Pagination.Total <= page {
			return nil
		}
	}
	return nil
}

func (p *Panel) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(p.URL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.Key)
	req.Header.Set("Accept", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("the panel refused the API key (%s)", resp.Status)
	default:
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.UnmarshalRead(io.LimitReader(resp.Body, 16<<20), v)
}

type ServerState struct {
	State     string `json:"state"`
	Suspended bool   `json:"is_suspended"`
	Config    struct {
		UUID string `json:"uuid"`
	} `json:"configuration"`
	Usage struct {
		Memory      int64   `json:"memory_bytes"`
		MemoryLimit int64   `json:"memory_limit_bytes"`
		CPU         float64 `json:"cpu_absolute"`
	} `json:"utilization"`
}

type Wings struct {
	Address string
	Token   string
	Client  *http.Client
}

func (w Wings) System(ctx context.Context) (time.Duration, error) {
	started := time.Now()
	err := w.get(ctx, "/api/system", nil)
	return time.Since(started), err
}

func (w Wings) Servers(ctx context.Context) ([]ServerState, error) {
	var out []ServerState
	return out, w.get(ctx, "/api/servers", &out)
}

func (w Wings) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(w.Address, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := w.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wings answered %s", resp.Status)
	}
	if v == nil {
		_, err := io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return err
	}
	return json.UnmarshalRead(io.LimitReader(resp.Body, 16<<20), v)
}

type Utilization struct {
	MemoryTotal uint64 `json:"memory_total"`
	MemoryUsed  uint64 `json:"memory_used"`
	DiskTotal   uint64 `json:"disk_total"`
	DiskUsed    uint64 `json:"disk_used"`
}

func (w Wings) Utilization(ctx context.Context) (Utilization, error) {
	var u Utilization
	return u, w.get(ctx, "/api/system/utilization", &u)
}
