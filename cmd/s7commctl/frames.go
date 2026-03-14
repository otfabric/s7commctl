package s7commctl

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/otfabric/go-cotp"
	"github.com/otfabric/s7comm/wire"
)

type frameEvent struct {
	Timestamp string             `json:"timestamp"`
	Source    string             `json:"source"`
	Summary   *wire.FrameSummary `json:"summary"`
	CotpType  string             `json:"cotp_type,omitempty"`
	Err       string             `json:"error,omitempty"`
}

func writeFrameEvent(w io.Writer, format, source string, summary *wire.FrameSummary, err error) error {
	e := frameEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Source:    source,
		Summary:   summary,
	}
	if summary != nil {
		e.CotpType = cotp.TPDUType(summary.COTPType).String()
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
		cotpTypeStr := cotp.TPDUType(summary.COTPType).String()
		_, werr := fmt.Fprintf(
			w,
			"%s source=%s tpkt=%d cotp=%s rosctr=0x%02X fn=0x%02X param=%d data=%d errClass=0x%02X errCode=0x%02X\n",
			e.Timestamp,
			source,
			summary.TPKTLength,
			cotpTypeStr,
			summary.ROSCTR,
			summary.Function,
			summary.ParamLength,
			summary.DataLength,
			summary.ErrorClass,
			summary.ErrorCode,
		)
		return werr
	}
}
