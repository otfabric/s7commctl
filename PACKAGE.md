Below is a pragmatic internal design that fits your *ctl style: clean CLI, reusable library, testable wire codecs, and room for future “record/replay” like your MCAP workflows.

⸻

Internal Go package layout

Repo layout

s7commctl/
├── cmd/
│   └── s7commctl/
│       ├── main.go
│       ├── root.go
│       ├── discover.go
│       ├── identify.go
│       ├── read.go
│       ├── write.go
│       ├── list_blocks.go
│       ├── block_upload.go
│       ├── diagnostic.go
│       └── fingerprint.go
├── internal/
│   ├── app/
│   │   ├── runner.go          # command orchestration; shared flags; output formatting
│   │   ├── output.go          # json/table/text + redaction + hexdump helpers
│   │   └── errors.go
│   ├── s7/
│   │   ├── client/
│   │   │   ├── client.go      # high-level API (Connect, Identify, ReadArea, ReadDB, ...)
│   │   │   ├── options.go     # timeouts, retries, rack/slot/tsap strategy, logger
│   │   │   ├── negotiate.go   # connect sequence: TCP -> TPKT/COTP -> S7 setup
│   │   │   ├── szl.go         # SZL helpers (module ID, component ID, diag buffer)
│   │   │   ├── blocks.go      # block list/upload primitives
│   │   │   ├── readwrite.go   # read/write var implementation
│   │   │   └── fingerprint.go # probe matrix, TSAP scanning, PDU size detection
│   │   ├── wire/
│   │   │   ├── tpkt.go        # RFC1006 wrapper
│   │   │   ├── cotp.go        # ISO 8073 CR/CC + Data TPDU
│   │   │   ├── s7.go          # S7 header + job/ack parsing
│   │   │   ├── pdu_setup.go   # setup communication param codec
│   │   │   ├── pdu_rw.go      # read/write var param+data codec
│   │   │   ├── pdu_szl.go     # SZL request/response codec
│   │   │   ├── pdu_blocks.go  # list/upload codecs
│   │   │   ├── errors.go      # error class/code decoding
│   │   │   └── hexdump.go
│   │   ├── model/
│   │   │   ├── areas.go       # area codes; address model; helpers
│   │   │   ├── types.go       # BOOL/INT/DINT/REAL/STRING decoding helpers
│   │   │   ├── device.go      # Identify result model
│   │   │   └── blocks.go      # Block info model
│   │   └── transport/
│   │       ├── conn.go        # net.Conn wrapper: deadlines, tracing, metrics hooks
│   │       └── trace.go       # optional pcap-like logging (future)
│   └── util/
│       ├── netx/              # cidr iteration, concurrency pools
│       └── retry/
├── testdata/
│   ├── pcaps/                 # optional
│   └── frames/                # golden hex frames for codecs
└── go.mod

Why this layout works well for you
	•	internal/s7/wire is pure encoding/decoding (no network). Easy golden tests.
	•	internal/s7/client is semantic operations (identify/read/blocks).
	•	CLI under cmd/ stays thin, like your current tools.
	•	TSAP negotiation & rack/slot guessing live in client/negotiate.go, not scattered in commands.

⸻

Define S7 PDU structs (wire layer)

The key is: don’t model “everything” upfront. Model:
	1.	TPKT
	2.	COTP (CR/CC + Data)
	3.	S7 header (common)
	4.	A few PDUs: SetupComm, ReadVar, WriteVar, SZL, Block List/Upload

Below are structs that are directly usable for encoding/decoding.

1) TPKT (RFC1006)

package wire

type TPKT struct {
	Version  uint8  // always 0x03
	Reserved uint8  // 0x00
	Length   uint16 // total length including header
	Payload  []byte // COTP TPDU
}

2) COTP (ISO 8073 / X.224)

You need CR/CC and Data TPDU.

package wire

type COTPType uint8

const (
	COTP_CR COTPType = 0xE0
	COTP_CC COTPType = 0xD0
	COTP_DT COTPType = 0xF0 // Data TPDU
)

type COTP struct {
	// For CR/CC: header includes dstRef/srcRef/class, then parameters
	// For DT: header includes EOT + TPDU number, then payload
	Type    COTPType
	DstRef  uint16 // CR/CC only
	SrcRef  uint16 // CR/CC only
	Class   uint8  // CR/CC only (low nibble typically 0)
	EOT     bool   // DT only
	TPDUNr  uint8  // DT only (7 bits)
	Params  []COTPParam // CR/CC only
	Payload []byte      // DT only: S7 payload
}

type COTPParamCode uint8

const (
	COTPParamSrcTSAP  COTPParamCode = 0xC1
	COTPParamDstTSAP  COTPParamCode = 0xC2
	COTPParamTPDUSize COTPParamCode = 0xC0
)

type COTPParam struct {
	Code  COTPParamCode
	Value []byte // length implicit from Value
}

// Convenience strongly-typed view used by negotiate:
type COTPConnectParams struct {
	SrcTSAP  uint16
	DstTSAP  uint16
	TPDUSize uint16 // decoded as bytes (2^(n)) representation; many stacks use 0x0A (1024)
}

3) S7comm common header

This is the invariant part you’ll parse for every request/response.

package wire

type ROSCTR uint8

const (
	ROSCTR_Job     ROSCTR = 0x01
	ROSCTR_Ack     ROSCTR = 0x02
	ROSCTR_AckData ROSCTR = 0x03
	ROSCTR_UserData ROSCTR = 0x07 // used heavily for SZL and other services
)

type S7Header struct {
	ProtocolID uint8  // 0x32
	ROSCTR     ROSCTR
	Reserved   uint16 // usually 0x0000
	PDURRef    uint16 // PDU reference (you increment per request)
	ParamLen   uint16
	DataLen    uint16
	// Present depending on ROSCTR:
	ErrorClass uint8  // only in AckData (and some acks)
	ErrorCode  uint8  // only in AckData (and some acks)
	Params     []byte // raw parameter bytes
	Data       []byte // raw data bytes
}

Decode rule:
	•	If ROSCTR == AckData, header includes ErrorClass and ErrorCode fields after DataLen.
	•	For Job typically no error fields.

You’ll likely implement ParseS7Header([]byte) (S7Header, int, error) returning bytes consumed.

4) Setup Communication (Job/AckData)

Minimal typed structs for the param block.

package wire

// Parameter function codes:
type S7Function uint8
const (
	S7Func_SetupCommunication S7Function = 0xF0
	S7Func_ReadVar            S7Function = 0x04
	S7Func_WriteVar           S7Function = 0x05
	// ... keep expanding as you implement
)

type SetupCommParam struct {
	Function     S7Function // 0xF0
	Reserved     uint8      // 0x00
	MaxAMQCaller uint16     // max parallel jobs caller -> callee
	MaxAMQCallee uint16     // max parallel jobs callee -> caller
	PDUSize      uint16     // negotiated max PDU size
}

5) Read/Write Var

S7 “ReadVar” and “WriteVar” are the workhorses.

Address item (“S7ANY” style)

package wire

type S7Area uint8

const (
	AreaPE S7Area = 0x81 // Inputs (I)
	AreaPA S7Area = 0x82 // Outputs (Q)
	AreaMK S7Area = 0x83 // Merkers (M)
	AreaDB S7Area = 0x84 // Data Blocks
	AreaCT S7Area = 0x1C // Counters
	AreaTM S7Area = 0x1D // Timers
)

type TransportSize uint8

const (
	TS_Bit  TransportSize = 0x01
	TS_Byte TransportSize = 0x02
	TS_Word TransportSize = 0x04
	TS_DWord TransportSize = 0x06
	TS_Real TransportSize = 0x08
	// In S7, “transport size” vs “data type” can be nuanced; keep minimal and map in model/types.go
)

type VarSpec uint8
const (
	VarSpec_S7Any VarSpec = 0x10
)

type S7AnyAddress struct {
	Spec       VarSpec      // 0x10
	Length     uint8        // 0x0A for S7Any
	Transport  TransportSize
	Count      uint16       // number of elements (bytes/bits depending)
	DBNumber   uint16       // 0 for non-DB
	Area       S7Area
	AddressBit uint32       // 24-bit address in bits. (byteOffset*8 + bit)
}

// ReadVar parameter contains N items
type ReadVarParam struct {
	Function S7Function // 0x04
	ItemCount uint8
	Items []S7AnyAddress
}

type WriteVarParam struct {
	Function S7Function // 0x05
	ItemCount uint8
	Items []S7AnyAddress
}

ReadVar response data items

package wire

type ReturnCode uint8

const (
	Ret_OK              ReturnCode = 0xFF
	Ret_HardwareFault   ReturnCode = 0x01
	Ret_AccessDenied    ReturnCode = 0x05
	Ret_InvalidAddress  ReturnCode = 0x0A
	// keep adding as you observe them
)

type DataTransportSize uint8
// Often differs from TransportSize; keep separate to avoid confusion in codec.

type ReadVarDataItem struct {
	ReturnCode ReturnCode
	Transport  DataTransportSize
	LengthBits uint16 // length in bits in S7 responses
	Data       []byte
}

// ReadVar data section contains N items
type ReadVarData struct {
	Items []ReadVarDataItem
}

WriteVar request data items

package wire

type WriteVarDataItem struct {
	ReturnCode ReturnCode // in request: usually not present; in response: present
	Transport  DataTransportSize
	LengthBits uint16
	Data       []byte
}

type WriteVarData struct {
	Items []WriteVarDataItem
}

6) SZL (System Zone List) over UserData

Most “identify” richness comes from SZL reads. These are typically carried in ROSCTR_UserData with specific “UserData” header structures.

To keep v1 sane: represent UserData as raw + typed SZL payload when you implement it.

package wire

// Generic UserData wrapper (kept minimal; you can flesh it out as you implement more UserData services)
type UserDataParam struct {
	// This area is structured (PRM header: type/function/subfunction/seq etc).
	// Start with raw to ship v1 quickly, and add typed fields as you decode.
	Raw []byte
}

type UserData struct {
	Param UserDataParam
	Data  []byte
}

// SZL request/response payload
type SZLRequest struct {
	SZLID   uint16 // e.g. 0x0011, 0x001C
	SZLIndex uint16 // often 0x0000
}

type SZLHeader struct {
	ReturnCode ReturnCode
	Transport  DataTransportSize
	LengthBits uint16
	// Then: SZL payload
}

type SZLResponse struct {
	SZLID    uint16
	SZLIndex uint16
	Entries  []byte // raw entry bytes; parsing into fields happens in internal/s7/client/szl.go
}

7) Block list / upload (Phase-1 structs)

Again: keep wire structs minimal; parse into models in internal/s7/model.

package wire

type BlockType uint8

const (
	Block_OB  BlockType = 0x38
	Block_DB  BlockType = 0x41
	Block_FC  BlockType = 0x43
	Block_FB  BlockType = 0x42
	// etc
)

type BlockListRequest struct {
	BlockType BlockType
}

type BlockListItem struct {
	BlockType BlockType
	Number    uint16
	// optional metadata raw fields depending on response
}

type BlockUploadRequest struct {
	BlockType BlockType
	Number    uint16
}

type BlockUploadResponse struct {
	// may arrive in segments, constrained by negotiated PDU size
	Bytes []byte
	More  bool
}


⸻

Implementation notes (so these structs don’t rot)

Wire codecs must be streaming-friendly

In internal/s7/wire implement:
	•	EncodeTPKT(payload []byte) []byte
	•	ParseTPKT(b []byte) (TPKT, int, error) (int = bytes consumed)
	•	EncodeCOTPCR(params COTPConnectParams) []byte
	•	ParseCOTP(b []byte) (COTP, int, error)
	•	EncodeS7(h S7Header) []byte
	•	ParseS7(b []byte) (S7Header, int, error)

Client should never hand-craft bytes

The client should call typed encoders:
	•	EncodeSetupCommJob(...)
	•	EncodeReadVarJob(items []S7AnyAddress)
	•	EncodeWriteVarJob(items, data)
	•	EncodeSZLJob(...) (UserData wrapper inside)
	•	etc.

Parsing “just enough”

For v1: parse enough to classify, read memory, and get identity fields. Keep anything else as Raw []byte until needed.
