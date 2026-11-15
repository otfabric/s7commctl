// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/otfabric/go-s7comm/model"
	"github.com/otfabric/go-s7comm/wire"
)

// usageCases are the validation failures every device command shares.
func deviceUsageCases(command ...string) []struct {
	name string
	args []string
	msg  string
} {
	with := func(extra ...string) []string { return append(append([]string{}, command...), extra...) }
	return []struct {
		name string
		args []string
		msg  string
	}{
		{"missing ip", with(), "IP address is required"},
		{"invalid ip", with("--ip", "not-an-ip"), "invalid IP address"},
		{"rack out of range", with("--ip", "127.0.0.1", "--rack", "8"), "rack must be 0-7"},
		{"slot out of range", with("--ip", "127.0.0.1", "--slot", "32"), "slot must be 0-31"},
	}
}

func TestDeviceCommands_SharedValidation(t *testing.T) {
	commands := [][]string{
		{"read"}, {"write"}, {"identify"}, {"diagnostic"}, {"fingerprint"}, {"record"},
		{"list", "blocks"}, {"list", "db"}, {"blocks"}, {"block", "info"}, {"block", "upload"},
		{"scan", "read-ranges"},
	}
	for _, command := range commands {
		for _, tc := range deviceUsageCases(command...) {
			t.Run(strings.Join(command, " ")+"/"+tc.name, func(t *testing.T) {
				runCLI(t, tc.args...).wantUsage(t, tc.msg)
			})
		}
	}
}

func TestDeviceCommands_PartialTSAP(t *testing.T) {
	// A partial TSAP pair is refused before anything reaches the network.
	commands := [][]string{
		{"read"}, {"identify"}, {"diagnostic"}, {"fingerprint"},
		{"list", "blocks"}, {"block", "info", "--type", "DB", "--number", "1"},
		{"block", "upload", "--type", "DB", "--number", "1"},
		{"scan", "read-ranges"},
		{"write", "--dangerous", "--force", "--value", "01"},
	}
	for _, command := range commands {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			args := append(append([]string{}, command...), "--ip", "127.0.0.1", "--local-tsap", "0x0100")
			runCLI(t, args...).wantUsage(t, "must be set together")
		})
	}
}

func TestDeviceCommands_ConnectFailure(t *testing.T) {
	port := closedPort(t)
	commands := [][]string{
		{"read"}, {"identify"}, {"diagnostic"}, {"fingerprint"},
		{"list", "blocks"}, {"block", "info", "--type", "DB", "--number", "1"},
		{"block", "upload", "--type", "DB", "--number", "1"},
		{"scan", "read-ranges"}, {"record", "--output", filepath.Join(t.TempDir(), "r.jsonl")},
		{"write", "--dangerous", "--force", "--value", "01"},
	}
	for _, command := range commands {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			args := append(append([]string{}, command...), "--ip", "127.0.0.1", "--port", port, "--timeout", "500")
			runCLI(t, args...).wantFailure(t, "connect failed")
		})
	}
}

func TestDeviceCommands_RefusedSetupIsAConnectFailure(t *testing.T) {
	for name, adm := range map[string]admission{"no COTP": admitNoCOTP, "no S7 setup": admitNoSetup} {
		t.Run(name, func(t *testing.T) {
			p := newFakePLC(t)
			p.admit = func(int, int) admission { return adm }
			runCLI(t, p.args("identify")...).wantFailure(t, "connect failed")
		})
	}
}

func TestRead_Plain(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("read", "--offset", "4", "--size", "4")...).ok(t).
		has(t, "status=success requested=4 returned=4 area=DB offset=4", "Area: DB DB1", "04 05 06 07")
}

func TestRead_Quiet_PrintsOnlyHex(t *testing.T) {
	p := newFakePLC(t)
	r := runCLI(t, p.args("read", "--offset", "16", "--size", "2", "--quiet")...).ok(t)
	if r.stdout != "1011\n" {
		t.Fatalf("stdout = %q, want %q", r.stdout, "1011\n")
	}
}

func TestRead_Quiet_EmptyPrintsNothing(t *testing.T) {
	p := newFakePLC(t)
	p.read = func(byte, int, int, int) fakeReply { return fakeReply{} }
	r := runCLI(t, p.args("read", "--quiet")...).ok(t)
	if r.stdout != "" {
		t.Fatalf("stdout = %q, want nothing", r.stdout)
	}
}

func TestRead_JSONDecodesTypedValues(t *testing.T) {
	p := newFakePLC(t)
	p.read = func(area byte, db, start, size int) fakeReply {
		if db != 7 || area != byte(model.AreaDB) {
			t.Errorf("read of area 0x%02X db %d, want DB7", area, db)
		}
		return fakeReply{data: []byte{0x01, 0x02}}
	}
	m := runCLI(t, p.args("read", "--db", "7", "--size", "2", "--type", "int", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, "success", "result", "status")
	wantField(t, m, "0102", "result", "raw_hex")
	wantField(t, m, "127.0.0.1", "target", "ip")
	wantField(t, m, float64(7), "request", "address", "db")
	wantField(t, m, "rack_slot", "request", "connection", "connection_mode")
	decoded := field(t, m, "result", "decoded").([]any)
	if len(decoded) != 1 || decoded[0] != float64(258) {
		t.Fatalf("decoded = %v, want [258]", decoded)
	}
}

func TestRead_AreaIsSentToThePLC(t *testing.T) {
	p := newFakePLC(t)
	var got atomic.Uint32
	p.read = func(area byte, _, _, _ int) fakeReply {
		got.Store(uint32(area))
		return fakeReply{data: []byte{1}}
	}
	runCLI(t, p.args("read", "--area", "M")...).ok(t).has(t, "Area: M\n")
	if got.Load() != uint32(model.AreaMerkers) {
		t.Fatalf("area on the wire = 0x%02X, want merkers", got.Load())
	}
}

func TestRead_ConnectionModes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
		mode  string
	}{
		{"explicit TSAP", []string{"--local-tsap", "0x0100", "--remote-tsap", "0x0301"}, "explicit_tsap"},
		{"auto rack/slot", []string{"--auto-rack-slot"}, "auto_rack_slot"},
		{"TSAP skips rack validation", []string{"--local-tsap", "0x0100", "--remote-tsap", "0x0301", "--rack", "99"}, "explicit_tsap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			args := append(p.args("read", "--format", "json"), tc.extra...)
			m := runCLI(t, args...).ok(t).jsonOut(t)
			wantField(t, m, tc.mode, "request", "connection", "connection_mode")
		})
	}
}

func TestRead_NonSuccessOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reply  fakeReply
		status string
	}{
		{"short", fakeReply{data: []byte{1}}, "short-read"},
		{"empty", fakeReply{}, "empty-read"},
		{"rejected", fakeReply{ret: 0x05}, "rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			p.read = func(byte, int, int, int) fakeReply { return tc.reply }

			// Reported, but the exit code stays 0 unless --strict-read is set.
			runCLI(t, p.args("read", "--size", "4")...).ok(t).has(t, "status="+tc.status)
			r := runCLI(t, p.args("read", "--size", "4", "--strict-read")...).has(t, "status="+tc.status)
			if code := exitCodeOf(t, r.err); code != 1 {
				t.Fatalf("exit code %d, want 1", code)
			}
		})
	}
}

func TestRead_InvalidInput(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("read", "--area", "Z")...).wantUsage(t, "unknown area")
	runCLI(t, p.args("read", "--offset", "-1")...).wantUsage(t, "read failed")
}

// TestAddressOutOfS7AnyRange: a DB number or offset the S7ANY address cannot
// carry is refused before anything is sent. go-s7comm before v0.9.0 wrapped
// them, so --db 65537 wrote to DB 1.
func TestAddressOutOfS7AnyRange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
	}{
		{"db above 65535", []string{"--db", "65537"}},
		{"offset past 24-bit bit address", []string{"--offset", "2097152"}},
		// Two bytes from the last addressable byte: read --size 2, write --value 0102.
		{"range ends past 24-bit bit address", []string{"--offset", "2097151"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			runCLI(t, p.args(append([]string{"read", "--size", "2"}, tc.flags...)...)...).wantUsage(t, "read failed")
			runCLI(t, p.args(append([]string{"write", "--value", "0102", "--dangerous", "--force"}, tc.flags...)...)...).
				wantUsage(t, "write failed")
			if w := p.recordedWrites(); len(w) != 0 {
				t.Fatalf("PLC received a write: %+v", w)
			}
		})
	}
}

func TestWrite_RequiresDangerous(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("write", "--value", "01", "--force")...).wantUsage(t, "--dangerous")
	if w := p.recordedWrites(); len(w) != 0 {
		t.Fatalf("PLC received a write: %+v", w)
	}
}

func TestWrite_Confirmation(t *testing.T) {
	for _, tc := range []struct {
		name, stdin string
		writes      int
	}{
		{"declined", "no\n", 0},
		{"no input", "", 0},
		{"accepted", "yes\n", 1},
		{"accepted in capitals", " YES \n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			withStdin(t, tc.stdin)
			r := runCLI(t, p.args("write", "--db", "9", "--offset", "4", "--type", "int", "--value", "258", "--dangerous")...)
			// The prompt names what is written, where and on which PLC.
			for _, want := range []string{"writes to PLC memory", "2 bytes (int 258)", "to DB9.DBB4", "area DB", "DB 9", "offset 4",
				"on 127.0.0.1 port " + p.port, "rack 0 slot 1", "Type 'yes'"} {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("prompt lacks %q: %q", want, r.stderr)
				}
			}
			if got := len(p.recordedWrites()); got != tc.writes {
				t.Fatalf("PLC received %d writes, want %d", got, tc.writes)
			}
			if tc.writes == 0 {
				if r.err == nil || !strings.Contains(r.err.Error(), "aborted") {
					t.Fatalf("error = %v, want an abort", r.err)
				}
				// 1 today, 2 once docs/exitcodes.md moves aborts to usage.
				if code := exitCodeOf(t, r.err); code != 1 && code != 2 {
					t.Fatalf("exit code %d, want 1 or 2", code)
				}
			} else {
				r.ok(t)
			}
		})
	}
}

func TestWrite_ReachesThePLC(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("write", "--db", "9", "--offset", "4", "--type", "int", "--value", "258", "--dangerous", "--force")...).
		ok(t).has(t, "Successfully wrote 2 bytes to DB9.DBB4")
	w := p.recordedWrites()
	if len(w) != 1 {
		t.Fatalf("PLC received %d writes, want 1", len(w))
	}
	if w[0].area != byte(model.AreaDB) || w[0].db != 9 || w[0].start != 4 || !bytes.Equal(w[0].data, []byte{1, 2}) {
		t.Fatalf("write = %+v", w[0])
	}
}

// TestWrite_PromptNamesTheConnectionAndValue: the prompt shows the connection
// as it is made and the value as it was given.
func TestWrite_PromptNamesTheConnectionAndValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		want  []string
	}{
		{"merkers", []string{"--area", "M", "--offset", "100", "--value", "0102"},
			[]string{"2 bytes (raw 0102) to MB100 (area M, offset 100)", "rack 0 slot 1"}},
		{"explicit TSAP", []string{"--local-tsap", "0x0100", "--remote-tsap", "0x0301", "--value", "01"},
			[]string{"local TSAP 0x0100, remote TSAP 0x0301"}},
		{"auto rack/slot", []string{"--auto-rack-slot", "--value", "01"}, []string{"rack and slot detected automatically"}},
		{"rack and slot", []string{"--rack", "1", "--slot", "3", "--value", "01"}, []string{"rack 1 slot 3"}},
		{"bit", []string{"--area", "Q", "--offset", "2", "--type", "bool", "--bit", "5", "--value", "true"},
			[]string{"1 bit (bool true) to Q2.5 (area Q, offset 2, bit 5)"}},
		{"string", []string{"--offset", "4", "--type", "string", "--value", "abc"},
			[]string{`4 bytes (string "abc") to DB1.DBB5 (area DB, DB 1, offset 5; current length and characters of the STRING at DB1.DBB4)`}},
		{"long value", []string{"--value", strings.Repeat("ab", 70)}, []string{"(raw " + strings.Repeat("ab", 64) + "...)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			withStdin(t, "no\n")
			r := runCLI(t, p.args(append([]string{"write", "--dangerous"}, tc.flags...)...)...)
			for _, want := range tc.want {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("prompt lacks %q: %q", want, r.stderr)
				}
			}
			if w := p.recordedWrites(); len(w) != 0 {
				t.Fatalf("PLC received a write: %+v", w)
			}
		})
	}
}

// TestWrite_BitWritesOneBit: --type bool --bit N writes bit N of the byte at
// --offset with a BIT item and nothing else.
func TestWrite_BitWritesOneBit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		area  model.Area
		off   int
		bit   int
		value byte
		label string
	}{
		{"DB", []string{"--db", "1", "--offset", "4", "--bit", "3", "--value", "true"}, model.AreaDB, 4, 3, 1, "DB1.DBX4.3"},
		{"outputs", []string{"--area", "Q", "--offset", "0", "--bit", "3", "--value", "true"}, model.AreaOutputs, 0, 3, 1, "Q0.3"},
		{"merkers, false", []string{"--area", "M", "--offset", "10", "--bit", "0", "--value", "false"}, model.AreaMerkers, 10, 0, 0, "M10.0"},
		{"inputs, bit 7", []string{"--area", "I", "--offset", "1", "--bit", "7", "--value", "1"}, model.AreaInputs, 1, 7, 1, "I1.7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			args := p.args(append([]string{"write", "--type", "bool", "--dangerous", "--force"}, tc.flags...)...)
			r := runCLI(t, args...).ok(t).has(t, "Successfully wrote 1 bit to "+tc.label+"\n")
			if strings.Contains(r.stderr, "whole byte") {
				t.Errorf("whole-byte warning on a bit write: %q", r.stderr)
			}
			w := p.recordedWrites()
			if len(w) != 1 {
				t.Fatalf("PLC received %d writes, want 1: %+v", len(w), w)
			}
			if w[0].transport != 0x01 || w[0].area != byte(tc.area) || w[0].bitAddr != tc.off*8+tc.bit ||
				!bytes.Equal(w[0].data, []byte{tc.value}) {
				t.Fatalf("write = %+v, want one BIT item at bit address %d with value %d", w[0], tc.off*8+tc.bit, tc.value)
			}

			m := runCLI(t, append(args, "--format", "json")...).ok(t).jsonOut(t)
			wantField(t, m, float64(tc.bit), "request", "bit")
			wantField(t, m, float64(1), "result", "written")
		})
	}
}

func TestWrite_BitValidation(t *testing.T) {
	p := newFakePLC(t)
	base := func(extra ...string) []string {
		return p.args(append([]string{"write", "--dangerous", "--force"}, extra...)...)
	}
	for _, tc := range []struct {
		name  string
		flags []string
		msg   string
	}{
		{"bit 8", []string{"--type", "bool", "--bit", "8", "--value", "true"}, "--bit must be 0-7, got 8"},
		{"negative bit", []string{"--type", "bool", "--bit", "-1", "--value", "true"}, "--bit must be 0-7, got -1"},
		{"not bool", []string{"--type", "int", "--bit", "3", "--value", "1"}, "--bit applies to --type bool only"},
		{"raw", []string{"--bit", "0", "--value", "01"}, "--bit applies to --type bool only"},
		{"not a bool value", []string{"--type", "bool", "--bit", "3", "--value", "maybe"}, "invalid value"},
		{"negative offset", []string{"--type", "bool", "--bit", "3", "--value", "true", "--offset", "-1"}, "write failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runCLI(t, base(tc.flags...)...).wantUsage(t, tc.msg)
		})
	}
	if w := p.recordedWrites(); len(w) != 0 {
		t.Fatalf("PLC received a write: %+v", w)
	}
}

// TestWrite_BoolRequiresBit: --type bool without --bit is a usage error before
// the prompt and before connecting; --type raw writes a whole byte.
func TestWrite_BoolRequiresBit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
	}{
		{"forced", []string{"--force"}},
		{"at the prompt", nil},
		{"an invalid value too", []string{"--force", "--value", "maybe"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			withStdin(t, "yes\n")
			args := append(p.args("write", "--offset", "4", "--type", "bool", "--value", "true", "--dangerous"), tc.flags...)
			r := runCLI(t, args...)
			r.wantUsage(t, "--type bool requires --bit 0-7")
			if strings.Contains(r.stderr, "Type 'yes'") {
				t.Errorf("prompted before refusing: %q", r.stderr)
			}
			if n := p.dialCount(); n != 0 {
				t.Fatalf("connected %d times, want 0", n)
			}
		})
	}

	p := newFakePLC(t)
	runCLI(t, p.args("write", "--offset", "4", "--type", "raw", "--value", "01", "--dangerous", "--force")...).ok(t).
		has(t, "Successfully wrote 1 bytes to DB1.DBB4\n")
	if w := p.recordedWrites(); len(w) != 1 || w[0].transport != 0x02 || w[0].start != 4 || !bytes.Equal(w[0].data, []byte{1}) {
		t.Fatalf("writes = %+v, want one byte 01 at 4", w)
	}
}

// memoryPLC is a fake PLC whose reads come from mem; its writes are recorded,
// and applyWrites puts them into mem.
func memoryPLC(t *testing.T, mem []byte) *fakePLC {
	t.Helper()
	p := newFakePLC(t)
	p.read = func(_ byte, _, start, size int) fakeReply {
		return fakeReply{data: append([]byte(nil), mem[start:start+size]...)}
	}
	return p
}

func applyWrites(p *fakePLC, mem []byte) {
	for _, w := range p.recordedWrites() {
		copy(mem[w.start:], w.data)
	}
}

// TestWrite_StringKeepsTheMaximumLength: byte 0 of an S7 STRING, its declared
// maximum, is read and left as it is; a value that fits is written from byte
// 1 with the current length. A value longer than the maximum is refused and
// nothing is written.
func TestWrite_StringKeepsTheMaximumLength(t *testing.T) {
	for _, tc := range []struct {
		name string
		max  byte
		n    int
		fits bool
	}{
		{"empty", 20, 0, true},
		{"1", 20, 1, true},
		{"maximum - 1", 20, 19, true},
		{"maximum", 20, 20, true},
		{"maximum + 1", 20, 21, false},
		{"maximum 0, empty", 0, 0, true},
		{"maximum 0, 1", 0, 1, false},
		{"maximum 1, 2", 1, 2, false},
		{"254 of 254", 254, 254, true},
		{"254 of 253", 253, 254, false},
		{"254 of byte 0 = 255", 255, 254, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// DB1 holds a STRING at offset 4 with maximum tc.max, empty.
			mem := make([]byte, 300)
			mem[4] = tc.max
			p := memoryPLC(t, mem)
			value := strings.Repeat("x", tc.n)
			r := runCLI(t, p.args("write", "--offset", "4", "--type", "string", "--value", value, "--dangerous", "--force")...)
			if !tc.fits {
				r.wantUsage(t, fmt.Sprintf("the value has %d octets and the STRING at DB1.DBB4 holds at most %d", tc.n, tc.max))
				if w := p.recordedWrites(); len(w) != 0 {
					t.Fatalf("PLC received a write: %+v", w)
				}
				return
			}
			r.ok(t).has(t, fmt.Sprintf("Successfully wrote %d bytes to DB1.DBB5 (current length and characters of the STRING at DB1.DBB4)\n", tc.n+1))
			applyWrites(p, mem)
			if mem[4] != tc.max {
				t.Fatalf("the maximum length is now %d, want %d", mem[4], tc.max)
			}
			want := append([]byte{byte(tc.n)}, value...)
			if got := mem[5 : 5+len(want)]; !bytes.Equal(got, want) {
				t.Fatalf("bytes 5.. = % X, want % X", got, want)
			}
			// The PLC's PDU of 240 octets splits a long value; nothing is written below offset 5.
			if w := p.recordedWrites(); len(w) == 0 || w[0].start != 5 {
				t.Fatalf("writes = %+v, want the first at offset 5", w)
			}
		})
	}

	mem := make([]byte, 300)
	mem[4] = 20
	p := memoryPLC(t, mem)
	runCLI(t, p.args("write", "--offset", "-1", "--type", "string", "--value", "abc", "--dangerous", "--force")...).wantUsage(t, "write failed")
	if w := p.recordedWrites(); len(w) != 0 {
		t.Fatalf("PLC received a write: %+v", w)
	}
	m := runCLI(t, p.args("write", "--offset", "4", "--type", "string", "--value", "abc", "--format", "json", "--dangerous", "--force")...).
		ok(t).jsonOut(t)
	wantField(t, m, float64(4), "result", "written")
	wantField(t, m, float64(4), "request", "offset")
}

// TestWrite_StringOver254IsRefused: a value of more than 254 octets is a
// usage error before the prompt and before connecting.
func TestWrite_StringOver254IsRefused(t *testing.T) {
	for _, n := range []int{255, 256, 300} {
		for _, force := range []bool{true, false} {
			t.Run(fmt.Sprintf("%d, force %v", n, force), func(t *testing.T) {
				mem := make([]byte, 300)
				mem[4] = 255
				p := memoryPLC(t, mem)
				withStdin(t, "yes\n")
				args := p.args("write", "--offset", "4", "--type", "string", "--value", strings.Repeat("x", n), "--dangerous")
				if force {
					args = append(args, "--force")
				}
				r := runCLI(t, args...)
				r.wantUsage(t, fmt.Sprintf("the value has %d octets; an S7 STRING holds at most 254", n))
				if strings.Contains(r.stderr, "Type 'yes'") {
					t.Errorf("prompted before refusing: %q", r.stderr)
				}
				if c := p.dialCount(); c != 0 {
					t.Fatalf("connected %d times, want 0", c)
				}
			})
		}
	}
}

// TestWrite_StringMaximumReadFails: when byte 0 cannot be read, the write
// stops with the read's error and nothing is written.
func TestWrite_StringMaximumReadFails(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply fakeReply
		code  int
	}{
		{"refused by the PLC", fakeReply{ret: 0x0A}, codeProtocol},
		{"no data", fakeReply{data: []byte{}}, codeProtocol},
		{"connection closed", fakeReply{hangUp: true}, codeTransport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			p.read = func(byte, int, int, int) fakeReply { return tc.reply }
			args := p.args("write", "--offset", "4", "--type", "string", "--value", "abc", "--dangerous", "--force")
			r := runCLI(t, args...).wantFailure(t, "write failed: read the maximum length of the STRING at DB1.DBB4")
			if code := exitCodeOf(t, r.err); code != tc.code {
				t.Errorf("exit code %d, want %d (error: %v)", code, tc.code, r.err)
			}
			if w := p.recordedWrites(); len(w) != 0 {
				t.Fatalf("PLC received a write: %+v", w)
			}
		})
	}
}

func TestFormatBitAddress(t *testing.T) {
	for _, tc := range []struct {
		area         model.Area
		db, off, bit int
		want         string
	}{
		{model.AreaDB, 1, 4, 3, "DB1.DBX4.3"},
		{model.AreaOutputs, 0, 0, 3, "Q0.3"},
		{model.AreaMerkers, 0, 10, 0, "M10.0"},
		{model.AreaInputs, 0, 1, 7, "I1.7"},
		{model.AreaTimer, 0, 2, 1, "T 2.1"},
	} {
		if got := formatBitAddress(tc.area, tc.db, tc.off, tc.bit); got != tc.want {
			t.Errorf("formatBitAddress(%v, %d, %d, %d) = %q, want %q", tc.area, tc.db, tc.off, tc.bit, got, tc.want)
		}
	}
}

func TestWrite_Quiet(t *testing.T) {
	p := newFakePLC(t)
	r := runCLI(t, p.args("write", "--value", "01", "--dangerous", "--force", "--quiet")...).ok(t)
	if r.stdout != "" {
		t.Fatalf("stdout = %q, want nothing", r.stdout)
	}
	if len(p.recordedWrites()) != 1 {
		t.Fatal("write did not reach the PLC")
	}
}

func TestWrite_JSON(t *testing.T) {
	p := newFakePLC(t)
	m := runCLI(t, p.args("write", "--value", "0102", "--area", "M", "--format", "JSON", "--dangerous", "--force")...).ok(t).jsonOut(t)
	wantField(t, m, true, "result", "ok")
	wantField(t, m, float64(2), "result", "written")
	wantField(t, m, "0102", "request", "value")
	wantField(t, m, "rack_slot", "request", "connection_mode")
}

func TestWrite_JSONTruncatesLongValue(t *testing.T) {
	p := newFakePLC(t)
	value := strings.Repeat("ab", 70)
	m := runCLI(t, p.args("write", "--value", value, "--format", "json", "--dangerous", "--force")...).ok(t).jsonOut(t)
	wantField(t, m, float64(70), "result", "written")
	wantField(t, m, value[:128]+"...", "request", "value")
	wantField(t, m, true, "request", "value_truncated")
}

func TestWrite_Failures(t *testing.T) {
	p := newFakePLC(t)
	base := func(extra ...string) []string {
		return append(p.args("write", "--dangerous", "--force"), extra...)
	}
	runCLI(t, base("--value", "01", "--area", "Z")...).wantUsage(t, "unknown area")
	runCLI(t, base("--value", "xyz")...).wantUsage(t, "invalid value")
	runCLI(t, base("--value", "1", "--type", "int", "--offset", "-1")...).wantUsage(t, "write failed")
	if w := p.recordedWrites(); len(w) != 0 {
		t.Fatalf("PLC received a write: %+v", w)
	}

	p.writeRet = 0x05
	runCLI(t, base("--value", "01")...).wantFailure(t, "write failed")

	// A write-protected DB: a CPU 1214C answers item code 0x03 (go-s7comm
	// testdata/captures/python-snap7). The PLC refused it: exit 6.
	p.writeRet = 0x03
	r := runCLI(t, base("--value", "01")...).wantFailure(t, "write failed")
	if code := exitCodeOf(t, r.err); code != 6 {
		t.Fatalf("write-protected DB: exit %d, want 6", code)
	}
	if !strings.Contains(strings.ToLower(r.err.Error()), "access") {
		t.Fatalf("write-protected DB: error %q does not say access was refused", r.err)
	}
}

func TestFormatWriteAddress(t *testing.T) {
	for _, tc := range []struct {
		area model.Area
		db   int
		off  int
		want string
	}{
		{model.AreaDB, 1, 0, "DB1.DBB0"},
		{model.AreaMerkers, 0, 100, "MB100"},
		{model.AreaInputs, 0, 3, "IB3"},
		{model.AreaOutputs, 0, 4, "QB4"},
		{model.AreaTimer, 0, 20, "T20"},
		{model.AreaCounter, 0, 5, "C5"},
	} {
		if got := formatWriteAddress(tc.area, tc.db, tc.off); got != tc.want {
			t.Errorf("formatWriteAddress(%v, %d, %d) = %q, want %q", tc.area, tc.db, tc.off, got, tc.want)
		}
	}
	if got := formatWriteAddress(model.Area(wire.AreaDataRecord), 0, 7); !strings.HasSuffix(got, " 7") {
		t.Errorf("other areas end in the offset, got %q", got)
	}
}

func TestIdentify_Formats(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("identify")...).ok(t).has(t, "S7 Device Information", "6ES7 315-2EH14-0AB0", "CPU 315-2 PN/DP", "S C-B1U393142011", "MMC 4A1AC019")
	runCLI(t, p.args("identify", "--format", "text")...).ok(t).has(t, "Order: 6ES7 315-2EH14-0AB0, Module: PLC_1, CPU: CPU 315-2 PN/DP, FW: V3.2.7")

	m := runCLI(t, p.args("identify", "--format", "JSON")...).ok(t).jsonOut(t)
	wantField(t, m, "6ES7 315-2EH14-0AB0", "result", "order_number")
	wantField(t, m, "PLC_1", "result", "module_name")
	wantField(t, m, "CPU 315-2 PN/DP", "result", "module_type")
	wantField(t, m, "CPU 315-2 PN/DP", "result", "cpu_type")
	wantField(t, m, "V3.2.7", "result", "fw_version")
	wantField(t, m, "S7300/ET200M station_1", "result", "system_name")
	wantField(t, m, "MMC 4A1AC019", "result", "memory_card_serial")
	wantField(t, m, nil, "result", "plant_id")
	wantField(t, m, "rack_slot", "request", "connection_mode")

	r := runCLI(t, p.args("identify", "--quiet")...).ok(t)
	if strings.TrimSpace(r.stdout) == "" || strings.Contains(r.stdout, "Device") {
		t.Fatalf("quiet output = %q, want only the CPU type", r.stdout)
	}
}

func TestIdentify_QuietFallsBackToOrderNumber(t *testing.T) {
	p := newFakePLC(t)
	p.szl = func(id, _ uint16) fakeReply {
		if id == wire.SZLComponentID {
			return fakeReply{data: componentSZL(map[uint16]string{3: "plant"})}
		}
		return defaultSZL(id, 0)
	}
	r := runCLI(t, p.args("identify", "--quiet")...).ok(t)
	if r.stdout != "6ES7 315-2EH14-0AB0\n" {
		t.Fatalf("stdout = %q", r.stdout)
	}
}

func TestIdentifyAndFingerprint_SZLFailure(t *testing.T) {
	p := newFakePLC(t)
	p.szl = func(uint16, uint16) fakeReply { return fakeReply{ret: 0x05} }
	runCLI(t, p.args("identify")...).wantFailure(t, "identify failed")
	runCLI(t, p.args("fingerprint")...).wantFailure(t, "identify failed")
}

func TestDiagnostic_Formats(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("diagnostic")...).ok(t).has(t, "CPU State: RUN", "Protection: Read/Write Protected", "0x4A01", "0x04", "2")
	runCLI(t, p.args("diagnostic", "--format", "text")...).ok(t).has(t, "2 entries")

	m := runCLI(t, p.args("diagnostic", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, "RUN", "result", "cpu_state")
	wantField(t, m, float64(2), "result", "diagnostic", "total_count")

	r := runCLI(t, p.args("diagnostic", "--quiet")...).ok(t)
	if r.stdout != "RUN\n" {
		t.Fatalf("quiet output = %q, want RUN", r.stdout)
	}
}

func TestDiagnostic_FormatIsCaseInsensitive(t *testing.T) {
	p := newFakePLC(t)
	m := runCLI(t, p.args("diagnostic", "--format", "JSON")...).ok(t).jsonOut(t)
	wantField(t, m, "RUN", "result", "cpu_state")
}

func TestDiagnostic_ReadFailure(t *testing.T) {
	p := newFakePLC(t)
	p.szl = func(uint16, uint16) fakeReply { return fakeReply{ret: 0x05} }
	r := runCLI(t, p.args("diagnostic", "--format", "json")...).wantFailure(t, "read diagnostic buffer failed")
	wantField(t, r.jsonOut(t), "runtime", "error", "code")

	r = runCLI(t, p.args("diagnostic")...).wantFailure(t, "read diagnostic buffer failed")
	if r.stdout != "" {
		t.Fatalf("text mode printed an envelope: %q", r.stdout)
	}
}

func TestDiagnostic_ConnectFailureJSONEnvelope(t *testing.T) {
	port := closedPort(t)
	r := runCLI(t, "diagnostic", "--ip", "127.0.0.1", "--port", port, "--timeout", "500", "--format", "json").
		wantFailure(t, "connect failed")
	m := r.jsonOut(t)
	wantField(t, m, "runtime", "error", "code")
	wantField(t, m, "127.0.0.1", "target", "ip")
	if msg := field(t, m, "error", "message").(string); !strings.HasPrefix(msg, "connect failed: ") {
		t.Fatalf("message = %q", msg)
	}
}

func TestFingerprint_Formats(t *testing.T) {
	p := newFakePLC(t)
	p.szl = func(id, index uint16) fakeReply {
		if id == wire.SZLComponentID {
			return fakeReply{data: componentSZL(map[uint16]string{7: "CPU S7-300"})}
		}
		return defaultSZL(id, index)
	}
	runCLI(t, p.args("fingerprint")...).ok(t).has(t, "S7 Device Fingerprint", "PDU Size", "240", "Read/Write Protected")
	runCLI(t, p.args("fingerprint", "--format", "text")...).ok(t).has(t, "PDU: 240")

	m := runCLI(t, p.args("fingerprint", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, float64(300), "result", "cpu_generation")
	wantField(t, m, float64(240), "result", "pdu_size")
	wantField(t, m, true, "result", "supports_read")
	wantField(t, m, true, "result", "upload_probe_ok")
	wantField(t, m, false, "result", "write_likely_allowed")
	wantField(t, m, "Read/Write Protected", "result", "protection")

	r := runCLI(t, p.args("fingerprint", "--quiet")...).ok(t)
	if strings.TrimSpace(r.stdout) == "" || strings.Contains(r.stdout, "\n\n") {
		t.Fatalf("quiet output = %q", r.stdout)
	}
}

func TestFingerprint_ReflectsDeviceRestrictions(t *testing.T) {
	p := newFakePLC(t)
	p.read = func(byte, int, int, int) fakeReply { return fakeReply{ret: 0x05} }
	p.szl = func(id, index uint16) fakeReply {
		switch id {
		case wire.SZLBlockList:
			return fakeReply{ret: 0x05}
		case wire.SZLProtectionInfo:
			return fakeReply{data: protectionSZL(1)} // Siemens level 1: no protection
		}
		return defaultSZL(id, index)
	}
	m := runCLI(t, p.args("fingerprint", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, false, "result", "supports_read")
	wantField(t, m, false, "result", "upload_probe_ok")
	wantField(t, m, true, "result", "write_likely_allowed")
}

// TestProtectionUnknown: when the protection level cannot be read, fingerprint
// and diagnostic report it as unknown with the reason, and fingerprint does not
// infer that writes are allowed.
func TestProtectionUnknown(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply fakeReply
	}{
		{"refused", fakeReply{ret: 0x05}},
		{"no record", fakeReply{data: szlList(40)}},
		{"short record", fakeReply{data: szlList(6, []byte{0, 4, 0, 1, 0, 0})}},
		{"level 4", fakeReply{data: protectionSZL(4)}},
		{"level 0", fakeReply{data: protectionSZL(0)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			p.szl = func(id, index uint16) fakeReply {
				if id == wire.SZLProtectionInfo {
					return tc.reply
				}
				return defaultSZL(id, index)
			}

			m := runCLI(t, p.args("fingerprint", "--format", "json")...).ok(t).jsonOut(t)
			wantField(t, m, nil, "result", "protection")
			wantField(t, m, false, "result", "write_likely_allowed")
			if msg, _ := field(t, m, "result", "protection_error").(string); msg == "" {
				t.Errorf("fingerprint: protection_error = %v, want the reason", field(t, m, "result", "protection_error"))
			}

			r := runCLI(t, p.args("fingerprint")...).ok(t).has(t, "│ Protection      │ unknown ")
			r.lacks(t, "No Protection")
			if !strings.Contains(r.stderr, "protection level unknown: ") {
				t.Errorf("fingerprint: stderr = %q, want the reason", r.stderr)
			}

			m = runCLI(t, p.args("diagnostic", "--format", "json")...).ok(t).jsonOut(t)
			wantField(t, m, nil, "result", "protection_level")
			if msg, _ := field(t, m, "result", "protection_error").(string); msg == "" {
				t.Errorf("diagnostic: protection_error = %v, want the reason", field(t, m, "result", "protection_error"))
			}
			runCLI(t, p.args("diagnostic")...).ok(t).has(t, "Protection: unknown (").lacks(t, "No Protection")
		})
	}
}

// TestProtectionKnown: a level that was read is reported as before, with no
// protection_error.
func TestProtectionKnown(t *testing.T) {
	for _, tc := range []struct {
		level byte
		text  string
		write bool
	}{
		{1, "No Protection", true},
		{2, "Write Protected", false},
		{3, "Read/Write Protected", false},
	} {
		p := newFakePLC(t)
		p.szl = func(id, index uint16) fakeReply {
			if id == wire.SZLProtectionInfo {
				return fakeReply{data: protectionSZL(tc.level)}
			}
			return defaultSZL(id, index)
		}
		m := runCLI(t, p.args("fingerprint", "--format", "json")...).ok(t).jsonOut(t)
		wantField(t, m, tc.text, "result", "protection")
		wantField(t, m, nil, "result", "protection_error")
		wantField(t, m, tc.write, "result", "write_likely_allowed")
		m = runCLI(t, p.args("diagnostic", "--format", "json")...).ok(t).jsonOut(t)
		wantField(t, m, tc.text, "result", "protection_level")
		wantField(t, m, nil, "result", "protection_error")
		r := runCLI(t, p.args("fingerprint")...).ok(t).has(t, tc.text)
		if r.stderr != "" {
			t.Errorf("level %d: stderr = %q", tc.level, r.stderr)
		}
	}
}

// deviceTexts are identification texts with characters that are not printable:
// ESC sequences, BEL, CR, DEL, a C1 control as a lone octet and as UTF-8, and
// octets that are not UTF-8.
var deviceTexts = map[uint16]string{
	2: "PLC\x1b]0;title\x07_1",
	7: "CPU 315\x1b[2K\rX\x7f\x9b\u009b\xff\xfe",
}

// wantPrintable fails the test if out holds a control character other than a
// newline, or octets that are not UTF-8.
func wantPrintable(t *testing.T, what, out string) {
	t.Helper()
	if !utf8.ValidString(out) {
		t.Errorf("%s: output is not valid UTF-8: %q", what, out)
	}
	for _, r := range out {
		if r != '\n' && unicode.IsControl(r) {
			t.Errorf("%s: control character %U in %q", what, r, out)
			return
		}
	}
}

// TestDeviceTextsArePrinted: identification texts from the device print as
// text in text, table and quiet output; JSON keeps them as they are.
func TestDeviceTextsArePrinted(t *testing.T) {
	p := newFakePLC(t)
	p.szl = func(id, index uint16) fakeReply {
		if id == wire.SZLComponentID {
			return fakeReply{data: componentSZL(deviceTexts)}
		}
		return defaultSZL(id, index)
	}
	for _, args := range [][]string{
		{"identify"}, {"identify", "--format", "text"}, {"identify", "--quiet"},
		{"fingerprint"}, {"fingerprint", "--format", "text"}, {"fingerprint", "--quiet"},
		{"profile"},
	} {
		r := runCLI(t, p.args(args...)...).ok(t)
		wantPrintable(t, strings.Join(args, " "), r.stdout)
		if !strings.Contains(r.stdout, `CPU 315\x1b[2K\x0dX`) {
			t.Errorf("%v: the CPU type is not shown escaped:\n%s", args, r.stdout)
		}
	}
	m := runCLI(t, p.args("identify", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, "PLC\x1b]0;title\x07_1", "result", "module_name")
}

func TestClassifyCPUGeneration(t *testing.T) {
	for _, tc := range []struct {
		order, cpu string
		want       int
	}{
		{"6ES7 515-2AM01", "CPU 1500", 1500},
		{"", "s7-1200", 1200},
		{"", "CPU 400", 400},
		{"", "CPU 300", 300},
		{"", "LOGO", 0},
		{"", "CPU 315-2 PN/DP", 300},
		{"", "CPU315-2DP", 300},
		{"", "CPU 319F-3 PN/DP", 300},
		{"", "CPU 412-2", 400},
		{"", "CPU 416-3 PN/DP", 400},
		{"", "CPU 1214C DC/DC/DC", 1200},
		{"", "cpu 1516-3 pn/dp", 1500},
		{"", "CPU 1512SP-1 PN", 1500},
		{"6ES7 315-2EH14-0AB0", "", 300},
		{"6ES7315-2EH14-0AB0", "", 300},
		{"6ES7 412-2EK07-0AB0", "", 400},
		{"6ES7 516-3AN02-0AB0", "", 1500},
		{"6ES7 315-2EH14-0AB0", "N/A", 300},
		{"6ES7 412-2EK07-0AB0", "CPU 224", 400}, // a model outside the families falls through
		{"6ES7 214-1AG40-0XB0", "", 0},          // S7-200 or S7-1200: not decidable
		{"", "CPU 224", 0},
		{"", "CPU 3150", 0},
		{"", "ET200S 1300", 0}, // digits alone are not a model
		{"6ES7 300-1AA00", "", 0},
		{"", "", 0},
	} {
		if got := classifyCPUGeneration(tc.order, tc.cpu); got != tc.want {
			t.Errorf("classifyCPUGeneration(%q, %q) = %d, want %d", tc.order, tc.cpu, got, tc.want)
		}
	}
}

func TestListAreas(t *testing.T) {
	r := runCLI(t, "list", "areas").ok(t)
	got := strings.Fields(r.stdout)
	if len(got) != len(completionAreas) {
		t.Fatalf("areas = %v, want %v", got, completionAreas)
	}
	for i := range got {
		if got[i] != completionAreas[i] {
			t.Errorf("area %d = %q, want %q", i, got[i], completionAreas[i])
		}
	}
}

func TestListBlocks_AllCommandsShareOutput(t *testing.T) {
	for _, command := range [][]string{{"list", "blocks"}, {"blocks"}, {"list", "db"}, {"list", "blocks", "--type", "DB"}} {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			p := newFakePLC(t)
			runCLI(t, p.args(command...)...).ok(t).has(t, "│ DB   │      1 │", "│ DB   │      2 │")
		})
	}
}

func TestListDB_EmptyFormatMeansTable(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("list", "db", "--format", "")...).ok(t).has(t, "│ DB   │      1 │")
}

func TestListBlocks_Formats(t *testing.T) {
	p := newFakePLC(t)
	r := runCLI(t, p.args("list", "blocks", "--format", "text")...).ok(t)
	if r.stdout != "DB1\nDB2\n" {
		t.Fatalf("stdout = %q", r.stdout)
	}
	runCLI(t, p.args("list", "blocks", "--format", "json")...).ok(t).has(t, `"type": "DB"`, `"number": 2`)
}

func TestListBlocks_NoneIsAnEmptyJSONArray(t *testing.T) {
	p := newFakePLC(t)
	p.szl = func(uint16, uint16) fakeReply { return fakeReply{} }
	r := runCLI(t, p.args("list", "blocks", "--format", "json")...).ok(t)
	if strings.TrimSpace(r.stdout) != "[]" {
		t.Fatalf("stdout = %q, want []", r.stdout)
	}
}

func TestListBlocks_Failures(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("list", "blocks", "--type", "XX")...).wantUsage(t, "unknown block type")

	p.szl = func(uint16, uint16) fakeReply { return fakeReply{ret: 0x05} }
	m := runCLI(t, p.args("list", "blocks", "--format", "json", "--type", "DB")...).
		wantFailure(t, "list blocks failed").jsonOut(t)
	wantField(t, m, "runtime", "error", "code")
	wantField(t, m, "DB", "request", "block_type")

	r := runCLI(t, p.args("list", "blocks")...).wantFailure(t, "list blocks failed")
	if r.stdout != "" {
		t.Fatalf("table mode printed an envelope: %q", r.stdout)
	}
}

func TestListBlocks_ConnectFailureJSONEnvelope(t *testing.T) {
	port := closedPort(t)
	m := runCLI(t, "list", "blocks", "--ip", "127.0.0.1", "--port", port, "--timeout", "500", "--format", "json").
		wantFailure(t, "connect failed").jsonOut(t)
	wantField(t, m, "127.0.0.1", "target", "ip")
	wantField(t, m, "runtime", "error", "code")
}

func TestBlockInfo_Formats(t *testing.T) {
	p := newFakePLC(t)
	base := func(extra ...string) []string {
		return append(p.args("block", "info", "--type", "db", "--number", "5"), extra...)
	}
	runCLI(t, base()...).ok(t).has(t, "│ Type       │ DB ", "│ LoadMemory │ 100 ", "│ MC7Size    │ 80 ")
	runCLI(t, base("--format", "text")...).ok(t).has(t, "DB5 load=100 local=20 mc7=80")

	m := runCLI(t, base("--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, float64(5), "request", "number")
	wantField(t, m, float64(100), "result", "load_memory")
	wantField(t, m, float64(20), "result", "local_data")

	r := runCLI(t, base("--quiet")...).ok(t)
	if r.stdout != "DB5\n" {
		t.Fatalf("stdout = %q", r.stdout)
	}
}

func TestUpload_Formats(t *testing.T) {
	p := newFakePLC(t)
	p.upload = [][]byte{{0xDE, 0xAD}, {0xBE, 0xEF}}
	base := func(extra ...string) []string {
		return append(p.args("block", "upload", "--type", "DB", "--number", "3"), extra...)
	}
	runCLI(t, base()...).ok(t).has(t, "│ DB   │ 3      │ 4    │")
	runCLI(t, base("--format", "text")...).ok(t).has(t, "DB3 size=4\ndeadbeef\n")

	m := runCLI(t, base("--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, "deadbeef", "result", "hex")
	wantField(t, m, float64(4), "result", "size")

	if r := runCLI(t, base("--quiet")...).ok(t); r.stdout != "DB3\n" {
		t.Fatalf("quiet: stdout = %q", r.stdout)
	}
	if r := runCLI(t, base("--format", "hex")...).ok(t); r.stdout != "deadbeef\n" {
		t.Fatalf("hex: stdout = %q", r.stdout)
	}
}

func TestBlockCommands_Failures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command []string
		fail    string
	}{
		{"info", []string{"block", "info"}, "block info failed"},
		{"upload", []string{"block", "upload"}, "block upload failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			p.szl = func(uint16, uint16) fakeReply { return fakeReply{ret: 0x05} }
			runCLI(t, p.args(append(tc.command, "--type", "DB")...)...).wantUsage(t, "--number must be > 0")
			runCLI(t, p.args(append(tc.command, "--type", "XX", "--number", "1")...)...).wantUsage(t, "unknown block type")
			runCLI(t, p.args(append(tc.command, "--type", "DB", "--number", "1")...)...).wantFailure(t, tc.fail)
		})
	}
}

func TestRecord_WritesSamples(t *testing.T) {
	for _, tc := range []struct {
		name, format string
		check        func(t *testing.T, lines []string)
	}{
		{"hex", "hex", func(t *testing.T, lines []string) {
			if lines[0] != "0001" || lines[1] != "0001" {
				t.Fatalf("lines = %q", lines)
			}
		}},
		{"jsonl", "jsonl", func(t *testing.T, lines []string) {
			for i, want := range []string{`"sample":0`, `"sample":1`} {
				if !strings.Contains(lines[i], want) || !strings.Contains(lines[i], `"raw_hex":"0001"`) ||
					!strings.Contains(lines[i], `"decoded":1`) {
					t.Fatalf("line %d = %q", i, lines[i])
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			out := filepath.Join(t.TempDir(), "rec")
			runCLI(t, p.args("record", "--size", "2", "--type", "int", "--every-ms", "1", "--count", "2",
				"--format", tc.format, "--output", out)...).ok(t).has(t, "Recorded 2 samples (0 failed) to "+out+"\n")
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if fi, err := os.Stat(out); err != nil {
				t.Fatal(err)
			} else if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
				t.Fatalf("output mode %v, want 0600", fi.Mode().Perm())
			}
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			if len(lines) != 2 {
				t.Fatalf("got %d lines, want 2:\n%s", len(lines), b)
			}
			tc.check(t, lines)
		})
	}
}

// recordLines returns the lines of a record output file.
func recordLines(t *testing.T, out string) []string {
	t.Helper()
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n")), "\n")
}

// TestRecord_ConnectionLost: the PLC closes the connection after one sample.
// Every later sample fails with a transport error; after --max-failures of
// them in a row (default 5) record stops with exit 4 and says the connection
// was lost. --max-failures 0 never stops, and the failures give exit 1.
func TestRecord_ConnectionLost(t *testing.T) {
	for _, tc := range []struct {
		name    string
		flags   []string
		samples int
		stopped bool
	}{
		{"default", nil, 6, true},
		{"max-failures 1", []string{"--max-failures", "1"}, 2, true},
		{"max-failures 3", []string{"--max-failures", "3"}, 4, true},
		{"max-failures 0", []string{"--max-failures", "0"}, 12, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			p.dropAfterReads = 1
			out := filepath.Join(t.TempDir(), "rec")
			args := append(p.args("record", "--every-ms", "1", "--count", "12", "--output", out), tc.flags...)
			r := runCLI(t, args...)
			failed := tc.samples - 1
			r.has(t, fmt.Sprintf("Recorded %d samples (%d failed) to %s\n", tc.samples, failed, out))
			if tc.stopped {
				r.wantCode(t, fmt.Sprintf("the connection to 127.0.0.1 was lost (%d consecutive samples failed; the last: transport-error", failed),
					func(code int) bool { return code == codeTransport }, "4 (transport)")
			} else {
				r.wantFindings(t, fmt.Sprintf("%d of %d samples failed; the last: transport-error", failed, tc.samples))
			}
			lines := recordLines(t, out)
			if len(lines) != tc.samples {
				t.Fatalf("got %d lines, want %d", len(lines), tc.samples)
			}
			for i, line := range lines {
				want := `"status":"transport-error"`
				if i == 0 {
					want = `"status":"success"`
				}
				if !strings.Contains(line, want) {
					t.Errorf("line %d = %q, want %s", i, line, want)
				}
			}
		})
	}
}

// TestRecord_FailedSamplesAreCounted: a sample the PLC refuses is a failed
// sample (exit 1 at the end) but no lost connection: record goes on.
func TestRecord_FailedSamplesAreCounted(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refuse  func(n int32) bool
		samples int
		failed  int
	}{
		{"one of three", func(n int32) bool { return n == 2 }, 3, 1},
		{"all, more than --max-failures", func(int32) bool { return true }, 7, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePLC(t)
			var reads atomic.Int32
			p.read = func(_ byte, _, start, size int) fakeReply {
				if tc.refuse(reads.Add(1)) {
					return fakeReply{ret: 0x0A}
				}
				return fakeReply{data: patternBytes(start, size)}
			}
			out := filepath.Join(t.TempDir(), "rec")
			r := runCLI(t, p.args("record", "--every-ms", "1", "--count", fmt.Sprint(tc.samples), "--output", out)...)
			r.has(t, fmt.Sprintf("Recorded %d samples (%d failed) to %s\n", tc.samples, tc.failed, out))
			r.wantFindings(t, fmt.Sprintf("%d of %d samples failed; the last: rejected", tc.failed, tc.samples))
			if n := len(recordLines(t, out)); n != tc.samples {
				t.Fatalf("got %d lines, want %d", n, tc.samples)
			}
		})
	}
}

func TestRecord_CancelEndsCleanly(t *testing.T) {
	p := newFakePLC(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.read = func(_ byte, _, start, size int) fakeReply {
		cancel() // the first sample stops the run
		return fakeReply{data: patternBytes(start, size)}
	}
	out := filepath.Join(t.TempDir(), "rec")
	r := runCLIContext(t, ctx, p.args("record", "--every-ms", "1", "--output", out)...).ok(t)
	if strings.Contains(r.stdout, "Recorded") {
		t.Fatalf("a cancelled run printed the summary: %q", r.stdout)
	}
}

// TestRecord_KeepsTheOutputWhenItCannotStart: an existing output file is left
// as it is when record fails before it has a connection.
func TestRecord_KeepsTheOutputWhenItCannotStart(t *testing.T) {
	const previous = "previous recording\n"
	for _, tc := range []struct {
		name  string
		args  func(out string) []string
		check func(t *testing.T, r cliResult)
	}{
		{"connection refused", func(out string) []string {
			return []string{"record", "--ip", "127.0.0.1", "--port", closedPort(t), "--timeout", "500", "--output", out}
		}, func(t *testing.T, r cliResult) { r.wantFailure(t, "connect failed") }},
		{"partial TSAP", func(out string) []string {
			return []string{"record", "--ip", "127.0.0.1", "--local-tsap", "1", "--output", out}
		}, func(t *testing.T, r cliResult) { r.wantUsage(t, "must be set together") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "record.jsonl")
			if err := os.WriteFile(out, []byte(previous), 0o600); err != nil {
				t.Fatal(err)
			}
			tc.check(t, runCLI(t, tc.args(out)...))
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != previous {
				t.Fatalf("output file = %q, want it unchanged", b)
			}
		})
	}
}

func TestRecord_Failures(t *testing.T) {
	p := newFakePLC(t)
	out := filepath.Join(t.TempDir(), "rec")
	base := func(extra ...string) []string { return append(p.args("record", "--output", out), extra...) }
	runCLI(t, base("--max-failures", "-1")...).wantUsage(t, "max-failures must be >= 0")
	if n := p.dialCount(); n != 0 {
		t.Fatalf("connected %d times before refusing --max-failures -1", n)
	}
	runCLI(t, base("--size", "0")...).wantUsage(t, "size must be > 0")
	runCLI(t, base("--every-ms", "0")...).wantUsage(t, "every-ms must be > 0")
	runCLI(t, base("--area", "Z")...).wantUsage(t, "unknown area")
	runCLI(t, base("--local-tsap", "1")...).wantUsage(t, "must be set together")
	runCLI(t, base("--offset", "-1", "--every-ms", "1", "--count", "1")...).wantUsage(t, "record read failed")
	runCLI(t, p.args("record", "--output", filepath.Join(out, "missing", "file"))...).wantFailure(t, "open output file")
}
