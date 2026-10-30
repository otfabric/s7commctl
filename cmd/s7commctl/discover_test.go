// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"testing"

	"github.com/otfabric/go-s7comm/client"

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

// recordDiscover replaces the scan and returns the config it receives.
func recordDiscover(t *testing.T) *config.DiscoverConfig {
	t.Helper()
	got := new(config.DiscoverConfig)
	old := discoverHosts
	discoverHosts = func(_ context.Context, cfg config.DiscoverConfig) ([]client.DiscoverResult, error) {
		*got = cfg
		return []client.DiscoverResult{{IP: "10.0.0.9", IsS7: true}, {IP: "10.0.0.10"}}, nil
	}
	t.Cleanup(func() { discoverHosts = old })
	return got
}

func TestDiscover_FlagsBeatProfile(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		args                               []string
		rackMin, rackMax, slotMin, slotMax int
	}{
		{"default is standard", nil, 0, 1, 0, 3},
		{"README example", []string{"--rack-min", "0", "--rack-max", "3", "--slot-min", "0", "--slot-max", "5"}, 0, 3, 0, 5},
		{"quick", []string{"--profile", "quick"}, 0, 0, 1, 2},
		{"full", []string{"--profile", "full"}, 0, 3, 0, 5},
		{"quick, one bound", []string{"--profile", "quick", "--rack-max", "2"}, 0, 2, 1, 2},
		{"full, narrowed", []string{"--profile", "full", "--slot-max", "1"}, 0, 3, 0, 1},
		{"value equal to the flag default", []string{"--profile", "full", "--rack-max", "1"}, 0, 1, 0, 5},
		{"explicit zero", []string{"--profile", "quick", "--slot-min", "0"}, 0, 0, 0, 2},
		{"single target", []string{"--rack-min", "1", "--slot-min", "2", "--slot-max", "2"}, 1, 1, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := recordDiscover(t)
			r := runCLI(t, append([]string{"discover", "--cidr", "10.0.0.0/30", "--quiet"}, tc.args...)...).ok(t)
			if r.stdout != "10.0.0.9\n" {
				t.Errorf("stdout = %q", r.stdout)
			}
			if got.RackMin != tc.rackMin || got.RackMax != tc.rackMax || got.SlotMin != tc.slotMin || got.SlotMax != tc.slotMax {
				t.Fatalf("rack %d..%d slot %d..%d, want rack %d..%d slot %d..%d",
					got.RackMin, got.RackMax, got.SlotMin, got.SlotMax, tc.rackMin, tc.rackMax, tc.slotMin, tc.slotMax)
			}
		})
	}
}

func TestDiscover_EnvBeatsProfile(t *testing.T) {
	resetCommands(context.Background())
	t.Setenv("S7COMMCTL_RACK_MAX", "2")
	t.Setenv("S7COMMCTL_SLOT_MIN", "4")
	t.Setenv("S7COMMCTL_SLOT_MAX", "not a number")
	t.Setenv("S7COMMCTL_RACK_MIN", "")

	cfg := config.DefaultDiscoverConfig()
	cfg.Profile = "quick"
	cfg.RackMax, cfg.SlotMin = 2, 4 // as config.LoadFromEnv left them
	got := resolveDiscoverConfig(discoverCmd, cfg)
	if got.RackMin != 0 || got.RackMax != 2 || got.SlotMin != 4 || got.SlotMax != 2 {
		t.Fatalf("rack %d..%d slot %d..%d", got.RackMin, got.RackMax, got.SlotMin, got.SlotMax)
	}
}

func TestDiscover_DefaultsMatchStandardProfile(t *testing.T) {
	want := config.DefaultDiscoverConfig()
	got := want
	applyDiscoverProfile(&got)
	if got != want {
		t.Fatalf("--help shows %+v, the standard profile scans %+v", want, got)
	}
}

func TestDiscover_InvalidRange(t *testing.T) {
	recordDiscover(t)
	args := []string{"discover", "--cidr", "10.0.0.0/30"}
	for _, tc := range []struct {
		flags []string
		msg   string
	}{
		{[]string{"--rack-min", "2"}, "rack range is empty: min 2 is above max 1"},
		{[]string{"--slot-min", "5"}, "slot range is empty: min 5 is above max 3"},
		{[]string{"--rack-min", "-1"}, "rack must be 0-7"},
		{[]string{"--rack-max", "8"}, "rack must be 0-7"},
		{[]string{"--slot-min", "-1"}, "slot must be 0-31"},
		{[]string{"--slot-max", "32"}, "slot must be 0-31"},
	} {
		runCLI(t, append(args, tc.flags...)...).wantUsage(t, tc.msg)
	}
}
