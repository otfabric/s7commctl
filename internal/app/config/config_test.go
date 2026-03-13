package config

import (
	"os"
	"testing"
)

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
