// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/otfabric/go-s7comm/wire"
)

const (
	tpktVersion         = 3
	tpktHeaderLength    = 4
	tpktMinPacketLength = 7
	tpktMaxPacketLength = 65535
)

// acceptConn waits for one client. Cancelling ctx ends the wait.
func acceptConn(ctx context.Context, ln net.Listener) (net.Conn, error) {
	stop := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stop()
	conn, err := ln.Accept()
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, ExitRuntime(cerr)
		}
		return nil, ExitConnect(fmt.Errorf("accept failed: %w", err))
	}
	return conn, nil
}

// readTPKTFrame reads one TPKT frame from r and returns the full frame (header + payload).
func readTPKTFrame(r io.Reader) ([]byte, error) {
	var hdr [tpktHeaderLength]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	if hdr[0] != tpktVersion {
		return nil, fmt.Errorf("tpkt: invalid version %d", hdr[0])
	}
	totalLen := int(binary.BigEndian.Uint16(hdr[2:4]))
	if totalLen < tpktMinPacketLength || totalLen > tpktMaxPacketLength {
		return nil, fmt.Errorf("tpkt: invalid length %d", totalLen)
	}
	frame := make([]byte, totalLen)
	copy(frame, hdr[:])
	if _, err := io.ReadFull(r, frame[tpktHeaderLength:]); err != nil {
		return nil, err
	}
	return frame, nil
}

// inspectTPKTFrame peels a TPKT header and returns a wire.FrameSummary for the TPDU.
func inspectTPKTFrame(frame []byte) (*wire.FrameSummary, error) {
	tpdu, err := peelTPKT(frame)
	if err != nil {
		return nil, err
	}
	return wire.InspectTPDU(tpdu)
}

// peelTPKT validates a complete TPKT frame and returns the TPDU payload.
func peelTPKT(frame []byte) ([]byte, error) {
	if len(frame) < tpktHeaderLength {
		return nil, fmt.Errorf("tpkt: packet too short")
	}
	if frame[0] != tpktVersion {
		return nil, fmt.Errorf("tpkt: invalid version %d", frame[0])
	}
	totalLen := int(binary.BigEndian.Uint16(frame[2:4]))
	if totalLen < tpktMinPacketLength || totalLen > tpktMaxPacketLength {
		return nil, fmt.Errorf("tpkt: invalid length %d", totalLen)
	}
	if totalLen != len(frame) {
		return nil, fmt.Errorf("tpkt: length mismatch declared=%d actual=%d", totalLen, len(frame))
	}
	return frame[tpktHeaderLength:], nil
}
