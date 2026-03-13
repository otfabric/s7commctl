s7commctl — a serious Siemens S7 protocol tool, not a toy “read DB1” script.

I’ll structure this as formal requirements so you can hand this to yourself or an AI agent and implement cleanly.

⸻

🎯 1️⃣ Scope & Positioning

s7commctl shall be:
	•	A professional S7 communication tool
	•	Supporting S7 over ISO-on-TCP (port 102)
	•	Focused on:
	•	Discovery
	•	Identification
	•	Memory access
	•	Block management
	•	Diagnostics
	•	Safe interaction

Out of scope (v1):
	•	HMI protocol reverse engineering
	•	Profinet DCP (layer 2)
	•	Safety CPU bypassing

⸻

🧠 2️⃣ Protocol Architecture Requirements

2.1 Supported Stack

TCP (102)
  → TPKT (RFC1006)
    → COTP (ISO 8073)
      → S7comm

Requirements
	•	Implement full COTP CR/CC negotiation
	•	Support configurable TSAP values
	•	Implement S7 Setup Communication
	•	Detect PDU size from Setup ACK

⸻

🧩 3️⃣ Core Capabilities (Functional Requirements)

3.1 Discovery

discover

Scan subnet for S7 devices.

Requirements:
	•	TCP port 102 detection
	•	COTP handshake validation
	•	S7 Setup confirmation
	•	Extract:
	•	PDU size
	•	TSAP accepted
	•	CPU type (if possible)

Output:

{
  "ip": "x.x.x.x",
  "s7": true,
  "pdu_size": 480,
  "tsap": "0001",
  "rack": 0,
  "slot": 1
}


⸻

3.2 Identify

identify

Use SZL (System Zone List) reads.

Must support:
	•	SZL ID 0x0011 → Module identification
	•	SZL ID 0x001C → Component identification
	•	Firmware version
	•	Order number
	•	CPU type
	•	Serial number (if available)

This is equivalent to:

modbusctl identify


⸻

3.3 Read Memory

read

Support:
	•	DB (Data Blocks)
	•	Inputs (I)
	•	Outputs (Q)
	•	Merkers (M)
	•	Timers (T)
	•	Counters (C)

Syntax example:

s7commctl read --db 1 --offset 0 --size 16
s7commctl read --area M --offset 100 --size 4

Requirements:
	•	Automatic multi-request splitting if > PDU size
	•	Big-endian decoding
	•	Optional typed decoding:
	•	BOOL
	•	INT
	•	DINT
	•	REAL
	•	STRING

⸻

3.4 Write Memory

write

Support:
	•	DB writes
	•	M area writes
	•	Output writes

Safety requirements:
	•	Default read-only mode
	•	--dangerous flag required for write
	•	Confirmation prompt unless --force

⸻

3.5 Block Management

list blocks

Support:
	•	OB
	•	DB
	•	FB
	•	FC
	•	SFB
	•	SFC

upload

Download block contents from PLC.

info block

Read metadata of block.

⸻

3.6 Diagnostics

diagnostic

Support:
	•	CPU state (RUN / STOP)
	•	Diagnostic buffer
	•	Protection level
	•	Rack/slot auto-detection

⸻

3.7 Protection & Security Detection

Detect:
	•	No protection
	•	Read protection
	•	Write protection
	•	Full protection

Flag insecure exposure.

⸻

🏗 4️⃣ Command Structure (Consistent with Your Tools)

s7commctl
├── discover
├── identify
├── read
├── write
├── list
│   ├── blocks
│   ├── db
│   └── areas
├── block
│   ├── info
│   └── upload
├── diagnostic
├── fingerprint
├── record
└── version


⸻

🔍 5️⃣ Fingerprinting Capability

fingerprint

Determine:
	•	Supported services
	•	PDU size
	•	Max parallel jobs
	•	TSAP profile
	•	CPU generation:
	•	S7-300
	•	S7-400
	•	S7-1200
	•	S7-1500
	•	S7-compatible (e.g., VIPA)

⸻

🧠 6️⃣ Technical Requirements (Implementation)

6.1 No External Heavy Dependencies

Avoid Snap7 bindings.

Pure Go implementation preferred.

Optional:
	•	ASN.1 not required (S7 is custom binary)

⸻

6.2 PDU Layer Design

Implement:
	•	COTP builder/parser
	•	S7 header builder/parser
	•	Job / Ack / Ack-Data decoding
	•	Error class/code parsing

Must decode:

ROSCTR:
  0x01 → Job
  0x03 → Ack_Data


⸻

6.3 PDU Size Handling

After Setup Communication:
	•	Store negotiated PDU size
	•	Split reads automatically
	•	Track max parallel jobs

⸻

6.4 TSAP Strategy

Support:
	•	Manual TSAP
	•	Auto rack/slot detection
	•	Brute rack 0–3, slot 0–5

⸻

🛡 7️⃣ Safety Requirements
	•	Default read-only mode
	•	Writes require explicit --write
	•	Show warning if PLC exposed publicly
	•	Rate limiting option
	•	Timeout configurable

⸻

📊 8️⃣ Output Modes

Support:
	•	Human readable
	•	JSON
	•	Quiet mode
	•	Hex dump mode

⸻

🔄 9️⃣ Advanced Features (Phase 2)
	•	Passive S7 sniff mode
	•	PCAP decode mode
	•	Record to MCAP (like modbusctl)
	•	Replay server (emulate S7 device)

⸻

🧠 What S7 Can Actually Do (Expert Insight)

S7 allows:
	•	Memory read/write
	•	Block upload/download
	•	CPU start/stop
	•	Password protection
	•	Diagnostics
	•	Time sync
	•	Firmware info
	•	Mode switching

It is much more powerful than Modbus.

And much less structured than IEC 61850.

⸻

📌 Summary

s7commctl must be:
	•	A full ISO+COTP+S7 stack
	•	Production-grade
	•	Safe-by-default
	•	Architecturally aligned with your OT toolchain
	•	Modular for integration into ot-scanner-hub
