// Package render formats check results as a colored table or JSON.
package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/tienkane/khealth/internal/check"
	"github.com/tienkane/khealth/internal/runner"
)

var (
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	gray   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	subtle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	bold   = lipgloss.NewStyle().Bold(true)
)

// styleFor returns the color style and label for a status.
func styleFor(s check.Status) (lipgloss.Style, string) {
	switch s {
	case check.Up:
		return green, "UP"
	case check.Warn:
		return yellow, "WARN"
	case check.Down:
		return red, "DOWN"
	default:
		return gray, "UNKNOWN"
	}
}

// StatusCell renders the colored "● LABEL" status cell for a result.
func statusCell(s check.Status) string {
	style, label := styleFor(s)
	return style.Render("● " + label)
}

func latencyCell(r check.Result) string {
	if r.Status == check.Unknown {
		return "—"
	}
	return formatLatency(r.Latency)
}

func formatLatency(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	default:
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
}

// Table renders results as an aligned, colored table.
func Table(results []check.Result) string {
	headers := []string{"STATUS", "NAME", "TYPE", "LATENCY", "DETAIL"}
	rows := make([][]string, 0, len(results)+1)
	rows = append(rows, []string{
		subtle.Render(headers[0]), subtle.Render(headers[1]), subtle.Render(headers[2]),
		subtle.Render(headers[3]), subtle.Render(headers[4]),
	})
	for _, r := range results {
		detail := r.Detail
		switch r.Status {
		case check.Down:
			detail = red.Render(detail)
		case check.Warn:
			detail = yellow.Render(detail)
		}
		rows = append(rows, []string{
			statusCell(r.Status),
			bold.Render(r.Name),
			subtle.Render(r.Type),
			latencyCell(r),
			detail,
		})
	}

	widths := make([]int, len(headers))
	for _, row := range rows {
		for i, cell := range row {
			if w := lipgloss.Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	var b strings.Builder
	for _, row := range rows {
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(cell)+3))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// SummaryLine renders a one-line tally of statuses.
func SummaryLine(s runner.Summary) string {
	var parts []string
	if s.Up > 0 {
		parts = append(parts, green.Render(fmt.Sprintf("%d up", s.Up)))
	}
	if s.Warn > 0 {
		parts = append(parts, yellow.Render(fmt.Sprintf("%d warn", s.Warn)))
	}
	if s.Down > 0 {
		parts = append(parts, red.Render(fmt.Sprintf("%d down", s.Down)))
	}
	if s.Unknown > 0 {
		parts = append(parts, gray.Render(fmt.Sprintf("%d unknown", s.Unknown)))
	}
	if len(parts) == 0 {
		return subtle.Render("no checks")
	}
	return strings.Join(parts, subtle.Render(" · "))
}

// JSONReport is the top-level shape of --json output.
type JSONReport struct {
	Checks  []check.Result `json:"checks"`
	Summary summaryJSON    `json:"summary"`
}

type summaryJSON struct {
	Up      int `json:"up"`
	Warn    int `json:"warn"`
	Down    int `json:"down"`
	Unknown int `json:"unknown"`
	Total   int `json:"total"`
}

// JSON renders results as indented JSON.
func JSON(results []check.Result, s runner.Summary) (string, error) {
	rep := JSONReport{
		Checks: results,
		Summary: summaryJSON{
			Up: s.Up, Warn: s.Warn, Down: s.Down, Unknown: s.Unknown,
			Total: s.Up + s.Warn + s.Down + s.Unknown,
		},
	}
	out, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}
