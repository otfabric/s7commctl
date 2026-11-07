// SPDX-License-Identifier: MIT

package main

import "github.com/otfabric/s7commctl/cmd/s7commctl"

// Injected at link time via -ldflags (see Makefile / release workflow).
// Empty when not injected: internal/buildinfo then reports what Go embeds, or dev/unknown.
var (
	version   string
	tag       string
	commit    string
	buildDate string
)

func main() {
	s7commctl.SetBuildInfo(version, tag, commit, buildDate)
	s7commctl.Execute()
}
