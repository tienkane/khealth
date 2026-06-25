package runner

import (
	"context"
	"testing"
	"time"

	"github.com/tienkane/khealth/internal/check"
)

func TestRunPreservesOrder(t *testing.T) {
	specs := []check.Spec{
		{Name: "a", Type: "command", Command: "sh", Args: []string{"-c", "exit 0"}},
		{Name: "b", Type: "command", Command: "sh", Args: []string{"-c", "exit 1"}},
		{Name: "c", Type: "tcp", Addr: "127.0.0.1:1"},
	}
	results := Run(context.Background(), specs)
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for i, want := range []string{"a", "b", "c"} {
		if results[i].Name != want {
			t.Errorf("results[%d].Name = %q, want %q", i, results[i].Name, want)
		}
	}
	if results[0].Status != check.Up {
		t.Errorf("a => %v, want up", results[0].Status)
	}
	if results[1].Status != check.Down {
		t.Errorf("b => %v, want down", results[1].Status)
	}
}

func TestRunConcurrent(t *testing.T) {
	// Three commands that each sleep ~80ms must finish well under 240ms when run
	// concurrently.
	specs := make([]check.Spec, 3)
	for i := range specs {
		specs[i] = check.Spec{Name: "s", Type: "command", Command: "sh", Args: []string{"-c", "sleep 0.08"}}
	}
	start := time.Now()
	Run(context.Background(), specs)
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Errorf("ran in %v; expected concurrent execution", elapsed)
	}
}

func TestSummarizeAndExitCode(t *testing.T) {
	results := []check.Result{
		{Status: check.Up}, {Status: check.Up},
		{Status: check.Warn},
		{Status: check.Down},
		{Status: check.Unknown},
	}
	s := Summarize(results)
	if s.Up != 2 || s.Warn != 1 || s.Down != 1 || s.Unknown != 1 {
		t.Errorf("summary = %+v", s)
	}
	if s.ExitCode() != 1 {
		t.Errorf("exit code with a down = %d, want 1", s.ExitCode())
	}

	noDown := Summarize([]check.Result{{Status: check.Up}, {Status: check.Unknown}, {Status: check.Warn}})
	if noDown.ExitCode() != 0 {
		t.Errorf("exit code without down = %d, want 0", noDown.ExitCode())
	}
}
