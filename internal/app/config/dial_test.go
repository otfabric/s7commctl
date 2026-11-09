// SPDX-License-Identifier: MIT

package config

import "testing"

func TestDialConfigFromEnvironment(t *testing.T) {
	t.Setenv("S7COMMCTL_PROXY", "socks5://127.0.0.1:1080")
	t.Setenv("S7COMMCTL_SOURCE_ADDR", "127.0.0.1")
	for _, cfg := range []any{new(DeviceConfig), new(DiscoverConfig), new(ProbeRackSlotConfig), new(CompareReadConfig), new(ProfileConfig)} {
		LoadFromEnv(cfg)
		var d DialConfig
		switch c := cfg.(type) {
		case *DeviceConfig:
			d = c.DialConfig
		case *DiscoverConfig:
			d = c.DialConfig
		case *ProbeRackSlotConfig:
			d = c.DialConfig
		case *CompareReadConfig:
			d = c.DialConfig
		case *ProfileConfig:
			d = c.DialConfig
		}
		if d.Proxy != "socks5://127.0.0.1:1080" || d.SourceAddr != "127.0.0.1" {
			t.Errorf("%T: %+v", cfg, d)
		}
	}
}

func TestDialConfigReachesOptions(t *testing.T) {
	c := DefaultDeviceConfig()
	base, err := c.ToClientOptions()
	if err != nil {
		t.Fatal(err)
	}
	c.Proxy = "socks5://127.0.0.1:1080"
	with, err := c.ToClientOptions()
	if err != nil || len(with) != len(base)+1 {
		t.Fatalf("options %d (err %v), want one more than %d for the dialer", len(with), err, len(base))
	}
	c.Proxy = "ftp://x:1"
	if _, err := c.ToClientOptions(); err == nil {
		t.Error("invalid proxy: no error")
	}

	p := ProbeRackSlotConfig{IP: "127.0.0.1"}
	req, err := p.ToProbeRequest()
	if err != nil || req.Dialer != nil {
		t.Fatalf("no flags: dialer set (%v)", err)
	}
	p.SourceAddr = "127.0.0.1"
	if req, err = p.ToProbeRequest(); err != nil || req.Dialer == nil {
		t.Fatalf("--source-addr: no dialer (%v)", err)
	}
	p.SourceAddr = "nope"
	if _, err = p.ToProbeRequest(); err == nil {
		t.Error("invalid source address: no error")
	}
}
