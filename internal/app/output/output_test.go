package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/otfabric/s7comm/client"
	"github.com/otfabric/s7comm/model"
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
