// SPDX-License-Identifier: MIT

package s7commctl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/otfabric/go-s7comm/client"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var discoverConfig = config.DefaultDiscoverConfig()

// discoverHosts runs the scan. client.Discover always dials port 102, which a
// test cannot bind, so the tests replace it to see the options that reach it.
var discoverHosts = func(ctx context.Context, cfg config.DiscoverConfig) ([]client.DiscoverResult, error) {
	return client.Discover(
		ctx,
		cfg.CIDR,
		client.WithDiscoverTimeout(cfg.Timeout),
		client.WithDiscoverParallel(cfg.Parallel),
		client.WithDiscoverRackSlotRange(cfg.RackMin, cfg.RackMax, cfg.SlotMin, cfg.SlotMax),
		client.WithDiscoverRateLimit(cfg.RateMS),
	)
}

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover S7 devices on a network",
	RunE:  runDiscover,
}

func init() {
	config.RegisterFlags(discoverCmd, &discoverConfig)
	config.LoadFromEnv(&discoverConfig)
	rootCmd.AddCommand(discoverCmd)
}

func runDiscover(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateCIDR(discoverConfig.CIDR); err != nil {
		return ExitUsage(err)
	}

	cfg := resolveDiscoverConfig(cmd, discoverConfig)
	if err := validateDiscoverRange(cfg); err != nil {
		return ExitUsage(err)
	}

	results, err := discoverHosts(cmd.Context(), cfg)
	if err != nil {
		return ExitRuntime(err)
	}

	var found []client.DiscoverResult
	for _, r := range results {
		if r.IsS7 {
			found = append(found, r)
		}
	}

	printDiscovered(found, cfg)
	return nil
}

// printDiscovered writes the S7 devices found to stdout in cfg.Format.
func printDiscovered(found []client.DiscoverResult, cfg config.DiscoverConfig) {
	quiet := cfg.Quiet || strings.ToLower(cfg.Format) == "quiet"
	if len(found) == 0 {
		if quiet {
			return
		}
		if strings.ToLower(cfg.Format) == output.FormatJSON {
			discoverResultsJSON(found) // emit [] for scriptability
		} else {
			fmt.Println("No S7 devices found")
		}
		return
	}

	if quiet {
		for _, d := range found {
			_, _ = fmt.Fprintln(os.Stdout, d.IP)
		}
		return
	}

	switch strings.ToLower(cfg.Format) {
	case output.FormatJSON:
		discoverResultsJSON(found)
	case output.FormatTable:
		_, _ = fmt.Fprintln(os.Stdout, "IP              Rack Slot PDU  TSAP")
		_, _ = fmt.Fprintln(os.Stdout, "--------------- ---- ---- ---- ------")
		for _, d := range found {
			_, _ = fmt.Fprintf(os.Stdout, "%-15s %4d %4d %4d %s\n", d.IP, d.Rack, d.Slot, d.PDUSize, d.TSAP)
		}
	default:
		for _, d := range found {
			_, _ = fmt.Fprintf(os.Stdout, "%s rack=%d slot=%d pdu=%d tsap=%s\n", d.IP, d.Rack, d.Slot, d.PDUSize, d.TSAP)
		}
	}
}

func discoverResultsJSON(results []client.DiscoverResult) {
	out := make([]map[string]interface{}, 0, len(results))
	for _, d := range results {
		m := map[string]interface{}{
			"ip":       d.IP,
			"port":     d.Port,
			"is_s7":    d.IsS7,
			"rack":     d.Rack,
			"slot":     d.Slot,
			"pdu_size": d.PDUSize,
			"tsap":     d.TSAP,
		}
		if d.Error != "" {
			m["error"] = d.Error
		}
		if d.AbandonedReason != "" {
			m["abandoned_reason"] = d.AbandonedReason
		}
		out = append(out, m)
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}

// resolveDiscoverConfig applies the profile to cfg, then restores each range
// bound the user set by flag or environment: those beat the profile.
func resolveDiscoverConfig(cmd *cobra.Command, cfg config.DiscoverConfig) config.DiscoverConfig {
	resolved := cfg
	applyDiscoverProfile(&resolved)
	for _, b := range []struct {
		flag, env string
		dst       *int
		value     int
	}{
		{"rack-min", "S7COMMCTL_RACK_MIN", &resolved.RackMin, cfg.RackMin},
		{"rack-max", "S7COMMCTL_RACK_MAX", &resolved.RackMax, cfg.RackMax},
		{"slot-min", "S7COMMCTL_SLOT_MIN", &resolved.SlotMin, cfg.SlotMin},
		{"slot-max", "S7COMMCTL_SLOT_MAX", &resolved.SlotMax, cfg.SlotMax},
	} {
		// config.LoadFromEnv skips a value that is not a number.
		_, envErr := strconv.Atoi(os.Getenv(b.env))
		if cmd.Flags().Changed(b.flag) || envErr == nil {
			*b.dst = b.value
		}
	}
	return resolved
}

// validateDiscoverRange rejects a range that is empty or outside what S7 addresses.
func validateDiscoverRange(cfg config.DiscoverConfig) error {
	if err := validate.ValidateRackSlot(cfg.RackMin, cfg.SlotMin); err != nil {
		return err
	}
	if err := validate.ValidateRackSlot(cfg.RackMax, cfg.SlotMax); err != nil {
		return err
	}
	if cfg.RackMin > cfg.RackMax {
		return fmt.Errorf("rack range is empty: min %d is above max %d", cfg.RackMin, cfg.RackMax)
	}
	if cfg.SlotMin > cfg.SlotMax {
		return fmt.Errorf("slot range is empty: min %d is above max %d", cfg.SlotMin, cfg.SlotMax)
	}
	return nil
}

func applyDiscoverProfile(cfg *config.DiscoverConfig) {
	switch strings.ToLower(cfg.Profile) {
	case "quick":
		cfg.RackMin, cfg.RackMax = 0, 0
		cfg.SlotMin, cfg.SlotMax = 1, 2
		if cfg.Timeout <= 0 {
			cfg.Timeout = 800
		}
		if cfg.Parallel <= 0 {
			cfg.Parallel = 20
		}
	case "full":
		cfg.RackMin, cfg.RackMax = 0, 3
		cfg.SlotMin, cfg.SlotMax = 0, 5
		if cfg.Timeout <= 0 {
			cfg.Timeout = 2500
		}
		if cfg.Parallel <= 0 {
			cfg.Parallel = 8
		}
	default: // standard
		cfg.RackMin, cfg.RackMax = 0, 1
		cfg.SlotMin, cfg.SlotMax = 0, 3
	}
}
