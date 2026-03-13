package output

import (
	"strings"
	"testing"

	"otfabric/s7comm/model"
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
