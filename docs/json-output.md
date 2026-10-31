# JSON output

The contract of `--format json` and `--format jsonl`. Field names and shapes are a public contract for scripts: a change is called out in RELEASE.md. Keys are snake_case, except in the frame events below. The values of `status` come from go-s7comm. Exit codes: [exitcodes.md](exitcodes.md).

`S7COMMCTL_FORMAT` sets the default of `--format`. `--quiet` overrides the format on the commands that have it: it prints a bare value or nothing.

## Envelope

Commands that talk to one PLC print one indented object on stdout:

| Key | Content |
|---|---|
| `target` | `ip` and `port`; `scan read-ranges` adds `rack` and `slot` |
| `request` | What was asked for: the connection fields below plus the command's own |
| `result` | The outcome, per command below. `scan read-ranges` has `summary` and `spans` instead |
| `error` | Only from `diagnostic` and `list blocks` when they fail: `{"code": "runtime", "message": "..."}`, in place of `result`. The process still exits with the code of the failure |

Other failures print a message on stderr and nothing on stdout. Exception: once connected, `read` prints its result for a failed read too and reports the outcome in `result.status`; with `--strict-read` it also exits non-zero.

### Connection fields

`request` has `connection_mode` and:

| `connection_mode` | Other keys |
|---|---|
| `rack_slot` | `rack`, `slot` |
| `auto_rack_slot` | `rack`, `slot` (the configured values) |
| `explicit_tsap` | `local_tsap`, `remote_tsap` |

`read` nests them under `request.connection`. `probe`, `compare`, `profile` and `scan read-ranges` do not print them.

## Commands

### `read`

`request.address`: `area`, `offset`, `size`, and `db` for the DB area.

`result`:

| Key | Type | Meaning |
|---|---|---|
| `status` | string | `success`, `short-read`, `empty-read`, `rejected`, `timeout`, `transport-error`, `protocol-error`, `inconclusive` |
| `status_detail` | string | `status`, then ` — ` and `message` when there is one. For people |
| `requested_length`, `returned_length` | number | Octets |
| `raw_hex` | string | The data, `""` when there is none |
| `warnings` | array of strings | Never null |
| `message` | string | Omitted when empty |
| `decoded` | array | One value per element with `--type` other than `raw`. `bool` gives at most 64 values, `string` one |

### `write`

`request`: connection fields, `area`, `db`, `offset`, `value` (the first 128 characters, followed by `...` and then `value_truncated: true`).

`result`: `ok` (true), `written` (octets).

### `identify`

`result`: `order_number`, `serial_number`, `module_name`, `cpu_type`, `fw_version`. Each is a string, or null when the PLC did not report it.

### `fingerprint`

`result`: `cpu_type`, `order_number`, `firmware_ver`, `module_name` (string or null); `cpu_generation`, `accepted_tsap`, `protection` (string); `pdu_size`, `max_amq_calling`, `max_amq_called`, `detected_rack`, `detected_slot` (number); `supports_read`, `write_likely_allowed`, `supports_szl`, `upload_probe_ok`, `supports_pi` (boolean). `accepted_tsap` is the remote TSAP in decimal. `supports_szl` is true whenever the command got this far, and `supports_pi` is not probed and stays false.

### `diagnostic`

`result`: `cpu_state`, `protection_level` (strings), `diagnostic` with `total_count` and `entries`. An entry has `timestamp` and `description` (string or null), and `event_id`, `event_class`, `priority`, `info1`, `info2` (numbers).

### `list blocks`, `blocks`

The exception to the envelope: on success an **array** of blocks, with `type`, `language` (strings); `number`, `flags`, `load_memory`, `local_data`, `mc7_size`, `sbb_length`, `check_sum` (numbers); `author`, `family`, `name`, `version` (string or null). `[]` when there are none. A failure is the `error` envelope, with `block_type` in `request`.

### `block info`

`request`: connection fields, `type`, `number`. `result`: `type`, `number`, `load_memory`, `local_data`, `mc7_size`.

### `block upload`

`request`: connection fields, `type`, `number`. `result`: `type`, `number`, `size` (octets), `hex`.

### `discover`

The other exception to the envelope: an **array** with one object per S7 device found: `ip`, `port`, `is_s7`, `rack`, `slot`, `pdu_size`, `tsap`, and `error` and `abandoned_reason` when they are set.

### `probe rackslot`

`request`: `rack_min`, `rack_max`, `slot_min`, `slot_max`, `strict`, `confirm`.

`result`:

| Key | Meaning |
|---|---|
| `candidates` | Every attempt |
| `valid` | The ones that count as valid, same shape |
| `setup_accepted`, `confirmed_by_query`, `tcp_only`, `flaky` | Counts |
| `repeat_runs`, `results_varied` | Only with more than one run. The rest of `result` describes the first |

A candidate has `rack`, `slot`, `stage`, `status`, `pdu_size`, `confirmed_by`, `confidence`, `error`; empty strings where nothing applies. With `--strict`, `status` shows `setup-ok` for `setup-only` and `valid-connect`, and `query-confirmed` for `valid-query`.

### `profile`

`request` is `{}`. `result`:

| Key | Type |
|---|---|
| `setup_support`, `rack_slot_confirmed`, `identify_transport_ok`, `identify_fields_found`, `block_list_support` | boolean |
| `recommended_rack`, `recommended_slot` | number, only when a rack and slot were found |
| `identify` | `cpu_type`, `order`, `module_name` (string or null), only when identify returned something |
| `block_count` | number, only when `block_list_support` is true |

### `compare read`

`request`: `area`, `db_number`, `offset`, `size`, `candidates` (array of `rack`, `slot`).

`result`: `rack_slot_insensitive` (boolean), `by_candidate`, and with `--repeat` above 1 `repeat_runs`, `data_stable_across_runs`, `stable_by_candidate`.

An entry of `by_candidate` has `rack`, `slot`, `status`, `returned_length`, `same_as_baseline` (`-` where it does not apply); `stable_across_runs`, `data_hex` and `message` when they apply.

### `scan read-ranges`

`request`: `area`, `start`, `end`, `step`, `probe_size`, and `db_number` for the DB area.

`summary`: `readable_count`, `empty_count`, `partial_count`, `failed_count`, `inconclusive_count`.

`spans`: `readable`, `empty`, `partial`, `failed`, `inconclusive`, each an array of `{"start", "end"}` with `status` when it is set. Arrays are never null.

## `jsonl` streams

One compact object per line.

### `record`

`jsonl` is its default format. `timestamp` (RFC 3339 in UTC), `sample` (number), `status`, `raw_hex`; `requested_length`, `returned_length` and `message` when there is a result; `decoded` with `--type` other than `raw`.

### Frame events: `sniff`, `replay`, `pcap-decode`

| Key | Meaning |
|---|---|
| `timestamp` | When the line was written, RFC 3339 in UTC. For `pcap-decode` not the capture time |
| `source` | The peer's address; for `pcap-decode` `srcip:port>dstip:port` |
| `summary` | The decoded frame, or null |
| `cotp_type` | `CR`, `CC`, `DR`, `DC`, `DT`, `ER`, `ED`, `AK`, `EA`, `RJ`, or `0xNN`; omitted without a summary |
| `error` | Omitted when there is none |

`summary` is go-s7comm's `wire.FrameSummary` as it marshals, so its keys are **not** snake_case: `TPDULength`, `COTPType`, `ROSCTR`, `Function`, `ParamLength`, `DataLength`, `ErrorClass`, `ErrorCode`, all numbers.
