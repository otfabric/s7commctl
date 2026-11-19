// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bytes"
	"sync/atomic"
	"testing"

	"github.com/otfabric/go-s7comm/model"
)

// On T and C, --offset is the element number and --size the number of
// elements: read --area T --offset 5 --size 2 asks for timers 5 and 6 and
// prints their four bytes.
func TestRead_TimersAndCountersByElement(t *testing.T) {
	for _, tc := range []struct {
		area string
		code model.Area
	}{{"T", model.AreaTimer}, {"C", model.AreaCounter}} {
		t.Run(tc.area, func(t *testing.T) {
			p := newFakePLC(t)
			var area, start, size atomic.Int32
			p.read = func(a byte, _, s, n int) fakeReply {
				area.Store(int32(a))
				start.Store(int32(s))
				size.Store(int32(n))
				return fakeReply{data: []byte{0x12, 0x34, 0x00, 0x07}}
			}
			runCLI(t, p.args("read", "--area", tc.area, "--offset", "5", "--size", "2", "--quiet")...).ok(t).has(t, "12340007\n")
			if model.Area(area.Load()) != tc.code || start.Load() != 5 || size.Load() != 2 {
				t.Fatalf("on the wire: area 0x%02X start %d size %d, want element 5, two elements", area.Load(), start.Load(), size.Load())
			}
		})
	}
}

// A write to C addresses the counter by element with transport size COUNTER
// and sends its two bytes.
func TestWrite_CounterByElement(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("write", "--area", "C", "--offset", "3", "--value", "0017", "--dangerous", "--force")...).ok(t)
	w := p.recordedWrites()
	if len(w) != 1 {
		t.Fatalf("PLC received %d writes, want 1", len(w))
	}
	if w[0].area != byte(model.AreaCounter) || w[0].transport != byte(model.AreaCounter) || w[0].start != 3 || !bytes.Equal(w[0].data, []byte{0x00, 0x17}) {
		t.Fatalf("write = %+v, want counter 3 by element with 00 17", w[0])
	}
}

// On T and C a value is whole elements of two bytes, and the byte-oriented
// forms (a bit, a bool, a STRING) do not apply: each is refused before
// anything reaches the network.
func TestWrite_TimersAndCountersRefuseByteForms(t *testing.T) {
	base := []string{"write", "--ip", "127.0.0.1", "--dangerous", "--force"}
	for name, extra := range map[string][]string{
		"odd length": {"--area", "T", "--value", "01"},
		"bit":        {"--area", "C", "--type", "bool", "--bit", "0", "--value", "true"},
		"bool":       {"--area", "T", "--type", "bool", "--value", "true"},
		"string":     {"--area", "C", "--type", "string", "--value", "ab"},
	} {
		t.Run(name, func(t *testing.T) {
			msg := "do not apply"
			if name == "odd length" {
				msg = "whole timers of two bytes"
			}
			runCLI(t, append(append([]string{}, base...), extra...)...).wantUsage(t, msg)
		})
	}
}
