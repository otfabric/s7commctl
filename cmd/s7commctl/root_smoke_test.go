// SPDX-License-Identifier: MIT

package s7commctl

import "testing"

func TestRootHasExpectedCommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}

	required := []string{"discover", "identify", "read", "write", "list", "block", "diagnostic", "fingerprint", "record", "version"}
	for _, name := range required {
		if !names[name] {
			t.Fatalf("missing command %q", name)
		}
	}
}
