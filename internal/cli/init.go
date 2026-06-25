package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var flagForce bool

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Write a starter khealth.yaml",
	Long: `init writes a commented khealth.yaml in the current directory (or at the
given path) covering every check type, ready to edit. Existing files are kept
unless --force is given.`,
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
		if err := os.WriteFile(path, []byte(scaffold), 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote %s — edit it, then run `khealth`.\n", path)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "overwrite an existing config")
}

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
