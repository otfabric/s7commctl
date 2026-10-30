// SPDX-License-Identifier: MIT

package validate

import (
	"bytes"
	"testing"
)

func TestValidateArea(t *testing.T) {
	validAreas := []string{"DB", "I", "Q", "M", "T", "C", "INPUTS", "OUTPUTS", "MERKERS", "COUNTER", "TIMER",
		"DATA_RECORD", "DR", "S7200AO", "IEC_COUNTER_200", "IEC_T200", "IEC_TIMER_200", "IEC_C200"}
	for _, area := range validAreas {
		if _, err := ValidateArea(area); err != nil {
			t.Errorf("expected %q valid, got %v", area, err)
		}
	}
	if _, err := ValidateArea("bad"); err == nil {
		t.Fatal("expected error for bad area")
	}
}

func TestValidateIP(t *testing.T) {
	if err := ValidateIP(""); err == nil {
		t.Fatal("expected error for empty IP")
	}
	if err := ValidateIP("invalid"); err == nil {
		t.Fatal("expected error for invalid IP")
	}
	if err := ValidateIP("192.168.1.1"); err != nil {
		t.Fatalf("expected valid IP: %v", err)
	}
	if err := ValidateIP("::1"); err != nil {
		t.Fatalf("expected valid IPv6: %v", err)
	}
}

func TestValidateCIDR(t *testing.T) {
	if err := ValidateCIDR(""); err == nil {
		t.Fatal("expected error for empty CIDR")
	}
	if err := ValidateCIDR("192.168.1.0/33"); err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
	if err := ValidateCIDR("192.168.1.0/24"); err != nil {
		t.Fatalf("expected valid CIDR: %v", err)
	}
}

func TestValidateRackSlot(t *testing.T) {
	if err := ValidateRackSlot(0, 0); err != nil {
		t.Fatalf("expected valid 0/0: %v", err)
	}
	if err := ValidateRackSlot(-1, 0); err == nil {
		t.Fatal("expected error for rack -1")
	}
	if err := ValidateRackSlot(8, 0); err == nil {
		t.Fatal("expected error for rack 8")
	}
	if err := ValidateRackSlot(0, 32); err == nil {
		t.Fatal("expected error for slot 32")
	}
}

func TestValidateBlockType(t *testing.T) {
	for _, bt := range []string{"OB", "DB", "FC", "FB", "SFC", "SFB"} {
		if _, err := ValidateBlockType(bt); err != nil {
			t.Errorf("expected %q valid: %v", bt, err)
		}
	}
	if _, err := ValidateBlockType("INVALID"); err == nil {
		t.Fatal("expected error for invalid block type")
	}
}

func TestParseTypedValue(t *testing.T) {
	b, err := ParseTypedValue("0x0102", "raw")
	if err != nil {
		t.Fatalf("raw parse failed: %v", err)
	}
	if len(b) != 2 {
		t.Fatalf("unexpected len: %d", len(b))
	}
	if _, err := ParseTypedValue("true", "bool"); err != nil {
		t.Fatalf("bool parse failed: %v", err)
	}
	if _, err := ParseTypedValue("false", "bool"); err != nil {
		t.Fatalf("bool false failed: %v", err)
	}
	b, err = ParseTypedValue("42", "int")
	if err != nil || len(b) != 2 {
		t.Fatalf("int parse: err=%v len=%d", err, len(b))
	}
	b, err = ParseTypedValue("1000000", "dint")
	if err != nil || len(b) != 4 {
		t.Fatalf("dint parse: err=%v len=%d", err, len(b))
	}
	b, err = ParseTypedValue("3.14", "real")
	if err != nil || len(b) != 4 {
		t.Fatalf("real parse: err=%v len=%d", err, len(b))
	}
	b, err = ParseTypedValue("hi", "string")
	if err != nil || len(b) < 3 {
		t.Fatalf("string parse: err=%v len=%d", err, len(b))
	}
	if _, err := ParseTypedValue("x", "badtype"); err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if _, err := ParseTypedValue("nothex", "raw"); err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestIsPublicIP(t *testing.T) {
	if IsPublicIP("192.168.1.10") {
		t.Fatal("expected private IP to be non-public")
	}
	if IsPublicIP("10.0.0.1") {
		t.Fatal("expected 10.x to be private")
	}
	if IsPublicIP("127.0.0.1") {
		t.Fatal("expected loopback to be non-public")
	}
	if !IsPublicIP("8.8.8.8") {
		t.Fatal("expected public IP")
	}
}

func TestParseHexBytes(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    []byte
		wantErr bool
	}{
		{in: "", want: []byte{}},
		{in: "0x0102", want: []byte{1, 2}},
		{in: "01 02", want: []byte{1, 2}},
		{in: "fF", want: []byte{0xFF}},
		{in: "abc", want: []byte{0x0A, 0xBC}},
		{in: "xy", wantErr: true},
		{in: "0x0g", wantErr: true},
	} {
		got, err := ParseHexBytes(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseHexBytes(%q) err = %v", tc.in, err)
			continue
		}
		if !tc.wantErr && !bytes.Equal(got, tc.want) {
			t.Errorf("ParseHexBytes(%q) = % X, want % X", tc.in, got, tc.want)
		}
	}
}

func TestParseTypedValue_Encodings(t *testing.T) {
	for _, tc := range []struct {
		value, typ string
		want       []byte
	}{
		{"true", "bool", []byte{1}},
		{"0", "BOOL", []byte{0}},
		{"-2", "int", []byte{0xFF, 0xFE}},
		{"0x0102", "int", []byte{1, 2}},
		{"65536", "dint", []byte{0, 1, 0, 0}},
		{"1", "real", []byte{0x3F, 0x80, 0, 0}},
	} {
		got, err := ParseTypedValue(tc.value, tc.typ)
		if err != nil {
			t.Errorf("ParseTypedValue(%q, %q): %v", tc.value, tc.typ, err)
			continue
		}
		if !bytes.Equal(got, tc.want) {
			t.Errorf("ParseTypedValue(%q, %q) = % X, want % X", tc.value, tc.typ, got, tc.want)
		}
	}
}

func TestParseTypedValue_String(t *testing.T) {
	got, err := ParseTypedValue("ok", "string")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("ok")) {
		t.Fatalf("encoded string % X does not hold the text", got)
	}
}

func TestParseTypedValue_OutOfRange(t *testing.T) {
	for _, tc := range [][2]string{
		{"maybe", "bool"}, {"40000", "int"}, {"x", "int"}, {"3000000000", "dint"}, {"x", "dint"},
		{"1e99", "real"}, {"x", "real"}, {"1", "lreal"},
	} {
		if _, err := ParseTypedValue(tc[0], tc[1]); err == nil {
			t.Errorf("ParseTypedValue(%q, %q) accepted", tc[0], tc[1])
		}
	}
}

func TestIsPublicIP_Ranges(t *testing.T) {
	for ip, want := range map[string]bool{
		"":             false,
		"garbage":      false,
		"127.0.0.1":    false,
		"169.254.1.1":  false,
		"10.1.2.3":     false,
		"172.16.0.1":   false,
		"172.31.255.1": false,
		"172.32.0.1":   true,
		"192.168.1.1":  false,
		"192.169.1.1":  true,
		"8.8.8.8":      true,
		"::1":          false,
		"fe80::1":      false,
		"fd00::1":      false,
		"2001:4860::1": true,
	} {
		if got := IsPublicIP(ip); got != want {
			t.Errorf("IsPublicIP(%q) = %v, want %v", ip, got, want)
		}
	}
}

func TestValidateAreaAndBlockType_Aliases(t *testing.T) {
	for _, area := range []string{"input", "OUTPUTS", "merker", "COUNTER", "timer", "DR", "S7200AO", "IEC_C200", "IEC_T200", "IEC_COUNTER_200", "IEC_TIMER_200"} {
		if _, err := ValidateArea(area); err != nil {
			t.Errorf("ValidateArea(%q): %v", area, err)
		}
	}
	for _, bt := range []string{"ob", "fc", "fb", "sfc", "sfb"} {
		if _, err := ValidateBlockType(bt); err != nil {
			t.Errorf("ValidateBlockType(%q): %v", bt, err)
		}
	}
}
