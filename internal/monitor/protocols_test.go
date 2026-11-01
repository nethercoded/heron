package monitor

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func serveTCP(t *testing.T, handle func(conn net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				handle(conn)
			}()
		}
	}()
	return ln.Addr().String()
}

func check(t *testing.T, c Checker, m *Monitor) Result {
	t.Helper()
	m.Timeout = 3 * time.Second
	return Registry{m.Kind: c}.Check(t.Context(), m)
}

func TestRedisChecker(t *testing.T) {
	addr := serveTCP(t, func(conn net.Conn) {
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if !strings.HasPrefix(line, "*") {
				continue
			}
			var parts []string
			n, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
			for range n {
				r.ReadString('\n')
				arg, _ := r.ReadString('\n')
				parts = append(parts, strings.TrimSpace(arg))
			}
			switch strings.ToUpper(parts[0]) {
			case "AUTH":
				if parts[1] == "s3cret" {
					io.WriteString(conn, "+OK\r\n")
				} else {
					io.WriteString(conn, "-WRONGPASS invalid password\r\n")
				}
			case "PING":
				io.WriteString(conn, "+PONG\r\n")
			case "ROLE":
				io.WriteString(conn, "*3\r\n$6\r\nmaster\r\n:0\r\n*0\r\n")
			}
		}
	})

	res := check(t, RedisChecker{}, &Monitor{Kind: KindRedis, Target: addr, Settings: Settings{Password: "s3cret"}})
	if res.Status != StatusUp || !strings.Contains(res.Message, "master") {
		t.Errorf("redis = %+v", res)
	}
	res = check(t, RedisChecker{}, &Monitor{Kind: KindRedis, Target: addr, Settings: Settings{Password: "nope"}})
	if res.Status != StatusDown || !strings.Contains(res.Message, "WRONGPASS") {
		t.Errorf("bad password = %+v", res)
	}
}

func TestSMTPChecker(t *testing.T) {
	addr := serveTCP(t, func(conn net.Conn) {
		r := bufio.NewReader(conn)
		io.WriteString(conn, "220 mail.example.test ESMTP ready\r\n")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				io.WriteString(conn, "250-mail.example.test\r\n250-SIZE 10240000\r\n250 8BITMIME\r\n")
			case strings.HasPrefix(line, "QUIT"):
				io.WriteString(conn, "221 bye\r\n")
				return
			}
		}
	})

	res := check(t, SMTPChecker{}, &Monitor{Kind: KindSMTP, Target: addr})
	if res.Status != StatusUp || !strings.Contains(res.Message, "mail.example.test") {
		t.Errorf("smtp = %+v", res)
	}
	res = check(t, SMTPChecker{}, &Monitor{Kind: KindSMTP, Target: addr, Settings: Settings{StartTLS: true}})
	if res.Status != StatusDown || !strings.Contains(res.Message, "STARTTLS") {
		t.Errorf("starttls required = %+v", res)
	}
}

func TestIMAPChecker(t *testing.T) {
	ok := serveTCP(t, func(conn net.Conn) { io.WriteString(conn, "* OK [CAPABILITY IMAP4rev1] Dovecot ready.\r\n") })
	bye := serveTCP(t, func(conn net.Conn) { io.WriteString(conn, "* BYE too many connections\r\n") })

	if res := check(t, IMAPChecker{}, &Monitor{Kind: KindIMAP, Target: ok}); res.Status != StatusUp || !strings.Contains(res.Message, "Dovecot") {
		t.Errorf("imap = %+v", res)
	}
	if res := check(t, IMAPChecker{}, &Monitor{Kind: KindIMAP, Target: bye}); res.Status != StatusDown {
		t.Errorf("imap bye = %+v", res)
	}
}

func TestDockerChecker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/containers/web/json":
			io.WriteString(w, `{"Name":"/web","State":{"Status":"running","Running":true,"Health":{"Status":"healthy"}}}`)
		case "/containers/worker/json":
			io.WriteString(w, `{"Name":"/worker","State":{"Status":"running","Running":true,"Health":{"Status":"unhealthy","Log":[{"Output":"curl: (7) refused"}]}}}`)
		case "/containers/batch/json":
			io.WriteString(w, `{"Name":"/batch","State":{"Status":"exited","Running":false,"ExitCode":137}}`)
		default:
			http.Error(w, `{"message":"No such container"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	host := "tcp://" + strings.TrimPrefix(srv.URL, "http://")

	tests := map[string]struct {
		status  Status
		message string
	}{
		"web":    {StatusUp, "healthy"},
		"worker": {StatusDown, "refused"},
		"batch":  {StatusDown, "exit code 137"},
		"ghost":  {StatusDown, "not found"},
	}
	for name, want := range tests {
		res := check(t, DockerChecker{}, &Monitor{Kind: KindDocker, Target: name, Settings: Settings{DockerHost: host}})
		if res.Status != want.status || !strings.Contains(res.Message, want.message) {
			t.Errorf("%s = %+v", name, res)
		}
	}
}

func TestGRPCChecker(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/grpc")
		w.Header().Set("Trailer", "Grpc-Status")
		status := byte(1)
		if bytes.Contains(body, []byte("payments")) {
			status = 2
		}
		frame := []byte{0, 0, 0, 0, 2, 0x08, status}
		w.Write(frame)
		w.Header().Set("Grpc-Status", "0")
	})
	srv := httptest.NewServer(h2c.NewHandler(handler, &http2.Server{}))
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")

	if res := check(t, GRPCChecker{}, &Monitor{Kind: KindGRPC, Target: addr}); res.Status != StatusUp {
		t.Errorf("grpc = %+v", res)
	}
	res := check(t, GRPCChecker{}, &Monitor{Kind: KindGRPC, Target: addr, Settings: Settings{Service: "payments"}})
	if res.Status != StatusDown || !strings.Contains(res.Message, "NOT_SERVING") {
		t.Errorf("grpc not serving = %+v", res)
	}
}

func TestMinecraftChecker(t *testing.T) {
	addr := serveTCP(t, func(conn net.Conn) {
		r := bufio.NewReader(conn)
		for range 2 {
			n, err := binary.ReadUvarint(r)
			if err != nil {
				return
			}
			io.CopyN(io.Discard, r, int64(n))
		}
		status := []byte(`{"version":{"name":"Paper 1.21.4","protocol":769},"players":{"online":3,"max":50,"sample":[]},"description":{"text":"hi"}}`)
		body := binary.AppendUvarint([]byte{0x00}, uint64(len(status)))
		body = append(body, status...)
		conn.Write(packet(body))
	})

	res := check(t, MinecraftChecker{}, &Monitor{Kind: KindMinecraft, Target: addr})
	if res.Status != StatusUp || !strings.Contains(res.Message, "3/50 players, Paper 1.21.4") {
		t.Errorf("minecraft = %+v", res)
	}
	res = check(t, MinecraftChecker{}, &Monitor{Kind: KindMinecraft, Target: addr, Settings: Settings{MinPlayers: 5}})
	if res.Status != StatusDegraded {
		t.Errorf("min players = %+v", res)
	}
}

func TestSteamChecker(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 1500)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if n == len(a2sInfo) {
				pc.WriteTo([]byte{0xff, 0xff, 0xff, 0xff, 'A', 1, 2, 3, 4}, from)
				continue
			}
			reply := []byte{0xff, 0xff, 0xff, 0xff, 'I', 17}
			for _, s := range []string{"Heron Rust #1", "Procedural Map", "rust", "Rust"} {
				reply = append(append(reply, s...), 0)
			}
			reply = append(reply, 0, 0, 42, 100, 0)
			pc.WriteTo(reply, from)
		}
	}()

	res := check(t, SteamChecker{}, &Monitor{Kind: KindSteam, Target: pc.LocalAddr().String()})
	if res.Status != StatusUp || !strings.Contains(res.Message, "42/100 players, Heron Rust #1, Rust on Procedural Map") {
		t.Errorf("steam = %+v", res)
	}
}

func TestNewKindsValidation(t *testing.T) {
	cases := []struct {
		mon    Monitor
		target string
		valid  bool
	}{
		{Monitor{Name: "mc", Kind: KindMinecraft, Target: "play.example.com"}, "play.example.com:25565", true},
		{Monitor{Name: "mail", Kind: KindIMAP, Target: "mail.example.com", Settings: Settings{TLS: true}}, "mail.example.com:993", true},
		{Monitor{Name: "db", Kind: KindPostgres, Target: "postgres://heron:pw@db:5432/app"}, "postgres://heron:pw@db:5432/app", true},
		{Monitor{Name: "db", Kind: KindPostgres, Target: "db:5432"}, "db:5432", false},
		{Monitor{Name: "ctr", Kind: KindDocker, Target: "web-1"}, "web-1", true},
		{Monitor{Name: "ctr", Kind: KindDocker, Target: "../etc"}, "../etc", false},
	}
	for _, c := range cases {
		c.mon.Normalize()
		err := c.mon.Validate()
		if (err == nil) != c.valid || c.mon.Target != c.target {
			t.Errorf("%s %q: target %q, err %v", c.mon.Kind, c.target, c.mon.Target, err)
		}
	}

	pg := Monitor{Kind: KindPostgres, Target: "postgres://heron:hunter2@db:5432/app"}
	if strings.Contains(pg.DisplayTarget(), "hunter2") {
		t.Errorf("password leaked: %s", pg.DisplayTarget())
	}
}
