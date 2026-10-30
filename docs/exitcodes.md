# Process exit codes

s7commctl's exit codes are a script-facing contract. Changes are noted in RELEASE.md.

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | The operation completed without success: `probe` found no valid rack/slot, a short read with `--strict-read`, a list or diagnostic failure, a write aborted at the prompt. Errors without a specific code, such as an unknown flag reported by cobra, also exit 1 |
| 2 | Usage or validation error: invalid flags, missing required arguments, malformed input, a write without `--dangerous` |
| 3 | Connection or runtime failure: TCP connect failed, timeout, protocol error, a file or listener that cannot be opened |

Defined in `cmd/s7commctl/exit.go`. Usage text is printed only on validation errors, not on runtime failures.
