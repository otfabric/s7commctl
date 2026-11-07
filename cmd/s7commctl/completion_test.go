// SPDX-License-Identifier: MIT

package s7commctl

import (
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// completeLine runs the hidden __complete command and returns the candidates (descriptions
// cut off) followed by the directive line (":4").
func completeLine(t *testing.T, args ...string) string {
	t.Helper()
	registerCompletions(rootCmd)
	r := runCLI(t, append([]string{cobra.ShellCompRequestCmd}, args...)...).ok(t)
	var words []string
	for line := range strings.SplitSeq(strings.TrimSpace(r.stdout), "\n") {
		words = append(words, strings.SplitN(line, "\t", 2)[0])
	}
	return strings.Join(words, " ")
}

func TestCompletion_Candidates(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"completion", ""}, "bash zsh fish powershell :4"},
		{[]string{"discover", "--format", ""}, "json table text :4"},
		{[]string{"scan", "read-ranges", "--format", ""}, "json table spans :4"},
		{[]string{"record", "--format", ""}, "jsonl hex :4"},
		{[]string{"compare", "read", "--area", "D"}, "DB DATA_RECORD :4"},
		{[]string{"list", "blocks", "--type", "F"}, "FC FB :4"},
		{[]string{"probe", "rackslot", "--confirm", ""}, "szl cpu-state any :4"},
		{[]string{"pcap-decode", "--file", ""}, "pcap :8"},
		{[]string{"replay", "--script", ""}, ":0"},
		{[]string{"read", "--ip", ""}, ":4"},
		{[]string{"identify", ""}, ":4"},
	} {
		if got := completeLine(t, c.args...); got != c.want {
			t.Errorf("completion of %v: %q, want %q", c.args, got, c.want)
		}
	}
}

// valueSet matches a usage that lists the values of a flag: "Output format: json,
// table, text", "Memory area: DB, I, Q, …", "one of …", "a|b".
var valueSet = regexp.MustCompile(`(^|: )([a-z0-9-]+|[A-Z0-9_*]+)( \([^)]*\))?(, ([a-z0-9-]+|[A-Z0-9_*]+)( \([^)]*\))?)+|\bone of\b|\b[a-z0-9-]+\|[a-z0-9-]+\b`)

// Every flag that takes a value completes, and one whose usage lists its values
// completes to them, not to file names.
func TestCompletion_EveryFlag(t *testing.T) {
	registerCompletions(rootCmd)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.NoOptDefVal != "" || f.Name == "help" {
				return
			}
			fn, ok := c.GetFlagCompletionFunc(f.Name)
			if !ok {
				t.Errorf("%s --%s has no completion", c.CommandPath(), f.Name)
				return
			}
			if valueSet.MatchString(f.Usage) || f.Name == "format" || f.Name == "type" {
				values, dir := fn(c, nil, "")
				if len(values) < 2 || dir != cobra.ShellCompDirectiveNoFileComp {
					t.Errorf("%s --%s (%q) completes %q, directive %d", c.CommandPath(), f.Name, f.Usage, values, dir)
				}
			}
		})
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(rootCmd)
}
