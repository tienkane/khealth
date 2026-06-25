// Package runner executes a set of check specs concurrently.
package runner

import (
	"context"
	"sync"

	"github.com/tienkane/khealth/internal/check"
)

// Run executes every spec concurrently and returns the results in the same
// order as specs. Each check enforces its own timeout, so a slow or hung check
// cannot block the others.
func Run(ctx context.Context, specs []check.Spec) []check.Result {
	results := make([]check.Result, len(specs))
	var wg sync.WaitGroup
	for i, s := range specs {
		wg.Add(1)
		go func(i int, s check.Spec) {
			defer wg.Done()
			results[i] = check.Run(ctx, s)
		}(i, s)
	}
	wg.Wait()
	return results
}

// Summary counts results by status.
type Summary struct {
	Up, Warn, Down, Unknown int
}

// Summarize tallies results by status.
func Summarize(results []check.Result) Summary {
	var s Summary
	for _, r := range results {
		switch r.Status {
		case check.Up:
			s.Up++
		case check.Warn:
			s.Warn++
		case check.Down:
			s.Down++
		default:
			s.Unknown++
		}
	}
	return s
}

// ExitCode is 1 if any check is Down, else 0. Warn and Unknown do not fail the
// run — only a definite Down does.
func (s Summary) ExitCode() int {
	if s.Down > 0 {
		return 1
	}
	return 0
}
