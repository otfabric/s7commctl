# Process exit codes

s7commctl's exit codes are a script-facing contract (`cmd/s7commctl/exit.go`). They follow the organisation table (otfabric/engineering `standards/api-design.md#cli-exit-codes`). Usage text is printed only on validation errors.

| Code | Kind | Meaning |
|------|------|---------|
| 0 | ok | Success |
| 1 | findings | The operation completed and reports a non-success: `probe` found no valid rack/slot, a short read with `--strict-read`, a `record` run with a failed sample |
| 2 | usage | Invalid flags (for example a negative `--request-timeout`), missing required arguments, malformed input, an address go-s7comm refuses before sending (negative offset, DB above 65535, a range past the S7ANY address limit), a write without `--dangerous` or aborted at the prompt, `write --type bool` without `--bit`, a `write --type string` value longer than 254 octets or than the STRING's declared maximum |
| 3 | invalid_input | A request larger than the negotiated PDU |
| 4 | transport | Connecting failed, the connection is lost (also `record` stopped by `--max-failures` after transport errors), a listener cannot start |
| 5 | timeout | A deadline passed: connecting took longer than `--timeout`, or one request longer than `--request-timeout` (`--timeout` when it is not set) (also `record` stopped by `--max-failures` when the last failed sample timed out) |
| 6 | protocol | The PLC refused the request (for example a write to a write-protected DB, item code `0x03`), or a malformed response or protocol failure |
| 9 | permission | The operating system refused an operation, for example binding port 102 |
| 10 | internal | An output file could not be opened or written |
| 130 | interrupted | SIGINT or SIGTERM before a result. `record` stopped this way ends with 0 |

`S7COMMCTL_FORMAT` sets the default of `--format`; `S7COMMCTL_OUTPUT_FORMAT` is accepted too and `S7COMMCTL_FORMAT` wins.

## Change in v0.11.0

`--timeout` bounds each request after connecting as well (README, [Timeouts](../README.md#timeouts)):

| Case | Before | Now |
|---|---|---|
| A request whose answer takes longer than `--timeout` (`identify`, `write`, `blocks`, `list blocks`, `list db`, `block info`, `block upload`, `diagnostic` reading the diagnostic buffer, `fingerprint` during identify) | waited for the answer; the code of the result | 5, `<operation> failed: …` ending in `context deadline exceeded` |
| The same for a later request of `fingerprint` or `diagnostic`, or identify or the block list of `profile` | waited; 0 | 0, that request and the ones after it reported as failed |
| The same for `read` | waited; the code of the result | status `timeout`: 0, or 1 with `--strict-read` |
| The same for a `record` sample | waited | a failed sample (`timeout`); the connection ends, so the next samples fail with `transport-error` and `--max-failures` stops the run with 4, or with 5 when it stops on the timed-out sample |
| `--timeout 0` or a negative value | no bound per request | no bound per request |

`scan read-ranges` bounded each probe read by `--timeout` before; its codes do not change.

## Change in v0.10.0

| Case | Before | Now |
|---|---|---|
| `record` with at least one sample whose status is not `success` | 0 | 1, summary `Recorded N samples (M failed) to <file>` |
| `record` after `--max-failures` consecutive samples (default 5) failed with `transport-error` or `timeout` | went on, 0 at `--count` | stops: 4 (`transport-error`) or 5 (`timeout`), the code of the last sample; `--max-failures 0` never stops |
| `write --type bool` without `--bit` | 0, the whole byte written | 2, nothing sent |
| `write --type string` with a value longer than 254 octets | 0, the value cut to 254 | 2, before connecting |
| `write --type string` with a value longer than the STRING's declared maximum (byte 0) | 0, the bytes after the STRING overwritten | 2, nothing written |
| `write --type string` when byte 0 cannot be read | — (byte 0 was not read; the write was sent) | the read's code: 4 when the connection is lost, 5 on a timeout, 6 when the PLC refuses it |

An interrupt still ends `record` with 0.

## Change in v0.9.0

An interrupt (SIGINT or SIGTERM) now ends these commands at once with 130:

| Case | Before | Now |
|---|---|---|
| `compare read` interrupted during a run, or with `--repeat N` during `--interval-ms` | 0, with the interrupted run printed as the result (after the interval) | 130, no result |
| `probe rackslot --repeat N` interrupted during `--interval-ms` | 130 after the interval | 130 at once |
| `replay --delay-ms` interrupted during the delay | 4 after the delay | 130 at once |
| `sniff`, `replay` interrupted just after the client connected, before the first read | 4 or 130 | 130 |

## Change in v0.4.0

An address or option that go-s7comm refuses before anything is sent (a `*client.ValidationError`, for example `--offset -1`, `--db 65537`, `scan --retries -1`) exits 2 instead of 6. Nothing reached the PLC, so 6 ("the PLC refused") was wrong.

## Mapping from versions before v0.3.0

| Case | Old | New |
|---|---|---|
| Probe found nothing, strict short read | 1 | 1 |
| Write aborted at the prompt | 1 | 2 |
| Unknown flag or command (cobra) | 1 | 2 |
| Connection failure | 3 | 4 |
| Timeout | 3 | 5 |
| Operation failed on the PLC, protocol error | 3 | 6 |
| Interrupted | killed by the signal | 130 |
