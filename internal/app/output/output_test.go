// SPDX-License-Identifier: MIT

package output

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"
)

func TestFormatDeviceInfoText(t *testing.T) {
	info := &model.DeviceInfo{OrderNumber: "6ES7", ModuleName: "CPU"}
	s := FormatDeviceInfo(info, FormatText)
	if !strings.Contains(s, "Order") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestFormatReadDataHex(t *testing.T) {
	s := FormatReadData([]byte{0xAA, 0xBB}, "raw", FormatHex)
	if !strings.Contains(s, "AABB") && !strings.Contains(s, "AA") {
		t.Fatalf("unexpected hexdump: %s", s)
	}
}

func TestFormatReadResult_TableAndMeta(t *testing.T) {
	result := &client.ReadResult{
		Status: "success", RequestedLength: 2, ReturnedLength: 2, Data: []byte{0x00, 0x01},
	}
	meta := &ReadResultMeta{IP: "192.168.1.1", Port: 102, Area: "DB", DBNumber: 1, Offset: 0, RequestedSize: 2}
	s := FormatReadResult(result, "int", FormatTable, meta)
	if !strings.Contains(s, "status=success") || !strings.Contains(s, "Area: DB") || !strings.Contains(s, "DB1") {
		t.Errorf("table output missing expected parts: %s", s)
	}
	// Plain format with connection request
	meta.ConnectionRequest = map[string]interface{}{"connection_mode": "rack_slot"}
	s = FormatReadResult(result, "raw", "plain", meta)
	if !strings.Contains(s, "Result: success") {
		t.Errorf("plain output: %s", s)
	}
}

func TestFormatReadData_Typed(t *testing.T) {
	// bool: at least one bit decoded
	s := FormatReadData([]byte{0x01}, "bool", "text")
	if !strings.Contains(s, "bit") {
		t.Errorf("bool format: %s", s)
	}
	// byte
	s = FormatReadData([]byte{0x41, 0x42}, "byte", "text")
	if !strings.Contains(s, "65") || !strings.Contains(s, "66") {
		t.Errorf("byte format: %s", s)
	}
}

// TestFormatReadResult_JSONContainsDecoded verifies that typed read results include a "decoded" array in JSON.
func TestFormatReadResult_JSONContainsDecoded(t *testing.T) {
	// Two bytes = one int16 (S7 big-endian 0x00 0x01 = 1). Status matches CLI convention.
	result := &client.ReadResult{
		Status:          "success",
		RequestedLength: 2,
		ReturnedLength:  2,
		Data:            []byte{0x00, 0x01},
		Warnings:        nil,
	}
	meta := &ReadResultMeta{IP: "192.168.1.1", Port: 102, Area: "DB", DBNumber: 1, Offset: 0, RequestedSize: 2}
	s := FormatReadResult(result, "int", FormatJSON, meta)
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	res, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("no result object: %v", out)
	}
	decoded, ok := res["decoded"]
	if !ok {
		t.Fatalf("no decoded field in result: %v", res)
	}
	arr, ok := decoded.([]interface{})
	if !ok {
		t.Fatalf("decoded is not array: %T %v", decoded, decoded)
	}
	if len(arr) < 1 {
		t.Fatalf("expected at least one decoded value: %v", arr)
	}
	// S7 int16 is big-endian: 0x00 0x01 = 1
	if n, ok := arr[0].(float64); !ok || n != 1 {
		t.Errorf("expected first decoded value 1, got %v", arr[0])
	}
}

func TestFingerprintView_ResultMap(t *testing.T) {
	view := &FingerprintView{
		CPUType:            "CPU 315-2 PN/DP",
		WriteLikelyAllowed: true,
		UploadProbeOK:      false,
	}
	m := FingerprintResultMap(view)
	if m["cpu_type"] != "CPU 315-2 PN/DP" {
		t.Errorf("unexpected cpu_type: %v", m["cpu_type"])
	}
	if m["write_likely_allowed"] != true {
		t.Errorf("expected write_likely_allowed true: %v", m["write_likely_allowed"])
	}
	if m["upload_probe_ok"] != false {
		t.Errorf("expected upload_probe_ok false: %v", m["upload_probe_ok"])
	}
}

func TestDeviceInfoResultMap(t *testing.T) {
	if DeviceInfoResultMap(nil) != nil {
		t.Fatal("nil info should return nil map")
	}
	info := &model.DeviceInfo{OrderNumber: "6ES7", ModuleName: "CPU", CPUType: "CPU 315"}
	m := DeviceInfoResultMap(info)
	if m["order_number"] != "6ES7" || m["module_name"] != "CPU" || m["cpu_type"] != "CPU 315" {
		t.Errorf("unexpected map: %v", m)
	}
	if m["serial_number"] != nil {
		t.Errorf("empty field should be nil for JSON null: %v", m["serial_number"])
	}
}

func TestFormatBlocks(t *testing.T) {
	// Empty list -> JSON array
	s := FormatBlocks(nil, FormatJSON)
	if !strings.Contains(s, "[") || !strings.Contains(s, "]") {
		t.Errorf("empty blocks JSON: %s", s)
	}
	one := []model.BlockInfo{{Type: model.BlockDB, Number: 1, Name: "DataBlock1"}}
	s = FormatBlocks(one, FormatJSON)
	if !strings.Contains(s, "DataBlock1") || !strings.Contains(s, "number") {
		t.Errorf("one block JSON: %s", s)
	}
	// Table format (Type, Number, Language, Flags columns)
	s = FormatBlocks(one, FormatTable)
	if !strings.Contains(s, "DB") || !strings.Contains(s, "1") {
		t.Errorf("one block table: %s", s)
	}
}

func TestFormatDeviceInfo_JSON(t *testing.T) {
	info := &model.DeviceInfo{OrderNumber: "6ES7", ModuleName: "CPU"}
	s := FormatDeviceInfo(info, FormatJSON)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["order_number"] != "6ES7" {
		t.Errorf("unexpected JSON: %v", m)
	}
}

func TestFormatDeviceInfo_Nil(t *testing.T) {
	if FormatDeviceInfo(nil, FormatTable) != "" {
		t.Fatal("nil info should return empty string")
	}
}

func TestDiagBufferResultMap(t *testing.T) {
	if DiagBufferResultMap(nil) != nil {
		t.Fatal("nil diag should return nil")
	}
	diag := &model.DiagBuffer{
		Entries:    []model.DiagEntry{{EventID: 1, Description: "Start"}},
		TotalCount: 1,
	}
	m := DiagBufferResultMap(diag)
	ev, ok := m["entries"].([]map[string]interface{})
	if !ok || len(ev) != 1 {
		t.Errorf("unexpected entries: %v", m)
	}
	if ev[0]["description"] != "Start" {
		t.Errorf("entry description: %v", ev[0])
	}
	if m["total_count"] != 1 {
		t.Errorf("total_count: %v", m["total_count"])
	}
}

func TestFormatDiagBuffer(t *testing.T) {
	if FormatDiagBuffer(nil, FormatJSON) != "null" {
		t.Fatal("nil diag JSON should return null")
	}
	diag := &model.DiagBuffer{
		Entries:    []model.DiagEntry{{EventID: 0x1234, EventClass: 0x08, Priority: 2, Description: "Msg"}},
		TotalCount: 1,
	}
	s := FormatDiagBuffer(diag, FormatTable)
	if !strings.Contains(s, "0x1234") {
		t.Errorf("table should contain event id: %s", s)
	}
	s = FormatDiagBuffer(diag, FormatJSON)
	if !strings.Contains(s, "entries") {
		t.Errorf("JSON should contain entries: %s", s)
	}
	s = FormatDiagBuffer(diag, "text")
	if !strings.Contains(s, "1 entries") {
		t.Errorf("default format: %s", s)
	}
}

func TestFormatFingerprint(t *testing.T) {
	if FormatFingerprint(nil, FormatJSON) != "null" {
		t.Fatal("nil view should return null")
	}
	view := &FingerprintView{CPUType: "CPU 315", PDUSize: 480}
	s := FormatFingerprint(view, FormatJSON)
	if !strings.Contains(s, "cpu_type") || !strings.Contains(s, "CPU 315") {
		t.Errorf("fingerprint JSON: %s", s)
	}
	s = FormatFingerprint(view, FormatTable)
	if !strings.Contains(s, "CPU Type") || !strings.Contains(s, "480") {
		t.Errorf("fingerprint table: %s", s)
	}
	s = FormatFingerprint(view, "text")
	if !strings.Contains(s, "CPU 315") || !strings.Contains(s, "480") {
		t.Errorf("fingerprint text: %s", s)
	}
}

func TestPrintJSON(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	PrintJSON(map[string]int{"a": 1})
	os.Stdout = old
	_ = w.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\n  \"a\": 1\n}\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestFormatDeviceInfo_EmptyFieldsAreNA(t *testing.T) {
	got := FormatDeviceInfo(&model.DeviceInfo{}, FormatText)
	if got != "Order: N/A, Module: N/A, CPU: N/A, FW: N/A\n" {
		t.Fatalf("text = %q", got)
	}
	table := FormatDeviceInfo(&model.DeviceInfo{ModuleName: strings.Repeat("m", 40), CPUType: "CPU 315"}, FormatTable)
	for _, want := range []string{"S7 Device Information", "│ Order Number    │ N/A ", strings.Repeat("m", 26) + "...", "│ CPU Type        │ CPU 315 "} {
		if !strings.Contains(table, want) {
			t.Errorf("no %q in:\n%s", want, table)
		}
	}
}

func TestDeviceInfoResultMap_Nil(t *testing.T) {
	if DeviceInfoResultMap(nil) != nil {
		t.Fatal("nil info must map to nil")
	}
}

func TestFormatBlocks_Formats(t *testing.T) {
	blocks := []model.BlockInfo{{Type: model.BlockDB, Number: 7, Flags: 3}}
	if got := FormatBlocks(blocks, FormatText); got != "DB7\n" {
		t.Errorf("text = %q", got)
	}
	table := FormatBlocks(blocks, FormatTable)
	if !strings.Contains(table, "│ DB   │      7 │") || !strings.Contains(table, "0x03") {
		t.Errorf("table:\n%s", table)
	}
	var out []map[string]interface{}
	if err := json.Unmarshal([]byte(FormatBlocks(blocks, FormatJSON)), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0]["number"] != float64(7) || out[0]["author"] != nil {
		t.Errorf("json = %v", out)
	}
}

func TestFingerprint_NilAndFormats(t *testing.T) {
	if FingerprintResultMap(nil) != nil {
		t.Error("nil view must map to nil")
	}
	if FormatFingerprint(nil, FormatTable) != "null" {
		t.Error("nil view must format as null")
	}
	view := &FingerprintView{CPUType: strings.Repeat("c", 35), PDUSize: 480, Protection: ""}
	table := FormatFingerprint(view, FormatTable)
	for _, want := range []string{strings.Repeat("c", 26) + "...", "│ PDU Size        │ 480 ", "│ Order Number    │ - "} {
		if !strings.Contains(table, want) {
			t.Errorf("no %q in:\n%s", want, table)
		}
	}
	if got := FormatFingerprint(&FingerprintView{CPUType: "c", PDUSize: 1}, FormatText); got != "CPU: c, PDU: 1\n" {
		t.Errorf("text = %q", got)
	}
}

func TestDiagBuffer_NilAndEntries(t *testing.T) {
	if DiagBufferResultMap(nil) != nil {
		t.Error("nil buffer must map to nil")
	}
	if FormatDiagBuffer(nil, FormatJSON) != "null" {
		t.Error("nil buffer must format as null")
	}
	diag := &model.DiagBuffer{TotalCount: 1, Entries: []model.DiagEntry{{EventID: 0x4A01, EventClass: 2, Priority: 3}}}
	if got := FormatDiagBuffer(diag, FormatText); got != "1 entries\n" {
		t.Errorf("text = %q", got)
	}
	if got := FormatDiagBuffer(diag, FormatTable); !strings.Contains(got, "│ 0x4A01   │ 0x02  │        3 │") {
		t.Errorf("table:\n%s", got)
	}
}

func TestFormatHexDump(t *testing.T) {
	if FormatHexDump(nil) != "" {
		t.Error("no data, no lines")
	}
	data := append([]byte("ABCDEFGHIJKLMNOP"), 0x00, 0x7F)
	got := FormatHexDump(data)
	want := "00000000  41 42 43 44 45 46 47 48  49 4A 4B 4C 4D 4E 4F 50  |ABCDEFGHIJKLMNOP|\n" +
		"00000010  00 7F                                             |..|\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatReadResult_Nil(t *testing.T) {
	if got := FormatReadResult(nil, "raw", FormatJSON, nil); got != "Result: (no result)\n" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatReadResult_TextWithoutMeta(t *testing.T) {
	got := FormatReadResult(&client.ReadResult{
		Status: "short-read", Data: []byte{0xAB}, ReturnedLength: 1, Message: "m", Warnings: []string{"w"},
	}, "raw", FormatTable, nil)
	for _, want := range []string{"Result: short-read\nReturned: 1 bytes\n", "Warning: w\n", "Message: m\n", "AB "} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "status=") {
		t.Errorf("summary line without metadata:\n%s", got)
	}
}

func TestFormatReadResult_JSONWithoutMetaAndConnection(t *testing.T) {
	result := &client.ReadResult{Status: "rejected", Message: "no"}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(FormatReadResult(result, "raw", FormatJSON, nil)), &m); err != nil {
		t.Fatal(err)
	}
	res := m["result"].(map[string]interface{})
	if res["status_detail"] != "rejected — no" || res["message"] != "no" {
		t.Errorf("result = %v", res)
	}
	if _, ok := m["target"]; ok {
		t.Error("target without metadata")
	}
	if w, ok := res["warnings"].([]interface{}); !ok || len(w) != 0 {
		t.Errorf("warnings must be an empty array, got %v", res["warnings"])
	}

	meta := &ReadResultMeta{IP: "h", Port: 1, Area: "M", Offset: 2, RequestedSize: 3}
	if err := json.Unmarshal([]byte(FormatReadResult(result, "raw", FormatJSON, meta)), &m); err != nil {
		t.Fatal(err)
	}
	req := m["request"].(map[string]interface{})
	if _, ok := req["connection"]; ok {
		t.Error("connection without ConnectionRequest")
	}
	if _, ok := req["address"].(map[string]interface{})["db"]; ok {
		t.Error("db for a non-DB area")
	}
}

func TestFormatReadResult_JSONDecodedByType(t *testing.T) {
	data := []byte{0x3F, 0x80, 0x00, 0x00}
	for _, tc := range []struct {
		typ  string
		want []interface{}
	}{
		{"byte", []interface{}{float64(0x3F), float64(0x80), float64(0), float64(0)}},
		{"INT", []interface{}{float64(0x3F80), float64(0)}},
		{"word", []interface{}{float64(0x3F80), float64(0)}},
		{"dint", []interface{}{float64(0x3F800000)}},
		{"dword", []interface{}{float64(0x3F800000)}},
		{"real", []interface{}{float64(1)}},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			var m map[string]interface{}
			s := FormatReadResult(&client.ReadResult{Status: "success", Data: data}, tc.typ, FormatJSON, nil)
			if err := json.Unmarshal([]byte(s), &m); err != nil {
				t.Fatal(err)
			}
			got, _ := m["result"].(map[string]interface{})["decoded"].([]interface{})
			if len(got) != len(tc.want) {
				t.Fatalf("decoded = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("decoded[%d] = %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDecodeValuesForJSON_Limits(t *testing.T) {
	if got := decodeValuesForJSON(make([]byte, 16), "bool"); len(got) != 64 {
		t.Errorf("bool values = %d, capped at 64", len(got))
	}
	if got := decodeValuesForJSON([]byte{1}, "dint"); got != nil {
		t.Errorf("too few bytes: %v", got)
	}
	if got := decodeValuesForJSON([]byte{0, 0}, "string"); got != nil {
		t.Errorf("empty string: %v", got)
	}
	if got := decodeValuesForJSON([]byte{4, 2, 'o', 'k'}, "string"); len(got) != 1 || got[0] != "ok" {
		t.Errorf("string: %v", got)
	}
	if got := decodeValuesForJSON([]byte{1}, "unknown"); got != nil {
		t.Errorf("unknown type: %v", got)
	}
}

func TestFormatReadData_Types(t *testing.T) {
	data := []byte{0x00, 0x02, 0x00, 0x03}
	for _, tc := range []struct {
		typ, want string
	}{
		{"bool", "bit 9: true"},
		{"byte", "[1]: 2 (0x02)"},
		{"int", "[1]: 3"},
		{"word", "[0]: 2 (0x0002)"},
		{"dint", "[0]: 131075"},
		{"dword", "[0]: 131075 (0x00020003)"},
		{"real", "[0]: "},
		{"string", `"\x00\x03"`},
	} {
		if got := FormatReadData(data, tc.typ, FormatText); !strings.Contains(got, tc.want) {
			t.Errorf("%s: no %q in %q", tc.typ, tc.want, got)
		}
	}
}

func TestFormatReadData_Formats(t *testing.T) {
	if got := FormatReadData([]byte{1, 2}, "raw", FormatText); !strings.HasPrefix(got, "00000000  01 02") {
		t.Errorf("raw text falls back to a hex dump: %q", got)
	}
	got := FormatReadData([]byte{1, 2}, "int", FormatHex)
	if !strings.Contains(got, "01 02") || !strings.Contains(got, "Interpreted as int:\n[0]: 258\n") {
		t.Errorf("hex: %q", got)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(FormatReadData([]byte{1, 2}, "int", FormatJSON)), &m); err != nil {
		t.Fatal(err)
	}
	if m["raw_hex"] != "0102" || m["length"] != float64(2) || len(m["values"].([]interface{})) != 1 {
		t.Errorf("json = %v", m)
	}
	if err := json.Unmarshal([]byte(FormatReadData([]byte{1}, "raw", FormatJSON)), &m); err != nil {
		t.Fatal(err)
	}
}
