package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tienkane/khealth/internal/discover"
)

var flagForce bool

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Write a khealth.yaml, seeded from what's running now",
	Long: `init probes the machine — running Docker containers, PM2 apps, and listening
ports owned by common dev runtimes — and writes a khealth.yaml seeded with what
it finds, so you start from your actual services instead of a blank template.

Things that can't be auto-detected (HTTP health URLs, a Postgres DSN) are left
as commented examples to fill in. If nothing is detected, a full template
covering every check type is written instead. Existing files are kept unless
--force is given.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "khealth.yaml"
		if len(args) == 1 {
			path = args[0]
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				path = filepath.Join(path, "khealth.yaml")
			}
		}
		if _, err := os.Stat(path); err == nil && !flagForce {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 6*time.Second)
		defer cancel()
		found := discover.Discover(ctx)

		if err := os.WriteFile(path, []byte(generateConfig(found)), 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote %s — detected %s.\n", path, summarize(found))
		if found.Empty() {
			fmt.Println("Nothing running was detected, so a full template was written. Edit it, then run `khealth`.")
		} else {
			fmt.Println("Review it (add any HTTP/Postgres checks from the examples), then run `khealth`.")
		}
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "overwrite an existing config")
}

// generateConfig renders a khealth.yaml from discovery, or the full static
// template when nothing was found.
func generateConfig(d discover.Result) string {
	if d.Empty() {
		return scaffold
	}

	used := map[string]bool{}
	uniq := func(name string) string {
		base := name
		for i := 2; used[name]; i++ {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		used[name] = true
		return name
	}

	var b strings.Builder
	b.WriteString("# khealth.yaml — seeded by `khealth init` from what's running now.\n")
	b.WriteString("# Edit freely: add the HTTP/Postgres checks at the bottom, set `warn:`\n")
	b.WriteString("# thresholds, rename things, and remove anything you don't care about.\n\n")
	b.WriteString("checks:\n")

	if len(d.Ports) > 0 {
		b.WriteString("\n  # — Listening ports (detected) —\n")
		for _, p := range d.Ports {
			writePortCheck(&b, uniq, p)
		}
	}
	if len(d.Docker) > 0 {
		b.WriteString("\n  # — Docker containers (running) —\n")
		for _, name := range d.Docker {
			writeCheck(&b, uniq(name), "docker", [][2]string{{"container", name}})
		}
	}
	if len(d.PM2) > 0 {
		b.WriteString("\n  # — PM2 apps —\n")
		for _, name := range d.PM2 {
			writeCheck(&b, uniq(name), "pm2", [][2]string{{"process", name}})
		}
	}

	b.WriteString(commentedExamples)
	return b.String()
}

// writePortCheck maps a detected port to the most useful check type. Ports that
// speak a credential-free protocol (Redis) get a real app-level check; database
// ports get a TCP reachability check, with the real "select 1" left as a
// commented example since it needs a DSN.
func writePortCheck(b *strings.Builder, uniq func(string) string, p discover.Port) {
	addr := fmt.Sprintf("localhost:%d", p.Port)
	switch p.Port {
	case 6379, 6380:
		writeCheck(b, uniq("redis"), "redis", [][2]string{{"addr", addr}})
	case 5432, 5433, 54322:
		writeCheck(b, uniq("postgres"), "tcp", [][2]string{{"addr", addr}})
	case 27017:
		writeCheck(b, uniq("mongo"), "tcp", [][2]string{{"addr", addr}})
	default:
		label := procLabel(p.Proc)
		if label == "" {
			label = "service"
		}
		name := uniq(fmt.Sprintf("%s-%d", label, p.Port))
		writeCheck(b, name, "port", [][2]string{{"port", strconv.Itoa(p.Port)}})
	}
}

func writeCheck(b *strings.Builder, name, typ string, fields [][2]string) {
	fmt.Fprintf(b, "  - name: %s\n", name)
	fmt.Fprintf(b, "    type: %s\n", typ)
	for _, f := range fields {
		fmt.Fprintf(b, "    %s: %s\n", f[0], f[1])
	}
}

// procLabel reduces a process name to a yaml-safe, lowercase token.
func procLabel(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out []rune
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			out = append(out, r)
		}
	}
	return strings.Trim(string(out), "-_")
}

func summarize(d discover.Result) string {
	parts := []string{
		plural(len(d.Ports), "listening port", "listening ports"),
		plural(len(d.Docker), "Docker container", "Docker containers"),
		plural(len(d.PM2), "PM2 app", "PM2 apps"),
	}
	return strings.Join(parts, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

const commentedExamples = `
  # — Add what can't be auto-detected (uncomment and edit) —
  #
  # - name: api            # an HTTP health endpoint
  #   type: http
  #   url: http://localhost:3000/health
  #   warn: 500ms
  #
  # - name: db             # Postgres / Supabase with a real "select 1"
  #   type: postgres
  #   dsn: postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
`

const scaffold = `# khealth.yaml — declare the services to check.
# Run "khealth" to check them all, or "khealth <name>..." for a subset.
#
# Each check has a name, a type, and the fields that type uses. Optional on any
# check: "timeout" (default 5s) and "warn" (latency above which it shows WARN).

checks:
  # HTTP endpoint — UP on 2xx/3xx, or set "expect" to require a status code.
  - name: api
    type: http
    url: http://localhost:3000/health
    warn: 500ms

  # TCP connect — UP if the port accepts a connection.
  - name: db-port
    type: tcp
    addr: localhost:5432

  # Local listening port — UP if something is listening, naming the process.
  - name: web
    type: port
    port: 8080

  # Running process — UP if a process name contains this string.
  - name: node
    type: process
    process: node

  # Shell command — UP on exit 0. Unknown if the command isn't found.
  - name: disk
    type: command
    command: sh
    args: ["-c", "df -h / | tail -1"]

  # Redis — UP on PONG (PING over the wire; no redis-cli needed).
  - name: redis
    type: redis
    addr: localhost:6379

  # Postgres / Supabase — UP if "select 1" succeeds.
  - name: postgres
    type: postgres
    dsn: postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable

  # Docker container — UP if running. Unknown if docker isn't installed.
  - name: cache
    type: docker
    container: my-redis

  # PM2 app — UP if "online". Unknown if pm2 isn't installed.
  - name: worker
    type: pm2
    process: my-worker
`
