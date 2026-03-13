# Siemens S7 Protocol Primer

This document gives a practical overview of the Siemens S7 communication stack used by `s7commctl`. It is intentionally brief and operator-focused: enough to understand how the protocol works, what the core terms mean, and how the CLI maps onto them.

---

## 1. Scope

`s7commctl` speaks the classic Siemens S7 protocol stack used by many PLCs and related devices over Ethernet.

Protocol layers:

- **TCP** on port **102**
- **TPKT** (RFC 1006)
- **COTP** (ISO transport connection protocol)
- **S7comm**

In other words:

Application data   -> S7comm
Transport wrapper  -> COTP
RFC1006 framing    -> TPKT
Network transport  -> TCP/102

This is often called:
	•	S7 over ISO-on-TCP
	•	S7comm over TCP
	•	RFC1006 transport for S7

⸻

2. High-level flow

A normal session usually looks like this:
	1.	Open a TCP connection to port 102
	2.	Exchange COTP connection messages
	3.	Negotiate S7 communication parameters with Setup Communication
	4.	Send S7 jobs such as:
	•	read variable
	•	write variable
	•	read SZL
	•	list blocks
	•	block upload
	•	diagnostics
	5.	Receive S7 acknowledgements / data responses

So “TCP reachable” does not automatically mean “usable S7 target”. A device may:
	•	accept TCP only
	•	accept TCP + COTP only
	•	accept setup communication broadly
	•	allow only a subset of S7 services
	•	expose only limited memory or metadata

That is why s7commctl separates probing, confirmation, reading, comparison, and profiling.

⸻

3. Core protocol components

3.1 TCP

The outer transport is standard TCP, usually on port 102.

What TCP reachability tells you:
	•	the endpoint is listening
	•	a network path exists
	•	the port is open

What it does not tell you:
	•	that the endpoint is a Siemens PLC
	•	that S7comm is actually available
	•	that a specific rack/slot combination is meaningful
	•	that reads, diagnostics, or block services will work

⸻

3.2 TPKT

TPKT is the RFC1006 framing layer that carries ISO transport PDUs on top of TCP.

Operationally:
	•	it adds a small length-prefixed wrapper
	•	it lets ISO transport traffic run over TCP
	•	it is required before COTP and S7comm messages can be exchanged

If TPKT parsing fails, the session is broken before meaningful S7 work can happen.

Typical tool symptoms:
	•	read TPKT header: EOF
	•	short transport reads
	•	connection reset during handshake or response

⸻

3.3 COTP

COTP is the ISO transport protocol layer above TPKT.

In practice, s7commctl uses COTP for:
	•	connection request / confirm
	•	transport setup before S7comm begins
	•	TSAP-based addressing of communication endpoints

A target may:
	•	accept the TCP connection
	•	reject the COTP connection
	•	accept COTP for many TSAP combinations without exposing useful PLC semantics

That is why “TCP-only” and “setup-accepted” are different states.

⸻

3.4 S7comm

S7comm is the Siemens application protocol carried inside COTP.

This is the layer that actually performs PLC-oriented operations such as:
	•	variable reads
	•	variable writes
	•	SZL access
	•	CPU state queries
	•	block enumeration
	•	block upload
	•	diagnostics

s7commctl is fundamentally an S7comm client with supporting tooling around discovery, probing, safety, decoding, and replay.

⸻

4. Core nomenclature

4.1 Rack and slot

Siemens devices are often addressed using rack and slot.

Conceptually:
	•	rack identifies a chassis or logical rack
	•	slot identifies a module position within that rack

In many classic Siemens setups:
	•	rack=0
	•	slot=2 or similar

However, real-world endpoints vary:
	•	some behave as expected and require a specific tuple
	•	some accept many tuples
	•	some appear rack/slot-insensitive
	•	some are gateways, protocol shims, simulators, or non-PLC endpoints speaking enough S7 to respond partially

s7commctl probe rackslot exists because rack/slot cannot always be assumed.

Important: a setup-accepted tuple is weaker evidence than a query-confirmed tuple.

⸻

4.2 TSAP

A TSAP is a transport service access point used during the COTP connection phase.

Two TSAP values are involved:
	•	local TSAP
	•	remote TSAP

In S7 environments, remote TSAP values often encode rack/slot-related addressing. The exact mapping is implementation-dependent, but operationally this means:
	•	rack/slot probing usually changes the remote TSAP used in connection attempts
	•	a successful connection can reflect acceptance of that TSAP, not necessarily a meaningful PLC endpoint

s7commctl exposes TSAP-related options because advanced users sometimes need direct control rather than inferred rack/slot addressing.

⸻

4.3 PDU size

After transport setup, S7 communication negotiates a PDU size.

A PDU is the protocol data unit exchanged by S7comm.

Why it matters:
	•	it limits request/response payload size
	•	it influences how large reads and writes can be in one operation
	•	it is useful for fingerprinting and capability assessment

A negotiated PDU size is evidence that setup communication succeeded, but not proof that higher-level operations will all work.

⸻

4.4 Job / Ack / Ack-Data

S7comm traffic commonly follows a request/response model:
	•	Job: client request
	•	Ack: acknowledgement
	•	Ack-Data: acknowledgement carrying data

You do not always need to reason about packet types directly when using s7commctl, but they explain why some operations can fail after setup:
	•	transport succeeded
	•	setup communication succeeded
	•	a specific S7 job was then rejected, truncated, or returned empty

⸻

4.5 SZL

SZL means System-Zustands-Liste (system status list).

This is a structured mechanism used by Siemens devices to expose metadata and status, such as:
	•	order number
	•	module name
	•	firmware version
	•	CPU family/type
	•	various system details

In s7commctl, SZL-backed behavior appears mainly in:
	•	identify
	•	parts of fingerprint
	•	optional probe confirmation strategies

Important caveat:
	•	not all targets expose the same SZL fields
	•	some return only partial data
	•	some support setup/read operations but expose little or no useful SZL metadata

So "N/A" or null fields do not necessarily mean “not Siemens”; they may simply mean “not exposed”.

⸻

4.6 Areas

S7 variable access is organized into areas.

Common areas supported by s7commctl include:
	•	DB — Data Blocks
	•	I — Inputs
	•	Q — Outputs
	•	M — Merkers / flags / internal memory
	•	T — Timers
	•	C — Counters

These are not equivalent:

DB

Structured application data blocks. Often the most useful area for program data.

I

Process inputs. Reflect incoming field/device state.

Q

Process outputs. Reflect outgoing PLC command/output state.

M

Internal memory bits/bytes/words used by the PLC program.

T / C

Special timer and counter areas.

A device may expose some areas but not others, or only expose short readable prefixes.

That is why scan read-ranges is useful.

⸻

4.7 Offsets and size

Reads are performed using:
	•	an offset
	•	a size

For example:
	•	read 16 bytes at offset 0 from DB1
	•	read 8 bytes at offset 32 from area I

In practice, several outcomes are possible:
	•	full success
	•	short read
	•	empty read
	•	explicit rejection
	•	transport/protocol failure

This distinction is important because many industrial targets do not behave like simple flat memory servers.

⸻

4.8 Blocks

Siemens PLC programs are organized into blocks.

Common block types include:
	•	OB — Organization Blocks
	•	DB — Data Blocks
	•	FB — Function Blocks
	•	FC — Functions
	•	SFB — System Function Blocks
	•	SFC — System Functions

s7commctl uses these concepts for:
	•	list blocks
	•	list db
	•	block info
	•	block upload

Whether block operations work depends on:
	•	device capabilities
	•	protection level
	•	CPU family/model
	•	access restrictions

⸻

4.9 Diagnostics

Diagnostics refers to PLC/system-oriented service access such as:
	•	CPU state
	•	protection level
	•	diagnostic buffer

These services are often more restricted than ordinary reads.

So it is normal to encounter targets where:
	•	reads work
	•	identify works partially
	•	diagnostic buffer fails
	•	block list is empty or unsupported

This does not necessarily mean the CLI is wrong; it often reflects the endpoint’s actual S7 surface.

⸻

5. How s7commctl interprets protocol behavior

5.1 Setup-accepted vs query-confirmed

A key design principle in s7commctl is to distinguish transport/setup success from meaningful higher-level confirmation.

Setup-accepted

The endpoint accepted transport setup and S7 communication setup.

This proves:
	•	TCP worked
	•	TPKT/COTP worked
	•	setup communication worked

This does not prove:
	•	the rack/slot is semantically correct
	•	SZL works
	•	reads work
	•	diagnostics work
	•	the endpoint is a full PLC

Query-confirmed

A follow-up S7 operation also succeeded, such as:
	•	SZL read
	•	CPU-state query
	•	another benign confirmation action

This is stronger evidence that the candidate is genuinely usable.

⸻

5.2 Read result semantics

Reads are intentionally classified instead of treating every non-error as success.

Important result types:
	•	success — requested bytes returned
	•	short-read — some bytes returned, fewer than requested
	•	empty-read — request accepted but target returned no payload
	•	rejected — target explicitly refused the request
	•	timeout / transport / protocol error — lower-level failure
	•	inconclusive — behavior insufficiently stable or clear

This matters because industrial endpoints are often partial, inconsistent, or rate-sensitive.

⸻

5.3 Rack/slot-insensitive behavior

If many rack/slot combinations:
	•	accept setup
	•	return the same data
	•	fail confirmation in the same way

then the endpoint may be:
	•	rack/slot-insensitive
	•	fronted by a gateway
	•	exposing a limited compatibility layer
	•	not behaving like a classic chassis/CPU model

That is why compare read and profile exist: they help distinguish “one correct tuple” from “many tuples behave the same”.

⸻

6. Practical command mapping

How the CLI maps to protocol concepts:

discover

Finds reachable endpoints and optionally probes rack/slot candidates.

probe rackslot

Tests rack/slot-derived connection tuples and classifies behavior as:
	•	TCP-only
	•	setup-accepted
	•	query-confirmed
	•	unstable / failed

identify

Uses SZL-oriented access to obtain metadata such as:
	•	order number
	•	module name
	•	firmware
	•	CPU type

read

Performs variable reads against:
	•	DB
	•	I
	•	Q
	•	M
	•	T
	•	C

write

Performs variable writes with explicit safety controls.

scan read-ranges

Finds readable, empty, short, or failed spans in an area.

compare read

Runs the same read over multiple candidate rack/slot tuples to see whether results differ.

list blocks, block info, block upload

Use block-oriented S7 services.

diagnostic

Uses diagnostic-oriented S7 services such as CPU state and diagnostic buffer.

fingerprint

Summarizes communication characteristics such as:
	•	PDU size
	•	TSAP behavior
	•	possible CPU family heuristics

sniff, pcap-decode, replay

Support protocol inspection, offline analysis, and deterministic testing.

⸻

7. Common operator misconceptions

“Port 102 is open, so the PLC is fully accessible.”

Not necessarily. TCP reachability is only the outermost layer.

“One accepted rack/slot means it is the correct rack/slot.”

Not necessarily. Setup acceptance can be broad and non-discriminating.

“If identify returns N/A, the target is not Siemens.”

Not necessarily. Many targets expose partial or no SZL metadata.

“If reads work, diagnostics and blocks should also work.”

Not necessarily. Different service families may be independently restricted.

“All memory areas should behave like a flat address space.”

Not true. Some areas may expose only short readable prefixes or no data at all.

⸻

8. Handling guidance

When interacting with an unknown endpoint, a sensible workflow is:
	1.	Probe setup behavior
	•	probe rackslot
	2.	Check for meaningful confirmation
	•	probe rackslot --strict
	3.	Try identification
	•	identify
	4.	Try bounded reads
	•	small reads from I, Q, M, and DB
	5.	Scan readable ranges
	•	scan read-ranges
	6.	Compare across candidate tuples
	•	compare read
	7.	Treat writes as hazardous
	•	only in authorized test/maintenance contexts

This matches the reality that S7 endpoints vary widely in how much they expose.

⸻

9. Safety note

S7comm is an operational industrial protocol. Even seemingly harmless actions can stress fragile devices or reveal exposed infrastructure.

Rules of thumb:
	•	prefer read-only operations first
	•	use small bounded reads
	•	rate-limit when probing large spaces
	•	avoid writes unless explicitly authorized
	•	treat public internet exposure as high risk
	•	assume some devices are unstable or fronted by gateways

s7commctl includes safety-oriented behavior, but protocol-level caution remains the operator’s responsibility.

⸻

10. Summary

At its core, S7 communication is:
	•	TCP/102
	•	carrying TPKT
	•	carrying COTP
	•	carrying S7comm

The important practical concepts are:
	•	rack/slot
	•	TSAP
	•	PDU size
	•	SZL
	•	areas (DB, I, Q, M, T, C)
	•	blocks
	•	diagnostic services
	•	distinction between setup acceptance and meaningful protocol confirmation

s7commctl is built around that distinction: not just “can I connect?”, but “what kind of S7 behavior is actually present here, and how confidently can I classify it?”

⸻


A few good companion files around this would be:

- `PROTOCOL.md` — what S7comm is
- `ARCHITECTURE.md` — how `s7commctl` and `otfabric/s7comm` are split
- `SAFETY.md` — operational and ethical guidance for probing/writing
- `GLOSSARY.md` — short alphabetic terms like TSAP, SZL, OB, FB, PDU, etc.

I can also produce a tighter version of your `README.md` that links cleanly to `PROTOCOL.md`, `SAFETY.md`, and `REQUIREMENTS.md`.