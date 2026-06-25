// Package tui provides the --watch dashboard: a Bubble Tea program that re-runs
// the checks on an interval and redraws the status table.
package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/tienkane/khealth/internal/check"
	"github.com/tienkane/khealth/internal/render"
	"github.com/tienkane/khealth/internal/runner"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("63")).Padding(0, 1)
	subtle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	spin       = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
)

type resultsMsg struct {
	results []check.Result
	summary runner.Summary
	at      time.Time
}

type tickMsg struct{}

type watchModel struct {
	ctx      context.Context
	specs    []check.Spec
	path     string
	interval time.Duration

	results    []check.Result
	summary    runner.Summary
	updated    time.Time
	refreshing bool
	width      int
}

// RunWatch starts the auto-refreshing dashboard.
func RunWatch(ctx context.Context, specs []check.Spec, path string, interval time.Duration) error {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	m := watchModel{ctx: ctx, specs: specs, path: path, interval: interval, refreshing: true}
	_, err := tea.NewProgram(m, tea.WithContext(ctx)).Run()
	return err
}

func (m watchModel) Init() tea.Cmd {
	return m.runChecks()
}

// runChecks executes the checks off the UI goroutine and reports back.
func (m watchModel) runChecks() tea.Cmd {
	ctx, specs := m.ctx, m.specs
	return func() tea.Msg {
		results := runner.Run(ctx, specs)
		return resultsMsg{results: results, summary: runner.Summarize(results), at: time.Now()}
	}
}

func (m watchModel) tick() tea.Cmd {
	return tea.Tick(m.interval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			if !m.refreshing {
				m.refreshing = true
				return m, m.runChecks()
			}
		}
		return m, nil

	case resultsMsg:
		m.results = msg.results
		m.summary = msg.summary
		m.updated = msg.at
		m.refreshing = false
		return m, m.tick()

	case tickMsg:
		// On Ctrl+C the root context is cancelled before Bubble Tea quits;
		// skip the refresh so checks don't briefly flash DOWN on the way out.
		if m.refreshing || m.ctx.Err() != nil {
			return m, nil
		}
		m.refreshing = true
		return m, m.runChecks()
	}
	return m, nil
}

func (m watchModel) View() tea.View {
	header := titleStyle.Render("khealth")
	status := subtle.Render("loading…")
	if !m.updated.IsZero() {
		status = subtle.Render("updated " + m.updated.Format("15:04:05"))
	}
	if m.refreshing {
		status += " " + spin.Render("↻")
	}

	var out string
	out += header + "  " + status + "\n"
	out += subtle.Render(m.path) + "\n\n"
	if len(m.results) == 0 {
		out += subtle.Render("running checks…") + "\n"
	} else {
		out += render.Table(m.results)
		out += "\n" + render.SummaryLine(m.summary) + "\n"
	}
	out += "\n" + subtle.Render(fmt.Sprintf("r refresh · q quit · auto every %s", m.interval))

	v := tea.NewView(out)
	v.AltScreen = true
	return v
}
