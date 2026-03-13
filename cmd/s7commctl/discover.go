package s7commctl

import (
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/s7comm/client"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var discoverConfig = config.DefaultDiscoverConfig()

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
		return err
	}

	applyDiscoverProfile(&discoverConfig)

	results, err := client.Discover(
		cmd.Context(),
		discoverConfig.CIDR,
		client.WithDiscoverTimeout(discoverConfig.Timeout),
		client.WithDiscoverParallel(discoverConfig.Parallel),
		client.WithDiscoverRackSlotRange(discoverConfig.RackMin, discoverConfig.RackMax, discoverConfig.SlotMin, discoverConfig.SlotMax),
		client.WithDiscoverRateLimit(discoverConfig.RateMS),
	)
	if err != nil {
		return err
	}

	var found []client.DiscoverResult
	for _, r := range results {
		if r.IsS7 {
			found = append(found, r)
		}
	}

	if len(found) == 0 {
		if discoverConfig.Format != "json" {
			fmt.Println("No S7 devices found")
		}
		return nil
	}

	if discoverConfig.Format == "quiet" {
		for _, d := range found {
			_, _ = fmt.Fprintln(os.Stdout, d.IP)
		}
		return nil
	}

	switch strings.ToLower(discoverConfig.Format) {
	case output.FormatJSON:
		output.PrintJSON(found)
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
