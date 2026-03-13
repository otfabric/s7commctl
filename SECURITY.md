# Security

## Reporting a vulnerability

If you believe you have found a security vulnerability in s7commctl, please report it responsibly.

- **Do not** open a public GitHub issue for security-sensitive bugs.
- Open a **private** security advisory on GitHub: **Security** → **Advisories** → **Report a vulnerability**, or contact the maintainers (see repository/org contacts).

Include a clear description, steps to reproduce, and the impact. We will acknowledge your report and work with you on a fix and disclosure timeline.

## Security-related considerations

- **Network and PLC access**: s7commctl connects to Siemens S7 devices over TCP (port 102). Use it only on networks and targets you are authorized to access. The CLI warns when the target IP appears publicly routable (`--no-exposure-warning` suppresses this in authorized environments).
- **Writes**: The `write` command can modify PLC memory and requires explicit `--dangerous` (and optionally `--force`). Use with care in production.
- **Credentials**: The tool does not store passwords; S7 over ISO-on-TCP (port 102) typically relies on network segmentation and PLC protection levels rather than application-level authentication. Ensure targets are not exposed to untrusted networks.

For protocol and safety behavior, see [README.md](README.md) and [PROTOCOL.md](PROTOCOL.md).
