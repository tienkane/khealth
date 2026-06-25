package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build metadata, injected via -ldflags by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("khealth %s (commit %s, built %s)\n", version, commit, date)
	},
}
