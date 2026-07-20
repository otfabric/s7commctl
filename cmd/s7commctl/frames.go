package s7commctl

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/otfabric/go-s7comm/wire"
)

type frameEvent struct {
	Timestamp string             `json:"timestamp"`
	Source    string             `json:"source"`
	Summary   *wire.FrameSummary `json:"summary"`
	CotpType  string             `json:"cotp_type,omitempty"`
	Err       string             `json:"error,omitempty"`
}

func cotpTypeString(code byte) string {
	// Match high nibble of X.224 TPDU type codes used by go-cotp.
	switch code & 0xF0 {
	case 0xE0:
		return "CR"
	case 0xD0:
		return "CC"
	case 0x80:
		return "DR"
	case 0xC0:
		return "DC"
	case 0xF0:
		return "DT"
	case 0x70:
		return "ER"
	case 0x10:
		return "ED"
	case 0x60:
		return "AK"
	case 0x20:
		return "EA"
	case 0x50:
		return "RJ"
	default:
		return fmt.Sprintf("0x%02X", code)
	}
}

func writeFrameEvent(w io.Writer, format, source string, summary *wire.FrameSummary, err error) error {
	e := frameEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Source:    source,
		Summary:   summary,
	}
	if summary != nil {
		e.CotpType = cotpTypeString(summary.COTPType)
	}
	if err != nil {
		e.Err = err.Error()
	}

	switch strings.ToLower(format) {
	case "json", "jsonl":
		b, _ := json.Marshal(e)
		_, werr := fmt.Fprintln(w, string(b))
		return werr
	default:
		if err != nil {
			_, werr := fmt.Fprintf(w, "%s source=%s error=%v\n", e.Timestamp, source, err)
			return werr
		}
		cotpTypeStr := cotpTypeString(summary.COTPType)
		fnStr := wire.FunctionCodeString(summary.Function)
		errStr := wire.HeaderErrorString(summary.ErrorClass, summary.ErrorCode)
		_, werr := fmt.Fprintf(
			w,
			"%s source=%s tpdu=%d cotp=%s rosctr=0x%02X fn=0x%02X (%s) param=%d data=%d err=%s\n",
			e.Timestamp,
			source,
			summary.TPDULength,
			cotpTypeStr,
			summary.ROSCTR,
			summary.Function,
			fnStr,
			summary.ParamLength,
			summary.DataLength,
			errStr,
		)
		return werr
	}
}
