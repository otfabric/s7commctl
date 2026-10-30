// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/wire"
)

// startServer runs a listening command (sniff, replay) in the background and
// returns a connection to it once it listens, with a func waiting for its result.
func startServer(t *testing.T, addr string, args ...string) (net.Conn, func() cliResult) {
	t.Helper()
	done := make(chan cliResult, 1)
	go func() { done <- runCLI(t, args...) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			t.Cleanup(func() { _ = conn.Close() })
			return conn, func() cliResult {
				t.Helper()
				select {
				case r := <-done:
					return r
				case <-time.After(10 * time.Second):
					t.Fatal("command did not finish")
					return cliResult{}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("command does not listen on %s: %v", addr, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	return net.JoinHostPort("127.0.0.1", closedPort(t))
}

func TestSniff_DecodesFramesUntilClose(t *testing.T) {
	addr := freeAddr(t)
	conn, wait := startServer(t, addr, "sniff", "--listen", addr)
	if _, err := conn.Write(s7ReadRequestFrame(5)); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	r := wait().ok(t).has(t, "sniff listening on "+addr, "cotp=DT", "fn=0x04 (Read Var)", "error=EOF")
	if strings.Contains(r.stdout, "rosctr=0x00") {
		t.Errorf("S7 header not decoded:\n%s", r.stdout)
	}
}

func TestSniff_MaxFramesAndJSONL(t *testing.T) {
	addr := freeAddr(t)
	conn, wait := startServer(t, addr, "sniff", "--listen", addr, "--format", "jsonl", "--max-frames", "1")
	if _, err := conn.Write(s7ReadRequestFrame(5)); err != nil {
		t.Fatal(err)
	}
	r := wait().ok(t)
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	var ev frameEvent
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &ev); err != nil {
		t.Fatalf("last line is not JSON: %v\n%s", err, r.stdout)
	}
	if ev.CotpType != "DT" || ev.Summary == nil || ev.Summary.Function != wire.FuncReadVar {
		t.Fatalf("event = %+v", ev)
	}
}

func TestSniff_InvalidFrameIsAFailure(t *testing.T) {
	addr := freeAddr(t)
	conn, wait := startServer(t, addr, "sniff", "--listen", addr)
	if _, err := conn.Write([]byte{0x04, 0, 0, 7}); err != nil {
		t.Fatal(err)
	}
	wait().wantFailure(t, "tpkt: invalid version 4")
}

func TestSniffAndReplay_ListenFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	busy := ln.Addr().String()
	runCLI(t, "sniff", "--listen", busy).wantFailure(t, "listen failed")
	runCLI(t, "replay", "--listen", busy).wantFailure(t, "listen failed")
}

func TestReplay_AnswersFromScript(t *testing.T) {
	first := tpktFrame([]byte{0x02, 0xF0, 0x80, 0xAA})
	second := tpktFrame([]byte{0x02, 0xF0, 0x80, 0xBB})
	script := filepath.Join(t.TempDir(), "script")
	body := "# responses\n" + hex.EncodeToString(first) + "\n\n" + hex.EncodeToString(second) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	addr := freeAddr(t)
	conn, wait := startServer(t, addr, "replay", "--listen", addr, "--script", script, "--delay-ms", "1")
	reader := bufio.NewReader(conn)
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	for i, want := range [][]byte{first, second} {
		if _, err := conn.Write(s7ReadRequestFrame(uint16(i + 1))); err != nil {
			t.Fatal(err)
		}
		got := make([]byte, len(want))
		if _, err := io.ReadFull(reader, got); err != nil {
			t.Fatalf("response %d: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("response %d = % X, want % X", i, got, want)
		}
	}
	// A request past the end of the script is logged and gets no answer.
	if _, err := conn.Write(s7ReadRequestFrame(3)); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	r := wait().ok(t).has(t, "replay listening on "+addr)
	if n := strings.Count(r.stdout, "fn=0x04"); n != 3 {
		t.Fatalf("logged %d requests, want 3:\n%s", n, r.stdout)
	}
}

func TestReplay_InvalidFrameIsAFailure(t *testing.T) {
	addr := freeAddr(t)
	conn, wait := startServer(t, addr, "replay", "--listen", addr)
	if _, err := conn.Write([]byte{0x03, 0, 0, 1}); err != nil {
		t.Fatal(err)
	}
	wait().wantFailure(t, "tpkt: invalid length 1")
}

func TestReplay_BadScriptIsUsage(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, "replay", "--script", filepath.Join(dir, "missing")).wantUsage(t, "open replay script")

	for name, line := range map[string]string{"not hex": "zz", "not TPKT": "0400000702F080", "too short": "0300"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, "script")
			if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadReplayScript(path); err == nil {
				t.Fatalf("script line %q accepted", line)
			}
		})
	}
}

func TestLoadReplayScript_NoPath(t *testing.T) {
	frames, err := loadReplayScript("  ")
	if err != nil || frames != nil {
		t.Fatalf("frames = %v, err = %v", frames, err)
	}
}

func TestLoadReplayScript_LineTooLong(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(path, []byte(strings.Repeat("0", 70000)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadReplayScript(path); err == nil || !strings.Contains(err.Error(), "read replay script") {
		t.Fatalf("err = %v", err)
	}
}

func TestPcapDecode_Text(t *testing.T) {
	frame := s7ReadRequestFrame(9)
	path := writePCAP(t,
		buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, frame),
		buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 8080, frame), // other port
		buildEtherIPv4TCP(t, "10.0.0.2", "10.0.0.1", 102, 45000, nil),    // bare ACK
		[]byte{1, 2, 3}, // not Ethernet
	)
	r := runCLI(t, "pcap-decode", "--file", path).ok(t).
		has(t, "source=10.0.0.1:45000>10.0.0.2:102", "cotp=DT", "fn=0x04 (Read Var)")
	if n := strings.Count(strings.TrimSpace(r.stdout), "\n") + 1; n != 1 {
		t.Fatalf("got %d events, want 1:\n%s", n, r.stdout)
	}
}

func TestPcapDecode_UndecodableFrameIsReported(t *testing.T) {
	bad := tpktFrame([]byte{0x02, 0xF0, 0x80, 0x32, 0x01}) // S7 header cut short
	path := writePCAP(t, buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, bad))
	runCLI(t, "pcap-decode", "--file", path).ok(t).has(t, "error=parse s7 header")
}

func TestPcapDecode_Failures(t *testing.T) {
	dir := t.TempDir()
	r := runCLI(t, "pcap-decode")
	if r.err == nil || !strings.Contains(r.err.Error(), `"file" not set`) {
		t.Fatalf("err = %v", r.err)
	}
	runCLI(t, "pcap-decode", "--file", filepath.Join(dir, "missing")).wantFailure(t, "open pcap")

	garbage := filepath.Join(dir, "garbage")
	if err := os.WriteFile(garbage, make([]byte, 24), 0o600); err != nil {
		t.Fatal(err)
	}
	runCLI(t, "pcap-decode", "--file", garbage).wantFailure(t, "unsupported pcap magic")
}

func TestReadPCAPPackets_Malformed(t *testing.T) {
	valid := buildPCAP(nil)
	notEthernet := append([]byte(nil), valid...)
	notEthernet[20] = 113 // Linux cooked capture

	for _, tc := range []struct {
		name string
		blob []byte
		want string
	}{
		{"short header", valid[:10], "read pcap global header"},
		{"linktype", notEthernet, "unsupported pcap linktype 113"},
		{"truncated payload", append(buildPCAP(nil), 0, 0, 0, 0, 0, 0, 0, 0, 8, 0, 0, 0, 8, 0, 0, 0, 1), "read pcap packet payload"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := readPCAPPackets(bytes.NewReader(tc.blob)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReadPCAPPackets_TolerantEnd(t *testing.T) {
	// A capture cut inside a packet header, and an empty packet record, are not errors.
	blob := append(buildPCAP(nil), make([]byte, 16)...)
	blob = append(blob, 1, 2, 3)
	packets, err := readPCAPPackets(bytes.NewReader(blob))
	if err != nil || len(packets) != 0 {
		t.Fatalf("packets = %v, err = %v", packets, err)
	}
}

type failingReader struct{ data []byte }

func (r *failingReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, errors.New("disk gone")
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestReadPCAPPackets_ReadError(t *testing.T) {
	_, err := readPCAPPackets(&failingReader{data: buildPCAP(nil)})
	if err == nil || !strings.Contains(err.Error(), "read pcap packet header") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseEthernetIPv4TCP_Rejects(t *testing.T) {
	valid := buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 1, 2, []byte{1})
	mutate := func(i int, v byte) []byte {
		b := append([]byte(nil), valid...)
		b[i] = v
		return b
	}
	for name, pkt := range map[string][]byte{
		"short frame":       valid[:13],
		"not IPv4":          mutate(12, 0x86),
		"short IP":          valid[:30],
		"IP version 6":      mutate(14, 0x65),
		"IHL below minimum": mutate(14, 0x44),
		"IHL past the end":  mutate(14, 0x4F),
		"UDP":               mutate(23, 17),
		"short TCP":         valid[:50],
		"data offset small": mutate(46, 0x40),
		"data offset large": mutate(46, 0xF0),
	} {
		if _, _, ok := parseEthernetIPv4TCP(pkt); ok {
			t.Errorf("%s: parsed", name)
		}
	}
}

func TestPopTPKTFrame_Incomplete(t *testing.T) {
	buf := []byte{0x03, 0x00, 0x00, 0x09, 0x02}
	frame, rest, ok := popTPKTFrame(buf)
	if ok || frame != nil || !bytes.Equal(rest, buf) {
		t.Fatalf("frame=% X rest=% X ok=%v", frame, rest, ok)
	}
	if _, _, ok := popTPKTFrame(nil); ok {
		t.Fatal("frame from an empty buffer")
	}
}

// FuzzPopTPKTFrame: a frame comes out valid, and the buffer always shrinks or stops.
func FuzzPopTPKTFrame(f *testing.F) {
	f.Add(s7ReadRequestFrame(1))
	f.Add([]byte{0x09, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80})
	f.Add([]byte{0x03, 0x00, 0xFF, 0xFF})
	f.Fuzz(func(t *testing.T, buf []byte) {
		for {
			frame, rest, ok := popTPKTFrame(buf)
			if !ok {
				if len(rest) > len(buf) {
					t.Fatalf("rest grew: %d > %d", len(rest), len(buf))
				}
				return
			}
			if _, err := peelTPKT(frame); err != nil {
				t.Fatalf("invalid frame % X: %v", frame, err)
			}
			if len(frame)+len(rest) > len(buf) {
				t.Fatalf("frame %d + rest %d > input %d", len(frame), len(rest), len(buf))
			}
			buf = rest
		}
	})
}

// FuzzReadPCAPPackets: any input ends in packets or an error, and no packet is
// larger than the limit. Parsing a packet must not panic either.
func FuzzReadPCAPPackets(f *testing.F) {
	f.Add(buildPCAP(nil))
	f.Add(buildPCAP([][]byte{{1, 2, 3}}))
	f.Fuzz(func(t *testing.T, blob []byte) {
		packets, err := readPCAPPackets(bytes.NewReader(blob))
		if err != nil {
			return
		}
		for _, pkt := range packets {
			if len(pkt) > pcapMaxPacketLength {
				t.Fatalf("packet of %d bytes", len(pkt))
			}
			parseEthernetIPv4TCP(pkt)
		}
	})
}

// FuzzParseEthernetIPv4TCP: an accepted packet yields a payload inside the packet.
func FuzzParseEthernetIPv4TCP(f *testing.F) {
	f.Add([]byte{})
	f.Add(make([]byte, 54))
	f.Fuzz(func(t *testing.T, pkt []byte) {
		if _, payload, ok := parseEthernetIPv4TCP(pkt); ok && len(payload) > len(pkt)-54 {
			t.Fatalf("payload %d from packet %d", len(payload), len(pkt))
		}
	})
}

func TestReadTPKTFrame_Malformed(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []byte
		want string
	}{
		"bad version": {[]byte{4, 0, 0, 7}, "invalid version"},
		"too short":   {[]byte{3, 0, 0, 6}, "invalid length"},
	} {
		if _, err := readTPKTFrame(bytes.NewReader(tc.in)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := readTPKTFrame(bytes.NewReader([]byte{3, 0})); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("cut header: err = %v", err)
	}
	if _, err := readTPKTFrame(bytes.NewReader([]byte{3, 0, 0, 9, 1})); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("cut payload: err = %v", err)
	}
}

func TestPeelTPKT_Malformed(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []byte
		want string
	}{
		"short":       {[]byte{3, 0}, "packet too short"},
		"bad version": {[]byte{4, 0, 0, 7, 0, 0, 0}, "invalid version"},
		"bad length":  {[]byte{3, 0, 0, 2}, "invalid length"},
		"mismatch":    {[]byte{3, 0, 0, 8, 0, 0, 0}, "length mismatch declared=8 actual=7"},
	} {
		if _, err := peelTPKT(tc.in); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := inspectTPKTFrame([]byte{3, 0}); err == nil {
		t.Error("inspectTPKTFrame accepted a short frame")
	}
}

func TestCotpTypeString(t *testing.T) {
	for code, want := range map[byte]string{
		0xE0: "CR", 0xD0: "CC", 0x80: "DR", 0xC0: "DC", 0xF0: "DT", 0x70: "ER",
		0x10: "ED", 0x60: "AK", 0x20: "EA", 0x50: "RJ", 0xE5: "CR", 0x30: "0x30",
	} {
		if got := cotpTypeString(code); got != want {
			t.Errorf("cotpTypeString(0x%02X) = %q, want %q", code, got, want)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("sink gone") }

func TestWriteFrameEvent(t *testing.T) {
	summary := &wire.FrameSummary{TPDULength: 7, COTPType: 0xF0, ROSCTR: 1, Function: wire.FuncReadVar}

	var b bytes.Buffer
	if err := writeFrameEvent(&b, "TEXT", "src", summary, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "source=src tpdu=7 cotp=DT rosctr=0x01 fn=0x04") {
		t.Errorf("text event: %s", b.String())
	}

	b.Reset()
	if err := writeFrameEvent(&b, "text", "src", nil, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "source=src error=boom") {
		t.Errorf("text error: %s", b.String())
	}

	b.Reset()
	if err := writeFrameEvent(&b, "json", "src", nil, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	var ev frameEvent
	if err := json.Unmarshal(b.Bytes(), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Err != "boom" || ev.Source != "src" || ev.CotpType != "" {
		t.Errorf("json error event: %+v", ev)
	}

	if err := writeFrameEvent(failingWriter{}, "text", "src", summary, nil); err == nil {
		t.Error("write error was dropped")
	}
}

func TestWriteSample(t *testing.T) {
	var b bytes.Buffer
	result := &client.ReadResult{Data: []byte{0, 42}, Status: "short-read", RequestedLength: 4, ReturnedLength: 2, Message: "short"}
	if err := writeSample(&b, "hex", "raw", 0, result); err != nil {
		t.Fatal(err)
	}
	if b.String() != "002a\n" {
		t.Errorf("hex sample = %q", b.String())
	}

	decode := func(t *testing.T, line []byte) map[string]interface{} {
		t.Helper()
		var m map[string]interface{}
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	b.Reset()
	if err := writeSample(&b, "jsonl", "int", 3, result); err != nil {
		t.Fatal(err)
	}
	m := decode(t, b.Bytes())
	wantField(t, m, float64(3), "sample")
	wantField(t, m, "short-read", "status")
	wantField(t, m, "short", "message")
	wantField(t, m, float64(42), "decoded")

	b.Reset()
	if err := writeSample(&b, "", "raw", 0, nil); err != nil { // nil result, default format
		t.Fatal(err)
	}
	m = decode(t, b.Bytes())
	wantField(t, m, "", "status")
	wantField(t, m, "", "raw_hex")
	if _, ok := m["requested_length"]; ok {
		t.Error("requested_length without a result")
	}

	if err := writeSample(failingWriter{}, "hex", "raw", 0, result); err == nil {
		t.Error("write error was dropped")
	}
}

func TestOutputValue(t *testing.T) {
	for _, tc := range []struct {
		data []byte
		typ  string
		want interface{}
	}{
		{[]byte{0x01}, "bool", true},
		{[]byte{0x00}, "bool", false},
		{[]byte{0xFF, 0xFE}, "int", int16(-2)},
		{[]byte{0x01}, "int", nil},
		{[]byte{0, 1, 0, 0}, "dint", int32(65536)},
		{[]byte{0, 1}, "dint", nil},
		{[]byte{0x3F, 0x80, 0, 0}, "real", float32(1)},
		{[]byte{0x3F}, "real", nil},
		{[]byte{4, 2, 'o', 'k'}, "string", "ok"},
		{[]byte{1}, "unknown", nil},
	} {
		if got := outputValue(tc.data, tc.typ); got != tc.want {
			t.Errorf("outputValue(% X, %q) = %v, want %v", tc.data, tc.typ, got, tc.want)
		}
	}
}

func TestVersion(t *testing.T) {
	old := [4]string{buildVersion, buildTag, buildCommit, buildDateStr}
	t.Cleanup(func() { SetBuildInfo(old[0], old[1], old[2], old[3]) })

	SetBuildInfo("1.2.3", "", "abc", "today")
	r := runCLI(t, "version").ok(t)
	if r.stdout != "s7commctl version 1.2.3 (commit: abc, built: today)\n" {
		t.Errorf("stdout = %q", r.stdout)
	}
	SetBuildInfo("1.2.3", "v1.2.3", "abc", "today")
	r = runCLI(t, "version").ok(t)
	if r.stdout != "s7commctl version 1.2.3 (tag: v1.2.3, commit: abc, built: today)\n" {
		t.Errorf("stdout = %q", r.stdout)
	}
}

func TestExecute_Success(t *testing.T) {
	resetCommands(context.Background())
	t.Cleanup(func() { resetCommands(context.Background()) })
	rootCmd.SetArgs([]string{"version"})
	stdout, _ := captureOutput(t, Execute)
	if !strings.HasPrefix(stdout, "s7commctl version ") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestCompletion_Scripts(t *testing.T) {
	for shell, want := range map[string]string{"bash": "bash completion", "zsh": "#compdef s7commctl", "fish": "fish completion"} {
		t.Run(shell, func(t *testing.T) {
			runCLI(t, "completion", shell).ok(t).has(t, want)
		})
	}
	if r := runCLI(t, "completion", "tcsh"); r.err == nil {
		t.Error("tcsh accepted")
	}
	if r := runCLI(t, "completion"); r.err == nil {
		t.Error("missing shell accepted")
	}
}

func TestCompletion_FlagValues(t *testing.T) {
	registerCompletions(rootCmd)
	for _, tc := range []struct {
		name string
		args []string
		want []string
		not  []string
	}{
		{"area", []string{"read", "--area", "d"}, []string{"DB", "DATA_RECORD"}, []string{"M"}},
		{"format", []string{"read", "--format", "j"}, []string{"json", "jsonl"}, []string{"table"}},
		{"confirm", []string{"probe", "rackslot", "--confirm", ""}, []string{"szl", "cpu-state", "any"}, nil},
		{"profile", []string{"discover", "--profile", "q"}, []string{"quick"}, []string{"full"}},
		{"data type", []string{"write", "--type", "r"}, []string{"raw", "real"}, []string{"OB"}},
		{"block type", []string{"block", "info", "--type", "s"}, []string{"SFC", "SFB"}, []string{"string"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"__complete"}, tc.args...)
			r := runCLI(t, args...).ok(t).has(t, tc.want...)
			r.lacks(t, tc.not...)
		})
	}
}

func TestFixedCompletion(t *testing.T) {
	complete := fixedCompletion([]string{"Alpha", "beta"})
	got, directive := complete(nil, nil, "AL")
	if len(got) != 1 || got[0] != "Alpha" {
		t.Fatalf("got %v", got)
	}
	if directive == 0 {
		t.Fatal("file completion must stay off")
	}
	if got, _ := complete(nil, nil, ""); len(got) != 2 {
		t.Fatalf("empty prefix: %v", got)
	}
	if got, _ := complete(nil, nil, "z"); got != nil {
		t.Fatalf("no match: %v", got)
	}
}

func TestExitError_Contract(t *testing.T) {
	for name, fn := range map[string]func(error) error{"usage": ExitUsage, "operation": ExitOperation, "runtime": ExitRuntime} {
		if err := fn(nil); err != nil {
			t.Errorf("%s(nil) = %v, want nil", name, err)
		}
		cause := errors.New("cause")
		err := fn(cause)
		if !errors.Is(err, cause) || err.Error() != "cause" {
			t.Errorf("%s does not wrap its cause: %v", name, err)
		}
	}
	if got := (&ExitError{Code: 7}).Error(); got != "exit 7" {
		t.Errorf("Error() = %q", got)
	}
}

func TestWarnIfPublicTarget(t *testing.T) {
	const public = "8.8.8.8"
	for _, tc := range []struct {
		name, ip, detail string
		quiet, suppress  bool
		want             string
	}{
		{name: "private", ip: "10.0.0.1"},
		{name: "quiet", ip: public, quiet: true},
		{name: "suppressed", ip: public, suppress: true},
		{name: "detail", ip: public, detail: "take care", want: "WARNING: target appears publicly routable; take care\n"},
		{name: "default", ip: public, want: "WARNING: target appears publicly routable; verify authorization and network exposure\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() { warnIfPublicTarget(tc.ip, tc.quiet, tc.detail, tc.suppress) })
			if stderr != tc.want {
				t.Fatalf("stderr = %q, want %q", stderr, tc.want)
			}
		})
	}
}
