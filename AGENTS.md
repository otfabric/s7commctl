# AGENTS.md — s7commctl

## What this is

Command-line tool for Siemens S7 PLCs over ISO-on-TCP (port 102) built on [go-s7comm](https://github.com/otfabric/go-s7comm). Pure Go. Profile: **cli**.

## Organisation standards

This repository follows **otfabric/engineering policy 1.1**, profile **cli** (`.otfabric/policy.yaml`, mode `enforce`). Where this file and the organisation standards disagree, this file wins here — say so in your report.

## Where the truth lives

| Question | Source |
|---|---|
| Exit codes (script-facing API) | docs/exitcodes.md |
| JSON output contract | docs/json-output.md |
| Commands and flags | README.md |
| Worked examples | EXAMPLES.md |
| Protocol notes | PROTOCOL.md |
| Security guidance | SECURITY.md |
| Release history | RELEASE.md (`## vX.Y.Z` … `---`) |

## Hard rules

1. Exit codes are a public contract: changing a code is a breaking change with a RELEASE.md note.
2. Writes require `--dangerous`. Read-only is the default.
3. Never lower the CI `coverage-min`, add lint suppressions, or skip or delete tests to make CI pass. Coverage is above the CLI floor of 85 % and CI enforces 90 %; raise it, never lower it.
4. go-s7comm is pinned to a released tag; no `replace` directives.

## Commands

| When you change… | Run |
|---|---|
| anything | `make check` |
| the command line, exit codes or `main` | `make e2e` (builds the binary and runs it) |
| a single test run | `make test` |

## Definition of Done

`make check` green; tests for new behaviour; README.md updated for new flags; RELEASE.md section for user-visible changes.

## Git and releases

Branches and PRs. Do not push to `main`, tag or run the Release workflow unless the maintainer asks in the session.
