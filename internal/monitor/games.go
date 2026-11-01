package monitor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type MinecraftChecker struct{}

type minecraftStatus struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Online int `json:"online"`
		Max    int `json:"max"`
	} `json:"players"`
}

func (MinecraftChecker) Check(ctx context.Context, m *Monitor) Result {
	started := time.Now()
	conn, err := dial(ctx, "tcp", m.Target, false)
	if err != nil {
		return Down(time.Since(started), "%s", describeNetError(err))
	}
	defer conn.Close()

	host, portText, _ := net.SplitHostPort(m.Target)
	port, _ := strconv.Atoi(portText)

	var handshake []byte
	handshake = binary.AppendUvarint(handshake, 0x00)
	handshake = binary.AppendUvarint(handshake, 767)
	handshake = binary.AppendUvarint(handshake, uint64(len(host)))
	handshake = append(handshake, host...)
	handshake = binary.BigEndian.AppendUint16(handshake, uint16(port))
	handshake = binary.AppendUvarint(handshake, 1)

	if _, err := conn.Write(append(packet(handshake), packet([]byte{0x00})...)); err != nil {
		return Down(time.Since(started), "handshake: %v", err)
	}

	r := bufio.NewReader(conn)
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return Down(time.Since(started), "read status: %v", err)
	}
	if length > 1<<20 {
		return Down(time.Since(started), "status response too large")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Down(time.Since(started), "read status: %v", err)
	}
	latency := time.Since(started)

	body := bytes.NewReader(payload)
	if id, err := binary.ReadUvarint(body); err != nil || id != 0x00 {
		return Down(latency, "unexpected packet")
	}
	size, err := binary.ReadUvarint(body)
	if err != nil || size > uint64(body.Len()) {
		return Down(latency, "malformed status")
	}
	raw := make([]byte, size)
	io.ReadFull(body, raw)

	var status minecraftStatus
	if err := json.Unmarshal(raw, &status, json.RejectUnknownMembers(false)); err != nil {
		return Down(latency, "decode status: %v", err)
	}
	return players(m, latency, status.Players.Online, status.Players.Max, status.Version.Name)
}

func packet(data []byte) []byte {
	out := binary.AppendUvarint(nil, uint64(len(data)))
	return append(out, data...)
}

type SteamChecker struct{}

var a2sInfo = append([]byte{0xff, 0xff, 0xff, 0xff, 'T'}, append([]byte("Source Engine Query"), 0)...)

func (SteamChecker) Check(ctx context.Context, m *Monitor) Result {
	started := time.Now()
	conn, err := dial(ctx, "udp", m.Target, false)
	if err != nil {
		return Down(time.Since(started), "%s", describeNetError(err))
	}
	defer conn.Close()

	reply, err := exchange(conn, a2sInfo)
	if err == nil && len(reply) >= 9 && reply[4] == 'A' {
		reply, err = exchange(conn, append(bytes.Clone(a2sInfo), reply[5:9]...))
	}
	latency := time.Since(started)
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return Down(latency, "no response from server")
		}
		return Down(latency, "query: %v", err)
	}

	info, err := parseA2S(reply)
	if err != nil {
		return Down(latency, "%v", err)
	}
	label := info.name
	if info.game != "" {
		label = fmt.Sprintf("%s, %s on %s", info.name, info.game, info.mapName)
	}
	return players(m, latency, info.players, info.max, label)
}

func exchange(conn net.Conn, msg []byte) ([]byte, error) {
	if _, err := conn.Write(msg); err != nil {
		return nil, err
	}
	buf := make([]byte, 1400)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

type a2s struct {
	name, mapName, game string
	players, max        int
}

func parseA2S(b []byte) (a2s, error) {
	var info a2s
	if len(b) < 6 || !bytes.Equal(b[:4], []byte{0xff, 0xff, 0xff, 0xff}) || b[4] != 'I' {
		return info, fmt.Errorf("unexpected A2S response")
	}
	rest := b[6:]
	fields := make([]string, 4)
	for i := range fields {
		end := bytes.IndexByte(rest, 0)
		if end < 0 {
			return info, fmt.Errorf("truncated A2S response")
		}
		fields[i] = string(rest[:end])
		rest = rest[end+1:]
	}
	if len(rest) < 4 {
		return info, fmt.Errorf("truncated A2S response")
	}
	info.name, info.mapName, info.game = fields[0], fields[1], fields[3]
	info.players, info.max = int(rest[2]), int(rest[3])
	return info, nil
}

func players(m *Monitor, latency time.Duration, online, max int, label string) Result {
	label = strings.TrimSpace(label)
	summary := fmt.Sprintf("%d/%d players", online, max)
	if label != "" {
		summary += ", " + truncate(label, 80)
	}
	if want := m.Settings.MinPlayers; want > 0 && online < want {
		return Degraded(latency, "only %s", summary)
	}
	return Up(latency, "%s", summary)
}
