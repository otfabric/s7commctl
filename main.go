package main

import "github.com/otfabric/s7commctl/cmd/s7commctl"

// Injected at link time via -ldflags (see Makefile / release workflow).
var (
	version   = "dev"
	tag       = ""
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	s7commctl.SetBuildInfo(version, tag, commit, buildDate)
	s7commctl.Execute()
}
