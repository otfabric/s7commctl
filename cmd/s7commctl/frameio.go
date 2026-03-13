package s7commctl

import (
	"fmt"
	"io"
)

func readTPKTFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if header[0] != 3 {
		return nil, fmt.Errorf("invalid tpkt version: %d", header[0])
	}

	length := int(header[2])<<8 | int(header[3])
	if length < 4 {
		return nil, fmt.Errorf("invalid tpkt length: %d", length)
	}

	frame := make([]byte, length)
	copy(frame, header)
	if _, err := io.ReadFull(r, frame[4:]); err != nil {
		return nil, err
	}
	return frame, nil
}
