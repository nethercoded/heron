package monitor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type DNSChecker struct{}

func (DNSChecker) Check(ctx context.Context, m *Monitor) Result {
	resolver := newResolver(m.Settings.Resolver)

	started := time.Now()
	records, err := lookup(ctx, resolver, m.Settings.RecordType, m.Target)
	latency := time.Since(started)
	if err != nil {
		return Down(latency, "%s lookup: %v", m.Settings.RecordType, err)
	}
	if len(records) == 0 {
		return Down(latency, "no %s records", m.Settings.RecordType)
	}

	if want := m.Settings.Expect; want != "" {
		for _, r := range records {
			if strings.EqualFold(strings.TrimSuffix(r, "."), strings.TrimSuffix(want, ".")) || strings.Contains(r, want) {
				return Up(latency, "%s", r)
			}
		}
		return Down(latency, "expected %q, got %s", want, strings.Join(records, ", "))
	}

	return Up(latency, "%s", strings.Join(records, ", "))
}

func newResolver(server string) *net.Resolver {
	if server == "" {
		return net.DefaultResolver
	}
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, server)
		},
	}
}

func lookup(ctx context.Context, r *net.Resolver, kind, host string) ([]string, error) {
	switch kind {
	case "A", "AAAA":
		network := "ip4"
		if kind == "AAAA" {
			network = "ip6"
		}
		addrs, err := r.LookupNetIP(ctx, network, host)
		out := make([]string, 0, len(addrs))
		for _, a := range addrs {
			out = append(out, a.Unmap().String())
		}
		return out, err
	case "CNAME":
		name, err := r.LookupCNAME(ctx, host)
		return []string{name}, err
	case "MX":
		mx, err := r.LookupMX(ctx, host)
		out := make([]string, 0, len(mx))
		for _, rec := range mx {
			out = append(out, fmt.Sprintf("%d %s", rec.Pref, rec.Host))
		}
		return out, err
	case "NS":
		ns, err := r.LookupNS(ctx, host)
		out := make([]string, 0, len(ns))
		for _, rec := range ns {
			out = append(out, rec.Host)
		}
		return out, err
	case "TXT":
		return r.LookupTXT(ctx, host)
	}
	return nil, fmt.Errorf("unsupported record type %s", kind)
}
