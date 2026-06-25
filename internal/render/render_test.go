package render

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/tienkane/khealth/internal/check"
	"github.com/tienkane/khealth/internal/runner"
)

var sample = []check.Result{
	{Name: "api", Type: "http", Status: check.Up, StatusStr: "up", Detail: "200 OK", Latency: 12 * time.Millisecond, LatencyMS: 12},
	{Name: "db", Type: "postgres", Status: check.Down, StatusStr: "down", Detail: "connection refused", Latency: 5 * time.Millisecond, LatencyMS: 5},
	{Name: "cache", Type: "docker", Status: check.Unknown, StatusStr: "unknown", Detail: "docker not installed"},
}

// plain strips ANSI escape sequences so we can assert on visible text.
func plain(s string) string {
	for {
		i := strings.IndexByte(s, 0x1b)
		if i < 0 {
			return s
		}
		j := strings.IndexByte(s[i:], 'm')
		if j < 0 {
			return s
		}
		s = s[:i] + s[i+j+1:]
	}
}

func TestTable(t *testing.T) {
	out := plain(Table(sample))
	for _, want := range []string{"api", "db", "cache", "UP", "DOWN", "UNKNOWN", "200 OK", "docker not installed", "12ms"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q\n%s", want, out)
		}
	}
	// Unknown rows show — for latency, not a fabricated 0ms.
	if !strings.Contains(out, "—") {
		t.Errorf("expected em-dash latency for unknown row\n%s", out)
	}
}

func TestSummaryLine(t *testing.T) {
	out := plain(SummaryLine(runner.Summarize(sample)))
	for _, want := range []string{"1 up", "1 down", "1 unknown"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q: %q", want, out)
		}
	}
}

func TestJSON(t *testing.T) {
	out, err := JSON(sample, runner.Summarize(sample))
	if err != nil {
		t.Fatal(err)
	}
	var rep JSONReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(rep.Checks) != 3 {
		t.Errorf("got %d checks, want 3", len(rep.Checks))
	}
	if rep.Summary.Total != 3 || rep.Summary.Down != 1 || rep.Summary.Unknown != 1 {
		t.Errorf("summary = %+v", rep.Summary)
	}
	if rep.Checks[0].StatusStr != "up" {
		t.Errorf("checks[0].status = %q, want up", rep.Checks[0].StatusStr)
	}
}

func TestWidthHelperSanity(t *testing.T) {
	// Guard against accidentally counting ANSI in column width math.
	if lipgloss.Width(green.Render("UP")) != 2 {
		t.Errorf("lipgloss.Width should ignore ANSI; got %d", lipgloss.Width(green.Render("UP")))
	}
}
