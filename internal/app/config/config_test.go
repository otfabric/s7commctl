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

func TestDefaultConfigs(t *testing.T) {
	rc := DefaultReadConfig()
	if rc.Area != "DB" || rc.Size != 1 {
		t.Errorf("DefaultReadConfig: area=%s size=%d", rc.Area, rc.Size)
	}
	wc := DefaultWriteConfig()
	if wc.Area != "DB" || wc.Format != "text" {
		t.Errorf("DefaultWriteConfig: area=%s format=%s", wc.Area, wc.Format)
	}
	bc := DefaultBlocksConfig()
	if bc.Format != "table" {
		t.Errorf("DefaultBlocksConfig: format=%s", bc.Format)
	}
	dc := DefaultDiagnosticConfig()
	if dc.Format != "table" {
		t.Errorf("DefaultDiagnosticConfig: format=%s", dc.Format)
	}
	fc := DefaultFingerprintConfig()
	if fc.Format != "table" {
		t.Errorf("DefaultFingerprintConfig: format=%s", fc.Format)
	}
}

func TestUseExplicitTSAP(t *testing.T) {
	dc := DefaultDeviceConfig()
	if dc.UseExplicitTSAP() {
		t.Fatal("default should not use explicit TSAP")
	}
	dc.LocalTSAP = "0x0100"
	dc.RemoteTSAP = "0x0200"
	if !dc.UseExplicitTSAP() {
		t.Fatal("expected UseExplicitTSAP true when both set")
	}
	dc.RemoteTSAP = ""
	if dc.UseExplicitTSAP() {
		t.Fatal("only one TSAP set should be false")
	}
}

func TestConnectionRequestMeta(t *testing.T) {
	// rack_slot mode (default)
	dc := DefaultDeviceConfig()
	dc.Rack, dc.Slot = 0, 1
	m := dc.ConnectionRequestMeta()
	if m["connection_mode"] != "rack_slot" {
		t.Errorf("expected rack_slot mode: %v", m["connection_mode"])
	}
	if m["rack"].(int) != 0 || m["slot"].(int) != 1 {
		t.Errorf("unexpected rack/slot: %v", m)
	}
	// explicit_tsap
	dc.LocalTSAP = "0x0100"
	dc.RemoteTSAP = "0x0200"
	m = dc.ConnectionRequestMeta()
	if m["connection_mode"] != "explicit_tsap" {
		t.Errorf("expected explicit_tsap: %v", m["connection_mode"])
	}
	if m["local_tsap"] != "0x0100" || m["remote_tsap"] != "0x0200" {
		t.Errorf("unexpected tsap: %v", m)
	}
	// auto_rack_slot
	dc.LocalTSAP = ""
	dc.RemoteTSAP = ""
	dc.AutoRack = true
	m = dc.ConnectionRequestMeta()
	if m["connection_mode"] != "auto_rack_slot" {
		t.Errorf("expected auto_rack_slot: %v", m["connection_mode"])
	}
}
