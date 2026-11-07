// SPDX-License-Identifier: MIT

package s7commctl

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/otfabric/s7commctl/internal/cli"
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

// completionShells are the shells "completion" writes a script for.
var completionShells = []string{"bash", "zsh", "fish", "powershell"}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate a shell completion script for bash, zsh, fish or PowerShell; README.md,
"Shell completion", says where to put it. Commands, flags and the values of the flags
with a fixed set of them (--area, --type, --format, --confirm, --profile) complete; the
pcap file of pcap-decode completes .pcap files.`,
	ValidArgs: completionShells,
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, out := cmd.Root(), cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return root.GenBashCompletionV2(out, true)
		case "zsh":
			return root.GenZshCompletion(out)
		case "fish":
			return root.GenFishCompletion(out, true)
		default:
			return root.GenPowerShellCompletionWithDesc(out)
		}
	},
}

// registerCompletions registers the completion of every flag and argument of the tree:
// the enum flags, the pcap file of pcap-decode, and nothing (not file names) for every
// other flag that takes a value (an address, a number, a TSAP) and every argument.
// Calling it again changes nothing.
func registerCompletions(root *cobra.Command) {
	registerEnumCompletions(root)
	cli.CompleteTree(root, cli.Completion{
		Files: map[string][]string{
			"pcap-decode --file": {"pcap"},
			"replay --script":    {},
			"record --output":    {},
		},
	})
}

// registerEnumCompletions walks the command tree and registers flag completion for enum flags.
func registerEnumCompletions(c *cobra.Command) {
	for _, sub := range c.Commands() {
		registerEnumCompletions(sub)
		// Per-command flag completions
		if f := sub.Flags().Lookup("area"); f != nil {
			_ = sub.RegisterFlagCompletionFunc("area", fixedCompletion(completionAreas))
		}
		if f := sub.Flags().Lookup("format"); f != nil {
			_ = sub.RegisterFlagCompletionFunc("format", fixedCompletion(formatsOf(f.Usage)))
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

// formatsOf returns the formats a --format usage lists ("Output format: json, table,
// text"), or every format when it lists none.
func formatsOf(usage string) []string {
	_, list, ok := strings.Cut(usage, "Output format: ")
	if !ok {
		return completionFormats
	}
	list, _, _ = strings.Cut(list, " [")
	var out []string
	for v := range strings.SplitSeq(list, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
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
