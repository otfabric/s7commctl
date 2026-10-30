# s7commctl Releases

## v0.3.0

**Date:** 2026-10-30
**Previous release:** v0.2.9

### Summary

Exit codes converge on the organisation table (ADR-0006). **Scripts that test for exit 3 must change.**

| Case | Old | New |
|---|---|---|
| Probe found nothing, strict short read | 1 | 1 |
| Write aborted at the prompt | 1 | 2 |
| Unknown flag or command | 1 | 2 |
| Connection failure | 3 | 4 |
| Timeout | 3 | 5 |
| Operation failed on the PLC, protocol error | 3 | 6 |
| Request larger than the PDU | 3 | 3 |
| Permission refused by the operating system | 3 | 9 |
| Output file cannot be opened | 3 | 10 |
| Interrupted | killed by the signal | 130 |

### Changed

- SIGINT and SIGTERM cancel every command through a root context. `record` stops cleanly.

### Added

- `S7COMMCTL_OUTPUT_FORMAT` is accepted besides `S7COMMCTL_FORMAT`, the canonical name, which wins.

### Fixed

- `discover`: `--rack-min`, `--rack-max`, `--slot-min`, `--slot-max` and their environment variables had no effect, because the profile always overwrote them. A bound set explicitly now overrides `--profile`. The defaults shown in `--help` are now those of the `standard` profile, the range actually scanned. An empty or unaddressable range exits 2.
- `discover --quiet` prints nothing when nothing is found, instead of `No S7 devices found`.
- `fingerprint`: the CPU generation is read from the model number, so `CPU 315-2` gives 300 instead of 0, and order numbers such as `6ES7 315-...` count.
- `diagnostic` honours `--format JSON` in capitals on success.
- `sniff` and `replay` stop on SIGINT and SIGTERM with exit 130, while waiting for a client and while reading.
- `pcap-decode`: little-endian captures no longer fail with `unsupported pcap linktype 16777216`, because the byte-order cases were swapped.
- `pcap-decode`: packet lengths above 262144 are refused, instead of sizing an allocation of up to 4 GiB.
- `pcap-decode`: each TCP direction has its own reassembly buffer, so an interleaved response no longer corrupts a split frame.
- `pcap-decode`: a TPKT frame is checked for its version before its length, so noise cannot stall a direction.

---

## v0.2.9

**Date:** 2026-10-19  
**Previous release:** v0.2.8

### Summary

Dependency refresh: the S7 client library moves to **`go-s7comm` v0.7.5**. The README now matches `go.mod` and documents the connection-mode flags.

### Highlights

- **Dependency bump:** **`github.com/otfabric/go-s7comm`** `v0.7.3` → **`v0.7.5`**, which pulls **`go-cotp` v1.0.4** and **`go-tpkt` v1.0.1** (indirect). go-s7comm reports no S7 protocol or result-model changes in either release.
- **README:** requirements say Go 1.25+ as in `go.mod`, the stale local `replace` note is gone, and `--auto-rack-slot`, `--brute-rack-slot`, `--local-tsap` and `--remote-tsap` are documented with their precedence and environment variables.

### Toolchain

- **Go:** `go 1.25` (see `go.mod`).

### Upgrade notes

- None. Commands, flags, output and exit codes are unchanged.

---

## v0.2.8

**Date:** 2026-07-21  
**Previous release:** v0.2.7

### Summary

Open-source license hygiene and README badge standardization for public MIT release.

### Highlights

- **License:** Root **`LICENSE`** is the standard MIT text with **`Copyright (c) 2026 OT Fabric`** (replaces `LICENSE.txt`). First-party Go sources carry **`// SPDX-License-Identifier: MIT`**. README license section points at `./LICENSE`.
- **Badges:** README badge block normalized (Go version from `go.mod`, pkg.go.dev, MIT, CI, Codecov, latest release).

### Toolchain

- **Go:** `go 1.25` (see `go.mod`).

### Upgrade notes

- None for CLI users. Forks/docs that linked **`LICENSE.txt`** should use **`LICENSE`**.

---

## v0.2.7

**Date:** 2026-07-21  
**Previous release:** v0.2.6

### Summary

This release upgrades the S7 client library to **`go-s7comm` v0.7.3** and stops depending directly on **`go-cotp`** / **`go-tpkt`**. Offline frame tooling (`sniff`, `replay`, `pcap-decode`) now peels TPKT locally and inspects COTP TPDUs via `wire.InspectTPDU`.

### Highlights

- **Dependency bump:** Track **`github.com/otfabric/go-s7comm` v0.7.3**. `go-cotp` and `go-tpkt` remain available only as **indirect** dependencies through go-s7comm.
- **Frame inspection:** Replace `wire.InspectFrame` with local TPKT peel + `wire.InspectTPDU`. Text frame output reports **`tpdu=`** (TPDU length) instead of **`tpkt=`**.
- **Standalone builds:** Makefile sets **`GOWORK=off`** so `make check` / builds work when a parent-directory `go.work` does not include this module.

### Toolchain

- **Go:** `go 1.25` (see `go.mod`).

### Upgrade notes

- After pulling v0.2.7, run **`go mod tidy`**.
- Scripts that parse text frame lines should expect **`tpdu=`** instead of **`tpkt=`**.

---

## v0.2.6

**Date:** 2026-03-24  
**Previous release:** v0.2.5

### Summary

This release switches the S7 client library to the renamed module **`github.com/otfabric/go-s7comm`**, ships **Windows** binaries alongside Linux and macOS, and embeds **version metadata at link time** (no `version.txt`). Prebuilt archives follow the same naming pattern as before, with `.zip` files for Windows.

### Highlights

- **Dependency rename:** All imports and `go.mod` now use **`github.com/otfabric/go-s7comm`** instead of **`github.com/otfabric/s7comm`**. The API is unchanged for consumers of the CLI binary; anyone building or forking the repo should run `go mod tidy` and update any local `replace` directives to point at the new module path. This release tracks **`go-s7comm` v0.6.3**.
- **Windows artifacts:** GitHub releases include Windows ZIPs (for example `s7commctl-0.2.6-windows-amd64.zip` and `s7commctl-0.2.6-windows-arm64.zip`), each containing the corresponding `s7commctl-0.2.6-windows-*.exe`, in addition to the existing Linux and Darwin `.tar.gz` bundles.
- **Version command / builds:** Binaries are built with **`-s -w`** and **`-X main.version=… -X main.tag=… -X main.commit=… -X main.buildDate=…`**, matching the shared release workflow. `s7commctl version` reports **version**, **commit**, and **build date**, and includes the **git tag** when the build is exactly on a tag. **`version.txt` is removed**; local **`Makefile`** derives **`VERSION`** / **`TAG`** from git for `make build`, `make run`, and `make release-all`.

### Toolchain

- **Go:** `go 1.25` (see `go.mod`).

### Upgrade notes

- After pulling v0.2.6, run **`go mod tidy`** so indirect dependencies resolve against **`go-s7comm`**.
- If you had a `replace` for `github.com/otfabric/s7comm`, retarget it to **`github.com/otfabric/go-s7comm`** (or drop it if you only consume released versions).

---
