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
Reports TCP reachability, COTP, S7 setup success, and optional follow-up confirmation (--strict).
Default: valid = setup accepted. With --strict: valid = setup + benign follow-up query (valid-query).
Use --confirm szl|cpu-state|any to choose confirmation strategy. Exit 0 if at least one valid; exit 1 if none.`,
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
	mode := "default (setup accepted = valid)"
	if probeRackSlotConfig.Strict {
		confirm := strings.ToLower(string(probeRackSlotConfig.Confirm))
		if confirm == "" {
			confirm = "any"
		}
		mode = fmt.Sprintf("strict (confirm=%s)", confirm)
		if probeRackSlotConfig.Retries > 0 {
			mode += fmt.Sprintf(", retries=%d", probeRackSlotConfig.Retries)
		}
	}
	fmt.Printf("Mode: %s\n\n", mode)
	fmt.Println("Rack  Slot  Stage   Status         PDU  ConfirmedBy  Confidence  Error")
	fmt.Println("----  ----  ------  -------------  ---  -----------  ----------  --------------")
	for _, c := range res.Candidates {
		errStr := c.Error
		if len(errStr) > 25 {
			errStr = errStr[:22] + "..."
		}
		confirmedBy := string(c.ConfirmedBy)
		if confirmedBy == "none" || confirmedBy == "" {
			confirmedBy = "-"
		}
		_, _ = fmt.Fprintf(os.Stdout, "%-4d  %-4d  %-6s  %-13s  %-3d  %-11s  %-10s  %s\n",
			c.Rack, c.Slot, c.Stage, c.Status, c.PDUSize, confirmedBy, c.Confidence, errStr)
	}
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("- Setup accepted: %d\n", res.SetupAccepted)
	fmt.Printf("- Confirmed by follow-up query: %d\n", res.ConfirmedByQuery)
	fmt.Printf("- TCP-only: %d\n", res.TCPOnly)
	if res.Flaky > 0 {
		fmt.Printf("- Flaky: %d\n", res.Flaky)
	}
	if len(res.Valid) > 0 {
		fmt.Println()
		fmt.Println("Confirmed valid combinations:")
		for _, c := range res.Valid {
			by := string(c.ConfirmedBy)
			if by == "none" || by == "" {
				by = "setup"
			}
			fmt.Printf("- rack=%d slot=%d (confirmed by %s, PDU=%d)\n", c.Rack, c.Slot, by, c.PDUSize)
		}
	}
}

func printProbeRackSlotText(res *client.RackSlotProbeResult) {
	fmt.Printf("target=%s candidates=%d valid=%d setup_accepted=%d confirmed_by_query=%d tcp_only=%d\n",
		res.Address, len(res.Candidates), len(res.Valid), res.SetupAccepted, res.ConfirmedByQuery, res.TCPOnly)
	for _, c := range res.Candidates {
		_, _ = fmt.Fprintf(os.Stdout, "  rack=%d slot=%d stage=%s status=%s pdu=%d confirmed_by=%s confidence=%s error=%s\n",
			c.Rack, c.Slot, c.Stage, c.Status, c.PDUSize, c.ConfirmedBy, c.Confidence, c.Error)
	}
}
