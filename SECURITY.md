# Security

## Supported versions

| Version | Supported |
| ------- | --------- |
| Latest release | Yes |
| Older releases | No |

## Reporting a vulnerability

If you believe you have found a security vulnerability in s7commctl, please report it responsibly.

- **Do not** open a public GitHub issue for security-sensitive bugs.
- Email **security@otfabric.io**, or open a **private** security advisory on GitHub: **Security** → **Advisories** → **Report a vulnerability**.

Include a clear description, steps to reproduce, and the impact. We will acknowledge your report and work with you on a fix and disclosure timeline.

## Security-related considerations

- **Network and PLC access**: s7commctl connects to Siemens S7 devices over TCP (port 102). Use it only on networks and targets you are authorized to access. The commands that connect to one PLC warn on stderr when the target IP appears publicly routable, and `discover` warns when its `--cidr` range holds publicly routable addresses; `--no-exposure-warning` suppresses either in authorized environments. The warning does not stop the command.
- **Proxies**: `--proxy` sends the connection, and the PLC traffic in clear text, through the SOCKS5 proxy you name; use only a proxy you control, such as an SSH jump host. The exposure warning checks the PLC's address, not the proxy's.
- **Writes**: The `write` command can modify PLC memory and requires explicit `--dangerous`. Unless `--force` is set, it asks for confirmation with a prompt that names the length, type and value, the address, area, DB, offset and bit, the IP and port, and the rack and slot or TSAPs; these can come from `S7COMMCTL_*` variables. `--type bool` writes one bit only with `--bit`; without it the whole byte is written and a warning says so. Use with care in production.
- **Device-supplied text**: identification texts come from the device. Text and table output (and `--quiet`) of `identify`, `fingerprint` and `profile` print non-printable characters and invalid UTF-8 as `\xNN`; JSON escapes them as JSON does.
- **Protection level**: when `fingerprint` or `diagnostic` cannot read the protection level, they report it as unknown (`null` and `protection_error` in JSON, `unknown` with the reason in text and table output), and `fingerprint` reports `write_likely_allowed: false`.
- **Credentials**: The tool does not store passwords; S7 over ISO-on-TCP (port 102) typically relies on network segmentation and PLC protection levels rather than application-level authentication. Ensure targets are not exposed to untrusted networks.

## Listeners

`sniff` and `replay` are the only commands that accept connections:

| Command | Default `--listen` | Accepts |
|---|---|---|
| `sniff` | `:15102` | The first client; its frames are decoded and printed |
| `replay` | `:1102` | The first client; it receives the scripted responses |

The defaults listen on every interface, without authentication or encryption: any host that can reach the port and connects first is the client, and a later client is not served. Use `--listen 127.0.0.1:PORT` (or `S7COMMCTL_SNIFF_LISTEN`, `S7COMMCTL_REPLAY_LISTEN`) to accept local clients only, and a host firewall when the listener must be reachable from elsewhere.

## Limits

| Input | Limit | Beyond it |
|---|---|---|
| A TPKT frame (`sniff`, `replay`, `pcap-decode`) | Version 3, 7 to 65535 octets (the 16-bit TPKT length) | Another header is not a frame: `sniff` and `replay` stop with an error, `pcap-decode` skips one octet and looks for the next header |
| A packet of a pcap capture | 262144 octets captured | `pcap-decode` stops with an error; frames decoded before it are printed |
| Memory of `pcap-decode` | One packet at a time, and per direction of a connection less than one TPKT maximum pending after each packet | Captures of any size are read packet by packet |
| Waiting for the next frame | `sniff` 120 s, `replay` 300 s | The command ends with exit 5 (timeout) |
| A `write --type string` value | 254 octets | Cut to 254 octets |

For protocol and safety behavior, see [README.md](README.md) and [PROTOCOL.md](PROTOCOL.md).
