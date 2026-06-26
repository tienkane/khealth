// Package discover probes the local machine for services worth health-checking
// — running Docker containers, PM2 apps, and listening ports owned by common
// dev runtimes — so `khealth init` can seed a config from what's actually there
// instead of a generic template. It reads system state only; it changes nothing.
package discover

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"
	"strings"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// Port is a detected listening port and the process behind it (best effort).
type Port struct {
	Port int
	Proc string
}

// Result is everything discovery found.
type Result struct {
	Docker []string // running container names
	PM2    []string // pm2 app names
	Ports  []Port   // local listening ports owned by dev runtimes
}

// Empty reports whether nothing was discovered.
func (r Result) Empty() bool {
	return len(r.Docker) == 0 && len(r.PM2) == 0 && len(r.Ports) == 0
}

// Discover probes Docker, PM2, and listening ports. Each probe degrades to
// empty when its backing tool is missing or errors — discovery never fails.
func Discover(ctx context.Context) Result {
	return Result{
		Docker: dockerContainers(ctx),
		PM2:    pm2Apps(ctx),
		Ports:  listeningPorts(ctx),
	}
}

func dockerContainers(ctx context.Context) []string {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Names}}").Output()
	if err != nil {
		return nil
	}
	return nonEmptyLines(string(out))
}

func pm2Apps(ctx context.Context) []string {
	if _, err := exec.LookPath("pm2"); err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "pm2", "jlist").Output()
	if err != nil {
		return nil
	}
	var procs []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(out, &procs) != nil {
		return nil
	}
	var names []string
	for _, p := range procs {
		if n := strings.TrimSpace(p.Name); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// maxPorts caps the number of detected ports so a noisy machine can't produce a
// runaway config.
const maxPorts = 20

func listeningPorts(ctx context.Context) []Port {
	conns, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return nil
	}
	seen := map[int]bool{}
	var ports []Port
	for _, c := range conns {
		if c.Status != "LISTEN" || c.Laddr.Port == 0 || !localIP(c.Laddr.IP) {
			continue
		}
		p := int(c.Laddr.Port)
		if seen[p] {
			continue
		}
		name := procName(c.Pid)
		// Surface a port only when we recognize it: a well-known dev port or a
		// process that is a typical dev runtime. This skips OS chatter (mDNS,
		// ControlCenter, …) so the seed config stays meaningful.
		if !knownPorts[p] && !devRuntimes[runtimeKey(name)] {
			continue
		}
		seen[p] = true
		ports = append(ports, Port{Port: p, Proc: name})
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i].Port < ports[j].Port })
	if len(ports) > maxPorts {
		ports = ports[:maxPorts]
	}
	return ports
}

func localIP(ip string) bool {
	switch ip {
	case "127.0.0.1", "::1", "0.0.0.0", "::", "":
		return true
	}
	return false
}

func procName(pid int32) string {
	if pid <= 0 {
		return ""
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return ""
	}
	n, _ := p.Name()
	return n
}

// runtimeKey normalizes a process name for the devRuntimes lookup.
func runtimeKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if i := strings.IndexAny(name, " \t"); i >= 0 {
		name = name[:i]
	}
	return name
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// devRuntimes are process names that typically front an app, DB, or cache in
// local development.
var devRuntimes = map[string]bool{
	"node": true, "deno": true, "bun": true, "ts-node": true, "tsx": true,
	"python": true, "python3": true, "ruby": true, "rails": true, "puma": true,
	"gunicorn": true, "uvicorn": true, "java": true, "dotnet": true, "php": true,
	"postgres": true, "postgresql": true, "redis-server": true, "mysqld": true,
	"mongod": true, "air": true, "caddy": true, "nginx": true, "next-server": true,
	"vite": true,
}

// knownPorts are ports commonly used by local dev services, surfaced even when
// the owning process can't be identified.
var knownPorts = map[int]bool{
	3000: true, 3001: true, 4000: true, 4321: true, 5000: true, 5173: true,
	5432: true, 5433: true, 6379: true, 6380: true, 8000: true, 8080: true,
	8081: true, 9000: true, 27017: true, 54321: true, 54322: true,
}
