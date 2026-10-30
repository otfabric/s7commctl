// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
)

// rangedPLC answers: 0-7 readable, 8-15 refused, 16-23 short, 24-31 empty.
func rangedPLC(t *testing.T) *fakePLC {
	t.Helper()
	p := newFakePLC(t)
	p.read = func(_ byte, _, start, size int) fakeReply {
		switch {
		case start < 8:
			return fakeReply{data: patternBytes(start, size)}
		case start < 16:
			return fakeReply{ret: 0x05}
		case start < 24:
			return fakeReply{data: patternBytes(start, size/2)}
		}
		return fakeReply{}
	}
	return p
}

func TestScan_Spans(t *testing.T) {
	p := rangedPLC(t)
	runCLI(t, p.args("scan", "read-ranges", "--end", "32", "--probe-size", "8")...).ok(t).
		has(t, "Area: DB DB1\n", "Readable spans:\n  [0..8) bytes 0-7  status=success",
			"Empty spans:\n  [24..32) bytes 24-31",
			"Partial spans (short-read):\n  [16..24) bytes 16-23  status=short-read",
			"Failed spans:\n  [8..16) bytes 8-15  status=rejected").
		lacks(t, "Offset  Status")
}

func TestScan_Table(t *testing.T) {
	p := rangedPLC(t)
	base := p.args("scan", "read-ranges", "--end", "32", "--probe-size", "8", "--format", "table")
	runCLI(t, base...).ok(t).has(t, "Offset  Status", "0       success", "Readable spans:")
	runCLI(t, append(base, "--summary-only")...).ok(t).has(t, "Readable spans:").lacks(t, "Offset  Status")
}

func TestScan_JSON(t *testing.T) {
	p := rangedPLC(t)
	m := runCLI(t, p.args("scan", "read-ranges", "--end", "32", "--probe-size", "8", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, "127.0.0.1", "target", "ip")
	wantField(t, m, float64(1), "request", "db_number")
	for _, key := range []string{"readable_count", "empty_count", "partial_count", "failed_count"} {
		wantField(t, m, float64(1), "summary", key)
	}
	wantField(t, m, float64(0), "summary", "inconclusive_count")
	spans := field(t, m, "spans", "partial").([]interface{})
	wantField(t, spans[0].(map[string]interface{}), "short-read", "status")
}

func TestScan_NonDBAreaHasNoDBNumber(t *testing.T) {
	p := newFakePLC(t)
	base := p.args("scan", "read-ranges", "--area", "M", "--end", "8", "--probe-size", "8")
	runCLI(t, base...).ok(t).lacks(t, " DB1")
	m := runCLI(t, append(base, "--format", "json")...).ok(t).jsonOut(t)
	if _, ok := field(t, m, "request").(map[string]interface{})["db_number"]; ok {
		t.Fatal("request.db_number is set for a non-DB area")
	}
}

func TestScan_MixedRepeatsAreInconclusive(t *testing.T) {
	p := newFakePLC(t)
	var n atomic.Int32
	p.read = func(_ byte, _, _, size int) fakeReply {
		return fakeReply{data: patternBytes(int(n.Add(1)), size)}
	}
	runCLI(t, p.args("scan", "read-ranges", "--end", "8", "--probe-size", "8", "--repeat", "2")...).ok(t).
		has(t, "Inconclusive spans:\n  [0..8) bytes 0-7")
}

func TestScan_Validation(t *testing.T) {
	p := newFakePLC(t)
	base := func(extra ...string) []string { return append(p.args("scan", "read-ranges"), extra...) }
	runCLI(t, base("--start", "8", "--end", "8")...).wantUsage(t, "end must be > start")
	runCLI(t, base("--probe-size", "0")...).wantUsage(t, "probe-size must be > 0")
	runCLI(t, base("--area", "Z")...).wantUsage(t, "unknown area")
	runCLI(t, base("--retries", "-1")...).wantFailure(t, "scan failed")
}

func TestSpanHuman(t *testing.T) {
	for _, tc := range []struct {
		start, end int
		want       string
	}{
		{0, 8, "bytes 0-7"},
		{5, 6, "bytes 5-5"},
		{5, 5, "bytes 5-5"},
		{9, 3, "bytes 9-9"},
	} {
		if got := spanHuman(tc.start, tc.end); got != tc.want {
			t.Errorf("spanHuman(%d, %d) = %q, want %q", tc.start, tc.end, got, tc.want)
		}
	}
}

func TestPrintScanReadRangesTable_TruncatesLongMessages(t *testing.T) {
	res := &client.RangeProbeResult{
		Area: model.AreaMerkers,
		Probes: []client.ReadProbeObservation{
			{Offset: 3, Result: client.ReadResult{Status: "rejected", Message: strings.Repeat("x", 40)}},
			{Offset: 4, Result: client.ReadResult{Status: "rejected", Message: "short"}},
		},
	}
	stdout, _ := captureOutput(t, func() { printScanReadRangesTable(res) })
	if !strings.Contains(stdout, strings.Repeat("x", 27)+"...\n") || strings.Contains(stdout, strings.Repeat("x", 28)) {
		t.Errorf("long message not cut to 30 columns:\n%s", stdout)
	}
	if !strings.Contains(stdout, "short\n") {
		t.Errorf("short message changed:\n%s", stdout)
	}
}

// probePLC accepts S7 only on rack 0 slot 2; other slots stop at COTP.
func probePLC(t *testing.T) *fakePLC {
	t.Helper()
	p := newFakePLC(t)
	p.admit = func(rack, slot int) admission {
		switch {
		case rack == 0 && slot == 2:
			return admitOK
		case slot == 1:
			return admitNoSetup
		}
		return admitNoCOTP
	}
	return p
}

func probeArgs(p *fakePLC, extra ...string) []string {
	base := p.args("probe", "rackslot", "--rack-max", "0", "--slot-max", "3", "--parallelism", "1")
	return append(base, extra...)
}

func TestProbe_Table(t *testing.T) {
	p := probePLC(t)
	runCLI(t, probeArgs(p)...).ok(t).
		has(t, "Target: 127.0.0.1", "Mode: default (setup accepted = valid)", "Setup-accepted combinations:",
			"- rack=0 slot=2 (confirmed by setup, PDU=240)", "Suggested default tuple: rack=0 slot=2",
			"Reason: setup accepted broadly")
}

func TestProbe_StrictConfirmsByQuery(t *testing.T) {
	p := probePLC(t)
	runCLI(t, probeArgs(p, "--strict", "--retries", "1")...).ok(t).
		has(t, "Mode: strict (confirm=any), retries=1", "Query-confirmed combinations:", "query-confirmed",
			"Reason: first query-confirmed candidate.", "confirmed by follow-up query (high confidence)")
}

func TestProbe_Text(t *testing.T) {
	p := probePLC(t)
	runCLI(t, probeArgs(p, "--format", "text")...).ok(t).
		has(t, "candidates=4 valid=1 setup_accepted=1", "rack=0 slot=2 stage=setup status=setup-only pdu=240")
}

func TestProbe_SummaryOnly(t *testing.T) {
	for _, format := range []string{"table", "text"} {
		t.Run(format, func(t *testing.T) {
			p := probePLC(t)
			runCLI(t, probeArgs(p, "--summary-only", "--format", format)...).ok(t).
				has(t, "Recommended connection tuple:\n- rack=0 slot=2", "Setup-accepted combinations: 1 total").
				lacks(t, "Rack  Slot", "Suggested default tuple")
		})
	}
}

func TestProbe_JSON(t *testing.T) {
	p := probePLC(t)
	m := runCLI(t, probeArgs(p, "--strict", "--confirm", "szl", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, "szl", "request", "confirm")
	wantField(t, m, float64(1), "result", "confirmed_by_query")
	valid := field(t, m, "result", "valid").([]interface{})
	if len(valid) != 1 {
		t.Fatalf("valid = %v", valid)
	}
	wantField(t, valid[0].(map[string]interface{}), "query-confirmed", "status")
	wantField(t, valid[0].(map[string]interface{}), float64(2), "slot")
}

func TestProbe_RepeatReportsRuns(t *testing.T) {
	p := probePLC(t)
	m := runCLI(t, probeArgs(p, "--repeat", "2", "--interval-ms", "1", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, float64(2), "result", "repeat_runs")
	wantField(t, m, false, "result", "results_varied")

	runCLI(t, probeArgs(p, "--repeat", "2")...).ok(t).has(t, "Repeat (2 runs):", "- Setup accepted: 1\n")
	runCLI(t, probeArgs(p, "--repeat", "2", "--format", "text")...).ok(t).has(t, "Repeat (2 runs):")
	runCLI(t, probeArgs(p, "--repeat", "2", "--summary-only")...).ok(t).has(t, "Repeat (2 runs):")
}

func TestProbe_NothingValidIsAFinding(t *testing.T) {
	p := newFakePLC(t)
	p.admit = func(int, int) admission { return admitNoCOTP }
	runCLI(t, probeArgs(p)...).wantFindings(t, "no valid rack/slot found").
		has(t, "Some addresses accepted TCP but not COTP/S7.")

	port := closedPort(t)
	runCLI(t, "probe", "rackslot", "--ip", "127.0.0.1", "--port", port, "--rack-max", "0", "--slot-max", "0").
		wantFindings(t, "no valid rack/slot found").
		has(t, "target unreachable or not speaking S7")
}

func TestProbe_ManyCandidatesCollapse(t *testing.T) {
	p := probePLC(t)
	wide := p.args("probe", "rackslot", "--rack-max", "3", "--slot-max", "5")
	runCLI(t, wide...).ok(t).
		has(t, "Summary by status:", "Examples (use --show-all for full table):", "Setup-accepted combinations: 1 total")
	runCLI(t, append(wide, "--show-all")...).ok(t).
		has(t, "Setup-accepted combinations:\n- rack=0 slot=2").lacks(t, "Summary by status:")
}

func TestProbe_Validation(t *testing.T) {
	runCLI(t, "probe", "rackslot").wantUsage(t, "IP address is required")
	runCLI(t, "probe", "rackslot", "--ip", "127.0.0.1", "--local-tsap", "1").wantUsage(t, "must be set together")
	runCLI(t, "probe", "rackslot", "--ip", "127.0.0.1", "--rack-min", "5", "--rack-max", "1").wantFailure(t, "probe failed")
}

func TestPrintProbeRepeatSummary(t *testing.T) {
	stable := &client.RackSlotProbeResult{SetupAccepted: 2, ConfirmedByQuery: 1, TCPOnly: 3, Flaky: 1}
	for _, tc := range []struct {
		name     string
		results  []*client.RackSlotProbeResult
		variance bool
		want     []string
	}{
		{"single run prints nothing", []*client.RackSlotProbeResult{stable}, false, nil},
		{
			"stable", []*client.RackSlotProbeResult{stable, stable}, false,
			[]string{"Repeat (2 runs):", "- Setup accepted: 2\n", "- Confirmed by query: 1\n", "- TCP-only: 3\n", "- Flaky: 1\n"},
		},
		{
			"ranges", []*client.RackSlotProbeResult{
				{SetupAccepted: 2, ConfirmedByQuery: 2, TCPOnly: 2, Flaky: 2},
				{SetupAccepted: 1, ConfirmedByQuery: 1, TCPOnly: 1, Flaky: 1},
				{SetupAccepted: 3, ConfirmedByQuery: 3, TCPOnly: 3, Flaky: 3},
			}, true,
			[]string{"Repeat (3 runs):", "- Setup accepted: 1–3\n", "- Confirmed by query: 1–3\n", "- TCP-only: 1–3\n", "- Flaky: 1–3\n", "Results varied"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _ := captureOutput(t, func() { printProbeRepeatSummary(tc.results, tc.variance) })
			if tc.want == nil && stdout != "" {
				t.Fatalf("stdout = %q, want nothing", stdout)
			}
			for _, s := range tc.want {
				if !strings.Contains(stdout, s) {
					t.Errorf("no %q in:\n%s", s, stdout)
				}
			}
		})
	}
}

func TestPrintProbeInterpretation(t *testing.T) {
	valid := []client.RackSlotCandidate{{Rack: 1, Slot: 2}}
	for _, tc := range []struct {
		name      string
		res       client.RackSlotProbeResult
		tuple     bool
		variance  bool
		want, not []string
	}{
		{name: "unreachable", want: []string{"target unreachable"}},
		{
			name: "varied", res: client.RackSlotProbeResult{TCPOnly: 1}, variance: true,
			want: []string{"Results varied across runs", "accepted TCP but not COTP/S7"},
		},
		{
			name: "broad setup", res: client.RackSlotProbeResult{SetupAccepted: 2, Valid: valid}, tuple: true,
			want: []string{"rack=1 slot=2", "Broad setup acceptance observed."},
		},
		{
			name: "tuple suppressed", res: client.RackSlotProbeResult{SetupAccepted: 2, Valid: valid},
			not: []string{"Suggested default tuple"},
		},
		{
			name: "confirmed and flaky", res: client.RackSlotProbeResult{SetupAccepted: 2, ConfirmedByQuery: 1, Flaky: 1},
			want: []string{"high confidence", "mixed results across retries"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _ := captureOutput(t, func() { printProbeInterpretation(&tc.res, tc.tuple, tc.variance) })
			for _, s := range tc.want {
				if !strings.Contains(stdout, s) {
					t.Errorf("no %q in:\n%s", s, stdout)
				}
			}
			for _, s := range tc.not {
				if strings.Contains(stdout, s) {
					t.Errorf("%q in:\n%s", s, stdout)
				}
			}
		})
	}
}

func TestPrintProbeRackSlotTable_DetailRows(t *testing.T) {
	long := strings.Repeat("e", 30)
	candidates := []client.RackSlotCandidate{{Rack: 1, Slot: 1, Error: long, ConfirmedBy: "none", Status: "valid-connect"}}
	res := &client.RackSlotProbeResult{Address: "h", Candidates: candidates, Flaky: 1, SetupAccepted: 1}

	stdout, _ := captureOutput(t, func() {
		printProbeRackSlotTable(res, false, []*client.RackSlotProbeResult{res}, probeDisplayOpts{Strict: true})
	})
	for _, want := range []string{"Mode: strict (confirm=any)", "setup-ok", strings.Repeat("e", 22) + "...", "- Flaky: 1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("no %q in:\n%s", want, stdout)
		}
	}

	var many []client.RackSlotCandidate
	for i := 0; i < 16; i++ {
		many = append(many, client.RackSlotCandidate{Slot: i, Error: long, ConfirmedBy: client.ConfirmSZL, Status: "unreachable"})
	}
	res = &client.RackSlotProbeResult{Address: "h", Candidates: many}
	stdout, _ = captureOutput(t, func() { printProbeRackSlotTable(res, false, nil, probeDisplayOpts{}) })
	if !strings.Contains(stdout, "- unreachable: 16") || strings.Count(stdout, strings.Repeat("e", 22)+"...") != 5 {
		t.Errorf("collapsed table must hold 5 examples per status:\n%s", stdout)
	}

	summary, _ := captureOutput(t, func() {
		printProbeSummaryOnly(&client.RackSlotProbeResult{Address: "h", Flaky: 2}, false, nil, probeDisplayOpts{})
	})
	if !strings.Contains(summary, "- Flaky: 2") {
		t.Errorf("no flaky line:\n%s", summary)
	}
}

func TestProbeVariance(t *testing.T) {
	a := &client.RackSlotProbeResult{SetupAccepted: 1}
	b := &client.RackSlotProbeResult{SetupAccepted: 2}
	for _, tc := range []struct {
		name    string
		results []*client.RackSlotProbeResult
		want    bool
	}{
		{"one run", []*client.RackSlotProbeResult{a}, false},
		{"same", []*client.RackSlotProbeResult{a, a}, false},
		{"different", []*client.RackSlotProbeResult{a, b}, true},
		{"last differs", []*client.RackSlotProbeResult{a, a, b}, true},
	} {
		if got := probeVariance(tc.results); got != tc.want {
			t.Errorf("%s: probeVariance = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestProbeStatusDisplay(t *testing.T) {
	for _, tc := range []struct {
		status string
		strict bool
		want   string
	}{
		{"valid-connect", false, "valid-connect"},
		{"valid-connect", true, "setup-ok"},
		{"setup-only", true, "setup-ok"},
		{"valid-query", true, "query-confirmed"},
		{"rejected", true, "rejected"},
	} {
		if got := probeStatusDisplay(tc.status, tc.strict); got != tc.want {
			t.Errorf("probeStatusDisplay(%q, %v) = %q, want %q", tc.status, tc.strict, got, tc.want)
		}
	}
}

func TestConfirmKindToDisplay(t *testing.T) {
	for k, want := range map[client.ConfirmationKind]string{
		client.ConfirmAny:      "any",
		client.ConfirmNone:     "none",
		client.ConfirmSZL:      "szl",
		client.ConfirmCPUState: "cpu-state",
		"other":                "none",
	} {
		if got := confirmKindToDisplay(k); got != want {
			t.Errorf("confirmKindToDisplay(%v) = %q, want %q", k, got, want)
		}
	}
}

func compareArgs(p *fakePLC, extra ...string) []string {
	base := []string{"compare", "read", "--ip", "127.0.0.1", "--port", p.port, "--timeout", "2000",
		"--candidates", "0/1,0/2", "--area", "DB", "--size", "4"}
	return append(base, extra...)
}

func TestCompare_Table(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, compareArgs(p)...).ok(t).
		has(t, "Read: area=", "Rack/slot-insensitive: true", "baseline", "yes").lacks(t, "Repeat runs")
}

func TestCompare_JSON(t *testing.T) {
	p := newFakePLC(t)
	m := runCLI(t, compareArgs(p, "--format", "json", "--area", "M")...).ok(t).jsonOut(t)
	wantField(t, m, true, "result", "rack_slot_insensitive")
	by := field(t, m, "result", "by_candidate").([]interface{})
	if len(by) != 2 {
		t.Fatalf("by_candidate = %v", by)
	}
	wantField(t, by[0].(map[string]interface{}), "baseline", "same_as_baseline")
	wantField(t, by[1].(map[string]interface{}), "yes", "same_as_baseline")
	wantField(t, by[0].(map[string]interface{}), "00010203", "data_hex")
	if len(field(t, m, "request", "candidates").([]interface{})) != 2 {
		t.Fatal("request.candidates lost a pair")
	}
}

func TestCompare_RepeatDetectsChangingData(t *testing.T) {
	p := newFakePLC(t)
	var n atomic.Int32
	p.read = func(_ byte, _, _, size int) fakeReply {
		return fakeReply{data: patternBytes(int(n.Add(1)), size)}
	}
	runCLI(t, compareArgs(p, "--repeat", "2", "--interval-ms", "1")...).ok(t).
		has(t, "Repeat runs: 2; data stable across runs: false", "Stable by candidate: [false false]")
	m := runCLI(t, compareArgs(p, "--repeat", "2", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, float64(2), "result", "repeat_runs")
	wantField(t, m, false, "result", "data_stable_across_runs")
}

func TestCompare_FailedCandidateIsReported(t *testing.T) {
	p := newFakePLC(t)
	p.admit = func(_, slot int) admission {
		if slot == 2 {
			return admitNoCOTP
		}
		return admitOK
	}
	runCLI(t, compareArgs(p)...).ok(t).has(t, "Rack/slot-insensitive: false", "transport-error", "peer disconnected")
	m := runCLI(t, compareArgs(p, "--format", "json")...).ok(t).jsonOut(t)
	by := field(t, m, "result", "by_candidate").([]interface{})
	wantField(t, by[1].(map[string]interface{}), "-", "same_as_baseline")
	if field(t, by[1].(map[string]interface{}), "message") == "" {
		t.Fatal("no message on the failed candidate")
	}
}

func TestCompare_LongErrorIsCut(t *testing.T) {
	p := newFakePLC(t)
	p.stop() // connection refused: the longest message a candidate can carry
	m := runCLI(t, compareArgs(p, "--format", "json")...).ok(t).jsonOut(t)
	by := field(t, m, "result", "by_candidate").([]interface{})
	full := field(t, by[0].(map[string]interface{}), "message").(string)
	if len(full) <= 30 {
		t.Fatalf("message %q is too short to exercise the cut", full)
	}
	runCLI(t, compareArgs(p)...).ok(t).has(t, full[:27]+"...\n").lacks(t, full)
}

func TestCompare_Validation(t *testing.T) {
	p := newFakePLC(t)
	base := func(extra ...string) []string {
		return append([]string{"compare", "read", "--ip", "127.0.0.1", "--port", p.port}, extra...)
	}
	runCLI(t, "compare", "read").wantUsage(t, "IP address is required")
	runCLI(t, base()...).wantUsage(t, "at least one candidate required")
	runCLI(t, base("--candidates", "0/1", "--area", "Z", "--size", "1")...).wantUsage(t, "unknown area")
	runCLI(t, base("--candidates", "0/1", "--area", "M")...).wantUsage(t, "size must be > 0")
	runCLI(t, base("--candidates", "0/1", "--area", "M", "--size", "1", "--offset", "-1")...).wantFailure(t, "compare read failed")
}

func TestParseCandidates(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    []client.RackSlot
		wantErr string
	}{
		{in: "  "},
		{in: "0/1", want: []client.RackSlot{{Rack: 0, Slot: 1}}},
		{in: " 0 / 1 , ,7/31,", want: []client.RackSlot{{Rack: 0, Slot: 1}, {Rack: 7, Slot: 31}}},
		{in: "01", wantErr: "expected rack/slot"},
		{in: "a/1", wantErr: "expected integers"},
		{in: "1/b", wantErr: "expected integers"},
		{in: "8/0", wantErr: "rack 0-7"},
		{in: "-1/0", wantErr: "rack 0-7"},
		{in: "0/32", wantErr: "slot 0-31"},
	} {
		got, err := parseCandidates(tc.in)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("parseCandidates(%q) err = %v, want %q", tc.in, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseCandidates(%q): %v", tc.in, err)
		}
		if len(got) != len(tc.want) {
			t.Errorf("parseCandidates(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseCandidates(%q)[%d] = %v, want %v", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func cmpCand(slot int, status string, data ...byte) client.CompareReadCandidate {
	return client.CompareReadCandidate{Slot: slot, Result: client.ReadResult{
		Status: client.ReadStatus(status), Data: data, ReturnedLength: len(data),
	}}
}

func TestComputeSameAsBaseline(t *testing.T) {
	for _, tc := range []struct {
		name string
		by   []client.CompareReadCandidate
		want []string
	}{
		{"first success is baseline", []client.CompareReadCandidate{cmpCand(0, "rejected"), cmpCand(1, "success", 1), cmpCand(2, "success", 1), cmpCand(3, "success", 2)},
			[]string{"-", "baseline", "yes", "no"}},
		{"success without data has no baseline", []client.CompareReadCandidate{cmpCand(0, "success")}, []string{"-"}},
	} {
		got := computeSameAsBaseline(&client.CompareReadResult{ByCandidate: tc.by})
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCompareDataStable(t *testing.T) {
	run := func(c ...client.CompareReadCandidate) *client.CompareReadResult {
		return &client.CompareReadResult{ByCandidate: c}
	}
	if stable, all := compareDataStable([]*client.CompareReadResult{run()}); stable != nil || !all {
		t.Fatalf("one run: %v %v", stable, all)
	}
	a, b := cmpCand(1, "success", 1), cmpCand(2, "success", 2)

	// Keyed by rack/slot: a changed order between runs is not a change of data.
	if stable, all := compareDataStable([]*client.CompareReadResult{run(a, b), run(b, a)}); !all || !stable[0] || !stable[1] {
		t.Fatalf("reordered runs: %v %v", stable, all)
	}
	for name, last := range map[string]*client.CompareReadResult{
		"changed data":   run(a, cmpCand(2, "success", 3)),
		"changed status": run(a, cmpCand(2, "rejected", 2)),
		"changed length": run(a, cmpCand(2, "success", 2, 2)),
	} {
		if stable, all := compareDataStable([]*client.CompareReadResult{run(a, b), last}); all || stable[1] || !stable[0] {
			t.Errorf("%s: stable=%v all=%v", name, stable, all)
		}
	}

	// The last run decides which candidates are reported; one that dropped out of
	// a later run than the last is unstable.
	stable, all := compareDataStable([]*client.CompareReadResult{run(a, b), run(a), run(a, b)})
	if all || !stable[0] || stable[1] {
		t.Errorf("vanished candidate: stable=%v all=%v", stable, all)
	}
}

func TestProfile_Text(t *testing.T) {
	p := newFakePLC(t)
	runCLI(t, p.args("profile")...).ok(t).
		has(t, "Target: 127.0.0.1", "Port: "+p.port, "Setup support:         yes", "Recommended rack/slot: 0/0",
			"Identify support:      yes", "Order:      6ES7 315-2EH14-0AB0", "Module:     CPU 315-2 PN/DP",
			"Block list support:    yes", "Block count: 2")
}

func TestProfile_JSON(t *testing.T) {
	p := newFakePLC(t)
	m := runCLI(t, p.args("profile", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, true, "result", "setup_support")
	wantField(t, m, float64(0), "result", "recommended_slot")
	wantField(t, m, float64(2), "result", "block_count")
	wantField(t, m, "6ES7 315-2EH14-0AB0", "result", "identify", "order")
}

func TestProfile_NoDevice(t *testing.T) {
	p := newFakePLC(t)
	p.admit = func(int, int) admission { return admitNoCOTP }
	runCLI(t, p.args("profile")...).ok(t).
		has(t, "Setup support:         no", "Recommended rack/slot: (none)", "Identify support:      no",
			"Block list support:    no", "(command failed or not attempted)")
	m := runCLI(t, p.args("profile", "--format", "json")...).ok(t).jsonOut(t)
	wantField(t, m, false, "result", "setup_support")
	result := field(t, m, "result").(map[string]interface{})
	for _, key := range []string{"recommended_rack", "identify", "block_count"} {
		if _, ok := result[key]; ok {
			t.Errorf("result.%s is set without a device", key)
		}
	}
}

func TestProfile_IdentifyWithoutFieldsIsPartial(t *testing.T) {
	p := newFakePLC(t)
	p.szl = func(id, index uint16) fakeReply {
		if id == 0x0011 || id == 0x001C {
			return fakeReply{}
		}
		return defaultSZL(id, index)
	}
	runCLI(t, p.args("profile")...).ok(t).has(t, "Identify support:      partial")
}

func TestProfile_Validation(t *testing.T) {
	runCLI(t, "profile").wantUsage(t, "IP address is required")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runCLIContext(t, ctx, "profile", "--ip", "127.0.0.1", "--port", closedPort(t), "--timeout", "0").
		wantFailure(t, "probe failed")
}

func TestProfileHelpers(t *testing.T) {
	if yesNo(true) != "yes" || yesNo(false) != "no" {
		t.Error("yesNo")
	}
	for _, s := range []string{"", "N/A"} {
		if profileStrOrNull(s) != nil {
			t.Errorf("profileStrOrNull(%q) is not nil", s)
		}
	}
	if profileStrOrNull("x") != "x" {
		t.Error("profileStrOrNull keeps a value")
	}
	for _, tc := range []struct {
		info *model.DeviceInfo
		want bool
	}{
		{nil, false},
		{&model.DeviceInfo{}, false},
		{&model.DeviceInfo{CPUType: "N/A"}, false},
		{&model.DeviceInfo{FWVersion: "V1"}, true},
	} {
		if got := profileHasIdentifyFields(tc.info); got != tc.want {
			t.Errorf("profileHasIdentifyFields(%+v) = %v, want %v", tc.info, got, tc.want)
		}
	}
}

func TestDiscover_Validation(t *testing.T) {
	runCLI(t, "discover").wantUsage(t, "CIDR range is required")
	runCLI(t, "discover", "--cidr", "10.0.0.0").wantUsage(t, "invalid CIDR")
}

func TestDiscover_NothingFound(t *testing.T) {
	args := []string{"discover", "--cidr", "127.0.0.1/32", "--timeout", "200"}
	runCLI(t, args...).ok(t).has(t, "No S7 devices found")
	r := runCLI(t, append(args, "--format", "json")...).ok(t)
	if strings.TrimSpace(r.stdout) != "[]" {
		t.Fatalf("stdout = %q, want []", r.stdout)
	}
}

func TestDiscover_QuietNothingFoundIsSilent(t *testing.T) {
	args := []string{"discover", "--cidr", "127.0.0.1/32", "--timeout", "200"}
	for _, extra := range [][]string{{"--quiet"}, {"--format", "quiet"}, {"--quiet", "--format", "json"}} {
		r := runCLI(t, append(args, extra...)...).ok(t)
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want nothing", extra, r.stdout)
		}
	}
}

func TestDiscover_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runCLIContext(t, ctx, "discover", "--cidr", "127.0.0.1/32").wantFailure(t, "context canceled")
}

func TestPrintDiscovered(t *testing.T) {
	found := []client.DiscoverResult{
		{IP: "10.0.0.1", Port: 102, IsS7: true, Rack: 0, Slot: 2, PDUSize: 240, TSAP: "0x0302"},
		{IP: "10.0.0.2", Port: 102, IsS7: true, Rack: 1, Slot: 3, PDUSize: 480, TSAP: "0x0323", Error: "e", AbandonedReason: "r"},
	}
	for _, tc := range []struct {
		name string
		cfg  config.DiscoverConfig
		want string
	}{
		{"quiet flag", config.DiscoverConfig{Quiet: true, Format: "table"}, "10.0.0.1\n10.0.0.2\n"},
		{"quiet format", config.DiscoverConfig{Format: "QUIET"}, "10.0.0.1\n10.0.0.2\n"},
		{"table", config.DiscoverConfig{Format: "table"}, "IP              Rack Slot PDU  TSAP\n" +
			"--------------- ---- ---- ---- ------\n" +
			"10.0.0.1           0    2  240 0x0302\n" +
			"10.0.0.2           1    3  480 0x0323\n"},
		{"text", config.DiscoverConfig{Format: "text"}, "10.0.0.1 rack=0 slot=2 pdu=240 tsap=0x0302\n" +
			"10.0.0.2 rack=1 slot=3 pdu=480 tsap=0x0323\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _ := captureOutput(t, func() { printDiscovered(found, tc.cfg) })
			if stdout != tc.want {
				t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, tc.want)
			}
		})
	}

	stdout, _ := captureOutput(t, func() { printDiscovered(found, config.DiscoverConfig{Format: "JSON"}) })
	var got []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results", len(got))
	}
	wantField(t, got[0], float64(240), "pdu_size")
	if _, ok := got[0]["error"]; ok {
		t.Error("a clean result carries an error key")
	}
	wantField(t, got[1], "e", "error")
	wantField(t, got[1], "r", "abandoned_reason")
}

func TestApplyDiscoverProfile_Defaults(t *testing.T) {
	for _, tc := range []struct {
		profile           string
		timeout, parallel int
		wantTimeout       int
		wantParallel      int
		rackMax, slotMax  int
	}{
		{"QUICK", 0, 0, 800, 20, 0, 2},
		{"quick", 100, 3, 100, 3, 0, 2},
		{"full", 0, 0, 2500, 8, 3, 5},
		{"full", 100, 3, 100, 3, 3, 5},
		{"standard", 0, 0, 0, 0, 1, 3},
		{"", 0, 0, 0, 0, 1, 3},
	} {
		cfg := config.DiscoverConfig{Profile: tc.profile, Timeout: tc.timeout, Parallel: tc.parallel}
		applyDiscoverProfile(&cfg)
		if cfg.Timeout != tc.wantTimeout || cfg.Parallel != tc.wantParallel || cfg.RackMax != tc.rackMax || cfg.SlotMax != tc.slotMax {
			t.Errorf("profile %q: %+v", tc.profile, cfg)
		}
	}
}
