// Package cli wires the cobra command: locate the config, run its checks
// (once, or repeatedly in --watch), and render the results as a table or JSON.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tienkane/khealth/internal/check"
	"github.com/tienkane/khealth/internal/config"
	"github.com/tienkane/khealth/internal/render"
	"github.com/tienkane/khealth/internal/runner"
	"github.com/tienkane/khealth/internal/tui"
)

var (
	flagConfig   string
	flagJSON     bool
	flagWatch    bool
	flagInterval time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "khealth [name...]",
	Short: "Unified health check for your local services",
	Long: `khealth runs the checks declared in khealth.yaml — HTTP endpoints, TCP ports,
processes, Docker containers, PM2 apps, Redis, Postgres/Supabase, and shell
commands — concurrently, and prints a green/red status table.

One command in the morning tells you whether everything is up. Pass check names
to run only those. A backing tool that isn't installed (docker, pm2) reports
UNKNOWN rather than failing the run — "can't tell" is not "it's down".`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	Example: `  khealth                 # run all checks once, print a table
  khealth api postgres    # run only the checks named "api" and "postgres"
  khealth --watch         # live auto-refreshing dashboard
  khealth --json          # machine-readable output
  khealth init            # scaffold a khealth.yaml`,
	RunE: run,
}

// exitCode is set by run() so the process can exit non-zero on a DOWN check
// without calling os.Exit from inside the cobra handler.
var exitCode int

// Execute runs the root command.
func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		exitCode = 1
	}
	stop()
	os.Exit(exitCode)
}

func init() {
	f := rootCmd.Flags()
	f.StringVarP(&flagConfig, "config", "c", "", "path to config file (default: search up from cwd, then ~/.config/khealth)")
	f.BoolVar(&flagJSON, "json", false, "print results as JSON")
	f.BoolVarP(&flagWatch, "watch", "w", false, "live dashboard that re-runs checks on an interval")
	f.DurationVarP(&flagInterval, "interval", "i", 5*time.Second, "refresh interval for --watch")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}} (commit " + commit + ", built " + date + ")\n")
}

func run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	specs, path, err := loadSpecs(args)
	if err != nil {
		return err
	}

	if flagWatch {
		if flagJSON {
			return errors.New("--watch and --json cannot be combined")
		}
		return tui.RunWatch(ctx, specs, path, flagInterval)
	}

	results := runner.Run(ctx, specs)
	summary := runner.Summarize(results)

	if flagJSON {
		out, err := render.JSON(results, summary)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(render.Table(results))
		fmt.Println()
		fmt.Println(render.SummaryLine(summary))
	}

	exitCode = summary.ExitCode()
	return nil
}

// loadSpecs resolves the config path, loads it, and filters by the requested
// check names (if any).
func loadSpecs(names []string) ([]check.Spec, string, error) {
	path := flagConfig
	if path == "" {
		found, err := config.Find()
		if err != nil {
			return nil, "", fmt.Errorf("%w (run `khealth init` to create one)", err)
		}
		path = found
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, "", err
	}
	specs := cfg.Checks
	if len(names) > 0 {
		filtered, err := filterByName(specs, names)
		if err != nil {
			return nil, "", err
		}
		specs = filtered
	}
	return specs, path, nil
}

func filterByName(specs []check.Spec, names []string) ([]check.Spec, error) {
	have := map[string]check.Spec{}
	for _, s := range specs {
		have[strings.ToLower(s.Name)] = s
	}
	var out []check.Spec
	var missing []string
	for _, n := range names {
		if s, ok := have[strings.ToLower(n)]; ok {
			out = append(out, s)
		} else {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("no check named %s (defined: %s)",
			strings.Join(missing, ", "), strings.Join(checkNames(specs), ", "))
	}
	return out, nil
}

func checkNames(specs []check.Spec) []string {
	names := make([]string, len(specs))
	for i, s := range specs {
		names[i] = s.Name
	}
	return names
}
