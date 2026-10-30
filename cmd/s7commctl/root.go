// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	buildVersion = "dev"
	buildTag     = ""
	buildCommit  = "unknown"
	buildDateStr = "unknown"
)

// SetBuildInfo sets version metadata (normally from main via -ldflags on main.version, main.tag, main.commit, main.buildDate).
func SetBuildInfo(version, tag, commit, buildDate string) {
	buildVersion = version
	buildTag = tag
	buildCommit = commit
	buildDateStr = buildDate
}

var rootCmd = &cobra.Command{
	Use:   "s7commctl",
	Short: "Siemens S7 protocol communication tool",
	Long:  `s7commctl is a professional tool for communicating with Siemens S7 PLCs using the S7comm protocol over ISO-on-TCP (RFC1006).`,
	// Do not print usage on runtime/execution failures (e.g. short-read, EOF, timeout).
	// Usage is only appropriate for invalid flags or missing required args.
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		if buildTag != "" {
			fmt.Printf("s7commctl version %s (tag: %s, commit: %s, built: %s)\n", buildVersion, buildTag, buildCommit, buildDateStr)
			return
		}
		fmt.Printf("s7commctl version %s (commit: %s, built: %s)\n", buildVersion, buildCommit, buildDateStr)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the CLI until it finishes or SIGINT or SIGTERM cancels it.
func Execute() {
	registerCompletions(rootCmd)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := rootCmd.ExecuteContext(ctx)
	stop()
	if err == nil {
		return
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		if ee.Err != nil {
			fmt.Fprintln(os.Stderr, ee.Err)
		}
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitCode(err))
}
