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
	warnIfPublicTarget(probeRackSlotConfig.IP, false, "verify authorization and network exposure", probeRackSlotConfig.NoExposureWarning)

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
		if probeRackSlotConfig.SummaryOnly {
			printProbeSummaryOnly(result)
		} else {
			printProbeRackSlotTable(result)
		}
	default:
		if probeRackSlotConfig.SummaryOnly {
			printProbeSummaryOnly(result)
		} else {
			printProbeRackSlotText(result)
		}
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
	showAll := probeRackSlotConfig.ShowAll
	candidates := res.Candidates
	const maxExamplesPerStatus = 5
	if !showAll && len(candidates) > 15 {
		// Collapsed: summary by status + first N examples per status
		statusCounts := make(map[string]int)
		statusExamples := make(map[string][]client.RackSlotCandidate)
		for _, c := range candidates {
			s := string(c.Status)
			statusCounts[s]++
			if len(statusExamples[s]) < maxExamplesPerStatus {
				statusExamples[s] = append(statusExamples[s], c)
			}
		}
		fmt.Println("Summary by status:")
		for s, n := range statusCounts {
			fmt.Printf("- %s: %d\n", s, n)
		}
		fmt.Println()
		fmt.Println("Examples (use --show-all for full table):")
		fmt.Println("Rack  Slot  Stage   Status         PDU  ConfirmedBy  Confidence  Error")
		fmt.Println("----  ----  ------  -------------  ---  -----------  ----------  --------------")
		for _, examples := range statusExamples {
			for _, c := range examples {
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
		}
	} else {
		fmt.Println("Rack  Slot  Stage   Status         PDU  ConfirmedBy  Confidence  Error")
		fmt.Println("----  ----  ------  -------------  ---  -----------  ----------  --------------")
		for _, c := range candidates {
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
		if probeRackSlotConfig.Strict {
			fmt.Println("Query-confirmed combinations:")
		} else {
			fmt.Println("Setup-accepted combinations:")
		}
		for _, c := range res.Valid {
			by := string(c.ConfirmedBy)
			if by == "none" || by == "" {
				by = "setup"
			}
			fmt.Printf("- rack=%d slot=%d (confirmed by %s, PDU=%d)\n", c.Rack, c.Slot, by, c.PDUSize)
		}
	}
	printProbeInterpretation(res)
}

func printProbeInterpretation(res *client.RackSlotProbeResult) {
	fmt.Println()
	fmt.Println("Interpretation:")
	if res.SetupAccepted == 0 && res.TCPOnly == 0 {
		fmt.Println("- No successful TCP or setup; target unreachable or not speaking S7.")
		return
	}
	// REQUIREMENTS #24: recommended connection tuple when we have valid candidates
	if len(res.Valid) > 0 {
		first := res.Valid[0]
		fmt.Printf("- Recommended connection tuple: rack=%d slot=%d\n", first.Rack, first.Slot)
		if res.ConfirmedByQuery > 0 {
			fmt.Println("- Reason: first query-confirmed candidate.")
		} else {
			fmt.Println("- Reason: setup accepted broadly; no discrimination observed.")
		}
	}
	if res.SetupAccepted > 0 && res.ConfirmedByQuery == 0 {
		fmt.Println("- Broad setup acceptance observed.")
		fmt.Println("- No follow-up query confirmation.")
		fmt.Println("- Endpoint may be rack/slot-insensitive or expose limited S7 services.")
		return
	}
	if res.ConfirmedByQuery > 0 {
		fmt.Println("- At least one rack/slot confirmed by follow-up query (high confidence).")
	}
	if res.TCPOnly > 0 {
		fmt.Println("- Some addresses accepted TCP but not COTP/S7.")
	}
	if res.Flaky > 0 {
		fmt.Println("- Some candidates produced mixed results across retries.")
	}
}

// printProbeSummaryOnly prints only summary and interpretation (REQUIREMENTS #25).
func printProbeSummaryOnly(res *client.RackSlotProbeResult) {
	fmt.Printf("Target: %s\n", res.Address)
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
		if probeRackSlotConfig.Strict {
			fmt.Println("Query-confirmed combinations:")
		} else {
			fmt.Println("Setup-accepted combinations:")
		}
		for _, c := range res.Valid {
			by := string(c.ConfirmedBy)
			if by == "none" || by == "" {
				by = "setup"
			}
			fmt.Printf("- rack=%d slot=%d (confirmed by %s, PDU=%d)\n", c.Rack, c.Slot, by, c.PDUSize)
		}
	}
	printProbeInterpretation(res)
}

func printProbeRackSlotText(res *client.RackSlotProbeResult) {
	fmt.Printf("target=%s candidates=%d valid=%d setup_accepted=%d confirmed_by_query=%d tcp_only=%d\n",
		res.Address, len(res.Candidates), len(res.Valid), res.SetupAccepted, res.ConfirmedByQuery, res.TCPOnly)
	for _, c := range res.Candidates {
		_, _ = fmt.Fprintf(os.Stdout, "  rack=%d slot=%d stage=%s status=%s pdu=%d confirmed_by=%s confidence=%s error=%s\n",
			c.Rack, c.Slot, c.Stage, c.Status, c.PDUSize, c.ConfirmedBy, c.Confidence, c.Error)
	}
}
