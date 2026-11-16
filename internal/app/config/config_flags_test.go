// SPDX-License-Identifier: MIT

package config

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/go-s7comm/client"
	"github.com/spf13/cobra"
)

func TestToClientOptions_Modes(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  DeviceConfig
		n    int
	}{
		{"rack/slot", DeviceConfig{Rack: 0, Slot: 1}, 1},
		{"auto", DeviceConfig{AutoRack: true}, 1},
		{"tsap wins over auto", DeviceConfig{LocalTSAP: "0x0100", RemoteTSAP: "0x0301", AutoRack: true}, 1},
		{"all options", DeviceConfig{Port: 1102, Timeout: 100, RateLimitMS: 5}, 5},
		// --timeout gives two options, the bound on connecting and on each
		// request; 0 or a negative value gives neither.
		{"timeout", DeviceConfig{Timeout: 100}, 3},
		{"timeout 0", DeviceConfig{Timeout: 0}, 1},
		{"negative timeout", DeviceConfig{Timeout: -1}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := tc.cfg.ToClientOptions()
			if err != nil {
				t.Fatal(err)
			}
			if len(opts) != tc.n {
				t.Fatalf("got %d options, want %d", len(opts), tc.n)
			}
		})
	}
}

func ms(n int) *int { return &n }

// TestRequestBound: --request-timeout bounds each request when it is set (0:
// no bound) and --timeout does when it is not, as before the flag existed. A
// negative value, or one too large for a time.Duration, is refused.
func TestRequestBound(t *testing.T) {
	type tc struct {
		name    string
		timeout int
		request *int
		want    time.Duration
		err     string
	}
	cases := []tc{
		{"not set", 5000, nil, 5 * time.Second, ""},
		{"not set, timeout 1", 1, nil, time.Millisecond, ""},
		{"not set, timeout 0", 0, nil, 0, ""},
		{"not set, negative timeout", -1, nil, 0, ""},
		{"0", 5000, ms(0), 0, ""},
		{"0, timeout 0", 0, ms(0), 0, ""},
		{"1", 5000, ms(1), time.Millisecond, ""},
		{"above timeout", 200, ms(60000), time.Minute, ""},
		{"below timeout", 5000, ms(200), 200 * time.Millisecond, ""},
		{"timeout 0", 0, ms(200), 200 * time.Millisecond, ""},
		{"negative timeout", -1, ms(200), 200 * time.Millisecond, ""},
		{"-1", 5000, ms(-1), 0, "request-timeout must be >= 0"},
		{"minimum int", 5000, ms(math.MinInt), 0, "request-timeout must be >= 0"},
	}
	if strconv.IntSize == 64 {
		limit := maxRequestTimeoutMS
		cases = append(cases,
			tc{"largest duration", 5000, ms(int(limit)), time.Duration(limit) * time.Millisecond, ""},
			tc{"above the largest duration", 5000, ms(int(limit + 1)), 0, "request-timeout must be at most"},
			tc{"maximum int", 5000, ms(math.MaxInt), 0, "request-timeout must be at most"},
		)
	} else {
		largest := math.MaxInt
		cases = append(cases, tc{"maximum int", 5000, ms(largest), time.Duration(largest) * time.Millisecond, ""})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, bound := range []func() (time.Duration, error){
				DeviceConfig{Timeout: c.timeout, RequestTimeout: c.request}.RequestBound,
				ProfileConfig{Timeout: c.timeout, RequestTimeout: c.request}.RequestBound,
			} {
				got, err := bound()
				if c.err != "" {
					if err == nil || !strings.Contains(err.Error(), c.err) {
						t.Fatalf("err = %v, want %q", err, c.err)
					}
					continue
				}
				if err != nil || got != c.want {
					t.Fatalf("got %v (err %v), want %v", got, err, c.want)
				}
			}
		})
	}
}

// TestToClientOptions_RequestTimeout: the options for each --request-timeout
// value next to --timeout (the connecting bound is WithTimeout, the request
// bound WithRequestTimeout, the rack/slot the first option).
func TestToClientOptions_RequestTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout int
		request *int
		n       int
	}{
		{"not set: both from --timeout", 100, nil, 3},
		{"not set, timeout 0: neither", 0, nil, 1},
		{"0: connecting only", 100, ms(0), 2},
		{"0, timeout 0: neither", 0, ms(0), 1},
		{"above 0: both", 100, ms(200), 3},
		{"above 0, timeout 0: requests only", 0, ms(200), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := DeviceConfig{Timeout: tc.timeout, RequestTimeout: tc.request}.ToClientOptions()
			if err != nil {
				t.Fatal(err)
			}
			if len(opts) != tc.n {
				t.Fatalf("got %d options, want %d", len(opts), tc.n)
			}
		})
	}
	_, err := DeviceConfig{Timeout: 100, RequestTimeout: ms(-1)}.ToClientOptions()
	if err == nil || !strings.Contains(err.Error(), "request-timeout must be >= 0") {
		t.Fatalf("negative: err = %v", err)
	}
}

// TestRequestTimeoutFlag: the flag and the environment set the field; without
// either it stays nil, which is not 0.
func TestRequestTimeoutFlag(t *testing.T) {
	parse := func(t *testing.T, args ...string) (*int, error) {
		t.Helper()
		cfg := DefaultDeviceConfig()
		cmd := &cobra.Command{Use: "x"}
		RegisterFlags(cmd, &cfg)
		f := cmd.Flags().Lookup("request-timeout")
		if f == nil {
			t.Fatal("flag --request-timeout is not registered")
		}
		if f.DefValue != "" || f.Value.Type() != "int" || !strings.Contains(f.Usage, "[env: S7COMMCTL_REQUEST_TIMEOUT]") {
			t.Fatalf("flag --request-timeout: default %q type %q usage %q", f.DefValue, f.Value.Type(), f.Usage)
		}
		err := cmd.ParseFlags(args)
		if err == nil && cfg.RequestTimeout != nil && f.Value.String() != strconv.Itoa(*cfg.RequestTimeout) {
			t.Errorf("String() = %q, field %d", f.Value.String(), *cfg.RequestTimeout)
		}
		return cfg.RequestTimeout, err
	}
	for _, tc := range []struct {
		name string
		args []string
		want *int
	}{
		{"not set", nil, nil},
		{"0", []string{"--request-timeout", "0"}, ms(0)},
		{"above 0", []string{"--request-timeout", "250"}, ms(250)},
		{"hex, as an int flag", []string{"--request-timeout", "0x10"}, ms(16)},
		{"negative: refused later, by RequestBound", []string{"--request-timeout=-1"}, ms(-1)},
		{"empty: not set", []string{"--request-timeout", "7", "--request-timeout="}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parse(t, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
				t.Fatalf("got %v, want %v", deref(got), deref(tc.want))
			}
		})
	}
	if _, err := parse(t, "--request-timeout", "soon"); err == nil || !strings.Contains(err.Error(), `invalid argument "soon"`) {
		t.Errorf("not a number: err = %v", err)
	}
	if _, err := parse(t, "--request-timeout", "99999999999999999999"); err == nil {
		t.Error("out of the int range: no error")
	}

	for _, tc := range []struct {
		env  string
		want *int
	}{
		{"", nil},
		{"0", ms(0)},
		{"250", ms(250)},
		{"-1", ms(-1)},
		{"soon", nil}, // skipped, as for every int variable
	} {
		t.Run("env "+tc.env, func(t *testing.T) {
			t.Setenv("S7COMMCTL_REQUEST_TIMEOUT", tc.env)
			for _, got := range func() []*int {
				d, p := DefaultDeviceConfig(), DefaultProfileConfig()
				LoadFromEnv(&d)
				LoadFromEnv(&p)
				return []*int{d.RequestTimeout, p.RequestTimeout}
			}() {
				if (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
					t.Fatalf("got %v, want %v", deref(got), deref(tc.want))
				}
			}
		})
	}
}

func deref(p *int) any {
	if p == nil {
		return "not set"
	}
	return *p
}

func TestToClientOptions_InvalidRemoteTSAP(t *testing.T) {
	_, err := DeviceConfig{LocalTSAP: "0x0100", RemoteTSAP: "bad"}.ToClientOptions()
	if err == nil || !strings.Contains(err.Error(), "invalid remote-tsap") {
		t.Fatalf("err = %v", err)
	}
}

func TestToProbeRequest_Mapping(t *testing.T) {
	req, err := ProbeRackSlotConfig{
		IP: "10.0.0.1", Port: 1102, RackMin: 1, RackMax: 2, SlotMin: 3, SlotMax: 4, Timeout: 300, Parallelism: 2,
		DelayMS: 7, First: true, Strict: true, Retries: 2, RetryDelayMS: 9, FirstConfirmed: true,
		LocalTSAP: "0x0100", RemoteTSAP: "770",
	}.ToProbeRequest()
	if err != nil {
		t.Fatal(err)
	}
	if req.Address != "10.0.0.1" || req.Port != 1102 || req.RackMin != 1 || req.RackMax != 2 || req.SlotMin != 3 || req.SlotMax != 4 {
		t.Errorf("target = %+v", req)
	}
	if req.Timeout != 300*time.Millisecond || req.Parallelism != 2 || req.DelayMS != 7 || req.RetryDelay != 9*time.Millisecond {
		t.Errorf("timing = %+v", req)
	}
	if !req.StopOnFirst || !req.Strict || !req.StopOnFirstConfirmed || req.Retries != 2 {
		t.Errorf("mode = %+v", req)
	}
	if req.Confirm != client.ConfirmAny {
		t.Errorf("strict without --confirm uses %v, want any", req.Confirm)
	}
	if req.LocalTSAP == nil || *req.LocalTSAP != 0x0100 || req.RemoteTSAP == nil || *req.RemoteTSAP != 770 {
		t.Errorf("TSAPs = %v %v", req.LocalTSAP, req.RemoteTSAP)
	}
}

func TestToProbeRequest_Defaults(t *testing.T) {
	req, err := ProbeRackSlotConfig{IP: "h", Retries: -1}.ToProbeRequest()
	if err != nil {
		t.Fatal(err)
	}
	if req.Port != 102 || req.Timeout != 2*time.Second || req.Parallelism != 4 || req.Retries != 0 {
		t.Fatalf("defaults = %+v", req)
	}
	if req.LocalTSAP != nil || req.RemoteTSAP != nil {
		t.Fatal("TSAPs set without flags")
	}
}

func TestToProbeRequest_InvalidTSAP(t *testing.T) {
	for _, tc := range []struct {
		local, remote, want string
	}{
		{"bad", "1", "invalid local-tsap"},
		{"1", "99999", "invalid remote-tsap"},
	} {
		_, err := ProbeRackSlotConfig{LocalTSAP: tc.local, RemoteTSAP: tc.remote}.ToProbeRequest()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("(%q, %q): err = %v, want %q", tc.local, tc.remote, err, tc.want)
		}
	}
}

func TestParseConfirmKind(t *testing.T) {
	for _, tc := range []struct {
		in     string
		strict bool
		want   client.ConfirmationKind
	}{
		{"szl", false, client.ConfirmSZL},
		{" SZL ", true, client.ConfirmSZL},
		{"cpu-state", false, client.ConfirmCPUState},
		{"any", false, client.ConfirmAny},
		{"", false, client.ConfirmNone},
		{"", true, client.ConfirmAny},
		{"none", false, client.ConfirmNone},
		{"none", true, client.ConfirmAny},
		{"bogus", false, client.ConfirmNone},
		{"bogus", true, client.ConfirmAny},
	} {
		if got := parseConfirmKind(tc.in, tc.strict); got != tc.want {
			t.Errorf("parseConfirmKind(%q, %v) = %v, want %v", tc.in, tc.strict, got, tc.want)
		}
	}
}

func TestDefaultConfigs_Commands(t *testing.T) {
	if s := DefaultScanReadRangesConfig(); s.Area != "DB" || s.End != 256 || s.ProbeSize != 8 || s.Format != "spans" || s.Port != 102 {
		t.Errorf("scan defaults: %+v", s)
	}
	if c := DefaultCompareReadConfig(); c.Port != 102 || c.Timeout != 5000 || c.Format != "table" {
		t.Errorf("compare defaults: %+v", c)
	}
	if p := DefaultProfileConfig(); p.Port != 102 || p.Timeout != 5000 || p.Format != "text" {
		t.Errorf("profile defaults: %+v", p)
	}
}

type flagFixture struct {
	DeviceConfig
	Name    string `flag:"name" env:"S7COMMCTL_TEST_NAME" desc:"A name"`
	Count   int    `flag:"count" desc:"A count"`
	On      bool   `flag:"on" desc:"A switch"`
	NoFlag  string
	Ignored []string `flag:"ignored" env:"S7COMMCTL_TEST_LIST" desc:"No flag type"`
}

func TestRegisterFlags(t *testing.T) {
	cfg := flagFixture{Name: "default", Count: 3}
	cfg.Port = 102
	cmd := &cobra.Command{Use: "x"}
	RegisterFlags(cmd, &cfg)

	for _, name := range []string{"name", "count", "on", "ip", "port", "rack", "quiet"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s is not registered", name)
		}
	}
	if cmd.Flags().Lookup("ignored") != nil {
		t.Error("a slice field has no flag type")
	}
	f := cmd.Flags().Lookup("name")
	if f.DefValue != "default" || !strings.Contains(f.Usage, "[env: S7COMMCTL_TEST_NAME]") {
		t.Errorf("flag --name: default %q usage %q", f.DefValue, f.Usage)
	}
	if u := cmd.Flags().Lookup("count").Usage; strings.Contains(u, "[env:") {
		t.Errorf("flag without env names one: %q", u)
	}

	if err := cmd.ParseFlags([]string{"--name", "n", "--count", "9", "--on", "--port", "1102"}); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "n" || cfg.Count != 9 || !cfg.On || cfg.Port != 1102 {
		t.Fatalf("flags did not reach the config: %+v", cfg)
	}
}

func TestLoadFromEnv_Types(t *testing.T) {
	t.Setenv("S7COMMCTL_TEST_NAME", "from-env")
	t.Setenv("S7COMMCTL_TEST_LIST", "a,b")
	t.Setenv("S7COMMCTL_PORT", "1102")
	t.Setenv("S7COMMCTL_QUIET", "true")
	cfg := flagFixture{}
	LoadFromEnv(&cfg)
	if cfg.Name != "from-env" || cfg.Port != 1102 || !cfg.Quiet {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.Ignored) != 2 || cfg.Ignored[1] != "b" {
		t.Fatalf("slice = %v", cfg.Ignored)
	}
}

func TestLoadFromEnv_InvalidValuesKeepDefaults(t *testing.T) {
	t.Setenv("S7COMMCTL_PORT", "not-a-number")
	t.Setenv("S7COMMCTL_QUIET", "not-a-bool")
	cfg := flagFixture{}
	cfg.Port = 102
	LoadFromEnv(&cfg)
	if cfg.Port != 102 || cfg.Quiet {
		t.Fatalf("cfg = %+v", cfg)
	}
}
