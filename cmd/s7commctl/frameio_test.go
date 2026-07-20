// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bytes"
	"testing"
)

func TestReadTPKTFrame(t *testing.T) {
	input := []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80}
	frame, err := readTPKTFrame(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("readTPKTFrame failed: %v", err)
	}
	if len(frame) != 7 {
		t.Fatalf("expected 7 bytes, got %d", len(frame))
	}
	if frame[0] != 0x03 {
		t.Fatalf("expected tpkt version 3, got %d", frame[0])
	}
}
