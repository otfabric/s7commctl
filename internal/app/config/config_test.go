package config

import (
	"os"
	"strings"
	"testing"
)

func TestToClientOptions_PartialTSAP_ReturnsError(t *testing.T) {
	dc := DefaultDeviceConfig()
	dc.IP = "192.168.1.1"
	dc.LocalTSAP = "0x0100"
	dc.RemoteTSAP = "" // only one set
	_, err := dc.ToClientOptions()
	if err == nil {
		t.Fatal("expected error for partial TSAP")
	}
	if !strings.Contains(err.Error(), "both") || !strings.Contains(err.Error(), "local-tsap") {
		t.Errorf("expected message about both TSAPs: %v", err)
	}
}

func TestToClientOptions_InvalidTSAP_ReturnsError(t *testing.T) {
	dc := DefaultDeviceConfig()
	dc.IP = "192.168.1.1"
	dc.LocalTSAP = "bad"
	dc.RemoteTSAP = "0x0200"
	_, err := dc.ToClientOptions()
	if err == nil {
		t.Fatal("expected error for invalid local-tsap")
	}
	if !strings.Contains(err.Error(), "local-tsap") {
		t.Errorf("expected message mentioning local-tsap: %v", err)
	}
}

func TestToClientOptions_ValidTSAP_Succeeds(t *testing.T) {
	dc := DefaultDeviceConfig()
	dc.IP = "192.168.1.1"
	dc.LocalTSAP = "0x0100"
	dc.RemoteTSAP = "0x0200"
	opts, err := dc.ToClientOptions()
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected at least one option")
	}
}

func TestToProbeRequest_PartialTSAP_ReturnsError(t *testing.T) {
	pc := DefaultProbeRackSlotConfig()
	pc.IP = "192.168.1.1"
	pc.LocalTSAP = "0x0100"
	pc.RemoteTSAP = ""
	_, err := pc.ToProbeRequest()
	if err == nil {
		t.Fatal("expected error for partial TSAP in probe")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("expected message about both TSAPs: %v", err)
	}
}

func TestLoadFromEnv(t *testing.T) {
	_ = os.Setenv("S7COMMCTL_IP", "192.168.0.10")
	_ = os.Setenv("S7COMMCTL_PORT", "1102")
	defer func() { _ = os.Unsetenv("S7COMMCTL_IP") }()
	defer func() { _ = os.Unsetenv("S7COMMCTL_PORT") }()

	cfg := DefaultIdentifyConfig()
	LoadFromEnv(&cfg)

	if cfg.IP != "192.168.0.10" {
		t.Fatalf("expected IP from env, got %s", cfg.IP)
	}
	if cfg.Port != 1102 {
		t.Fatalf("expected port from env, got %d", cfg.Port)
	}
}

func TestDefaultDiscoverConfigRange(t *testing.T) {
	cfg := DefaultDiscoverConfig()
	if cfg.RackMax < cfg.RackMin || cfg.SlotMax < cfg.SlotMin {
		t.Fatal("invalid default rack/slot range")
	}
}
