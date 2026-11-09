# Command reference

Every command of s7commctl and its flags, as `s7commctl <command> --help` prints them. Most flags can also be set with the environment variable named in brackets; see the README's [configuration section](../README.md#configuration-via-environment-variables). Exit codes: [exitcodes.md](exitcodes.md); JSON output: [json-output.md](json-output.md).

## `s7commctl block`

Block operations

Subcommands: `block info`, `block upload`.

## `s7commctl block info`

Get block metadata

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--number` | int | Block number |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Block type: OB, DB, FC, FB |

## `s7commctl block upload`

Upload block contents

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--number` | int | Block number |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Block type: OB, DB, FC, FB |

## `s7commctl blocks`

Alias/shortcut for listing blocks. Use "list blocks" for the same behavior under the list namespace.

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--number` | int | Block number |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Block type: OB, DB, FC, FB |

## `s7commctl compare`

Compare reads or behavior across rack/slot candidates

Subcommands: `compare read`.

## `s7commctl compare read`

Connect to each candidate (rack/slot), perform the same read, report results and whether all returned identical data (rack/slot-insensitive).

| Flag | Type | Meaning |
|---|---|---|
| `--area` | string | Memory area (DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200) [env: S7COMMCTL_AREA] |
| `--candidates` | string | Rack/slot pairs: 0/0,0/1,0/2 [env: S7COMMCTL_COMPARE_CANDIDATES] |
| `--db` | int | Data block number [env: S7COMMCTL_DB] |
| `--format` | string | Output format: json, table [env: S7COMMCTL_FORMAT] (default "table") |
| `--interval-ms` | int | Delay between repeat runs in ms [env: S7COMMCTL_COMPARE_INTERVAL_MS] |
| `--ip` | string | Target IP [env: S7COMMCTL_IP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--offset` | int | Byte offset [env: S7COMMCTL_OFFSET] |
| `--port` | int | Port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--repeat` | int | Repeat compare N times (for dynamic data) [env: S7COMMCTL_COMPARE_REPEAT] |
| `--size` | int | Number of bytes [env: S7COMMCTL_SIZE] |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout per candidate in ms [env: S7COMMCTL_TIMEOUT] (default 5000) |

## `s7commctl completion`

Generate a shell completion script for bash, zsh, fish or PowerShell: `s7commctl completion bash`, `zsh`, `fish` or `powershell`; README.md, "Shell completion", says where to put it.

No flags.

## `s7commctl diagnostic`

Read diagnostic buffer from S7 device

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |

## `s7commctl discover`

Discover S7 devices on a network

| Flag | Type | Meaning |
|---|---|---|
| `--cidr` | string | CIDR range to scan [env: S7COMMCTL_CIDR] |
| `--format` | string | Output format: json, table, text [env: S7COMMCTL_FORMAT] (default "table") |
| `--parallel` | int | Number of parallel probes [env: S7COMMCTL_PARALLEL] (default 10) |
| `--profile` | string | Probe profile: quick (rack 0, slot 1-2), standard (rack 0-1, slot 0-3), full (rack 0-3, slot 0-5) [env: S7COMMCTL_DISCOVER_PROFILE] (default "standard") |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Only print discovered IP addresses (one per line) [env: S7COMMCTL_QUIET] |
| `--rack-max` | int | Maximum rack to probe, overrides --profile [env: S7COMMCTL_RACK_MAX] (default 1) |
| `--rack-min` | int | Minimum rack to probe, overrides --profile [env: S7COMMCTL_RACK_MIN] |
| `--rate-ms` | int | Delay between rack/slot probes in ms [env: S7COMMCTL_RATE_MS] |
| `--slot-max` | int | Maximum slot to probe, overrides --profile [env: S7COMMCTL_SLOT_MAX] (default 3) |
| `--slot-min` | int | Minimum slot to probe, overrides --profile [env: S7COMMCTL_SLOT_MIN] |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout per host in ms [env: S7COMMCTL_TIMEOUT] (default 2000) |

## `s7commctl fingerprint`

Fingerprint an S7 device

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |

## `s7commctl identify`

Identify an S7 device

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format [env: S7COMMCTL_FORMAT] (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |

## `s7commctl list`

List S7 entities

Subcommands: `list areas`, `list blocks`, `list db`.

## `s7commctl list areas`

List supported memory areas

No flags.

## `s7commctl list blocks`

List program blocks

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--number` | int | Block number |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Block type: OB, DB, FC, FB |

## `s7commctl list db`

List DB blocks

| Flag | Type | Meaning |
|---|---|---|
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--format` | string | Output format (default "table") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--number` | int | Block number |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Block type: OB, DB, FC, FB |

## `s7commctl pcap-decode`

Best-effort decoder for clean Ethernet/IPv4/TCP captures. Does not perform full TCP stream reassembly (no sequence, retransmit, or out-of-order handling).

| Flag | Type | Meaning |
|---|---|---|
| `--file` | string | Path to .pcap file [env: S7COMMCTL_PCAP_FILE] |
| `--format` | string | Output format: text, jsonl [env: S7COMMCTL_FORMAT] (default "text") |
| `--port` | int | TCP port filter [env: S7COMMCTL_PORT] (default 102) |

## `s7commctl probe`

Probe targets for S7 connectivity

Subcommands: `probe rackslot`.

## `s7commctl probe rackslot`

Probe a specific IP for valid Siemens S7 rack/slot pairs.

| Flag | Type | Meaning |
|---|---|---|
| `--confirm` | string | Confirmation strategy: szl, cpu-state, any (default with --strict: any) [env: S7COMMCTL_PROBE_CONFIRM] |
| `--delay-ms` | int | Delay between attempts in ms [env: S7COMMCTL_PROBE_DELAY_MS] |
| `--first` | bool | Stop after first valid rack/slot [env: S7COMMCTL_PROBE_FIRST] |
| `--first-confirmed` | bool | Stop after first valid-query (strict mode) [env: S7COMMCTL_PROBE_FIRST_CONFIRMED] |
| `--format` | string | Output format: json, table, text [env: S7COMMCTL_FORMAT] (default "table") |
| `--interval-ms` | int | Delay between repeat runs in ms [env: S7COMMCTL_PROBE_INTERVAL_MS] |
| `--ip` | string | Target IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--parallelism` | int | Concurrent probes [env: S7COMMCTL_PROBE_PARALLELISM] (default 4) |
| `--port` | int | TCP port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--rack-max` | int | Maximum rack to probe [env: S7COMMCTL_RACK_MAX] (default 7) |
| `--rack-min` | int | Minimum rack to probe [env: S7COMMCTL_RACK_MIN] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--repeat` | int | Run probe N times and report variance (for rate-sensitive endpoints) [env: S7COMMCTL_PROBE_REPEAT] |
| `--retries` | int | Retry count per candidate for inconclusive results [env: S7COMMCTL_PROBE_RETRIES] |
| `--retry-delay-ms` | int | Delay between retries in ms [env: S7COMMCTL_PROBE_RETRY_DELAY_MS] |
| `--show-all` | bool | Show all candidates in table including non-valid [env: S7COMMCTL_PROBE_SHOW_ALL] |
| `--slot-max` | int | Maximum slot to probe [env: S7COMMCTL_SLOT_MAX] (default 31) |
| `--slot-min` | int | Minimum slot to probe [env: S7COMMCTL_SLOT_MIN] |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--strict` | bool | Only count valid-query as valid (confirm with follow-up) [env: S7COMMCTL_PROBE_STRICT] |
| `--summary-only` | bool | Print only summary and interpretation (no table or examples) [env: S7COMMCTL_PROBE_SUMMARY_ONLY] |
| `--timeout` | int | Per-attempt timeout in ms [env: S7COMMCTL_TIMEOUT] (default 2000) |

## `s7commctl profile`

Probe target for valid rack/slot, then run identify and list blocks to summarize setup support, rack/slot discrimination, identify, and block support.

| Flag | Type | Meaning |
|---|---|---|
| `--format` | string | Output format: text, json [env: S7COMMCTL_FORMAT] (default "text") |
| `--ip` | string | Target IP address [env: S7COMMCTL_IP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--port` | int | TCP port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |

## `s7commctl read`

Read data from an S7 device

| Flag | Type | Meaning |
|---|---|---|
| `--area` | string | Memory area: DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_COUNTER_200, IEC_TIMER_200 [env: S7COMMCTL_AREA] (default "DB") |
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--db` | int | Data block number [env: S7COMMCTL_DB] (default 1) |
| `--format` | string | Output format [env: S7COMMCTL_FORMAT] (default "plain") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--offset` | int | Byte offset [env: S7COMMCTL_OFFSET] |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--size` | int | Number of bytes [env: S7COMMCTL_SIZE] (default 1) |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--strict-read` | bool | Fail when status is not success (e.g. short-read, empty-read) [env: S7COMMCTL_STRICT_READ] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Data type [env: S7COMMCTL_TYPE] (default "raw") |

## `s7commctl record`

Record periodic S7 reads to a local file

| Flag | Type | Meaning |
|---|---|---|
| `--area` | string | Memory area: DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200 [env: S7COMMCTL_AREA] (default "DB") |
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--count` | int | Number of samples (0 = run until interrupted) [env: S7COMMCTL_COUNT] |
| `--db` | int | Data block number [env: S7COMMCTL_DB] (default 1) |
| `--every-ms` | int | Sampling interval in milliseconds [env: S7COMMCTL_EVERY_MS] (default 1000) |
| `--format` | string | Output format: jsonl, hex [env: S7COMMCTL_FORMAT] (default "jsonl") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--offset` | int | Byte offset [env: S7COMMCTL_OFFSET] |
| `--output` | string | Output file path [env: S7COMMCTL_OUTPUT] (default "record.jsonl") |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--size` | int | Number of bytes per sample [env: S7COMMCTL_SIZE] (default 1) |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Data type [env: S7COMMCTL_TYPE] (default "raw") |

## `s7commctl replay`

Accepts incoming frames and optionally responds with scripted raw TPKT hex frames, one response per request. Single-client mode: accepts one TCP connection; processes one request/response pair per scripted line until the connection closes.

| Flag | Type | Meaning |
|---|---|---|
| `--delay-ms` | int | Delay before each scripted response [env: S7COMMCTL_REPLAY_DELAY_MS] |
| `--format` | string | Output format: text, jsonl [env: S7COMMCTL_FORMAT] (default "text") |
| `--listen` | string | Replay listen address [env: S7COMMCTL_REPLAY_LISTEN] (default ":1102") |
| `--script` | string | Script file with hex-encoded response frames [env: S7COMMCTL_REPLAY_SCRIPT] |

## `s7commctl scan`

Scan targets for readable ranges or behavior

Subcommands: `scan read-ranges`.

## `s7commctl scan read-ranges`

Probe an area from start to end by step; consolidate adjacent readable probes into spans. Read-only.

| Flag | Type | Meaning |
|---|---|---|
| `--area` | string | Memory area: DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200 [env: S7COMMCTL_AREA] (default "DB") |
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--db` | int | Data block number (for DB area) [env: S7COMMCTL_DB] (default 1) |
| `--end` | int | End offset (exclusive) [env: S7COMMCTL_SCAN_END] (default 256) |
| `--format` | string | Output format: json, table, spans [env: S7COMMCTL_FORMAT] (default "spans") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--probe-size` | int | Bytes to read per probe [env: S7COMMCTL_PROBE_SIZE] (default 8) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--repeat` | int | Repeat each probe N times for stability [env: S7COMMCTL_SCAN_REPEAT] |
| `--retries` | int | Retries per probe [env: S7COMMCTL_SCAN_RETRIES] |
| `--retry-delay-ms` | int | Delay between retries in ms [env: S7COMMCTL_SCAN_RETRY_DELAY_MS] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--start` | int | Start offset (inclusive) [env: S7COMMCTL_SCAN_START] |
| `--step` | int | Step between probes (0 = use probe-size) [env: S7COMMCTL_SCAN_STEP] |
| `--summary-only` | bool | Print only span summary (no per-probe table) [env: S7COMMCTL_SCAN_SUMMARY_ONLY] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |

## `s7commctl sniff`

Listens for raw TPKT frames on a local TCP socket and decodes them without initiating any PLC communication. Single-client mode: accepts one TCP connection, then decodes frames until the connection closes.

| Flag | Type | Meaning |
|---|---|---|
| `--format` | string | Output format: text, jsonl [env: S7COMMCTL_FORMAT] (default "text") |
| `--listen` | string | Listen address for passive frame feed [env: S7COMMCTL_SNIFF_LISTEN] (default ":15102") |
| `--max-frames` | int | Stop after this many frames (0 = unlimited) [env: S7COMMCTL_MAX_FRAMES] |

## `s7commctl version`

Print version information: one line on stdout, or one JSON object with `--json` (or `S7COMMCTL_FORMAT=json`).

| Flag | Type | Meaning |
|---|---|---|
| `--json` | bool | Print the version as one JSON object |

```text
s7commctl 0.6.0 (tag v0.6.0, commit 1802d0f9afe8642d1e278120b76fcdfac967cb9e, built 2026-11-05T15:41:49Z, go-cotp 1.2.0, go-s7comm 0.12.0, go-tpkt 1.1.0, go1.26.10, linux/amd64)
{"name":"s7commctl","version":"0.6.0","tag":"v0.6.0","commit":"1802d0f9afe8642d1e278120b76fcdfac967cb9e","built":"2026-11-05T15:41:49Z","libraries":[{"name":"go-cotp","version":"1.2.0"},{"name":"go-s7comm","version":"0.12.0"},{"name":"go-tpkt","version":"1.1.0"}],"go":"go1.26.10","platform":"linux/amd64"}
```

The OT Fabric libraries the binary is built with follow the build date. A build without a tag leaves out `tag …, ` (and has `"tag": ""`); a development build (`go build`, `go run`, `make build` off a tag) is version `dev`, with the commit Go embeds (`-dirty` for a modified tree) and build date `unknown`.

## `s7commctl write`

Write data to an S7 device

| Flag | Type | Meaning |
|---|---|---|
| `--area` | string | Memory area (DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200) [env: S7COMMCTL_AREA] (default "DB") |
| `--auto-rack-slot` | bool | Auto-detect rack/slot [env: S7COMMCTL_AUTO_RACK_SLOT] |
| `--brute-rack-slot` | bool | Brute rack 0-3 and slot 0-5 [env: S7COMMCTL_BRUTE_RACK_SLOT] |
| `--dangerous` | bool | Confirm write operation |
| `--db` | int | Data block number [env: S7COMMCTL_DB] (default 1) |
| `--force` | bool | Skip confirmation |
| `--format` | string | Output format: text, json [env: S7COMMCTL_FORMAT] (default "text") |
| `--ip` | string | S7 device IP address [env: S7COMMCTL_IP] |
| `--local-tsap` | string | Local TSAP (decimal or 0x hex) [env: S7COMMCTL_LOCAL_TSAP] |
| `--no-exposure-warning` | bool | Suppress public routable IP warning (e.g. authorized environments) [env: S7COMMCTL_NO_EXPOSURE_WARNING] |
| `--offset` | int | Byte offset [env: S7COMMCTL_OFFSET] |
| `--port` | int | S7 port (default 102) [env: S7COMMCTL_PORT] (default 102) |
| `--proxy` | string | Connect through a SOCKS5 proxy: socks5://host:port (socks5h:// resolves names at the proxy) [env: S7COMMCTL_PROXY] |
| `--quiet` | bool | Minimal output [env: S7COMMCTL_QUIET] |
| `--rack` | int | PLC rack number [env: S7COMMCTL_RACK] |
| `--rate-limit-ms` | int | Minimum delay between protocol operations in ms [env: S7COMMCTL_RATE_LIMIT_MS] |
| `--remote-tsap` | string | Remote TSAP (decimal or 0x hex) [env: S7COMMCTL_REMOTE_TSAP] |
| `--slot` | int | PLC slot number [env: S7COMMCTL_SLOT] (default 1) |
| `--source-addr` | string | Local IP address to connect from (multi-homed hosts) [env: S7COMMCTL_SOURCE_ADDR] |
| `--timeout` | int | Timeout in milliseconds [env: S7COMMCTL_TIMEOUT] (default 5000) |
| `--type` | string | Data type (default "raw") |
| `--value` | string | Value to write |
