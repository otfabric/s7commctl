# s7commctl

[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/otfabric/s7commctl.svg)](https://pkg.go.dev/github.com/otfabric/s7commctl)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/otfabric/s7commctl/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/s7commctl/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/otfabric/s7commctl/graph/badge.svg)](https://codecov.io/gh/otfabric/s7commctl)
[![Release](https://img.shields.io/github/v/release/otfabric/s7commctl?label=release)](https://github.com/otfabric/s7commctl/releases)

A professional Siemens S7 communication CLI for ISO-on-TCP (port 102), implemented in pure Go.

## Table of contents

- [What it does](#what-it-does)
- [Requirements implementation status](#requirements-implementation-status)
- [Repository layout](#repository-layout)
- [Requirements](#requirements)
- [Build](#build)
- [Quality targets](#quality-targets)
- [Usage](#usage)
  - [Command tree](#command-tree)
  - [Shell completion](#shell-completion)
  - [Exit codes](#exit-codes)
- [Common examples](#common-examples)
- [Configuration via environment variables](#configuration-via-environment-variables)
- [Safety model](#safety-model)
- [Testing](#testing)
- [CI](#ci)
- [Output schema conventions (JSON)](#output-schema-conventions-json)
- [Notes and limitations](#notes-and-limitations)
- [License](#license)

## What it does

s7commctl provides a practical operator and engineering toolset for:

- Discovery of reachable S7 devices on a subnet
- Device identification using SZL queries
- Memory reads and writes (with safety controls)
- Block listing, block metadata, and block upload
- Diagnostics (CPU state, protection level, diagnostic buffer)
- Fingerprinting of negotiated connection capabilities

## Requirements implementation status

Based on REQUIREMENTS.md, current status is:

- Implemented
	- Discovery workflow with TCP/102 probing, rack/slot probing, negotiated PDU/TSAP reporting
	- Identify command using SZL-backed device metadata (CPU/order/module/firmware where exposed)
	- Read for DB, I, Q, M, T, C and extended areas (DATA_RECORD, S7200AO, IEC_COUNTER_200, IEC_TIMER_200)
	- Typed read/write support for BOOL, INT, DINT, REAL, STRING
	- Write safety model: explicit `--dangerous`, confirmation prompt, optional `--force`
	- Block management: list, DB listing, block info, block upload (OB/DB/FB/FC/SFB/SFC support path)
	- Diagnostics: CPU state, protection level, diagnostic buffer
	- Fingerprinting: CPU generation heuristic, PDU, AMQ calling/called, TSAP/rack/slot detection
	- TSAP strategy: manual TSAP, auto rack/slot, brute rack/slot probing
	- Safety controls: public exposure warning, rate limit option, configurable timeout
	- Output modes: human/table, JSON, quiet, hex dump
	- Advanced commands: sniff, pcap-decode, replay scaffold, periodic record
- Partial
	- Discovery CPU type extraction is not guaranteed on every target/model


Protocol stack:

- TCP (port 102)
- TPKT (RFC1006)
- COTP (ISO 8073)
- S7comm

For an introduction to S7comm concepts and how the CLI maps onto them, see **[PROTOCOL.md](PROTOCOL.md)**. For read-only command examples against a single host, see **[EXAMPLES.md](EXAMPLES.md)**.

## Repository layout

- cmd/s7commctl: Cobra CLI commands
- internal/app/config: CLI flag/env configuration
- internal/app/validate: input validation and typed parsing
- internal/app/output: renderers for json/table/text/hex output
- main.go: CLI entrypoint
- go.mod: tool module with dependency on `otfabric/go-s7comm`

Library code is consumed from the separate `otfabric/go-s7comm` module.

## Requirements

- Go 1.22+
- Network connectivity to S7 targets on TCP/102

The project uses `replace github.com/otfabric/go-s7comm => ../s7comm` so the CLI builds with Go 1.22 against the local `s7comm` repo (which must have `go 1.22` in its `go.mod`). For CI, the workflow checks out `otfabric/go-s7comm` so the replace resolves.

## Build

```bash
make build
```

Binary output:

- bin/s7commctl

## Quality targets

The project includes a full quality gate target:

```bash
make quality
```

This runs:

- formatting (`fmt`), then staticcheck and golangci-lint
- vet
- tests
- coverage

Quality targets execute for this module (`.`).

Other useful targets:

```bash
make fmt
make test
make vet
make lint
make build
```

## Usage

```bash
s7commctl --help
```

### Command tree

```text
s7commctl
├── discover
├── identify
├── read
├── write
├── blocks
├── list
│   ├── blocks
│   ├── db
│   └── areas
├── block
│   ├── info
│   └── upload
├── diagnostic
├── fingerprint
├── probe
│   └── rackslot
├── scan
│   └── read-ranges
├── compare
│   └── read
├── profile
├── record
├── sniff
├── pcap-decode
├── replay
├── completion
└── version
```

### Shell completion

Generate tab-completion for bash, zsh, or fish. Enum flags (e.g. `--area`, `--format`, `--type`, `--confirm`, `--profile`) complete with their allowed values.

```bash
# Bash
source <(s7commctl completion bash)

# Zsh
source <(s7commctl completion zsh)

# Fish
s7commctl completion fish | source
```

To install permanently (e.g. Bash): `s7commctl completion bash > /etc/bash_completion.d/s7commctl` (or `~/.local/share/bash-completion/completions/s7commctl`).

### Exit codes

Commands use the following exit code conventions (suitable for scripting):

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Target/operation non-success (e.g. probe found no valid rack/slot, read returned short-read with `--strict-read`, list/diagnostic failed) |
| 2 | Usage or validation error (invalid flags, missing required args, malformed input) |
| 3 | Connection or runtime failure (TCP connect failed, timeout, protocol error) |

Note: Cobra is configured so that usage/help is not printed on runtime failures (only on validation errors).

## Common examples

### 1. Discover devices

```bash
s7commctl discover --cidr 192.168.1.0/24 --format json
```

Advanced rack/slot probing:

```bash
s7commctl discover --cidr 192.168.1.0/24 --rack-min 0 --rack-max 3 --slot-min 0 --slot-max 5
```

For script-friendly output (one IP per line), use `--quiet` or `--format quiet`; both are supported intentionally.

### 2. Determine valid rack/slot values

Probe a target for accepted rack/slot combinations (setup accepted = valid):

```bash
s7commctl probe rackslot --ip 192.168.1.10 --format table
```

Strictly confirm combinations using a benign follow-up query (only `valid-query` counts as valid):

```bash
s7commctl probe rackslot --ip 192.168.1.10 --strict
```

Use a specific confirmation strategy:

```bash
s7commctl probe rackslot --ip 192.168.1.10 --confirm szl
s7commctl probe rackslot --ip 192.168.1.10 --confirm cpu-state
s7commctl probe rackslot --ip 192.168.1.10 --confirm any
```

Retry inconclusive candidates:

```bash
s7commctl probe rackslot --ip 192.168.1.10 --strict --retries 3 --retry-delay-ms 200
```

Stop on first confirmed valid (strict mode):

```bash
s7commctl probe rackslot --ip 192.168.1.10 --strict --first-confirmed
```

Custom rack/slot range and JSON output:

```bash
s7commctl probe rackslot --ip 192.168.1.10 --rack-min 0 --rack-max 3 --slot-min 0 --slot-max 7 --format json
```

Use `--summary-only` to print only counts, one recommended connection tuple, and interpretation (no table and no full list of combinations). With many candidates, default table output collapses to summary + examples + one-line total (use `--show-all` for the full list). Interpretation uses “Suggested default tuple” when rack/slot is not discriminated. In strict mode, status is shown as **setup-ok** (setup accepted, no query) or **query-confirmed** (setup + follow-up OK). Use `--repeat N` and `--interval-ms` to run the probe N times and see variance (e.g. “Setup accepted: 255–256 across 3 runs” and “Results varied across runs; endpoint may be rate-sensitive or unstable”). Use `--no-exposure-warning` (or env `S7COMMCTL_NO_EXPOSURE_WARNING`) to suppress the public routable IP warning. Probe JSON uses the same target/request/result envelope as other commands, with snake_case keys in result (`setup_accepted`, `confirmed_by_query`, `candidates`, `valid`, etc.).

- **valid-connect**: setup communication succeeded; follow-up query not attempted or failed.
- **valid-query**: setup succeeded and a benign follow-up query (SZL, CPU state, etc.) also succeeded.

Exit code: 0 if at least one valid rack/slot found; 1 if none; &gt;1 for usage or runtime errors.

### 3. Scan readable ranges

Discover which address ranges in an area return readable data (read-only):

```bash
s7commctl scan read-ranges --ip 192.168.1.10 --area I --start 0 --end 256 --probe-size 8
s7commctl scan read-ranges --ip 192.168.1.10 --area DB --db 1 --start 0 --end 1024 --probe-size 32 --format json
```

Output: consolidated readable/empty/failed/inconclusive spans and optional per-probe table.

### 4. Compare read across rack/slot candidates

Run the same read on multiple rack/slot pairs and see if results are identical (rack/slot-insensitive):

```bash
s7commctl compare read --ip 192.168.1.10 --area DB --db 1 --offset 0 --size 32 --candidates 0/0,0/1,0/2
```

Table output includes a **SameAsBaseline** column (baseline/yes/no). Use `--repeat` and `--interval-ms` for multiple runs when data may change (e.g. Q area).

### 5. Profile (capabilities summary)

Summarize setup support, rack/slot, identify, and block list in one run:

```bash
s7commctl profile --ip 192.168.1.10
s7commctl profile --ip 192.168.1.10 --format json
```

### 6. Identify a PLC

```bash
s7commctl identify --ip 192.168.1.10 --rack 0 --slot 1 --format json
```

### 7. Read memory

Read 16 bytes from DB1 offset 0:

```bash
s7commctl read --ip 192.168.1.10 --area DB --db 1 --offset 0 --size 16 --format hex
```

Output includes result status (success, short-read, empty-read, rejected, etc.), requested/returned length, and warnings. Use `--strict-read` to fail the command when the result is not success.

Read markers and decode as INT values (with `--format json`, the result includes a `decoded` array of typed values):

```bash
s7commctl read --ip 192.168.1.10 --area M --offset 100 --size 8 --type int
```

### 8. Write memory safely

Write requires explicit dangerous mode. Without force, a confirmation prompt is shown.

```bash
s7commctl write --ip 192.168.1.10 --area DB --db 1 --offset 0 --type int --value 123 --dangerous
```

Non-interactive (automation):

```bash
s7commctl write --ip 192.168.1.10 --area DB --db 1 --offset 0 --type int --value 123 --dangerous --force
```

### 9. List blocks

```bash
s7commctl list blocks --ip 192.168.1.10 --format table
s7commctl list db --ip 192.168.1.10 --format json
```

The top-level `blocks` command is an alias for `list blocks` (same behavior).

With `--format json`, list blocks and diagnostic print a structured error envelope (`target`, `request`, `error` with `code` and `message`) to stdout on failure.

### 10. Block metadata and upload

```bash
s7commctl block info --ip 192.168.1.10 --type DB --number 1 --format json
s7commctl block upload --ip 192.168.1.10 --type DB --number 1 --format json
```

### 11. Diagnostics

```bash
s7commctl diagnostic --ip 192.168.1.10 --format table
```

Includes:

- CPU state
- protection level
- diagnostic buffer

### 12. Fingerprint

```bash
s7commctl fingerprint --ip 192.168.1.10 --format json
```

### 13. Record periodic reads

Record DB snapshots every second to JSONL:

```bash
s7commctl record --ip 192.168.1.10 --area DB --db 1 --offset 0 --size 8 --every-ms 1000 --count 60 --output samples.jsonl
```

Hex-line output mode:

```bash
s7commctl record --ip 192.168.1.10 --area M --offset 100 --size 4 --format hex --output samples.hex
```

### 14. Passive sniff decode

Listen for raw TPKT streams on a local TCP socket and decode frames (single-client mode: one connection at a time).

```bash
s7commctl sniff --listen :15102 --format jsonl
```

### 15. Offline pcap decode

Best-effort decode of S7-over-TCP frames from a pcap capture (no full TCP stream reassembly):

```bash
s7commctl pcap-decode --file capture.pcap --port 102 --format text
```

### 16. Replay server scaffold

Run a deterministic replay endpoint with scripted response frames (hex TPKT, one per line). Single-client mode: one TCP connection; one response per scripted line until the connection closes.

```bash
s7commctl replay --listen :1102 --script replay.hex --format jsonl
```

## Configuration via environment variables

Most command flags support `S7COMMCTL_*` environment variables, for example:

- S7COMMCTL_IP
- S7COMMCTL_PORT
- S7COMMCTL_RACK
- S7COMMCTL_SLOT
- S7COMMCTL_TIMEOUT
- S7COMMCTL_AUTO_RACK_SLOT
- S7COMMCTL_LOCAL_TSAP
- S7COMMCTL_REMOTE_TSAP

Example:

```bash
export S7COMMCTL_IP=192.168.1.10
export S7COMMCTL_RACK=0
export S7COMMCTL_SLOT=1
s7commctl identify
```

## Safety model

- Read operations are default-safe
- Write operations require `--dangerous`
- Interactive confirmation is required unless `--force` is set

Always validate addressing and scope in a non-production or controlled maintenance window before performing writes.

## Testing

Run all tests:

```bash
go test ./...
```

Current tests focus on:

- command wiring/smoke coverage
- config/env flag loading
- input parsing and validation
- output formatting
- write safety confirmation helpers
- pcap/replay/frame parsing helpers

Note: low-level protocol codec coverage is primarily maintained in the `otfabric/go-s7comm` library module.

## CI

GitHub Actions workflow is provided at `.github/workflows/ci.yml` and runs:

- `make quality`
- `make build-all`

## Output schema conventions (JSON)

Commands that operate on a single target use a consistent envelope when `--format json` is set:

- **target**: `ip`, `port` (connection endpoint only).
- **request**: includes `connection_mode` (`rack_slot` | `auto_rack_slot` | `explicit_tsap`) plus mode-specific fields (`rack`/`slot` or `local_tsap`/`remote_tsap`) and command-specific params (e.g. `area`, `offset`, `type`, `number`). For **read**, request has `connection` (when present) and `address` (area, db, offset, size). For **compare read**, request holds the read params and candidates.
- **result**: operation outcome (device info, fingerprint, blocks, read data, etc.). Effective/detected rack/slot appear in result when relevant (e.g. fingerprint).

Error output uses the same envelope with an **error** object (`code`, `message`) instead of result. **Discover** intentionally returns a **raw array** of discovered devices (one object per device); it is a multi-result query, not a single-target envelope. List blocks returns an array of blocks; single-target commands use the envelope above. **Write** request may include `value_truncated: true` when the value is long; the `value` field is then a 128-character preview.

## Notes and limitations

- Some PLC families expose different SZL field layouts; identify/fingerprint values can vary by model/firmware.
- Block upload behavior is dependent on PLC capabilities and protection levels.
- Replay is a scaffold framework for deterministic test workflows and is not a full PLC simulator.

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).

For contributing and security reporting, see [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
