# Siemens S7 Protocol Primer

This document gives a practical, operator‑focused overview of the Siemens S7 communication stack used by `s7commctl`. It explains how the protocol layers fit together, what the core terms mean, and how the CLI maps onto them.

---

## 1. Scope

`s7commctl` speaks the classic Siemens S7 protocol stack used by many PLCs and related devices over Ethernet.

Protocol layers:

- **TCP** on port **102**
- **TPKT** (RFC 1006)
- **COTP** (ISO transport connection protocol)
- **S7comm**

Conceptually:

- **Application data** → S7comm
- **Transport wrapper** → COTP
- **RFC1006 framing** → TPKT
- **Network transport** → TCP/102

You will sometimes see this called:

- S7 over ISO‑on‑TCP
- S7comm over TCP
- RFC1006 transport for S7

---

## 2. High‑level flow

A typical S7 communication session looks like this:

1. Open a TCP connection to port 102.
2. Exchange COTP connection messages (TSAP handshake).
3. Negotiate S7 communication parameters with **Setup Communication**.
4. Send S7 jobs such as:
   - read variable
   - write variable
   - read SZL
   - list blocks
   - block upload
   - diagnostics
5. Receive S7 acknowledgements / data responses.

Important: **“TCP reachable” does not automatically mean “usable S7 target”.** A device may:

- accept TCP only
- accept TCP + COTP only
- accept setup communication broadly but limit S7 services
- allow only a subset of S7 jobs
- expose only limited memory or metadata

`s7commctl` separates **probing**, **confirmation**, **reading**, **comparison**, and **profiling** precisely because of this.

---

## 3. Core protocol components

### 3.1 TCP

The outer transport is standard TCP, usually on port 102.

What TCP reachability tells you:

- the endpoint is listening
- a network path exists
- the port is open

What it does **not** tell you:

- that the endpoint is a Siemens PLC
- that S7comm is actually available
- that a specific rack/slot combination is meaningful
- that reads, diagnostics, or block services will work

---

### 3.2 TPKT

TPKT is the RFC1006 framing layer that carries ISO transport PDUs on top of TCP.

Operationally, TPKT:

- adds a small length‑prefixed wrapper around transport PDUs
- allows ISO transport traffic to run over TCP
- is required before COTP and S7comm messages can be exchanged

If TPKT parsing fails, the session breaks before any meaningful S7 work can happen.

Typical tool symptoms:

- `read TPKT header: EOF`
- short transport reads
- connection reset during handshake or response

---

### 3.3 COTP

COTP is the ISO transport protocol layer above TPKT.

`s7commctl` uses COTP for:

- connection request / confirm
- transport setup before S7comm begins
- TSAP‑based addressing of communication endpoints

A target may:

- accept the TCP connection
- reject the COTP connection
- accept COTP for many TSAP combinations without exposing useful PLC semantics

This is why **“TCP‑only”** and **“setup‑accepted”** are distinct outcomes.

---

### 3.4 S7comm

S7comm is the Siemens application protocol carried inside COTP.

This is the layer that actually performs PLC‑oriented operations such as:

- variable reads
- variable writes
- SZL access
- CPU state queries
- block enumeration
- block upload
- diagnostics

`s7commctl` is fundamentally an S7comm client, with additional tooling for discovery, probing, safety, decoding, and replay.

---

## 4. Core nomenclature

### 4.1 Rack and slot

Siemens devices are often addressed using **rack** and **slot**.

Conceptually:

- **rack**: identifies a chassis or logical rack
- **slot**: identifies a module position within that rack

In many classic setups you will see:

- `rack = 0`
- `slot = 2` (or similar)

Real‑world endpoints vary, though:

- some behave as expected and require a specific tuple
- some accept many tuples
- some appear rack/slot‑insensitive
- some are gateways, protocol shims, simulators, or non‑PLC endpoints speaking enough S7 to respond partially

`s7commctl probe rackslot` exists because rack/slot cannot always be assumed.

**Important:** a **setup‑accepted** tuple is weaker evidence than a **query‑confirmed** tuple.

---

### 4.2 TSAP

A TSAP is a **T**ransport **S**ervice **A**ccess **P**oint used during the COTP connection phase.

Two TSAP values are involved:

- **local TSAP**
- **remote TSAP**

In S7 environments, remote TSAP values often encode rack/slot‑related addressing. Operationally, this means:

- rack/slot probing usually changes the remote TSAP in connection attempts
- a successful connection reflects acceptance of that TSAP, not necessarily a meaningful PLC endpoint

`s7commctl` exposes TSAP‑related options so advanced users can override inferred rack/slot addressing when needed.

---

### 4.3 PDU size

After transport setup, S7 communication negotiates a **PDU size**.

A PDU (Protocol Data Unit) is the S7comm message payload exchanged between client and PLC.

Why PDU size matters:

- it limits request/response payload size
- it influences how large reads and writes can be in a single operation
- it is useful for fingerprinting and capability assessment

A negotiated PDU size is evidence that setup communication succeeded, but **not** proof that all higher‑level operations will work.

---

### 4.4 Job / Ack / Ack‑Data

S7comm traffic commonly follows a request/response model:

- **Job**: client request
- **Ack**: acknowledgement
- **Ack‑Data**: acknowledgement carrying data

You do not normally need to reason about packet types when using `s7commctl`, but they explain cases where operations fail **after** setup:

- transport succeeded
- setup communication succeeded
- a specific S7 job was then rejected, truncated, or returned empty

---

### 4.5 SZL

SZL means **System‑Zustands‑Liste** (system status list).

It is a structured mechanism used by Siemens devices to expose metadata and status, such as:

- order number
- module name
- firmware version
- CPU family/type
- various system details

In `s7commctl`, SZL‑backed behavior appears mainly in:

- `identify`
- parts of `fingerprint`
- optional probe confirmation strategies

Important caveats:

- not all targets expose the same SZL fields
- some return only partial data
- some support setup/read operations but expose little or no useful SZL metadata

So `"N/A"` or `null` fields do **not** necessarily mean “not Siemens”; they may simply mean “not exposed”.

---

### 4.6 Areas

S7 variable access is organized into **areas**.

Common areas supported by `s7commctl` include:

- **DB** — Data Blocks
- **I** — Inputs
- **Q** — Outputs
- **M** — Merkers / flags / internal memory
- **T** — Timers
- **C** — Counters

These areas are not equivalent:

- **DB** – structured application data blocks; often the most useful for program data.
- **I** – process inputs; reflect incoming field/device state.
- **Q** – process outputs; reflect outgoing PLC command/output state.
- **M** – internal memory bits/bytes/words used by the PLC program.
- **T / C** – special timer and counter areas.

A device may expose some areas but not others, or only expose short readable prefixes. That is why `scan read-ranges` is useful.

---

### 4.7 Offsets and size

Reads are performed using:

- an **offset**
- a **size** (number of bytes)

Examples:

- read 16 bytes at offset 0 from DB1
- read 8 bytes at offset 32 from area I

In practice, outcomes include:

- full success
- short read
- empty read
- explicit rejection
- transport/protocol failure

This distinction matters because many industrial targets do **not** behave like simple flat memory servers.

---

### 4.8 Blocks

Siemens PLC programs are organized into **blocks**.

Common block types:

- **OB** — Organization Blocks
- **DB** — Data Blocks
- **FB** — Function Blocks
- **FC** — Functions
- **SFB** — System Function Blocks
- **SFC** — System Functions

`s7commctl` uses these for:

- `list blocks`
- `list db`
- `block info`
- `block upload`

Whether block operations work depends on:

- device capabilities
- protection level
- CPU family/model
- access restrictions

---

### 4.9 Diagnostics

Diagnostics refers to PLC/system‑oriented services such as:

- CPU state
- protection level
- diagnostic buffer

These services are often more restricted than ordinary reads.

It is normal to encounter targets where:

- reads work
- identify works partially
- diagnostic buffer fails
- block list is empty or unsupported

This does not necessarily mean the CLI is wrong; it often reflects the endpoint’s actual S7 surface.

---

## 5. How s7commctl interprets protocol behavior

### 5.1 Setup‑accepted vs query‑confirmed

A key design principle in `s7commctl` is to distinguish **transport/setup success** from **meaningful higher‑level confirmation**.

**Setup‑accepted**

The endpoint accepted transport setup and S7 communication setup.

This proves:

- TCP worked
- TPKT/COTP worked
- setup communication worked

This does **not** prove:

- the rack/slot is semantically correct
- SZL works
- reads work
- diagnostics work
- the endpoint is a full PLC

**Query‑confirmed**

A follow‑up S7 operation also succeeded, such as:

- SZL read
- CPU‑state query
- another benign confirmation action

This is stronger evidence that the candidate is genuinely usable.

---

### 5.2 Read result semantics

Reads are intentionally classified instead of treating every non‑error as “success”.

Key result types:

- `success` — requested bytes returned
- `short-read` — some bytes returned, fewer than requested
- `empty-read` — request accepted but target returned no payload
- `rejected` — target explicitly refused the request
- `timeout` / transport / protocol error — lower‑level failure
- `inconclusive` — behavior insufficiently stable or clear

This matters because industrial endpoints are often partial, inconsistent, or rate‑sensitive.

---

### 5.3 Rack/slot‑insensitive behavior

If many rack/slot combinations:

- accept setup
- return the same data
- fail confirmation in the same way

then the endpoint may be:

- rack/slot‑insensitive
- fronted by a gateway
- exposing a limited compatibility layer
- not behaving like a classic chassis/CPU model

That is why `compare read` and `profile` exist: they help distinguish “one correct tuple” from “many tuples behave the same”.

---

## 6. Practical command mapping

How the CLI maps to protocol concepts:

- **`discover`** – finds reachable endpoints and optionally probes rack/slot candidates.
- **`probe rackslot`** – tests rack/slot‑derived connection tuples and classifies behavior (TCP‑only, setup‑accepted, query‑confirmed, unstable/failed).
- **`identify`** – uses SZL access to obtain metadata such as order number, module name, firmware, and CPU type.
- **`read`** – performs variable reads against DB/I/Q/M/T/C areas.
- **`write`** – performs variable writes with explicit safety controls.
- **`scan read-ranges`** – finds readable, empty, partial, or failed spans in an area.
- **`compare read`** – runs the same read over multiple candidate rack/slot tuples to see whether results differ.
- **`list blocks`, `block info`, `block upload`** – use block‑oriented S7 services.
- **`diagnostic`** – uses diagnostic‑oriented S7 services such as CPU state and diagnostic buffer.
- **`fingerprint`** – summarizes communication characteristics (PDU size, TSAP behavior, possible CPU family heuristics).
- **`sniff`, `pcap-decode`, `replay`** – support protocol inspection, offline analysis, and deterministic testing.

---

## 7. Common operator misconceptions

- **“Port 102 is open, so the PLC is fully accessible.”**  
  Not necessarily. TCP reachability is only the outermost layer.

- **“One accepted rack/slot means it is the correct rack/slot.”**  
  Not necessarily. Setup acceptance can be broad and non‑discriminating.

- **“If identify returns N/A, the target is not Siemens.”**  
  Not necessarily. Many targets expose partial or no SZL metadata.

- **“If reads work, diagnostics and blocks should also work.”**  
  Not necessarily. Different service families may be independently restricted.

- **“All memory areas behave like a flat address space.”**  
  Not true. Some areas may expose only short readable prefixes or no data at all.

---

## 8. Handling guidance

When interacting with an unknown endpoint, a sensible workflow is:

1. **Probe setup behavior** – `probe rackslot`.
2. **Check for meaningful confirmation** – `probe rackslot --strict`.
3. **Try identification** – `identify`.
4. **Try bounded reads** – small reads from I, Q, M, and DB.
5. **Scan readable ranges** – `scan read-ranges`.
6. **Compare across candidate tuples** – `compare read`.
7. **Treat writes as hazardous** – only in authorized test/maintenance contexts.

This matches the reality that S7 endpoints vary widely in how much they expose.

---

## 9. Safety note

S7comm is an operational industrial protocol. Even seemingly harmless actions can stress fragile devices or reveal exposed infrastructure.

Rules of thumb:

- prefer read‑only operations first
- use small bounded reads
- rate‑limit when probing large spaces
- avoid writes unless explicitly authorized
- treat public internet exposure as high risk
- assume some devices are unstable or fronted by gateways

`s7commctl` includes safety‑oriented behavior, but protocol‑level caution remains the operator’s responsibility.

---

## 10. Summary

At its core, S7 communication is:

- TCP/102
- carrying TPKT
- carrying COTP
- carrying S7comm

Important practical concepts:

- rack/slot
- TSAP
- PDU size
- SZL
- areas (DB, I, Q, M, T, C)
- blocks
- diagnostic services
- the distinction between **setup acceptance** and **meaningful protocol confirmation**

`s7commctl` is built around that distinction: not just “can I connect?”, but “what kind of S7 behavior is actually present here, and how confidently can I classify it?”
