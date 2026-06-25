package check

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

func init() {
	register("port", checkPort)
	register("process", checkProcess)
	register("command", checkCommand)
}

func checkPort(ctx context.Context, s Spec) Result {
	if s.Port <= 0 {
		return down("no port configured")
	}
	conns, err := gnet.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return unknown("cannot read sockets: " + cleanErr(err))
	}
	for _, c := range conns {
		if c.Status == "LISTEN" && c.Laddr.Port == uint32(s.Port) {
			detail := "listening"
			if c.Pid > 0 {
				if p, err := process.NewProcess(c.Pid); err == nil {
					if n, err := p.Name(); err == nil {
						detail = fmt.Sprintf("listening (%s, pid %d)", n, c.Pid)
					}
				}
			}
			return up(detail)
		}
	}
	return down(fmt.Sprintf("nothing listening on :%d", s.Port))
}

func checkProcess(ctx context.Context, s Spec) Result {
	if s.Process == "" {
		return down("no process name configured")
	}
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return unknown("cannot list processes: " + cleanErr(err))
	}
	q := strings.ToLower(s.Process)
	for _, p := range procs {
		name, _ := p.Name()
		if strings.Contains(strings.ToLower(name), q) {
			return up(fmt.Sprintf("running (%s, pid %d)", name, p.Pid))
		}
	}
	return down("not running")
}

func checkCommand(ctx context.Context, s Spec) Result {
	if s.Command == "" {
		return down("no command configured")
	}
	out, err := exec.CommandContext(ctx, s.Command, s.Args...).CombinedOutput()
	if err != nil {
		// A missing or non-executable binary is "can't tell", not "it's down".
		if errors.Is(err, exec.ErrNotFound) {
			return unknown(s.Command + " not found")
		}
		if errors.Is(err, os.ErrPermission) {
			return unknown(s.Command + ": permission denied")
		}
		if line := firstLine(string(out)); line != "" {
			return down(line)
		}
		return down(cleanErr(err))
	}
	if line := firstLine(string(out)); line != "" {
		return up(line)
	}
	return up("exit 0")
}
