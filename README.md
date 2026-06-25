# khealth

> One command to check everything running on your machine — HTTP endpoints, TCP
> ports, processes, Docker containers, PM2 apps, Redis, Postgres/Supabase, and
> shell commands — in a single green/red status table.

[![CI](https://github.com/tienkane/khealth/actions/workflows/ci.yml/badge.svg)](https://github.com/tienkane/khealth/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/tienkane/khealth)](https://goreportcard.com/report/github.com/tienkane/khealth)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

![demo](./demo.gif)

You open the terminal in the morning and wonder: is the API up? Is Postgres
reachable? Did that Docker container survive the reboot? Instead of five
different commands, declare your services once in `khealth.yaml` and run
`khealth`. Everything is checked **concurrently** and printed as a table.

A backing tool that isn't installed (no `docker`, no `pm2`) reports **UNKNOWN**,
not DOWN — "can't tell" is not the same as "it's broken".

## Features

- 🟢 **One table, every service** — HTTP, TCP, listening port, process, shell
  command, Redis, Postgres/Supabase, Docker, PM2 — checked in parallel.
- 🩺 **Honest statuses** — `UP` / `WARN` (slow) / `DOWN` / `UNKNOWN`. A `warn`
  threshold flags services that respond but are sluggish.
- 👀 **`--watch` dashboard** — a live, auto-refreshing view for keeping an eye
  on things while you work.
- 🤖 **Scriptable** — `--json` for machines, exit code `1` when anything is
  `DOWN` (so it slots into CI, pre-commit hooks, or a launchd job).
- 🔌 **No agents, no daemons, few deps** — Redis is checked by speaking RESP on
  the wire (no `redis-cli`), Postgres with a real `select 1`.
- 🧰 **`khealth init`** — scaffolds a commented config covering every check type.

## Install

```sh
# Homebrew
brew tap tienkane/tap
brew trust tienkane/tap
brew install --cask khealth

# Go
go install github.com/tienkane/khealth@latest
```

## Quick start

```sh
khealth init          # write a starter khealth.yaml
$EDITOR khealth.yaml  # describe your services
khealth               # check them all
```

## Usage

```sh
khealth                 # run every check once, print a table
khealth api postgres    # run only the checks named "api" and "postgres"
khealth --watch         # live, auto-refreshing dashboard (q to quit)
khealth -w -i 2s        # …refreshing every 2 seconds
khealth --json          # machine-readable output
khealth -c ./other.yaml # use a specific config file
```

`khealth` walks up from the current directory looking for `khealth.yaml` (or
`.khealth.yaml`), then falls back to `~/.config/khealth/khealth.yaml` — so a
project-local config wins, with a personal default behind it.

### Flags

| Flag               | Description                                                   |
| ------------------ | ------------------------------------------------------------- |
| `-c, --config`     | Path to the config file (default: search up, then `~/.config`)|
| `-w, --watch`      | Live dashboard that re-runs checks on an interval             |
| `-i, --interval`   | Refresh interval for `--watch` (default `5s`)                 |
| `--json`           | Print results as JSON                                         |

### Exit code

`0` if no check is `DOWN`; `1` if any check is `DOWN`. `WARN` and `UNKNOWN` do
**not** fail the run — only a definite failure does. This makes `khealth` safe
to drop into a script that should only break on a real outage.

## Configuration

Each check has a `name`, a `type`, and the fields that type needs. Optional on
any check: `timeout` (default `5s`) and `warn` (latency above which it shows
`WARN`). Durations are strings like `500ms`, `2s`, or a bare number of seconds.

```yaml
checks:
  - name: api               # HTTP — UP on 2xx/3xx, or set "expect" for a code
    type: http
    url: http://localhost:3000/health
    warn: 500ms

  - name: db-port           # TCP connect
    type: tcp
    addr: localhost:5432

  - name: web               # local listening port (names the process)
    type: port
    port: 8080

  - name: node              # a running process (name contains this string)
    type: process
    process: node

  - name: disk              # shell command — UP on exit 0
    type: command
    command: sh
    args: ["-c", "df -h / | tail -1"]

  - name: redis             # PING over the wire — no redis-cli needed
    type: redis
    addr: localhost:6379

  - name: postgres          # Postgres / Supabase — runs "select 1"
    type: postgres
    dsn: postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable

  - name: cache             # Docker container — UNKNOWN if docker isn't installed
    type: docker
    container: my-redis

  - name: worker            # PM2 app — UNKNOWN if pm2 isn't installed
    type: pm2
    process: my-worker
```

### Check types

| Type       | Key fields                | UP when…                                  |
| ---------- | ------------------------- | ----------------------------------------- |
| `http`     | `url`, `expect`           | status is 2xx/3xx (or equals `expect`)    |
| `tcp`      | `addr` or `port`          | the port accepts a connection             |
| `port`     | `port`                    | something is locally listening on it      |
| `process`  | `process`                 | a running process name contains it        |
| `command`  | `command`, `args`         | the command exits `0`                     |
| `redis`    | `addr` or `port` (6379)   | the server replies to `PING`              |
| `postgres` | `dsn`                     | `select 1` succeeds                       |
| `docker`   | `container`               | the container is running                  |
| `pm2`      | `process`                 | the app's pm2 status is `online`          |

`docker` and `pm2` report **UNKNOWN** when their CLI isn't installed; `command`
reports UNKNOWN when the binary isn't found. Everything else that can't connect
is **DOWN**.

## Building from source

```sh
git clone https://github.com/tienkane/khealth
cd khealth
go build .
go test ./...
```

Requires Go 1.25+.

## License

[MIT](LICENSE).
