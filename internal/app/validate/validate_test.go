package validate

import "testing"

func TestValidateArea(t *testing.T) {
	if _, err := ValidateArea("DB"); err != nil {
		t.Fatalf("expected DB valid, got %v", err)
	}
	if _, err := ValidateArea("bad"); err == nil {
		t.Fatal("expected error for bad area")
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
}

func TestIsPublicIP(t *testing.T) {
	if IsPublicIP("192.168.1.10") {
		t.Fatal("expected private IP to be non-public")
	}
	if !IsPublicIP("8.8.8.8") {
		t.Fatal("expected public IP")
	}
}
