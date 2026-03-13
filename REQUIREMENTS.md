Yes. The current behavior shows two real product gaps:
	1.	range discovery for readable memory regions
	2.	better read result semantics/output, especially partial, empty, and failed reads

And there are a few other improvements that naturally follow from what you observed.

Below is a clean requirements set you can drop into your project docs.

Requirements: Readability Discovery and Read Result Quality

Background

Current commands can successfully establish S7 setup and read some memory areas, but several operator-facing problems remain:
	•	the user often does not know which offsets/ranges are actually readable
	•	some reads return partial or empty payloads without clear explanation
	•	current output does not clearly distinguish:
	•	successful reads
	•	short reads
	•	empty reads
	•	failed reads
	•	unsupported/out-of-range requests
	•	rack/slot may not be a meaningful discriminator on some endpoints
	•	some areas (for example I/Q/M) may expose only a limited window, while DB works more broadly

The CLI should help operators discover usable memory regions safely and understand read outcomes precisely.

⸻

Goals

The system shall:
	•	help users find readable address ranges without prior PLC memory knowledge
	•	classify read outcomes clearly and honestly
	•	expose partial/empty/failed results explicitly
	•	provide operator-friendly and machine-readable output
	•	remain safe and read-only by default

⸻

Scope

This improvement applies to:
	•	public Go API in otfabric/s7comm
	•	CLI in otfabric/s7commctl

Focus areas:
	•	read behavior
	•	readable range discovery
	•	output/result semantics
	•	operator diagnostics

⸻

Functional requirements

FR-1 Readable range discovery command

The CLI shall provide a command that scans an area and identifies ranges that are meaningfully readable.

Recommended command shape:

s7commctl scan read-ranges

or

s7commctl probe ranges

Inputs should include:
	•	--ip
	•	--rack
	•	--slot
	•	--area
	•	--db for DB area
	•	--start
	•	--end
	•	--step
	•	--probe-size
	•	--timeout
	•	--format

FR-2 Meaningful readable range definition

The system shall treat a region as readable if the target returns actual data for the requested read probe.

“Meaningful” shall include:
	•	non-zero data
	•	all-zero data
	•	stable repeated data
	•	changing live data

A region shall not be considered meaningfully readable if:
	•	zero bytes are returned for a non-zero request
	•	the request fails
	•	the target explicitly rejects the request
	•	the response is malformed or shorter than protocol minimums

FR-3 Readability classification

The system shall classify each probed range into at least:
	•	readable
	•	short-read
	•	empty-read
	•	rejected
	•	timeout
	•	transport-error
	•	protocol-error

FR-4 Range consolidation

The range discovery command shall consolidate adjacent readable probe windows into larger readable spans.

Example:
	•	probes at offsets 0, 8, 16 are readable
	•	probe at 24 fails

Then the output should summarize a readable span such as:
	•	offset 0..23 readable

FR-5 Area-specific behavior reporting

The system shall report discovered behavior per area independently.

Examples:
	•	DB1 readable from 0..127
	•	I readable from 0..7 only
	•	Q readable from 0..7 only
	•	M readable from 0..15 only

FR-6 Optional content heuristics

The system may optionally classify readable regions using heuristics such as:
	•	all-zero
	•	mostly-zero
	•	changing
	•	stable
	•	patterned

These heuristics must not affect whether a region is considered readable.

FR-7 Repeated probe mode

The system should support repeated reads for the same range to determine whether returned data is stable or changing.

Suggested flags:
	•	--repeat N
	•	--interval-ms N

FR-8 Safe-by-default scanning

The range discovery feature shall be strictly read-only and shall not perform writes.

FR-9 Size-aware read validation

For all read commands, the system shall compare:
	•	requested byte count
	•	returned byte count

and classify the result accordingly.

FR-10 Explicit empty-read handling

If a non-zero-sized read returns zero data bytes, the system shall not present this as ordinary success.

It shall be classified explicitly as:
	•	empty-read
or a more specific status if known.

FR-11 Explicit short-read handling

If the returned data length is smaller than the requested size, the result shall be classified as:
	•	short-read

and both requested and returned sizes shall be shown.

FR-12 Explicit success handling

If the returned data length matches the requested size, the result shall be classified as:
	•	success

FR-13 Mixed outcome support

If a future multi-item read returns mixed item results, the system shall support reporting:
	•	successful items
	•	failed items
	•	short/empty items
individually

FR-14 Human-readable output sections

For read commands, human-readable output shall clearly separate:
	•	request metadata
	•	success section
	•	warnings section
	•	failure section

FR-15 Machine-readable output sections

JSON output for reads shall include at least:
	•	target metadata
	•	request metadata
	•	status
	•	requested length
	•	returned length
	•	raw hex
	•	warnings
	•	error text if applicable

FR-16 Read summary for scan results

The range scan command shall summarize:
	•	readable spans
	•	empty spans
	•	failed spans
	•	inconclusive spans

FR-17 Inconclusive result classification

The system shall support an inconclusive classification for ranges where transport instability or retries prevent a clear verdict.

FR-18 Retry support

The system should support retries for range scanning and ordinary reads.

Suggested flags:
	•	--retries
	•	--retry-delay-ms

FR-19 Read result consistency checks

The system should support optional repeated reads to identify:
	•	stable values
	•	changing values
	•	inconsistent/flaky responses

FR-20 Rack/slot-insensitive endpoint reporting

If the same benign read across multiple rack/slot combinations returns identical data and behavior, the system should be able to classify the endpoint as:
	•	rackslot-insensitive
or equivalent wording.

⸻

CLI requirements

CLI-1 New range discovery command

The CLI shall expose a command for readable range discovery.

Recommended forms:

s7commctl scan read-ranges

or

s7commctl probe read-ranges

Recommended submodes:
	•	span
	•	map

Examples:

s7commctl scan read-ranges --ip 192.168.1.10 --area I --start 0 --end 256 --probe-size 8
s7commctl scan read-ranges --ip 192.168.1.10 --area DB --db 1 --start 0 --end 1024 --probe-size 32

CLI-2 Better read output

The read command shall present:
	•	request
	•	result classification
	•	returned length
	•	payload
	•	warnings/errors

Example desired plain output:

Target: 89.24.18.184
Area: I
Offset: 32
Requested: 16 bytes

Result: empty-read
Returned: 0 bytes
Warning: target returned no data for requested range

CLI-3 Better read JSON

Example desired JSON:

{
  "target": {
    "ip": "89.24.18.184",
    "rack": 0,
    "slot": 1,
    "area": "I",
    "db": 0
  },
  "request": {
    "offset": 32,
    "size": 16
  },
  "result": {
    "status": "empty-read",
    "requested_length": 16,
    "returned_length": 0,
    "raw_hex": "",
    "warnings": [
      "target returned no data for requested range"
    ]
  }
}

CLI-4 Summary modes for scanning

The range scan command shall support at least:
	•	detailed per-probe output
	•	consolidated span output
	•	JSON output

CLI-5 Compare mode

The CLI should support a compare mode for checking whether different rack/slot pairs expose different data.

Example:

s7commctl compare read --ip 192.168.1.10 --area I --offset 0 --size 8 --candidates 0/0,0/1,0/2

CLI-6 Honest wording

The CLI shall avoid language that implies certainty beyond the evidence.

Examples:
	•	prefer setup accepted
	•	prefer query confirmed
	•	prefer readable span detected
	•	avoid implying full validity when only setup succeeded

⸻

API requirements

API-1 Read result model

The public API shall expose a richer read result model.

Suggested shape:

type ReadStatus string

const (
    ReadStatusSuccess      ReadStatus = "success"
    ReadStatusShortRead    ReadStatus = "short-read"
    ReadStatusEmptyRead    ReadStatus = "empty-read"
    ReadStatusRejected     ReadStatus = "rejected"
    ReadStatusTimeout      ReadStatus = "timeout"
    ReadStatusTransportErr ReadStatus = "transport-error"
    ReadStatusProtocolErr  ReadStatus = "protocol-error"
    ReadStatusInconclusive ReadStatus = "inconclusive"
)

type ReadResult struct {
    Status          ReadStatus
    RequestedLength int
    ReturnedLength  int
    Data            []byte
    Warnings        []string
    Error           string

    // optional protocol detail
    ItemStatus      string
}

API-2 Range scan API

The public API shall expose a range scan capability.

Suggested shape:

type RangeProbeRequest struct {
    Address     string
    Port        int
    Rack        int
    Slot        int
    Area        MemoryArea
    DBNumber    int
    Start       int
    End         int
    Step        int
    ProbeSize   int
    Timeout     time.Duration
    Retries     int
    Repeat      int
    Interval    time.Duration
}

type ReadableSpan struct {
    Start      int
    End        int
    Status     ReadStatus
    Stable     *bool
    AllZero    *bool
    Notes      []string
}

type RangeProbeResult struct {
    Spans      []ReadableSpan
    Probes     []ReadProbeObservation
}

API-3 Per-probe observations

The API should retain raw per-probe observations in addition to consolidated spans.

API-4 Protocol detail exposure

Where available, the API should expose protocol-level item return/result codes for read requests.

API-5 Strict result semantics

The API shall not silently normalize:
	•	empty reads
	•	short reads
	•	malformed responses
into plain success.

API-6 Repeat/consistency support

The API should support repeated reads and consistency classification.

⸻

Output and UX requirements

UX-1 Plain output clarity

Plain output shall always show:
	•	requested size
	•	returned size
	•	status

UX-2 Hex output clarity

Hex output shall still include a small status header before raw data when the result is not a perfect full-length success.

UX-3 No silent empties

The CLI shall not emit “nothing” for a read command without also printing an explicit status.

UX-4 Warning separation

Warnings such as “publicly routable target” shall not obscure the actual command result.

UX-5 Consistent formatting

Human-readable and JSON output should encode the same semantic result.

⸻

Non-functional requirements

NFR-1 Safety

All discovery/scanning commands shall remain read-only.

NFR-2 Predictability

Result classifications shall be deterministic and documented.

NFR-3 Honest reporting

The tool shall avoid overstating confidence or interpreting ambiguous behavior as certainty.

NFR-4 Performance

Range scanning shall support bounded step sizes and optional parallelism without overwhelming the target.

NFR-5 Extensibility

The result model shall allow future extension for:
	•	multi-item reads
	•	typed decoding
	•	vendor-specific diagnostics

⸻

Recommended new commands

1. scan read-ranges

Find readable spans.

Examples:

s7commctl scan read-ranges --ip 89.24.18.184 --rack 0 --slot 1 --area I --start 0 --end 256 --probe-size 8
s7commctl scan read-ranges --ip 89.24.18.184 --rack 0 --slot 1 --area DB --db 1 --start 0 --end 1024 --probe-size 32

2. compare read

Compare the same read across rack/slot candidates.

s7commctl compare read --ip 89.24.18.184 --area DB --db 1 --offset 0 --size 32 --candidates 0/0,0/1,0/2

3. Optional scan behavior

Higher-level behavior profile:
	•	setup support
	•	readable areas
	•	readable spans
	•	engineering-function support
	•	rack/slot sensitivity

⸻

Other suggestions

Yes — a few more:

Add --strict-read

For read, fail the command when:
	•	returned length != requested length
	•	status is empty-read
	•	status is short-read

This is very useful for automation.

Add --allow-short

For exploratory work, allow short reads but label them clearly.

Add typed decode warnings

If the user requests a type decode, validate that returned length is enough for that type.

Example:
	•	asking for REAL but only 2 bytes returned should be an explicit error

Add capability fingerprint summary

A command that summarizes:
	•	setup support
	•	identify support
	•	diagnostics support
	•	block support
	•	readable areas
	•	rack/slot sensitivity

This would be very useful for field work.

⸻

Recommended priority order

Phase 1
	•	richer ReadResult
	•	explicit success/short-read/empty-read/failure
	•	better read plain + JSON output
	•	no more silent empty results

Phase 2
	•	scan read-ranges
	•	span consolidation
	•	repeat mode
	•	retries

Phase 3
	•	compare read
	•	rack/slot-insensitive detection
	•	capability summary command
