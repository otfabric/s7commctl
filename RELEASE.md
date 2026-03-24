# s7commctl Releases

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
