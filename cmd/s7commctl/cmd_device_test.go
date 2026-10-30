// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

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
	decoded := field(t, m, "result", "decoded").([]interface{})
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
	runCLI(t, p.args("read", "--offset", "-1")...).wantFailure(t, "read failed")
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
			r := runCLI(t, p.args("write", "--value", "01", "--dangerous")...)
			if !strings.Contains(r.stderr, "writes to PLC memory") {
				t.Errorf("no warning on stderr: %q", r.stderr)
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
		ok(t).has(t, "Successfully wrote 2 bytes to DB9.DBX4")
	w := p.recordedWrites()
	if len(w) != 1 {
		t.Fatalf("PLC received %d writes, want 1", len(w))
	}
	if w[0].area != byte(model.AreaDB) || w[0].db != 9 || w[0].start != 4 || !bytes.Equal(w[0].data, []byte{1, 2}) {
		t.Fatalf("write = %+v", w[0])
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
	runCLI(t, base("--value", "1", "--type", "int", "--offset", "-1")...).wantFailure(t, "write failed")

	p.writeRet = 0x05
	runCLI(t, base("--value", "01")...).wantFailure(t, "write failed")
}

func TestFormatWriteAddress(t *testing.T) {
	for _, tc := range []struct {
		area model.Area
		db   int
		off  int
		want string
	}{
		{model.AreaDB, 1, 0, "DB1.DBX0"},
		{model.AreaMerkers, 0, 100, "M100"},
		{model.AreaInputs, 0, 3, "I3"},
		{model.AreaOutputs, 0, 4, "Q4"},
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
	runCLI(t, p.args("identify")...).ok(t).has(t, "S7 Device Information", "6ES7 315-2EH14-0AB0", "CPU 315-2 PN/DP")
	runCLI(t, p.args("identify", "--format", "text")...).ok(t).has(t, "Order: 6ES7 315-2EH14-0AB0, Module: CPU 315-2 PN/DP")

	m := runCLI(t, p.args("identify", "--format", "JSON")...).ok(t).jsonOut(t)
	wantField(t, m, "6ES7 315-2EH14-0AB0", "result", "order_number")
	wantField(t, m, "CPU 315-2 PN/DP", "result", "module_name")
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
			return fakeReply{data: paddedSZL(34, "plant")}
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
	runCLI(t, p.args("diagnostic")...).ok(t).has(t, "CPU State: RUN", "Protection: Read/Write Protected", "0x4A01", "0x02", "4")
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
			return fakeReply{data: paddedSZL(34, "CPU S7-300")}
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
			return fakeReply{data: []byte{0, 0, byte(model.ProtectionNone), 0}}
		}
		return defaultSZL(id, index)
	}
	m := runCLI(t, p.args("fingerprint", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, false, "result", "supports_read")
	wantField(t, m, false, "result", "upload_probe_ok")
	wantField(t, m, true, "result", "write_likely_allowed")
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
				"--format", tc.format, "--output", out)...).ok(t).has(t, "Recorded 2 samples to "+out)
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			if len(lines) != 2 {
				t.Fatalf("got %d lines, want 2:\n%s", len(lines), b)
			}
			tc.check(t, lines)
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

func TestRecord_Failures(t *testing.T) {
	p := newFakePLC(t)
	out := filepath.Join(t.TempDir(), "rec")
	base := func(extra ...string) []string { return append(p.args("record", "--output", out), extra...) }
	runCLI(t, base("--size", "0")...).wantUsage(t, "size must be > 0")
	runCLI(t, base("--every-ms", "0")...).wantUsage(t, "every-ms must be > 0")
	runCLI(t, base("--area", "Z")...).wantUsage(t, "unknown area")
	runCLI(t, base("--local-tsap", "1")...).wantUsage(t, "must be set together")
	runCLI(t, base("--offset", "-1", "--every-ms", "1", "--count", "1")...).wantFailure(t, "record read failed")
	runCLI(t, p.args("record", "--output", filepath.Join(out, "missing", "file"))...).wantFailure(t, "open output file")
}
