// Package check defines health checks and runs them. A Spec (from config) is
// dispatched by Type to a registered checker, which returns a Status. Each check
// type degrades gracefully: when a backing tool is missing (docker, pm2) the
// result is Unknown, not Down — "can't tell" is distinct from "it's down".
package check

import (
	"context"
	"strings"
	"time"

	"github.com/tienkane/khealth/internal/text"
)

// Status is the outcome of a check.
type Status int

const (
	Up      Status = iota // reachable / healthy
	Warn                  // reachable but slow/degraded
	Down                  // unreachable / failing
	Unknown               // could not be determined (e.g. tool not installed)
)

func (s Status) String() string {
	switch s {
	case Up:
		return "up"
	case Warn:
		return "warn"
	case Down:
		return "down"
	default:
		return "unknown"
	}
}

// Spec is one configured check. Only the fields relevant to its Type are used.
type Spec struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`
	URL       string   `yaml:"url,omitempty"`       // http
	Expect    int      `yaml:"expect,omitempty"`    // http expected status code
	Addr      string   `yaml:"addr,omitempty"`      // tcp / redis (host:port)
	Port      int      `yaml:"port,omitempty"`      // port / tcp / redis
	Process   string   `yaml:"process,omitempty"`   // process / pm2 name
	Container string   `yaml:"container,omitempty"` // docker container
	Command   string   `yaml:"command,omitempty"`   // command
	Args      []string `yaml:"args,omitempty"`      // command args
	DSN       string   `yaml:"dsn,omitempty"`       // postgres
	Timeout   Duration `yaml:"timeout,omitempty"`   // per-check timeout
	Warn      Duration `yaml:"warn,omitempty"`      // latency above this => Warn
}

// Result is what a check produced.
type Result struct {
	Name      string        `json:"name"`
	Type      string        `json:"type"`
	Status    Status        `json:"-"`
	StatusStr string        `json:"status"`
	Detail    string        `json:"detail,omitempty"`
	Latency   time.Duration `json:"-"`
	LatencyMS int64         `json:"latencyMs"`
}

// Func runs one check. It returns only Status+Detail; Run fills the rest.
type Func func(ctx context.Context, s Spec) Result

var registry = map[string]Func{}

func register(typ string, fn Func) { registry[typ] = fn }

// Types lists the registered check types.
func Types() []string {
	out := make([]string, 0, len(registry))
	for t := range registry {
		out = append(out, t)
	}
	return out
}

const defaultTimeout = 5 * time.Second

// Run dispatches a spec to its checker, applying the timeout, measuring latency,
// and applying the Warn threshold.
func Run(ctx context.Context, s Spec) Result {
	fn, ok := registry[s.Type]
	if !ok {
		return finalize(Result{Status: Unknown, Detail: "unknown check type: " + s.Type}, s, 0)
	}
	timeout := s.Timeout.D()
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	res := fn(cctx, s)
	elapsed := time.Since(start)

	if res.Status == Up && s.Warn.D() > 0 && elapsed > s.Warn.D() {
		res.Status = Warn
		if res.Detail == "" {
			res.Detail = "slow"
		}
	}
	return finalize(res, s, elapsed)
}

func finalize(r Result, s Spec, elapsed time.Duration) Result {
	r.Name = s.Name
	r.Type = s.Type
	r.Latency = elapsed
	r.LatencyMS = elapsed.Milliseconds()
	r.StatusStr = r.Status.String()
	r.Detail = text.Sanitize(r.Detail)
	return r
}

// Result builders for checkers.
func up(detail string) Result      { return Result{Status: Up, Detail: detail} }
func down(detail string) Result    { return Result{Status: Down, Detail: detail} }
func unknown(detail string) Result { return Result{Status: Unknown, Detail: detail} }

// firstLine returns the first non-empty line, trimmed and length-capped.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			if len(t) > 80 {
				t = t[:79] + "…"
			}
			return t
		}
	}
	return ""
}

// cleanErr shortens a dial/connection error to its meaningful tail.
func cleanErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// "dial tcp 127.0.0.1:6379: connect: connection refused" -> "connection refused"
	if i := strings.LastIndex(msg, ": "); i >= 0 && i+2 < len(msg) {
		tail := msg[i+2:]
		if len(tail) < len(msg) && tail != "" {
			msg = tail
		}
	}
	return firstLine(msg)
}
