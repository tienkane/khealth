package check

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, s Spec) Result {
	t.Helper()
	return Run(context.Background(), s)
}

func TestHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if r := run(t, Spec{Name: "ok", Type: "http", URL: srv.URL}); r.Status != Up {
		t.Errorf("200 => %v, want up (detail %q)", r.Status, r.Detail)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	if r := run(t, Spec{Name: "bad", Type: "http", URL: bad.URL}); r.Status != Down {
		t.Errorf("500 => %v, want down", r.Status)
	}

	// expect override: 500 is the expected code => up
	if r := run(t, Spec{Name: "exp", Type: "http", URL: bad.URL, Expect: 500}); r.Status != Up {
		t.Errorf("500 with expect=500 => %v, want up", r.Status)
	}

	if r := run(t, Spec{Name: "nourl", Type: "http"}); r.Status != Down {
		t.Errorf("missing url => %v, want down", r.Status)
	}

	// Redirects are not followed: the status judged is the 302 the URL itself
	// returned, not the 500 it points at. With following enabled this would be
	// down; without, the 3xx is in range and reports up.
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, bad.URL, http.StatusFound)
	}))
	defer redir.Close()
	if r := run(t, Spec{Name: "redir", Type: "http", URL: redir.URL}); r.Status != Up {
		t.Errorf("302 (not followed) => %v, want up (detail %q)", r.Status, r.Detail)
	}
}

func TestTCPAndPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	addr := ln.Addr().String()
	_, portStr, _ := net.SplitHostPort(addr)

	if r := run(t, Spec{Name: "tcp", Type: "tcp", Addr: addr}); r.Status != Up {
		t.Errorf("tcp open => %v, want up", r.Status)
	}
	// Nothing should be listening here.
	if r := run(t, Spec{Name: "tcpdown", Type: "tcp", Addr: "127.0.0.1:1"}); r.Status != Down {
		t.Errorf("tcp closed => %v, want down", r.Status)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	if r := run(t, Spec{Name: "port", Type: "port", Port: port}); r.Status != Up {
		t.Errorf("port listening => %v, want up (detail %q)", r.Status, r.Detail)
	}
}

func TestCommand(t *testing.T) {
	if r := run(t, Spec{Name: "ok", Type: "command", Command: "sh", Args: []string{"-c", "exit 0"}}); r.Status != Up {
		t.Errorf("exit 0 => %v, want up", r.Status)
	}
	if r := run(t, Spec{Name: "fail", Type: "command", Command: "sh", Args: []string{"-c", "exit 1"}}); r.Status != Down {
		t.Errorf("exit 1 => %v, want down", r.Status)
	}
	if r := run(t, Spec{Name: "missing", Type: "command", Command: "khealth-does-not-exist-xyz"}); r.Status != Unknown {
		t.Errorf("missing binary => %v, want unknown", r.Status)
	}
}

func TestRedisFake(t *testing.T) {
	cases := []struct {
		reply string
		want  Status
	}{
		{"+PONG\r\n", Up},
		{"-NOAUTH Authentication required.\r\n", Up},
		{"$garbage\r\n", Down},
	}
	for _, c := range cases {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go func(reply string) {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			bufio.NewReader(conn).ReadString('\n') // consume PING
			conn.Write([]byte(reply))
		}(c.reply)

		r := run(t, Spec{Name: "redis", Type: "redis", Addr: ln.Addr().String()})
		if r.Status != c.want {
			t.Errorf("reply %q => %v, want %v (detail %q)", strings.TrimSpace(c.reply), r.Status, c.want, r.Detail)
		}
		ln.Close()
	}
}

func TestUnknownType(t *testing.T) {
	r := run(t, Spec{Name: "x", Type: "bogus"})
	if r.Status != Unknown {
		t.Errorf("unknown type => %v, want unknown", r.Status)
	}
}

func TestRedisSplitReply(t *testing.T) {
	// Server writes the +PONG reply in two packets to exercise the full-line
	// read (a single conn.Read could see only "+PO").
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		bufio.NewReader(conn).ReadString('\n')
		conn.Write([]byte("+PO"))
		time.Sleep(20 * time.Millisecond)
		conn.Write([]byte("NG\r\n"))
	}()
	if r := run(t, Spec{Name: "redis", Type: "redis", Addr: ln.Addr().String()}); r.Status != Up {
		t.Errorf("split +PONG => %v, want up (detail %q)", r.Status, r.Detail)
	}
}

func TestWarnThreshold(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(40 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := run(t, Spec{Name: "slow", Type: "http", URL: srv.URL, Warn: Duration(10 * time.Millisecond)})
	if r.Status != Warn {
		t.Errorf("slow response => %v, want warn", r.Status)
	}
}
