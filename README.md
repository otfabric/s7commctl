# s7commctl

A professional Siemens S7 communication CLI for ISO-on-TCP (port 102), implemented in pure Go.

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
	- Read for DB, I, Q, M, T, C areas
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
- Pending
	- MCAP output for record command (current outputs: jsonl, hex)

Protocol stack:

- TCP (port 102)
- TPKT (RFC1006)
- COTP (ISO 8073)
- S7comm

## Repository layout

- cmd/s7commctl: Cobra CLI commands
- internal/app/config: CLI flag/env configuration
- internal/app/validate: input validation and typed parsing
- internal/app/output: renderers for json/table/text/hex output
- main.go: CLI entrypoint
- go.mod: tool module with dependency on `otfabric/s7comm`

Library code is consumed from the separate `otfabric/s7comm` module.

## Requirements

- Go 1.25+
- Network connectivity to S7 targets on TCP/102

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

- formatting check (`fmt-check`)
- lint (`golangci-lint` if available, fallback to `go vet`)
- vet
- tests

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
├── record
├── sniff
├── pcap-decode
├── replay
└── version
```

## Common examples

### 1. Discover devices

```bash
s7commctl discover --cidr 192.168.1.0/24 --format json
```

Advanced rack/slot probing:

```bash
s7commctl discover --cidr 192.168.1.0/24 --rack-min 0 --rack-max 3 --slot-min 0 --slot-max 5
```

### 2. Identify a PLC

```bash
s7commctl identify --ip 192.168.1.10 --rack 0 --slot 1 --format json
```

### 3. Read memory

Read 16 bytes from DB1 offset 0:

```bash
s7commctl read --ip 192.168.1.10 --area DB --db 1 --offset 0 --size 16 --format hex
```

Read merkers and decode as INT values:

```bash
s7commctl read --ip 192.168.1.10 --area M --offset 100 --size 8 --type int
```

### 4. Write memory safely

Write requires explicit dangerous mode. Without force, a confirmation prompt is shown.

```bash
s7commctl write --ip 192.168.1.10 --area DB --db 1 --offset 0 --type int --value 123 --dangerous
```

Non-interactive (automation):

```bash
s7commctl write --ip 192.168.1.10 --area DB --db 1 --offset 0 --type int --value 123 --dangerous --force
```

### 5. List blocks

```bash
s7commctl list blocks --ip 192.168.1.10 --format table
s7commctl list db --ip 192.168.1.10 --format json
```

### 6. Block metadata and upload

```bash
s7commctl block info --ip 192.168.1.10 --type DB --number 1 --format json
s7commctl block upload --ip 192.168.1.10 --type DB --number 1 --format json
```

### 7. Diagnostics

```bash
s7commctl diagnostic --ip 192.168.1.10 --format table
```

Includes:

- CPU state
- protection level
- diagnostic buffer

### 8. Fingerprint

```bash
s7commctl fingerprint --ip 192.168.1.10 --format json
```

### 9. Record periodic reads

Record DB snapshots every second to JSONL:

```bash
s7commctl record --ip 192.168.1.10 --area DB --db 1 --offset 0 --size 8 --every-ms 1000 --count 60 --output samples.jsonl
```

Hex-line output mode:

```bash
s7commctl record --ip 192.168.1.10 --area M --offset 100 --size 4 --format hex --output samples.hex
```

### 10. Passive sniff decode

Listen for raw TPKT streams on a local TCP socket and decode frames:

```bash
s7commctl sniff --listen :15102 --format jsonl
```

### 11. Offline pcap decode

Decode S7-over-TCP frames from a pcap capture:

```bash
s7commctl pcap-decode --file capture.pcap --port 102 --format text
```

### 12. Replay server scaffold

Run a deterministic replay endpoint with scripted response frames (hex TPKT, one per line):

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

Note: low-level protocol codec coverage is primarily maintained in the `otfabric/s7comm` library module.

## CI

GitHub Actions workflow is provided at `.github/workflows/ci.yml` and runs:

- `make quality`
- `make build-all`

## Notes and limitations

- Some PLC families expose different SZL field layouts; identify/fingerprint values can vary by model/firmware.
- Block upload behavior is dependent on PLC capabilities and protection levels.
- Replay is a scaffold framework for deterministic test workflows and is not a full PLC simulator.
- MCAP serializer integration remains pending.

## License

See repository license terms.
