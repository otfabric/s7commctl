package output

import (
	"encoding/json"
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
