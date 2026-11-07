// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/otfabric/s7commctl/internal/buildinfo"
)

// SetBuildInfo sets version metadata (normally from main via -ldflags on main.version, main.tag, main.commit, main.buildDate).
func SetBuildInfo(version, tag, commit, buildDate string) {
	buildinfo.Set(version, tag, commit, buildDate)
}

var rootCmd = &cobra.Command{
	Use:   "s7commctl",
	Short: "Siemens S7 protocol communication tool",
	Long:  `s7commctl is a professional tool for communicating with Siemens S7 PLCs using the S7comm protocol over ISO-on-TCP (RFC1006).`,
	// Do not print usage on runtime/execution failures (e.g. short-read, EOF, timeout).
	// Usage is only appropriate for invalid flags or missing required args.
	SilenceUsage: true,
}

// versionJSON is the --json flag of version.
var versionJSON bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long: `Print the version of s7commctl on one line: version, tag, commit, build date, the OT
Fabric libraries it is built with (go-s7comm), Go version and platform. --json, or
S7COMMCTL_FORMAT=json, prints it as one JSON object.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		info := buildinfo.Get("s7commctl")
		line := info.String()
		if versionJSON || strings.EqualFold(strings.TrimSpace(os.Getenv("S7COMMCTL_FORMAT")), "json") {
			line = info.JSON()
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "Print the version as one JSON object")
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
	if ee, ok := errors.AsType[*ExitError](err); ok {
		if ee.Err != nil {
			fmt.Fprintln(os.Stderr, ee.Err)
		}
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitCode(err))
}
