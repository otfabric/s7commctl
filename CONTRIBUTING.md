# Contributing to otfabric/s7commctl

Thank you for your interest in contributing to s7commctl. This document explains how to get started.

## Development setup

- **Go**: 1.25 or later.
- **Network**: Optional; required only for live tests against S7 targets.

```sh
git clone https://github.com/otfabric/s7commctl.git
cd s7commctl
go mod download
```

## Running tests

- **Unit tests**: `make test` (runs tests with race detector).
- **Coverage**: `make coverage` to generate `coverage.out`; `make cover` to view the report in the browser.
- **Quick test** (no race): `go test ./...`

By default, tests do not require a live PLC; they cover CLI wiring, config, validation, and output formatting.

## Code style and linting

- **Format**: `make fmt` (gofmt; gofumpt if available).
- **Lint**: `make lint` (staticcheck) and `make lint-ci` (golangci-lint). Install with:
  - `go install honnef.co/go/tools/cmd/staticcheck@latest`
  - `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **Vet**: `make vet` (go vet).

Please run `make fmt` and `make quality` (or at least `make check`) before submitting a PR.

## Submitting changes

1. Open an issue or pick an existing one to discuss the change.
2. Fork the repo, create a branch, and make your changes.
3. Add or update tests as needed.
4. Run `make quality` (format, lint, vet, test).
5. Open a pull request with a clear description and reference to the issue.

## Project context


- **Protocol background**: [PROTOCOL.md](PROTOCOL.md) introduces S7comm concepts and how the CLI maps onto them.
- **Library**: The protocol and client logic live in the separate [otfabric/go-s7comm](https://github.com/otfabric/go-s7comm) module; this repo is the CLI and consumes it via `go.mod`.

## Error handling and output

- Prefer `fmt.Errorf("...: %w", err)` so callers can use `errors.Is` / `errors.As`.
- Runtime failures (e.g. connect error, short-read) should not trigger Cobra usage output; keep `SilenceUsage: true` on the root command.
