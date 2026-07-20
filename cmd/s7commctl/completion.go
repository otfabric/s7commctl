// SPDX-License-Identifier: MIT

package s7commctl

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Completion enum sets for flags (read-only; used by shell completion).
var (
	completionAreas      = []string{"DB", "I", "Q", "M", "T", "C", "DATA_RECORD", "S7200AO", "IEC_COUNTER_200", "IEC_TIMER_200"}
	completionDataTypes  = []string{"raw", "bool", "int", "dint", "real", "string"}
	completionBlockTypes = []string{"OB", "DB", "FC", "FB", "SFC", "SFB"}
	completionFormats    = []string{"json", "table", "text", "plain", "hex", "jsonl", "spans"}
	completionConfirm    = []string{"szl", "cpu-state", "any"}
	completionProfile    = []string{"quick", "standard", "full"}
)

func init() {
	rootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion script",
	Long: `Generate a shell completion script for bash, zsh, or fish.
Load the script in your shell to get tab-completion of commands and flags.

Examples:
  # Bash (current session)
  source <(s7commctl completion bash)
  # Bash (persistent)
  s7commctl completion bash > /etc/bash_completion.d/s7commctl

  # Zsh
  source <(s7commctl completion zsh)

  # Fish
  s7commctl completion fish | source
`,
	ValidArgs: []string{"bash", "zsh", "fish"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]
		switch shell {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		default:
			return fmt.Errorf("unsupported shell: %s", shell)
		}
	},
}

// registerCompletions walks the command tree and registers flag completion for enum flags.
func registerCompletions(c *cobra.Command) {
	for _, sub := range c.Commands() {
		registerCompletions(sub)
		// Per-command flag completions
		if f := sub.Flags().Lookup("area"); f != nil {
			_ = sub.RegisterFlagCompletionFunc("area", fixedCompletion(completionAreas))
		}
		if f := sub.Flags().Lookup("format"); f != nil {
			_ = sub.RegisterFlagCompletionFunc("format", fixedCompletion(completionFormats))
		}
		if f := sub.Flags().Lookup("confirm"); f != nil {
			_ = sub.RegisterFlagCompletionFunc("confirm", fixedCompletion(completionConfirm))
		}
		if f := sub.Flags().Lookup("profile"); f != nil {
			_ = sub.RegisterFlagCompletionFunc("profile", fixedCompletion(completionProfile))
		}
		if f := sub.Flags().Lookup("type"); f != nil {
			// Data type for read/write/record; block type for block info/upload and list blocks
			switch sub.Name() {
			case "read", "write", "record":
				_ = sub.RegisterFlagCompletionFunc("type", fixedCompletion(completionDataTypes))
			case "info", "upload", "blocks":
				_ = sub.RegisterFlagCompletionFunc("type", fixedCompletion(completionBlockTypes))
			default:
				_ = sub.RegisterFlagCompletionFunc("type", fixedCompletion(completionDataTypes))
			}
		}
	}
}

// fixedCompletion returns a completion function that suggests the given words, filtered by toComplete.
func fixedCompletion(words []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var out []string
		toComplete = strings.ToLower(toComplete)
		for _, w := range words {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(w), toComplete) {
				out = append(out, w)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
