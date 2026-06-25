package check

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/jackc/pgx/v5"
)

func init() {
	register("postgres", checkPostgres)
	register("docker", checkDocker)
	register("pm2", checkPM2)
}

func checkPostgres(ctx context.Context, s Spec) Result {
	if s.DSN == "" {
		return down("no dsn configured")
	}
	conn, err := pgx.Connect(ctx, s.DSN)
	if err != nil {
		return down(cleanErr(err))
	}
	defer conn.Close(ctx)
	var one int
	if err := conn.QueryRow(ctx, "select 1").Scan(&one); err != nil {
		return down(cleanErr(err))
	}
	return up("query ok")
}

func checkDocker(ctx context.Context, s Spec) Result {
	if s.Container == "" {
		return down("no container configured")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return unknown("docker not installed")
	}
	out, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", s.Container).CombinedOutput()
	if err != nil {
		o := strings.ToLower(string(out))
		switch {
		case strings.Contains(o, "no such object"), strings.Contains(o, "no such container"):
			return down("no such container")
		case strings.Contains(o, "cannot connect"), strings.Contains(o, "is the docker daemon running"):
			return unknown("docker daemon not running")
		case strings.Contains(o, "permission denied"):
			return unknown("docker: permission denied")
		default:
			// The daemon couldn't tell us the container's state (bad DOCKER_HOST,
			// TLS error, an unfamiliar runtime's wording, …). That's "can't tell",
			// not a stopped container.
			if line := firstLine(string(out)); line != "" {
				return unknown(line)
			}
			return unknown(cleanErr(err))
		}
	}
	if strings.TrimSpace(string(out)) == "true" {
		return up("running")
	}
	return down("stopped")
}

// pm2Proc mirrors the part of `pm2 jlist` we read.
type pm2Proc struct {
	Name   string `json:"name"`
	PM2Env struct {
		Status string `json:"status"`
	} `json:"pm2_env"`
}

func checkPM2(ctx context.Context, s Spec) Result {
	if s.Process == "" {
		return down("no process name configured")
	}
	if _, err := exec.LookPath("pm2"); err != nil {
		return unknown("pm2 not installed")
	}
	out, err := exec.CommandContext(ctx, "pm2", "jlist").Output()
	if err != nil {
		return unknown("pm2 jlist failed: " + cleanErr(err))
	}
	var procs []pm2Proc
	if err := json.Unmarshal(out, &procs); err != nil {
		return unknown("cannot parse pm2 output")
	}
	for _, p := range procs {
		if p.Name == s.Process {
			if p.PM2Env.Status == "online" {
				return up("online")
			}
			return down(p.PM2Env.Status)
		}
	}
	return down("not in pm2 list")
}
