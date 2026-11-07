# s7commctl Releases

## v0.5.0

**Date:** 2026-11-07
**Previous release:** v0.4.1

### Summary

Built on go-s7comm v0.12.0 (from v0.10.2). go-s7comm v0.11.0 fixed requests that real CPUs could not answer: until now `identify`, `fingerprint`, `diagnostic`, `profile`, `list blocks`, `blocks`, `block info` and `block upload` sent malformed or wrong user data and upload requests to a real S7-300/S7-400, and decoded SZL answers at wrong offsets. go-s7comm now sends, byte for byte, the requests of Snap7 and TIA Portal V13, and decodes the recorded answers of a CPU 315-2 PN/DP and an IM151-8 PN/DP CPU correctly ([go-s7comm testdata/captures](https://github.com/otfabric/go-s7comm/blob/main/testdata/captures/README.md)). No flag or exit code changes.

### Fixed (through go-s7comm)

- `identify`, `fingerprint`, `profile`: the SZL request lacked its method byte; fields are now read from SZL 0x0011/0x001C by record index. `module_name` is the module's name (e.g. `PLC_1`), `cpu_type` its module type (e.g. `CPU 315-2 PN/DP`), `fw_version` `Vx.y.z`.
- `diagnostic`: CPU state from the SZL 0x0424 mode, protection level from SZL 0x0232 index 4 (Siemens level 1 none, 2 write protected, 3 read/write protected), diagnostic entries from the 20-byte records: `event_class` is the event ID's high nibble, `priority` the priority class, `timestamp` the BCD time stamp.
- `list blocks`, `blocks`, `block info`: block functions instead of SZL 0x0111/0x0113; long lists are collected over several packets; block info carries the block number the CPU reports.
- `block upload`: the CPU's file name and 4-byte upload ID; segment lengths in bytes.

### Added

- `identify`: `module_type`, `system_name`, `plant_id`, `copyright` and `memory_card_serial` in JSON (string or null) and in the table. The one-line `text` format is unchanged.

### Changed

- Output values of `identify`, `diagnostic` and the block commands against a real CPU change, as they are now decoded correctly (see Fixed). Scripts that matched the earlier, wrongly decoded values need updating.

### Tests

- The test PLC answers in the format real CPUs use (user data responses, SZL records, block functions, upload IDs), taken from go-s7comm's recorded captures.

---

## v0.4.1

**Date:** 2026-11-07
**Previous release:** v0.4.0

### Summary

Built on go-s7comm v0.10.2 (from v0.9.0). No command, flag, output or exit code changes.

### Changed

- go-s7comm v0.9.0 → v0.10.2. The client now follows the Siemens length units for Read Var answers with data transport size DINTEGER, REAL or OCTET STRING (bytes, not bits) and treats an answer with a transport size that is not a data transport size as a protocol error. s7commctl's reads use BYTE and BIT only, so its results do not change. go-s7comm also gained an in-memory test server (package `server`), which s7commctl does not use.
- README: the go-s7comm version it names was out of date (v0.7.5).

---

## v0.4.0

**Date:** 2026-11-03
**Previous release:** v0.3.2

### Summary

Built on go-s7comm v0.9.0, which refuses addresses the S7ANY format cannot carry. **Before, `write --db 65537` wrote to DB 1** and an offset past byte 2097151 wrapped the same way; both are now refused before anything is sent. Go 1.26 is the minimum.

### Changed

- **Exit code:** an address or option go-s7comm refuses before sending (negative offset, DB above 65535, a range past the S7ANY limit, `scan --retries -1`, an inverted `probe` rack range) exits 2 (usage) instead of 6 (protocol). See docs/exitcodes.md.
- **`record`:** the output file is created with mode 0600 instead of 0644; recordings hold process values.
- **Go:** `go 1.26` (see `go.mod`); CI tests Go 1.26 and 1.27. `go fix` applied.

### Dependencies

- `go-s7comm` → v0.9.0, `go-cotp` → v1.2.0, `go-tpkt` → v1.1.0.

### Tests

- Read and write of a DB above 65535, an offset past the S7ANY range and a range ending past it exit 2 and send no write.
- Fuzz targets for TPKT frames read from a peer (`sniff`, `replay`) and for replay scripts.
- SAST (gosec, opengrep) at low severity and low confidence: 2 findings, 0 open (file mode and path of `record`, `replay`; `pcap-decode` paths cleaned the same way).

---

## v0.3.2

**Date:** 2026-10-31
**Previous release:** v0.3.1

### Changed

- Policy 1.2: build output in `bin/` and `release/`, a per-package coverage floor, and more tests and CI. No change in behaviour.
- Built on go-s7comm v0.8.1, go-cotp v1.1.1, go-tpkt v1.0.2.

---

## v0.3.1

**Date:** 2026-10-30
**Previous release:** v0.3.0

### Dependencies

- `go-s7comm` → v0.8.0: a short module ID reply no longer panics, and timeouts now cover S7 setup, so a silent device cannot block `connect`, `discover` or the rack and slot probe.

---

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
