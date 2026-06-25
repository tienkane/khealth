package check

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func init() {
	register("http", checkHTTP)
	register("tcp", checkTCP)
	register("redis", checkRedis)
}

// httpClient does not follow redirects, so the status code we judge is the one
// the configured URL actually returned — a health endpoint that 301s elsewhere
// should not be reported UP on the strength of the redirect target. Keep-alives
// are disabled so repeated --watch runs don't accumulate pooled connections.
var httpClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Transport:     &http.Transport{DisableKeepAlives: true},
}

func checkHTTP(ctx context.Context, s Spec) Result {
	if s.URL == "" {
		return down("no url configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return down(cleanErr(err))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return down(cleanErr(err))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))

	statusText := fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	if s.Expect > 0 {
		ok = resp.StatusCode == s.Expect
	}
	if ok {
		return up(statusText)
	}
	return down(statusText)
}

// addrFor resolves an address from Addr or Port, defaulting the host to
// localhost and the port to fallbackPort.
func addrFor(s Spec, fallbackPort int) string {
	if s.Addr != "" {
		return s.Addr
	}
	port := s.Port
	if port == 0 {
		port = fallbackPort
	}
	if port == 0 {
		return ""
	}
	return fmt.Sprintf("localhost:%d", port)
}

func checkTCP(ctx context.Context, s Spec) Result {
	addr := addrFor(s, 0)
	if addr == "" {
		return down("no addr/port configured")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return down(cleanErr(err))
	}
	conn.Close()
	return up("reachable")
}

func checkRedis(ctx context.Context, s Spec) Result {
	addr := addrFor(s, 6379)
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return down(cleanErr(err))
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	}
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return down(cleanErr(err))
	}
	// RESP replies end in \r\n; read the whole first line rather than a single
	// packet, which may arrive split.
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && line == "" {
		return down("no reply: " + cleanErr(err))
	}
	reply := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(reply, "+PONG"):
		return up("PONG")
	case strings.HasPrefix(reply, "+"):
		// Any RESP simple-string reply (e.g. proxies answer +OK) means the
		// server is talking to us.
		return up("reachable")
	case strings.HasPrefix(reply, "-NOAUTH"), strings.HasPrefix(reply, "-ERR") && strings.Contains(strings.ToLower(reply), "auth"):
		return up("reachable (auth required)")
	case strings.HasPrefix(reply, "-"):
		return up("reachable (" + strings.TrimPrefix(reply, "-") + ")")
	default:
		return down("unexpected reply: " + reply)
	}
}
