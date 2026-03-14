package s7commctl

import (
	"io"

	"github.com/otfabric/go-tpkt"
)

// readTPKTFrame reads one TPKT frame from r and returns the full frame (header + payload)
// using go-tpkt.Reader and reconstructing the frame for wire.InspectFrame.
func readTPKTFrame(r io.Reader) ([]byte, error) {
	tr := tpkt.NewReader(r)
	payload, err := tr.ReadFrame()
	if err != nil {
		return nil, err
	}
	return tpkt.Encode(payload)
}
