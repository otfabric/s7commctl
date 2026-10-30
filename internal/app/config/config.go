// SPDX-License-Identifier: MIT

// Package config provides CLI configuration and flag registration.
package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/go-s7comm/client"

	"github.com/spf13/cobra"
)

// DeviceConfig contains common connection parameters
type DeviceConfig struct {
	IP                string `flag:"ip" env:"S7COMMCTL_IP" desc:"S7 device IP address"`
	Port              int    `flag:"port" env:"S7COMMCTL_PORT" desc:"S7 port (default 102)"`
	Rack              int    `flag:"rack" env:"S7COMMCTL_RACK" desc:"PLC rack number"`
	Slot              int    `flag:"slot" env:"S7COMMCTL_SLOT" desc:"PLC slot number"`
	AutoRack          bool   `flag:"auto-rack-slot" env:"S7COMMCTL_AUTO_RACK_SLOT" desc:"Auto-detect rack/slot"`
	BruteRack         bool   `flag:"brute-rack-slot" env:"S7COMMCTL_BRUTE_RACK_SLOT" desc:"Brute rack 0-3 and slot 0-5"`
	LocalTSAP         string `flag:"local-tsap" env:"S7COMMCTL_LOCAL_TSAP" desc:"Local TSAP (decimal or 0x hex)"`
	RemoteTSAP        string `flag:"remote-tsap" env:"S7COMMCTL_REMOTE_TSAP" desc:"Remote TSAP (decimal or 0x hex)"`
	Timeout           int    `flag:"timeout" env:"S7COMMCTL_TIMEOUT" desc:"Timeout in milliseconds"`
	RateLimitMS       int    `flag:"rate-limit-ms" env:"S7COMMCTL_RATE_LIMIT_MS" desc:"Minimum delay between protocol operations in ms"`
	Quiet             bool   `flag:"quiet" env:"S7COMMCTL_QUIET" desc:"Minimal output"`
	NoExposureWarning bool   `flag:"no-exposure-warning" env:"S7COMMCTL_NO_EXPOSURE_WARNING" desc:"Suppress public routable IP warning (e.g. authorized environments)"`
}

// ToClientOptions converts CLI config to library options.
// Connection mode precedence: 1) explicit TSAP 2) auto rack/slot 3) manual rack/slot.
// Returns error if TSAP is partially set or invalid.
func (c DeviceConfig) ToClientOptions() ([]client.Option, error) {
	var opts []client.Option

	// Explicit TSAP takes precedence
	if c.LocalTSAP != "" || c.RemoteTSAP != "" {
		if (c.LocalTSAP == "") != (c.RemoteTSAP == "") {
			return nil, fmt.Errorf("both --local-tsap and --remote-tsap must be set together")
		}
		local, err1 := strconv.ParseUint(c.LocalTSAP, 0, 16)
		remote, err2 := strconv.ParseUint(c.RemoteTSAP, 0, 16)
		if err1 != nil {
			return nil, fmt.Errorf("invalid local-tsap %q: %w", c.LocalTSAP, err1)
		}
		if err2 != nil {
			return nil, fmt.Errorf("invalid remote-tsap %q: %w", c.RemoteTSAP, err2)
		}
		opts = append(opts, client.WithTSAP(uint16(local), uint16(remote)))
	} else if c.AutoRack {
		opts = append(opts, client.WithAutoRackSlot(c.BruteRack))
	} else {
		opts = append(opts, client.WithRackSlot(c.Rack, c.Slot))
	}

	if c.Port > 0 {
		opts = append(opts, client.WithPort(c.Port))
	}
	if c.Timeout > 0 {
		opts = append(opts, client.WithTimeout(time.Duration(c.Timeout)*time.Millisecond))
	}
	if c.RateLimitMS > 0 {
		opts = append(opts, client.WithRateLimit(time.Duration(c.RateLimitMS)*time.Millisecond))
	}
	return opts, nil
}

// UseExplicitTSAP reports whether both local and remote TSAP are set (connection mode precedence 1).
func (c DeviceConfig) UseExplicitTSAP() bool {
	return c.LocalTSAP != "" && c.RemoteTSAP != ""
}

// ConnectionRequestMeta returns a map for JSON "request" describing connection mode and params (connection_mode, rack, slot, local_tsap, remote_tsap).
// Use when building target/request/result envelopes so request reflects actual mode instead of raw rack/slot when TSAP or auto-rack is used.
func (c DeviceConfig) ConnectionRequestMeta() map[string]interface{} {
	out := make(map[string]interface{})
	if c.UseExplicitTSAP() {
		out["connection_mode"] = "explicit_tsap"
		out["local_tsap"] = c.LocalTSAP
		out["remote_tsap"] = c.RemoteTSAP
		return out
	}
	if c.AutoRack {
		out["connection_mode"] = "auto_rack_slot"
		out["rack"] = c.Rack
		out["slot"] = c.Slot
		return out
	}
	out["connection_mode"] = "rack_slot"
	out["rack"] = c.Rack
	out["slot"] = c.Slot
	return out
}

// DefaultDeviceConfig returns sensible defaults
func DefaultDeviceConfig() DeviceConfig {
	return DeviceConfig{Port: 102, Rack: 0, Slot: 1, Timeout: 5000}
}

// DiscoverConfig for the discover command
type DiscoverConfig struct {
	CIDR     string `flag:"cidr" env:"S7COMMCTL_CIDR" desc:"CIDR range to scan"`
	Timeout  int    `flag:"timeout" env:"S7COMMCTL_TIMEOUT" desc:"Timeout per host in ms"`
	Parallel int    `flag:"parallel" env:"S7COMMCTL_PARALLEL" desc:"Number of parallel probes"`
	RackMin  int    `flag:"rack-min" env:"S7COMMCTL_RACK_MIN" desc:"Minimum rack to probe"`
	RackMax  int    `flag:"rack-max" env:"S7COMMCTL_RACK_MAX" desc:"Maximum rack to probe"`
	SlotMin  int    `flag:"slot-min" env:"S7COMMCTL_SLOT_MIN" desc:"Minimum slot to probe"`
	SlotMax  int    `flag:"slot-max" env:"S7COMMCTL_SLOT_MAX" desc:"Maximum slot to probe"`
	Profile  string `flag:"profile" env:"S7COMMCTL_DISCOVER_PROFILE" desc:"Probe profile: quick, standard, full"`
	RateMS   int    `flag:"rate-ms" env:"S7COMMCTL_RATE_MS" desc:"Delay between rack/slot probes in ms"`
	Format   string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: json, table, text"`
	Quiet    bool   `flag:"quiet" env:"S7COMMCTL_QUIET" desc:"Only print discovered IP addresses (one per line)"`
}

func DefaultDiscoverConfig() DiscoverConfig {
	return DiscoverConfig{Timeout: 2000, Parallel: 10, RackMin: 0, RackMax: 3, SlotMin: 0, SlotMax: 5, Profile: "standard", RateMS: 0, Format: "table"}
}

// IdentifyConfig for identify command
type IdentifyConfig struct {
	DeviceConfig
	Format string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format"`
}

func DefaultIdentifyConfig() IdentifyConfig {
	return IdentifyConfig{DeviceConfig: DefaultDeviceConfig(), Format: "table"}
}

// ReadConfig for read command
type ReadConfig struct {
	DeviceConfig
	Area       string `flag:"area" env:"S7COMMCTL_AREA" desc:"Memory area: DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_COUNTER_200, IEC_TIMER_200"`
	DBNumber   int    `flag:"db" env:"S7COMMCTL_DB" desc:"Data block number"`
	Offset     int    `flag:"offset" env:"S7COMMCTL_OFFSET" desc:"Byte offset"`
	Size       int    `flag:"size" env:"S7COMMCTL_SIZE" desc:"Number of bytes"`
	Type       string `flag:"type" env:"S7COMMCTL_TYPE" desc:"Data type"`
	Format     string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format"`
	StrictRead bool   `flag:"strict-read" env:"S7COMMCTL_STRICT_READ" desc:"Fail when status is not success (e.g. short-read, empty-read)"`
}

func DefaultReadConfig() ReadConfig {
	return ReadConfig{DeviceConfig: DefaultDeviceConfig(), Area: "DB", DBNumber: 1, Size: 1, Type: "raw", Format: "plain"}
}

// WriteConfig for write command
type WriteConfig struct {
	DeviceConfig
	Area      string `flag:"area" env:"S7COMMCTL_AREA" desc:"Memory area (DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200)"`
	DBNumber  int    `flag:"db" env:"S7COMMCTL_DB" desc:"Data block number"`
	Offset    int    `flag:"offset" env:"S7COMMCTL_OFFSET" desc:"Byte offset"`
	Value     string `flag:"value" desc:"Value to write"`
	Type      string `flag:"type" desc:"Data type"`
	Format    string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: text, json"`
	Dangerous bool   `flag:"dangerous" desc:"Confirm write operation"`
	Force     bool   `flag:"force" desc:"Skip confirmation"`
}

func DefaultWriteConfig() WriteConfig {
	return WriteConfig{DeviceConfig: DefaultDeviceConfig(), Area: "DB", DBNumber: 1, Type: "raw", Format: "text"}
}

// BlocksConfig for block commands
type BlocksConfig struct {
	DeviceConfig
	BlockType string `flag:"type" desc:"Block type: OB, DB, FC, FB"`
	Number    int    `flag:"number" desc:"Block number"`
	Format    string `flag:"format" desc:"Output format"`
}

func DefaultBlocksConfig() BlocksConfig {
	return BlocksConfig{DeviceConfig: DefaultDeviceConfig(), Format: "table"}
}

// DiagnosticConfig for diagnostic command
type DiagnosticConfig struct {
	DeviceConfig
	Format string `flag:"format" desc:"Output format"`
}

func DefaultDiagnosticConfig() DiagnosticConfig {
	return DiagnosticConfig{DeviceConfig: DefaultDeviceConfig(), Format: "table"}
}

// FingerprintConfig for fingerprint command
type FingerprintConfig struct {
	DeviceConfig
	Format string `flag:"format" desc:"Output format"`
}

func DefaultFingerprintConfig() FingerprintConfig {
	return FingerprintConfig{DeviceConfig: DefaultDeviceConfig(), Format: "table"}
}

// ProbeRackSlotConfig for probe rackslot command
type ProbeRackSlotConfig struct {
	IP                string `flag:"ip" env:"S7COMMCTL_IP" desc:"Target IP address"`
	NoExposureWarning bool   `flag:"no-exposure-warning" env:"S7COMMCTL_NO_EXPOSURE_WARNING" desc:"Suppress public routable IP warning"`
	Port              int    `flag:"port" env:"S7COMMCTL_PORT" desc:"TCP port (default 102)"`
	RackMin           int    `flag:"rack-min" env:"S7COMMCTL_RACK_MIN" desc:"Minimum rack to probe"`
	RackMax           int    `flag:"rack-max" env:"S7COMMCTL_RACK_MAX" desc:"Maximum rack to probe"`
	SlotMin           int    `flag:"slot-min" env:"S7COMMCTL_SLOT_MIN" desc:"Minimum slot to probe"`
	SlotMax           int    `flag:"slot-max" env:"S7COMMCTL_SLOT_MAX" desc:"Maximum slot to probe"`
	Timeout           int    `flag:"timeout" env:"S7COMMCTL_TIMEOUT" desc:"Per-attempt timeout in ms"`
	Parallelism       int    `flag:"parallelism" env:"S7COMMCTL_PROBE_PARALLELISM" desc:"Concurrent probes"`
	DelayMS           int    `flag:"delay-ms" env:"S7COMMCTL_PROBE_DELAY_MS" desc:"Delay between attempts in ms"`
	First             bool   `flag:"first" env:"S7COMMCTL_PROBE_FIRST" desc:"Stop after first valid rack/slot"`
	Strict            bool   `flag:"strict" env:"S7COMMCTL_PROBE_STRICT" desc:"Only count valid-query as valid (confirm with follow-up)"`
	Confirm           string `flag:"confirm" env:"S7COMMCTL_PROBE_CONFIRM" desc:"Confirmation strategy: szl, cpu-state, any (default with --strict: any)"`
	Retries           int    `flag:"retries" env:"S7COMMCTL_PROBE_RETRIES" desc:"Retry count per candidate for inconclusive results"`
	RetryDelayMS      int    `flag:"retry-delay-ms" env:"S7COMMCTL_PROBE_RETRY_DELAY_MS" desc:"Delay between retries in ms"`
	FirstConfirmed    bool   `flag:"first-confirmed" env:"S7COMMCTL_PROBE_FIRST_CONFIRMED" desc:"Stop after first valid-query (strict mode)"`
	ShowAll           bool   `flag:"show-all" env:"S7COMMCTL_PROBE_SHOW_ALL" desc:"Show all candidates in table including non-valid"`
	SummaryOnly       bool   `flag:"summary-only" env:"S7COMMCTL_PROBE_SUMMARY_ONLY" desc:"Print only summary and interpretation (no table or examples)"`
	Repeat            int    `flag:"repeat" env:"S7COMMCTL_PROBE_REPEAT" desc:"Run probe N times and report variance (for rate-sensitive endpoints)"`
	RepeatIntervalMS  int    `flag:"interval-ms" env:"S7COMMCTL_PROBE_INTERVAL_MS" desc:"Delay between repeat runs in ms"`
	LocalTSAP         string `flag:"local-tsap" env:"S7COMMCTL_LOCAL_TSAP" desc:"Local TSAP (decimal or 0x hex)"`
	RemoteTSAP        string `flag:"remote-tsap" env:"S7COMMCTL_REMOTE_TSAP" desc:"Remote TSAP (decimal or 0x hex)"`
	Format            string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: json, table, text"`
}

func DefaultProbeRackSlotConfig() ProbeRackSlotConfig {
	return ProbeRackSlotConfig{
		Port:        102,
		RackMin:     0,
		RackMax:     7,
		SlotMin:     0,
		SlotMax:     31,
		Timeout:     2000,
		Parallelism: 4,
		DelayMS:     0,
		Format:      "table",
	}
}

// ToProbeRequest converts ProbeRackSlotConfig to client.RackSlotProbeRequest.
// Returns error if TSAP is partially set or invalid.
func (c ProbeRackSlotConfig) ToProbeRequest() (client.RackSlotProbeRequest, error) {
	req := client.DefaultRackSlotProbeRequest(c.IP)
	req.Port = c.Port
	if req.Port == 0 {
		req.Port = 102
	}
	req.RackMin = c.RackMin
	req.RackMax = c.RackMax
	req.SlotMin = c.SlotMin
	req.SlotMax = c.SlotMax
	req.Timeout = time.Duration(c.Timeout) * time.Millisecond
	if req.Timeout <= 0 {
		req.Timeout = 2 * time.Second
	}
	req.Parallelism = c.Parallelism
	if req.Parallelism <= 0 {
		req.Parallelism = 4
	}
	req.DelayMS = c.DelayMS
	req.StopOnFirst = c.First
	req.Strict = c.Strict
	req.Confirm = parseConfirmKind(c.Confirm, c.Strict)
	req.Retries = c.Retries
	if req.Retries < 0 {
		req.Retries = 0
	}
	req.RetryDelay = time.Duration(c.RetryDelayMS) * time.Millisecond
	req.StopOnFirstConfirmed = c.FirstConfirmed
	if c.LocalTSAP != "" || c.RemoteTSAP != "" {
		if (c.LocalTSAP == "") != (c.RemoteTSAP == "") {
			return req, fmt.Errorf("both --local-tsap and --remote-tsap must be set together")
		}
		local, err1 := strconv.ParseUint(c.LocalTSAP, 0, 16)
		remote, err2 := strconv.ParseUint(c.RemoteTSAP, 0, 16)
		if err1 != nil {
			return req, fmt.Errorf("invalid local-tsap %q: %w", c.LocalTSAP, err1)
		}
		if err2 != nil {
			return req, fmt.Errorf("invalid remote-tsap %q: %w", c.RemoteTSAP, err2)
		}
		l, r := uint16(local), uint16(remote)
		req.LocalTSAP = &l
		req.RemoteTSAP = &r
	}
	return req, nil
}

// ScanReadRangesConfig for scan read-ranges command
type ScanReadRangesConfig struct {
	DeviceConfig
	Area         string `flag:"area" env:"S7COMMCTL_AREA" desc:"Memory area: DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200"`
	DBNumber     int    `flag:"db" env:"S7COMMCTL_DB" desc:"Data block number (for DB area)"`
	Start        int    `flag:"start" env:"S7COMMCTL_SCAN_START" desc:"Start offset (inclusive)"`
	End          int    `flag:"end" env:"S7COMMCTL_SCAN_END" desc:"End offset (exclusive)"`
	Step         int    `flag:"step" env:"S7COMMCTL_SCAN_STEP" desc:"Step between probes (0 = use probe-size)"`
	ProbeSize    int    `flag:"probe-size" env:"S7COMMCTL_PROBE_SIZE" desc:"Bytes to read per probe"`
	Retries      int    `flag:"retries" env:"S7COMMCTL_SCAN_RETRIES" desc:"Retries per probe"`
	RetryDelayMS int    `flag:"retry-delay-ms" env:"S7COMMCTL_SCAN_RETRY_DELAY_MS" desc:"Delay between retries in ms"`
	Repeat       int    `flag:"repeat" env:"S7COMMCTL_SCAN_REPEAT" desc:"Repeat each probe N times for stability"`
	SummaryOnly  bool   `flag:"summary-only" env:"S7COMMCTL_SCAN_SUMMARY_ONLY" desc:"Print only span summary (no per-probe table)"`
	Format       string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: json, table, spans"`
}

func DefaultScanReadRangesConfig() ScanReadRangesConfig {
	return ScanReadRangesConfig{
		DeviceConfig: DefaultDeviceConfig(),
		Area:         "DB",
		DBNumber:     1,
		Start:        0,
		End:          256,
		Step:         0,
		ProbeSize:    8,
		Format:       "spans",
	}
}

// CompareReadConfig for compare read command
type CompareReadConfig struct {
	IP                string `flag:"ip" env:"S7COMMCTL_IP" desc:"Target IP"`
	NoExposureWarning bool   `flag:"no-exposure-warning" env:"S7COMMCTL_NO_EXPOSURE_WARNING" desc:"Suppress public routable IP warning"`
	Port              int    `flag:"port" env:"S7COMMCTL_PORT" desc:"Port (default 102)"`
	Candidates        string `flag:"candidates" env:"S7COMMCTL_COMPARE_CANDIDATES" desc:"Rack/slot pairs: 0/0,0/1,0/2"`
	Area              string `flag:"area" env:"S7COMMCTL_AREA" desc:"Memory area (DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200)"`
	DBNumber          int    `flag:"db" env:"S7COMMCTL_DB" desc:"Data block number"`
	Offset            int    `flag:"offset" env:"S7COMMCTL_OFFSET" desc:"Byte offset"`
	Size              int    `flag:"size" env:"S7COMMCTL_SIZE" desc:"Number of bytes"`
	Timeout           int    `flag:"timeout" env:"S7COMMCTL_TIMEOUT" desc:"Timeout per candidate in ms"`
	Repeat            int    `flag:"repeat" env:"S7COMMCTL_COMPARE_REPEAT" desc:"Repeat compare N times (for dynamic data)"`
	IntervalMS        int    `flag:"interval-ms" env:"S7COMMCTL_COMPARE_INTERVAL_MS" desc:"Delay between repeat runs in ms"`
	Format            string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: json, table"`
}

func DefaultCompareReadConfig() CompareReadConfig {
	return CompareReadConfig{Port: 102, Timeout: 5000, Format: "table"}
}

// ProfileConfig for profile/capabilities command
type ProfileConfig struct {
	IP                string `flag:"ip" env:"S7COMMCTL_IP" desc:"Target IP address"`
	Port              int    `flag:"port" env:"S7COMMCTL_PORT" desc:"TCP port (default 102)"`
	Timeout           int    `flag:"timeout" env:"S7COMMCTL_TIMEOUT" desc:"Timeout in milliseconds"`
	NoExposureWarning bool   `flag:"no-exposure-warning" env:"S7COMMCTL_NO_EXPOSURE_WARNING" desc:"Suppress public routable IP warning"`
	Format            string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: text, json"`
}

func DefaultProfileConfig() ProfileConfig {
	return ProfileConfig{Port: 102, Timeout: 5000, Format: "text"}
}

func parseConfirmKind(s string, strict bool) client.ConfirmationKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "szl":
		return client.ConfirmSZL
	case "cpu-state":
		return client.ConfirmCPUState
	case "any":
		return client.ConfirmAny
	case "none", "":
		if strict {
			return client.ConfirmAny
		}
		return client.ConfirmNone
	default:
		if strict {
			return client.ConfirmAny
		}
		return client.ConfirmNone
	}
}

// RegisterFlags registers flags based on struct tags
func RegisterFlags(cmd *cobra.Command, cfg interface{}) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		flagName := field.Tag.Get("flag")
		desc := field.Tag.Get("desc")
		envVar := field.Tag.Get("env")

		if field.Anonymous {
			RegisterFlags(cmd, v.Field(i).Addr().Interface())
			continue
		}
		if flagName == "" {
			continue
		}
		if envVar != "" {
			desc = desc + " [env: " + envVar + "]"
		}

		switch field.Type.Kind() {
		case reflect.String:
			cmd.Flags().StringVar(v.Field(i).Addr().Interface().(*string), flagName, v.Field(i).String(), desc)
		case reflect.Int:
			cmd.Flags().IntVar(v.Field(i).Addr().Interface().(*int), flagName, int(v.Field(i).Int()), desc)
		case reflect.Bool:
			cmd.Flags().BoolVar(v.Field(i).Addr().Interface().(*bool), flagName, v.Field(i).Bool(), desc)
		}
	}
}

// LoadFromEnv populates config from environment
func LoadFromEnv(cfg interface{}) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if fieldType.Anonymous && field.Kind() == reflect.Struct {
			LoadFromEnv(field.Addr().Interface())
			continue
		}

		tag := fieldType.Tag.Get("env")
		if tag == "" {
			continue
		}
		envValue := os.Getenv(tag)
		if envValue == "" && tag == "S7COMMCTL_FORMAT" {
			envValue = os.Getenv("S7COMMCTL_OUTPUT_FORMAT")
		}
		if envValue == "" {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Int:
			if val, err := strconv.Atoi(envValue); err == nil {
				field.SetInt(int64(val))
			}
		case reflect.Bool:
			if val, err := strconv.ParseBool(envValue); err == nil {
				field.SetBool(val)
			}
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				parts := strings.Split(envValue, ",")
				field.Set(reflect.ValueOf(parts))
			}
		}
	}
}
