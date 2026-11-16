# s7commctl Releases

## v0.12.0

**Date:** 2026-11-16
**Previous release:** v0.11.0

`--request-timeout` sets the bound for each request apart from the bound for connecting. A run without it behaves as in v0.11.0.

### Added

- `--request-timeout` (milliseconds, `S7COMMCTL_REQUEST_TIMEOUT`) on `identify`, `read`, `write`, `blocks`, `list blocks`, `list db`, `block info`, `block upload`, `diagnostic`, `fingerprint`, `scan read-ranges`, `record` and `profile`: the bound for each request after connecting (sending one request and receiving its answer), through go-s7comm's `WithRequestTimeout`. `--timeout` keeps bounding connecting.
  - Not set (the default): each request is bounded by `--timeout`, as in v0.11.0, and by nothing when `--timeout` is 0 or negative.
  - `0`: no bound per request, while `--timeout` still bounds connecting. `--timeout 2000 --request-timeout 0` gives a connecting bound other than 5 s without a bound per request, which v0.11.0 had no setting for.
  - Above 0: that bound for each request, whatever `--timeout` is, so a PLC that is slow to answer can get a longer bound than connecting.
  - Negative: a usage error before connecting, exit 2 with `request-timeout must be >= 0 (0: no bound per request), got N`. A value above 9223372036854 (the largest number of milliseconds a duration holds) is refused the same way.
  - A request that ends at `--request-timeout` gives the exit code and the error of one that ends at `--timeout`: exit 5 with `<operation> failed: …` ending in `context deadline exceeded`; a read reports status `timeout`, and `record` counts a failed sample.
  - `profile` keeps its bound of twice `--timeout` (0: 10 s) on the whole run, and that bound applies to its requests too: a larger `--request-timeout` does not lengthen the run.
  - `scan read-ranges` keeps bounding each probe read by `--timeout` (0: 5 s) as well; the earlier of the two ends a probe read.
  - `discover`, `probe rackslot` and `compare read` have no `--request-timeout`: they keep their own `--timeout`, per host, attempt or candidate.

### Changed

- The help text of `--timeout` names `--request-timeout` (docs/commands.md). README.md (Timeouts, environment variables), docs/exitcodes.md (codes 2 and 5) and SECURITY.md (Limits) describe the new flag. No exit code changes.

---

## v0.11.0

**Date:** 2026-11-16
**Previous release:** v0.10.2

No command or flag is added. `--timeout` bounds each request as well as connecting.

### Breaking

- `--timeout` (default 5000 ms, `S7COMMCTL_TIMEOUT`) bounds each request after connecting, not only connecting: sending one request and receiving its answer, through go-s7comm's `WithRequestTimeout`. It applies to `identify`, `read`, `write`, `blocks`, `list blocks`, `list db`, `block info`, `block upload`, `diagnostic`, `fingerprint`, `scan read-ranges`, `record` and to identify and the block list of `profile`. A command of several requests bounds each of them; the pauses of `--rate-limit-ms` are not requests.

  Who notices: a run against a PLC, or a path to it, that takes longer than `--timeout` to answer one request. Before, the command waited for the answer and reported its result; now it ends at the timeout, and the timeout ends the connection:
  - `identify`, `write`, `blocks`, `list blocks`, `list db`, `block info`, `block upload`, `diagnostic` while it reads the diagnostic buffer, and `fingerprint` during identify: exit 5 with `<operation> failed: …` ending in `context deadline exceeded`, the code and the error of a timeout while connecting. A `write` was sent before its answer was due, so the PLC may have carried it out.
  - `read`: status `timeout` (`"status": "timeout"` in JSON), exit 0, or 1 with `--strict-read`, as for any failed read.
  - `record`: the sample fails with `timeout`. Nothing reconnects, so the next samples fail with `transport-error`, and `record` stops after `--max-failures` consecutive failures (default 5) with exit 4, or with 5 when it stops on the timed-out sample (`--max-failures 1`).
  - A later request of `fingerprint` or `diagnostic` (protection level, CPU state, the read and block list probes of `fingerprint`), and identify or the block list of `profile`: reported as failed, with exit 0, as when the PLC refuses it (protection `unknown` with `protection_error`, `cpu_state` `UNKNOWN`, `Identify support: no`); the requests after it fail as well.
  - `scan read-ranges` bounded each probe read by `--timeout` before; it does not change. `discover`, `probe rackslot` and `compare read` keep their own `--timeout`, unchanged.

  To keep the old handling: `--timeout 0` (or `S7COMMCTL_TIMEOUT=0`) sets no bound per request, as before, and keeps 5 s for connecting, the bound that `--timeout 0` gave before; a negative value does the same, as before. Or a `--timeout` larger than the slowest answer, which also bounds connecting by that value. There is no setting for a connecting bound other than 5 s without a bound per request.

### Changed

- Built on go-s7comm v0.17.1 (was v0.17.0) (go-s7comm RELEASE.md, v0.17.1). s7commctl closes the client only after its calls have returned, so its output does not change.
- The help text of `--timeout` says what it bounds (docs/commands.md). README.md has a Timeouts section; docs/exitcodes.md lists the change; SECURITY.md lists the wait for an answer under Limits.

---

## v0.10.2

**Date:** 2026-11-16
**Previous release:** v0.10.1

No command or flag is added or changed.

### Changed

- Built on go-s7comm v0.17.0 (was v0.16.5) (go-s7comm RELEASE.md, v0.17.0). Its `### Breaking` entries concern a `Close` called while another call is in flight, and a call that waits for its turn behind another; s7commctl closes its client only after its calls have returned and makes one call at a time, so nothing changes in its output or exit status. The new `WithRequestTimeout` is not used yet.

---

## v0.10.1

**Date:** 2026-11-16
**Previous release:** v0.10.0

No command or flag is added or changed.

### Changed

- Built on go-s7comm v0.16.5 (was v0.16.4), go-cotp v1.4.2 (was v1.4.1) and go-tpkt v1.2.2 (was v1.2.1) (go-s7comm RELEASE.md, v0.16.5; go-cotp RELEASE.md, v1.4.2; go-tpkt RELEASE.md, v1.2.2). For the tool this means fewer allocations per COTP data segment and TPKT packet (a segment of at most 8192 octets and a packet of at most 8196 octets reuse the buffer of the previous one, and a TSDU of one segment is returned without a copy); nothing changes on the wire or in the output.
- CI: the documentation check runs docsync v0.4.0 (was v0.3.x).

---

## v0.10.0

**Date:** 2026-11-15
**Previous release:** v0.9.0

### Summary

Four behaviour changes that a script can see: `write --type bool` requires `--bit`; `write --type string` reads the STRING's declared maximum first and refuses a value that does not fit; `sniff` and `replay` listen on 127.0.0.1 by default; `record` counts failed samples, exits 1 when one failed and stops when the connection is lost. Each is listed under Breaking with what a script sees now and how to keep the old handling. JSON and JSONL output are unchanged.

### Breaking

- `write --type bool` without `--bit` is a usage error: exit 2 with `--type bool requires --bit 0-7, the bit of the byte at --offset to write; --type raw writes a whole byte`, before the prompt and before connecting, so nothing is sent. Before, it wrote the whole byte at `--offset` (`0x01` or `0x00`) with exit 0, and v0.9.0 printed a warning on stderr; that warning is gone. To keep the old handling: no flag restores it. `--type raw --value 01` (or `00`) writes the same whole byte; there is no `--type byte`, `--type raw` is the type that writes bytes.
- `write --type string` reads byte 0 of the STRING at `--offset`, the maximum length the PLC program declares, before it writes (one Read Var of one octet, after the prompt):
  - A value longer than that maximum: exit 2 with `the value has N octets and the STRING at DB1.DBB4 holds at most M (its byte 0); nothing was written`; no Write Var is sent. Before, the write went ahead and overwrote the bytes after the STRING.
  - A value longer than 254 octets: exit 2 with `the value has N octets; an S7 STRING holds at most 254`, before the prompt and before connecting. Before, it was cut to 254 octets with exit 0.
  - When byte 0 cannot be read, the write stops with `write failed: read the maximum length of the STRING at …` and the read's exit code (4 when the connection is lost, 5 on a timeout, 6 when the PLC refuses it); no Write Var is sent.
  - A value that fits is written as in v0.9.0: current length and characters from `--offset`+1, byte 0 unchanged, the same success line and JSON.

  To keep the old handling: no flag restores it. `--type raw --offset <offset+1> --value <length octet and characters in hex>` writes those octets without reading byte 0 and without a length check.
- `sniff` listens on `127.0.0.1:15102` and `replay` on `127.0.0.1:1102` by default (were `:15102` and `:1102`, every interface). The first stdout line names the new address (`sniff listening on 127.0.0.1:15102`), and a client on another host is refused. To keep the old handling: `--listen :15102` and `--listen :1102`, or `S7COMMCTL_SNIFF_LISTEN=:15102` and `S7COMMCTL_REPLAY_LISTEN=:1102`.
- `record` counts failed samples, those whose status is not `success`:
  - The summary reads `Recorded N samples (M failed) to <file>`, also when M is 0 (was `Recorded N samples to <file>`).
  - The command exits 1 when a sample failed, with `M of N samples failed; the last: <status> (<reason>)` on stderr (was 0).
  - Nothing reconnects. After `--max-failures` consecutive samples that failed with `transport-error` or `timeout` (default 5), `record` stops, prints the summary, says on stderr `record stopped: the connection to <ip> was lost (K consecutive samples failed; the last: …)` and exits 4 (`transport-error`) or 5 (`timeout`), the code of the last sample. Before, it went on writing failure lines until `--count` or an interrupt, and exited 0. A sample with another failed status (`rejected`, `short-read`, `empty-read`, `protocol-error`) counts as failed but not as a lost connection, and ends a run of consecutive ones.
  - An interrupt still ends `record` with 0 and no summary; the JSONL and hex lines are unchanged.

  To keep the old handling: `--max-failures 0` keeps `record` running after a lost connection. No flag restores exit 0 with failed samples or the old summary line; a script that accepted failed samples can treat exit 1 as a completed run (each line's `status` says which samples failed).

### Added

- `record --max-failures N` (`S7COMMCTL_MAX_FAILURES`): the number of consecutive samples failing with a transport error or timeout after which `record` stops; default 5, `0` never stops, a negative value is a usage error (exit 2) before connecting.

### Changed

- `write --help`: `--bit` is described as required with `--type bool`. `sniff --help` and `replay --help` name the loopback default and `--listen :PORT` for every interface.

### Documentation

- README: BOOL and STRING writes, `record` failures and `--max-failures`, the `sniff` and `replay` defaults, the safety model. SECURITY.md: writes, "Listeners" with the new defaults, "Limits" for STRING values and `record`. docs/exitcodes.md: the cases of this release. docs/commands.md: `record --max-failures`, `write --bit`, the `sniff` and `replay` defaults. docs/json-output.md: the STRING refusal. EXAMPLES.md: `record`, `sniff`, `replay`.

### Tests

- `write --type bool` without `--bit`, with `--force`, at the prompt and with an invalid value: exit 2, no prompt, no connection; `--type raw --value 01` writes the whole byte.
- `write --type string` against a simulated DB: boundary table of the declared maximum (0, 1, maximum − 1, maximum, maximum + 1, 253, 254, 255) with byte 0 unchanged and nothing written for a refused value; values of 255, 256 and 300 octets refused before the prompt and before connecting; byte 0 refused by the PLC, returned empty, and the connection closed on the read.
- `sniff` and `replay`: the default `--listen` is a loopback address, and `--help` says so.
- `record` against a fake PLC that closes the connection after one sample: the default stops after five failed samples with exit 4, `--max-failures` 1 and 3 stop earlier, 0 runs to `--count` with exit 1; refused samples are counted without stopping; `--max-failures -1` is a usage error before connecting. The status-to-exit-code mapping (4, 5, 6) has a table test.

---

## v0.9.0

**Date:** 2026-11-15
**Previous release:** v0.8.4

### Summary

`write` gains `--bit` to write one BOOL, and `write --type string` leaves a STRING's maximum length unchanged. `fingerprint` and `diagnostic` report a protection level they could not read as unknown; text output of device-supplied strings escapes non-printable characters; `pcap-decode` decodes a capture packet by packet and ends each packet's TCP data where its IPv4 datagram ends; an interrupt ends a `--repeat` pause or a replay delay at once. Nothing is removed; JSON gains keys and, where noted, a null value.

### Added

- `write --bit 0-7`: with `--type bool`, writes only that bit of the byte at `--offset` (go-s7comm `WriteBit`, bit address offset·8+bit). The success line names the bit address, for example `DB1.DBX4.3` or `Q0.3`. `--bit` with another type or outside 0-7 is a usage error (exit 2) and nothing is sent. JSON: `request.bit`, and `result.written` is 1.
- `write --type bool` without `--bit` prints a warning on stderr that it writes the whole byte (all eight bits) and names `--bit`. The write itself is unchanged.
- `discover` prints a warning on stderr when `--cidr` holds publicly routable addresses, also with `--quiet`, and scans as before; `--no-exposure-warning` (`S7COMMCTL_NO_EXPOSURE_WARNING`) suppresses it.
- JSON: `result.protection_error` in `fingerprint` and `diagnostic`, null when the protection level was read.

### Changed

- The `write` confirmation prompt names what is written and where: length, type and value, the address, area, DB, offset (and bit), IP and port, and the rack and slot (or the TSAPs, or automatic detection). Before, it said only that the operation writes to PLC memory. `--dangerous`, `--force` and the answer `yes` are unchanged.
- The `write` success line labels a byte address as a byte: `DB1.DBB4`, `MB100`, `IB3`, `QB4` (was `DB1.DBX4`, `M100`, `I3`, `Q4`).
- An interrupt (SIGINT or SIGTERM) during the pause between `compare read` or `probe rackslot` runs (`--repeat`, `--interval-ms`) or during `replay --delay-ms` ends the command at once with exit 130. A `compare read` run cut short by an interrupt is no result: exit 130, where it exited 0 and printed that run. docs/exitcodes.md lists the cases.
- `record` connects before it opens its `--output` file, so an existing file is not emptied when the connection cannot be made.
- `pcap-decode` decodes each packet as it reads it: memory follows the largest packet rather than the size of the capture, and the frames before a damaged packet are printed before the error. Taking a frame from a stream no longer copies the rest of the stream.
- `sniff --help` and `replay --help` say that the default address listens on every interface without authentication.

### Fixed

- `write --type string` writes the STRING's current length (byte 1) and its characters from `--offset`+1, and leaves byte 0, the maximum length the PLC program declares, unchanged; before, byte 0 was overwritten with the value's length. The success line names the STRING, and `result.written` is the value's length plus one.
- `fingerprint` and `diagnostic`: a protection level that could not be read is reported as unknown (`null` in `protection` and `protection_level`, the reason in `protection_error`; `unknown` in table and text output, and `fingerprint` also on stderr), and `fingerprint` reports `write_likely_allowed: false`. Before, it was reported as "No Protection".
- `pcap-decode` takes a packet's TCP payload only up to the IPv4 total length, so Ethernet padding and trailers are not read as stream data; IP fragments and packets whose total length does not fit the captured data are skipped. A total length of 0 (segmentation offload) uses the captured length.
- `identify`, `fingerprint` and `profile` print non-printable characters and invalid UTF-8 in device-supplied strings as `\xNN` in text, table and `--quiet` output, and table cells are cut between characters. JSON output is unchanged.
- `sniff` and `replay` exit 130 when interrupted right after the client connects, before the first read (it could be 4).

### Documentation

- README: `write --bit`, BOOL and STRING writes, the prompt, the `discover` warning, the `sniff` and `replay` listeners, how `pcap-decode` reads a capture, the safety model. Corrected: the go-s7comm version note, the CI description, which commands print the JSON `error` envelope, and a reference to a requirements document that is not in the repository.
- SECURITY.md: new "Listeners" and "Limits" sections; the exposure warnings, the write prompt, device-supplied text and the protection level.
- docs/commands.md: `write --bit`, `discover --no-exposure-warning`, the `sniff` and `replay` descriptions. docs/json-output.md: `write` `bit` and `written`, `protection` and `protection_level` may be null, `protection_error`. docs/exitcodes.md: the interrupt cases of this release.

### Tests

- `write`: `--bit` on DB, Q, M and I sends one BIT item at offset·8+bit and no byte write; invalid `--bit` sends nothing; bool without `--bit` warns; a STRING write against a simulated DB keeps byte 0 for short, empty and long values; the prompt names target, value and connection for each connection mode.
- `fingerprint`, `diagnostic`: the protection SZL refused, without a record, with a short record and with a level the library does not know (0, 4), in JSON, table and text; levels 1 to 3 as before.
- Device-supplied strings with control characters, C1 bytes and invalid UTF-8 through `identify`, `fingerprint` and `profile` in every text format; `output.Printable` and table cells.
- `pcap-decode`: payload bounded by the IPv4 total length (table of lengths), fragments, a padded packet inside a split frame, frames before a damaged packet, allocation budgets for a capture of 128 maximal packets and for 32768 frames in one segment, the pending stream data after every packet size; `FuzzParseEthernetIPv4TCP` checks the payload against the datagram.
- Interrupts during the `compare read` and `probe rackslot` interval, during a `compare read` run and during the `replay` delay, with `testing/synctest` for the wait itself; the `sniff` and `replay` cancel tests wait for the accepted connection instead of sleeping, and cover an interrupt before the first read.
- `discover` with a public and a private range, with and without `--no-exposure-warning`; `validate.CIDRHasPublicIP` against `IsPublicIP` address by address; `record` leaves an existing output file unchanged when it cannot connect.

---

## v0.8.4

**Date:** 2026-11-15
**Previous release:** v0.8.3

No command or flag is added or changed.

### Changed

- Built on go-s7comm v0.16.4 (was v0.16.3) (go-s7comm RELEASE.md, v0.16.4). For the tool this means:
  - A command whose request times out ends its session; a command that makes several requests reports that it is not connected for the ones after it, where each of them failed with a reference mismatch before.
  - An answer longer than the PDU size the tool proposed, a block list of more than 65,536 entries and an upload of more than 8 MiB end the command with a protocol error.
  - A write to a device that negotiated an odd PDU size is sent in chunks of even length.

---

## v0.8.3

**Date:** 2026-11-14
**Previous release:** v0.8.2

No change to the command.

### Changed

- Built on go-s7comm v0.16.3 (was v0.16.2), go-cotp v1.4.1 (was v1.4.0) and go-tpkt v1.2.1 (was v1.2.0). go-tpkt v1.2.1 refuses a write count from the underlying writer that is negative or larger than what was asked; go-cotp v1.4.1 reads each TPDU of an open connection up to the negotiated TPDU size (go-cotp and go-tpkt RELEASE.md).

---

## v0.8.2

**Date:** 2026-11-14
**Previous release:** v0.8.1

No change to the command.

### Changed

- Built on go-s7comm v0.16.2 (was v0.16.1), go-cotp v1.4.0 (was v1.3.0) and go-tpkt v1.2.0 (was v1.1.0). go-cotp v1.3.1 holds a TSDU being reassembled in one buffer bounded by `MaxTSDULength`; v1.4.0 refuses a DC, ER, AK, EA or RJ with octets after its header and encoder values that do not fit their field (go-cotp RELEASE.md, v1.3.1 and v1.4.0).

---

## v0.8.1

**Date:** 2026-11-14
**Previous release:** v0.8.0

No change to commands or output.

### Changed

- Built on go-s7comm v0.16.1 (was v0.16.0). The client splits an answer into its parameter and data sections with `int` arithmetic; behaviour is the same.

---

## v0.8.0

**Date:** 2026-11-14
**Previous release:** v0.7.2

### Changed

- Built on go-s7comm v0.16.0 (was v0.15.2):
  - `scan read-ranges` bounds each probe by `--timeout` (default 5000 ms); before, a probe waited as long as the command ran.
  - A read whose chunk comes back short now stops there: `short-read` output holds only the bytes from the start address up to that chunk.
  - The client refuses transport messages longer than 65535 bytes.

---

## v0.7.2

**Date:** 2026-11-14
**Previous release:** v0.7.1

No change to commands or output.

### Changed

- Built on go-s7comm v0.15.2 (was v0.15.1), which is built on go-cotp v1.3.0. The client no longer fails the connection when the responding transport selector of the connection confirm differs from the called one (X.224 §13.3.4).
- CI: otfabric/engineering policy 1.6, the shared workflows v3.8.0, and one pull request workflow with a single required check.

---

## v0.7.1

**Date:** 2026-11-11
**Previous release:** v0.7.0

### Changed

- Built on go-s7comm v0.15.1. Its recorded traffic now includes a real S7-1200 (CPU 1214C, firmware V4.2): it refuses a write to a write-protected DB with item code `0x03`, "object access is not allowed" ([python-snap7#976](https://github.com/gijzelaerr/python-snap7/issues/976#issuecomment-6800464989)). `write` reports such a refusal as access refused and exits 6. No change in behaviour.

### Documentation

- docs/exitcodes.md: exit 6 names a write to a write-protected DB as an example.

### Tests

- `write` to a DB the PLC refuses with item code `0x03` exits 6, and its error says access was refused.

---

## v0.7.0

**Date:** 2026-11-09
**Previous release:** v0.6.1

### Summary

Two flags on every command that connects to a PLC set how the connection is opened: `--proxy` reaches a plant network through a SOCKS5 proxy such as an SSH jump host, and `--source-addr` picks the local interface. Built on go-s7comm v0.15.0, whose scan helpers take a dialer too, so the flags also cover `discover`, `probe rackslot`, `compare read` and `profile`.

### Added

- `--proxy socks5://host:port` (`S7COMMCTL_PROXY`): connect through a SOCKS5 proxy (RFC 1928, CONNECT, no authentication); `socks5h://` sends host names to the proxy. Proxy credentials, other schemes, a missing port or a path are usage errors (exit 2). A refusal by the proxy is a connection failure naming the proxy and its reply.
- `--source-addr <ip>` (`S7COMMCTL_SOURCE_ADDR`): connect from that local IP address; with `--proxy`, the connection to the proxy.
- Both on `identify`, `read`, `write`, `list`, `block`, `blocks`, `diagnostic`, `fingerprint`, `scan`, `record`, `probe rackslot`, `compare read`, `profile` and `discover`.

### Breaking changes

- `compare read`: `--timeout` now bounds each candidate's read as well as its connect (go-s7comm v0.15.0). A PLC that accepted setup and then went silent used to block the command indefinitely; it now times out. A PLC that answers more slowly than `--timeout` now times out too: raise `--timeout` for it.

### Changed

- go-s7comm v0.15.0 (was v0.12.0): `client.WithDialer` and the dialer of the scan helpers.

### Tests

- `internal/app/netdial`: flag validation, source address binding, SOCKS5 CONNECT for IPv4, IPv6 and host names (`socks5` and `socks5h`), every reply and handshake failure, a silent proxy against the deadline, cancel. A context that ends as the handshake finishes fails the dial on one path instead of a timing-dependent branch.
- `internal/socks5test`: the test proxy's listen failure; it is at 100 % coverage, as is `internal/app/netdial`.
- `cmd/s7commctl`: each connecting command through a test proxy to the fake PLC, `discover` over a whole range with the real `client.Discover` (the proxy forwards every CONNECT to the fake PLC), a refusing proxy, `--source-addr`, and invalid values as usage errors on every command.

---

## v0.6.1

**Date:** 2026-11-08
**Previous release:** v0.6.0

### Summary

Built on go-cotp v1.3.0, and release binaries are built with the latest Go 1.26 patch release.

### Changed

- Built on go-cotp v1.3.0 (was v1.2.0; an indirect dependency through go-s7comm). The COTP client no longer fails the handshake when the connection confirm's responding transport selector (0xC2) differs from the configured remote selector; X.224 §13.3.4 allows it to differ, and it is kept as the negotiated remote selector. go-cotp also adds the server option `ServerConfig.EchoSelectors`, off by default. `version` lists `go-cotp 1.3.0`.
- `go.mod` declares `go 1.26.0` (was `go 1.26`), as the other OT Fabric repositories do; the minimum Go version is unchanged. Release binaries are built with the latest Go 1.26 patch release (otfabric/.github release workflow v3.6.0); with this `go` line the earlier workflow would have built with exactly go1.26.0.

---

## v0.6.0

**Date:** 2026-11-07
**Previous release:** v0.5.0

### Summary

`version` prints one line, as every OT Fabric CLI does, or JSON with `--json`. Shell completion supports PowerShell, completes the formats each command accepts and `.pcap` files, and no longer offers file names for addresses and numbers.

### Added

- `version --json` (and `version` with `S7COMMCTL_FORMAT=json`): one object `{"name","version","tag","commit","built","libraries","go","platform"}`, every key present; `libraries` lists the OT Fabric libraries of the build (go-cotp, go-s7comm, go-tpkt) with their versions.
- `completion powershell`. `completion` refused it: its own command allowed only bash, zsh and fish.
- `--format` completes the formats its command lists (`discover`: json, table, text; `scan read-ranges`: json, table, spans; `record`: jsonl, hex; …), the union only where the usage lists none. `pcap-decode --file` completes `.pcap` files, `replay --script` and `record --output` any file; every other value flag (IP, port, TSAP, numbers) and argument completes nothing instead of file names.

### Changed

- `version` prints `s7commctl 0.6.0 (tag v0.6.0, commit <sha>, built <date>, go-cotp 1.2.0, go-s7comm 0.12.0, go-tpkt 1.1.0, go1.26.10, linux/amd64)` instead of `s7commctl version 0.6.0 (tag: v0.6.0, commit: <short sha>, built: <date>)`: no `version` word and colons, the full commit, the libraries, Go version and platform. A build without a tag leaves out the tag; a development build (a pseudo-version too) is `dev`, with the commit Go embeds (`-dirty` when modified). Scripts that read the old line should use `version --json`.
- `completion bash` writes the bash-completion v2 script (`GenBashCompletionV2`), which completes flag values with descriptions.
- `make build` injects the full commit SHA, and a version and tag only when HEAD is exactly on a tag (otherwise the binary reports `dev`).

---

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
