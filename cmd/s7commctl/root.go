package s7commctl

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

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
		fmt.Printf("s7commctl version %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	registerCompletions(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			if ee.Err != nil {
				fmt.Fprintln(os.Stderr, ee.Err)
			}
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
