# s7commctl examples (read-only)

All examples below were run against **192.168.0.1**. No `write` or `block upload` commands are included. Output is captured from live runs; your device may differ.

---

## Version

```bash
s7commctl version
```

**Example output:**

```
s7commctl version dev (commit: unknown, built: unknown)
```

---

## Discover

Use `--cidr` for discovery (single host = `/32`).

```bash
s7commctl discover --cidr 192.168.0.1/32
s7commctl discover --cidr 192.168.0.1/32 --format json
s7commctl discover --cidr 192.168.0.1/32 --quiet
```

**Example output (table):**

```
IP              Rack Slot PDU  TSAP
--------------- ---- ---- ---- ------
192.168.0.1       0    0  480 0x0300
```

**Example output (JSON):**

```json
[
  {
    "ip": "192.168.0.1",
    "is_s7": true,
    "pdu_size": 480,
    "port": 102,
    "rack": 0,
    "slot": 0,
    "tsap": "0x0300"
  }
]
```

**Example output (quiet):**

```
192.168.0.1
```

---

## Probe rack/slot

Find valid rack/slot combinations. Use `--no-exposure-warning` to suppress the public-IP warning in examples.

```bash
s7commctl probe rackslot --ip 192.168.0.1
s7commctl probe rackslot --ip 192.168.0.1 --summary-only --no-exposure-warning
s7commctl probe rackslot --ip 192.168.0.1 --rack-min 0 --rack-max 0 --slot-min 0 --slot-max 1
s7commctl probe rackslot --ip 192.168.0.1 --rack-min 0 --rack-max 0 --slot-min 0 --slot-max 2 --format json
```

**Example output (table, limited range):**

```
Target: 192.168.0.1
Mode: default (setup accepted = valid)

Rack  Slot  Stage   Status         PDU  ConfirmedBy  Confidence  Error
----  ----  ------  -------------  ---  -----------  ----------  --------------
0     0     setup   setup-only     480  -            low         
0     1     setup   setup-only     480  -            low         

Summary:
- Setup accepted: 2
- Confirmed by follow-up query: 0
- TCP-only: 0

Setup-accepted combinations:
- rack=0 slot=0 (confirmed by setup, PDU=480)
- rack=0 slot=1 (confirmed by setup, PDU=480)

Interpretation:
- Suggested default tuple: rack=0 slot=0
- Reason: setup accepted broadly; no discrimination observed.
```

**Example output (summary-only):**

```
Target: 192.168.0.1

Summary:
- Setup accepted: 256
- Confirmed by follow-up query: 0
- TCP-only: 0

Recommended connection tuple:
- rack=0 slot=0
Setup-accepted combinations: 256 total (use --show-all or --format json for full list).

Interpretation:
- Broad setup acceptance observed.
- No follow-up query confirmation.
- Endpoint may be rack/slot-insensitive or expose limited S7 services.
```

**Example output (JSON):**

```json
{
  "request": {
    "confirm": "none",
    "rack_max": 0,
    "rack_min": 0,
    "slot_max": 2,
    "slot_min": 0,
    "strict": false
  },
  "result": {
    "candidates": [
      {
        "confidence": "low",
        "confirmed_by": "",
        "error": "",
        "pdu_size": 480,
        "rack": 0,
        "slot": 0,
        "stage": "setup",
        "status": "setup-only"
      },
      {
        "confidence": "low",
        "confirmed_by": "",
        "error": "",
        "pdu_size": 480,
        "rack": 0,
        "slot": 1,
        "stage": "setup",
        "status": "setup-only"
      },
      {
        "confidence": "low",
        "confirmed_by": "",
        "error": "",
        "pdu_size": 480,
        "rack": 0,
        "slot": 2,
        "stage": "setup",
        "status": "setup-only"
      }
    ],
    "confirmed_by_query": 0,
    "flaky": 0,
    "setup_accepted": 3,
    "tcp_only": 0,
    "valid": [ ... ]
  },
  "target": {
    "ip": "192.168.0.1",
    "port": 102
  }
}
```

---

## Identify

Device identification (SZL-based: CPU, order, module, firmware). Output depends on device support.

```bash
s7commctl identify --ip 192.168.0.1 --rack 0 --slot 0
s7commctl identify --ip 192.168.0.1 --rack 0 --slot 1 --format json
```

*On this target, identify may fail with connection EOF; other PLCs often return CPU type, order number, and module name.*

---

## Read memory

Examples use `--rack 0 --slot 0` (valid for 192.168.0.1). Add `--no-exposure-warning` to suppress the public-IP warning.

**Inputs (I), outputs (Q), markers (M):**

```bash
s7commctl read --ip 192.168.0.1 --rack 0 --slot 0 --area I --offset 0 --size 8 --format hex
s7commctl read --ip 192.168.0.1 --rack 0 --slot 0 --area I --offset 0 --size 8 --format json
s7commctl read --ip 192.168.0.1 --rack 0 --slot 0 --area M --offset 0 --size 4 --format hex
s7commctl read --ip 192.168.0.1 --rack 0 --slot 0 --area Q --offset 0 --size 4 --format hex
```

**Example output (I, hex):**

```
status=success requested=8 returned=8 area=I offset=0

Area: I
Offset: 0
Requested: 8 bytes

Result: success
Returned: 8 bytes

00000000  28 00 00 00 00 00 00 00                           |(.......|
```

**Example output (I, JSON):**

```json
{
  "request": {
    "address": {
      "area": "I",
      "offset": 0,
      "size": 8
    },
    "connection": {
      "connection_mode": "rack_slot",
      "rack": 0,
      "slot": 0
    }
  },
  "result": {
    "raw_hex": "2800000000000000",
    "requested_length": 8,
    "returned_length": 8,
    "status": "success",
    "status_detail": "success",
    "warnings": []
  },
  "target": {
    "ip": "192.168.0.1",
    "port": 102
  }
}
```

**Example output (M, hex):**

```
status=success requested=4 returned=4 area=M offset=0

Area: M
Offset: 0
Requested: 4 bytes

Result: success
Returned: 4 bytes

00000000  00 00 00 00                                       |....|
```

**Example output (Q, hex):**

```
status=success requested=4 returned=4 area=Q offset=0

Area: Q
Offset: 0
Requested: 4 bytes

Result: success
Returned: 4 bytes

00000000  48 00 00 00                                       |H...|
```

**Data blocks (DB):**

```bash
s7commctl read --ip 192.168.0.1 --rack 0 --slot 0 --area DB --db 1 --offset 0 --size 8 --format hex
```

**Example output:**

```
status=success requested=8 returned=8 area=DB offset=0

Area: DB DB1
Offset: 0
Requested: 8 bytes

Result: success
Returned: 8 bytes

00000000  00 00 00 00 00 00 00 00                           |........|
```

---

## List blocks and list areas

```bash
s7commctl list blocks --ip 192.168.0.1 --rack 0 --slot 0
s7commctl list db --ip 192.168.0.1 --rack 0 --slot 0
s7commctl blocks --ip 192.168.0.1 --rack 0 --slot 0
s7commctl list areas
```

**Example output (list areas):**

```
DB
I
Q
M
T
C
```

*`list blocks` / `list db` output depends on the PLC; some devices may close the connection (EOF) during block enumeration.*

---

## Block info

Metadata for a block (no upload). Requires a valid block type and number.

```bash
s7commctl block info --ip 192.168.0.1 --type DB --number 1 --rack 0 --slot 0
s7commctl block info --ip 192.168.0.1 --type DB --number 1 --format json
```

---

## Diagnostic

CPU state, protection level, diagnostic buffer.

```bash
s7commctl diagnostic --ip 192.168.0.1 --rack 0 --slot 0
s7commctl diagnostic --ip 192.168.0.1 --rack 0 --slot 0 --format json
```

*Output depends on device; some targets may return EOF during diagnostic read.*

---

## Fingerprint

Negotiated capabilities and connection fingerprint.

```bash
s7commctl fingerprint --ip 192.168.0.1 --rack 0 --slot 0
s7commctl fingerprint --ip 192.168.0.1 --format json
```

*Uses identify internally; output depends on device support.*

---

## Profile

Capabilities summary (setup + identify + list blocks). Fast way to see what the target supports.

```bash
s7commctl profile --ip 192.168.0.1
s7commctl profile --ip 192.168.0.1 --format json
```

**Example output:**

```
Target: 192.168.0.1
Port: 102

Setup support:         yes
Rack/slot confirmed:   no
Recommended rack/slot: 0/0

Identify support:      no
Block list support:    no
  (command failed or not attempted)
```

---

## Scan read-ranges

Find readable address ranges in an area (read-only probes).

```bash
s7commctl scan read-ranges --ip 192.168.0.1 --rack 0 --slot 0 --area I --start 0 --end 32 --probe-size 8
s7commctl scan read-ranges --ip 192.168.0.1 --rack 0 --slot 0 --area M --start 0 --end 128 --probe-size 8
s7commctl scan read-ranges --ip 192.168.0.1 --rack 0 --slot 0 --area DB --db 1 --start 0 --end 256 --probe-size 32
s7commctl scan read-ranges --ip 192.168.0.1 --rack 0 --slot 0 --area I --start 0 --end 128 --probe-size 8 --summary-only
s7commctl scan read-ranges --ip 192.168.0.1 --rack 0 --slot 0 --area I --start 0 --end 128 --probe-size 8 --format json
```

**Example output:**

```
Area: I
Readable spans:
  [0..8) bytes 0-7  status=success
Empty spans:
  [8..32) bytes 8-31
```

---

## Compare read

Run the same read on multiple rack/slot pairs; see if results are identical.

```bash
s7commctl compare read --ip 192.168.0.1 --area I --offset 0 --size 4 --candidates 0/0,0/1
s7commctl compare read --ip 192.168.0.1 --area I --offset 0 --size 4 --candidates 0/0,0/1,0/2
s7commctl compare read --ip 192.168.0.1 --area I --offset 0 --size 4 --candidates 0/0,0/1,0/2 --format json
s7commctl compare read --ip 192.168.0.1 --area DB --db 1 --offset 0 --size 32 --candidates 0/0,0/1,0/2
```

**Example output (table):**

```
Target: 192.168.0.1
Read: area=I offset=0 size=4
Rack/slot-insensitive: true

Rack  Slot  Status         Returned  SameAsBaseline  Error
----  ----  -------------  --------  --------------  -----
0     0     success        4         baseline        
0     1     success        4         yes             
```

**Example output (JSON):**

```json
{
  "request": {
    "area": "I",
    "candidates": [
      { "rack": 0, "slot": 0 },
      { "rack": 0, "slot": 1 },
      { "rack": 0, "slot": 2 }
    ],
    "db_number": 0,
    "offset": 0,
    "size": 4
  },
  "result": {
    "by_candidate": [
      {
        "data_hex": "28000000",
        "rack": 0,
        "returned_length": 4,
        "same_as_baseline": "baseline",
        "slot": 0,
        "status": "success"
      },
      {
        "data_hex": "28000000",
        "rack": 0,
        "returned_length": 4,
        "same_as_baseline": "yes",
        "slot": 1,
        "status": "success"
      },
      {
        "data_hex": "28000000",
        "rack": 0,
        "returned_length": 4,
        "same_as_baseline": "yes",
        "slot": 2,
        "status": "success"
      }
    ],
    "rack_slot_insensitive": true
  },
  "target": {
    "ip": "192.168.0.1",
    "port": 102
  }
}
```

---

## Record (periodic reads to file)

Read-only; samples are written to a local file. Stop with Ctrl+C or use `--count`.

```bash
s7commctl record --ip 192.168.0.1 --rack 0 --slot 0 --area DB --db 1 --offset 0 --size 8 --every-ms 1000 --count 5 --output samples.jsonl
s7commctl record --ip 192.168.0.1 --rack 0 --slot 0 --area M --offset 100 --size 4 --format hex --every-ms 500 --count 10 --output samples.hex
```

---

## Sniff, pcap-decode, replay

**Sniff** — passive decode on a TCP stream (no PLC write):

```bash
s7commctl sniff --listen :15102 --format jsonl
```

**pcap-decode** — offline decode (no network):

```bash
s7commctl pcap-decode --file capture.pcap --port 102 --format text
```

**Replay** — scripted responses (no PLC):

```bash
s7commctl replay --listen :1102 --script replay.hex --format jsonl
```

---

## JSON and scripting

Pipe JSON to `jq` for pretty-print or filtering:

```bash
s7commctl probe rackslot --ip 192.168.0.1 --format json | jq .
s7commctl identify --ip 192.168.0.1 --rack 0 --slot 0 --format json | jq .
s7commctl read --ip 192.168.0.1 --rack 0 --slot 0 --area I --offset 0 --size 8 --format json | jq .
s7commctl list blocks --ip 192.168.0.1 --rack 0 --slot 0 --format json | jq .
s7commctl profile --ip 192.168.0.1 --format json | jq .
s7commctl compare read --ip 192.168.0.1 --area I --offset 0 --size 4 --candidates 0/0,0/1,0/2 --format json | jq .
```

Check exit code after `--strict-read`:

```bash
s7commctl read --ip 192.168.0.1 --rack 0 --slot 0 --area I --offset 0 --size 8 --strict-read; echo $?
```

---

## Excluded commands

- **write** — modifies PLC memory; not included in this read-only examples file.
- **block upload** — downloads block content from the PLC; not included here.

See [README.md](README.md) for full usage and safety notes.
