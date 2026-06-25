// Command khealth runs a set of configured health checks — HTTP, TCP, ports,
// processes, Docker, PM2, Redis, Postgres — concurrently and prints a
// green/red status table, so one command tells you what's up on your machine.
package main

import "github.com/tienkane/khealth/internal/cli"

func main() {
	cli.Execute()
}
