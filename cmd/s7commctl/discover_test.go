package s7commctl

import (
	"testing"

	"github.com/otfabric/s7commctl/internal/app/config"
)

func TestApplyDiscoverProfileQuick(t *testing.T) {
	cfg := config.DefaultDiscoverConfig()
	cfg.Profile = "quick"
	applyDiscoverProfile(&cfg)

	if cfg.RackMax != 0 || cfg.SlotMin != 1 || cfg.SlotMax != 2 {
		t.Fatalf("quick profile range mismatch: rack=%d..%d slot=%d..%d", cfg.RackMin, cfg.RackMax, cfg.SlotMin, cfg.SlotMax)
	}
}

func TestApplyDiscoverProfileFull(t *testing.T) {
	cfg := config.DefaultDiscoverConfig()
	cfg.Profile = "full"
	applyDiscoverProfile(&cfg)

	if cfg.RackMax != 3 || cfg.SlotMax != 5 {
		t.Fatalf("full profile range mismatch: rack=%d..%d slot=%d..%d", cfg.RackMin, cfg.RackMax, cfg.SlotMin, cfg.SlotMax)
	}
}
