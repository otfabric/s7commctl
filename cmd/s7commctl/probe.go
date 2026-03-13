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

var probeRackSlotConfig = config.DefaultProbeRackSlotConfig()

var probeCmd = &cobra.Command{
	Use:   "probe",
	Short: "Probe targets for S7 connectivity",
}

var probeRackSlotCmd = &cobra.Command{
	Use:   "rackslot",
	Short: "Probe a single host for valid rack/slot combinations",
	Long: `Probe a specific IP for valid Siemens S7 rack/slot pairs.
Reports TCP reachability, COTP, S7 setup success, and optional SZL query result.
Exit 0 if at least one valid rack/slot found; exit 1 if none.`,
	RunE: runProbeRackSlot,
}

func init() {
	rootCmd.AddCommand(probeCmd)
	probeCmd.AddCommand(probeRackSlotCmd)
	config.RegisterFlags(probeRackSlotCmd, &probeRackSlotConfig)
	config.LoadFromEnv(&probeRackSlotConfig)
}

func runProbeRackSlot(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(probeRackSlotConfig.IP); err != nil {
		return err
	}

	req := probeRackSlotConfig.ToProbeRequest()
	result, err := client.ProbeRackSlots(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("probe failed: %w", err)
	}

	format := strings.ToLower(probeRackSlotConfig.Format)
	switch format {
	case output.FormatJSON:
		output.PrintJSON(result)
	case output.FormatTable:
		printProbeRackSlotTable(result)
	default:
		printProbeRackSlotText(result)
	}

	if len(result.Valid) == 0 {
		os.Exit(1)
	}
	return nil
}

func printProbeRackSlotTable(res *client.RackSlotProbeResult) {
	fmt.Printf("Target: %s\n", res.Address)
	fmt.Println("Rack  Slot  Status          PDU   LocalTSAP  RemoteTSAP  Error")
	fmt.Println("----  ----  --------------  ----  ---------  ----------  -----")
	for _, c := range res.Candidates {
		errStr := c.Error
		if len(errStr) > 20 {
			errStr = errStr[:17] + "..."
		}
		_, _ = fmt.Fprintf(os.Stdout, "%-4d  %-4d  %-14s  %-4d  %-9d  %-10d  %s\n",
			c.Rack, c.Slot, c.Classification, c.PDUSize, c.LocalTSAP, c.RemoteTSAP, errStr)
	}
	if len(res.Valid) > 0 {
		fmt.Println()
		fmt.Printf("Valid: %d combination(s)\n", len(res.Valid))
		for _, c := range res.Valid {
			fmt.Printf("  rack=%d slot=%d (PDU=%d)\n", c.Rack, c.Slot, c.PDUSize)
		}
	}
}

func printProbeRackSlotText(res *client.RackSlotProbeResult) {
	fmt.Printf("target=%s candidates=%d valid=%d\n", res.Address, len(res.Candidates), len(res.Valid))
	for _, c := range res.Candidates {
		_, _ = fmt.Fprintf(os.Stdout, "  rack=%d slot=%d classification=%s pdu=%d error=%s\n",
			c.Rack, c.Slot, c.Classification, c.PDUSize, c.Error)
	}
}
