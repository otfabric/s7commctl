# s7commctl Releases

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
