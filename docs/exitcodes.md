# Process exit codes

s7commctl's exit codes are a script-facing contract (`cmd/s7commctl/exit.go`). They follow the organisation table (otfabric/engineering `standards/api-design.md#cli-exit-codes`). Usage text is printed only on validation errors.

| Code | Kind | Meaning |
|------|------|---------|
| 0 | ok | Success |
| 1 | findings | The operation completed and reports a non-success: `probe` found no valid rack/slot, a short read with `--strict-read` |
| 2 | usage | Invalid flags, missing required arguments, malformed input, an address go-s7comm refuses before sending (negative offset, DB above 65535, a range past the S7ANY address limit), a write without `--dangerous` or aborted at the prompt |
| 3 | invalid_input | A request larger than the negotiated PDU |
| 4 | transport | Connecting failed, the connection is lost, a listener cannot start |
| 5 | timeout | A deadline passed |
| 6 | protocol | The PLC refused the request (for example a write to a write-protected DB, item code `0x03`), or a malformed response or protocol failure |
| 9 | permission | The operating system refused an operation, for example binding port 102 |
| 10 | internal | An output file could not be opened or written |
| 130 | interrupted | SIGINT or SIGTERM before a result. `record` stopped this way ends with 0 |

`S7COMMCTL_FORMAT` sets the default of `--format`; `S7COMMCTL_OUTPUT_FORMAT` is accepted too and `S7COMMCTL_FORMAT` wins.

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
