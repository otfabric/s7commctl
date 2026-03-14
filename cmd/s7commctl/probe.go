package s7commctl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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
		return ExitUsage(err)
	}
	warnIfPublicTarget(probeRackSlotConfig.IP, false, "verify authorization and network exposure", probeRackSlotConfig.NoExposureWarning)

	req, err := probeRackSlotConfig.ToProbeRequest()
	if err != nil {
		return ExitUsage(err)
	}
	repeat := probeRackSlotConfig.Repeat
	if repeat < 1 {
		repeat = 1
	}
	interval := time.Duration(probeRackSlotConfig.RepeatIntervalMS) * time.Millisecond

	var results []*client.RackSlotProbeResult
	for i := 0; i < repeat; i++ {
		result, err := client.ProbeRackSlots(cmd.Context(), req)
		if err != nil {
			return ExitRuntime(fmt.Errorf("probe failed: %w", err))
		}
		results = append(results, result)
		if i < repeat-1 && interval > 0 {
			time.Sleep(interval)
		}
	}
	result := results[len(results)-1]
	variance := repeat > 1 && probeVariance(results)
	opts := probeDisplayOpts{
		Strict:      probeRackSlotConfig.Strict,
		ShowAll:     probeRackSlotConfig.ShowAll,
		SummaryOnly: probeRackSlotConfig.SummaryOnly,
		Retries:     probeRackSlotConfig.Retries,
		Confirm:     confirmKindToDisplay(req.Confirm),
	}

	format := strings.ToLower(probeRackSlotConfig.Format)
	switch format {
	case output.FormatJSON:
		printProbeResultJSON(result, variance, results, &probeRackSlotConfig, opts)
	case output.FormatTable:
		if opts.SummaryOnly {
			printProbeSummaryOnly(result, variance, results, opts)
		} else {
			printProbeRackSlotTable(result, variance, results, opts)
		}
	default:
		if opts.SummaryOnly {
			printProbeSummaryOnly(result, variance, results, opts)
		} else {
			printProbeRackSlotText(result, variance, results, opts)
		}
	}

	if len(result.Valid) == 0 {
		return ExitOperation(fmt.Errorf("no valid rack/slot found"))
	}
	return nil
}

// confirmKindToDisplay returns a display string for the resolved confirmation kind (for request envelope and text output).
func confirmKindToDisplay(k client.ConfirmationKind) string {
	switch k {
	case client.ConfirmAny:
		return "any"
	case client.ConfirmNone:
		return "none"
	case client.ConfirmSZL:
		return "szl"
	case client.ConfirmCPUState:
		return "cpu-state"
	default:
		return "none"
	}
}

// probeDisplayOpts carries display options for probe output (avoids global config in helpers).
type probeDisplayOpts struct {
	Strict      bool
	ShowAll     bool
	SummaryOnly bool
	Retries     int
	Confirm     string
}

// probeVariance returns true if setup_accepted, confirmed_by_query, tcp_only or flaky differ across runs.
func probeVariance(results []*client.RackSlotProbeResult) bool {
	if len(results) < 2 {
		return false
	}
	first := results[0]
	for _, r := range results[1:] {
		if r.SetupAccepted != first.SetupAccepted || r.ConfirmedByQuery != first.ConfirmedByQuery ||
			r.TCPOnly != first.TCPOnly || r.Flaky != first.Flaky {
			return true
		}
	}
	return false
}

// probeStatusDisplay maps client status to display string in strict mode (IMPROVEMENT_PLAN 3.1).
func probeStatusDisplay(status string, strict bool) string {
	if !strict {
		return status
	}
	switch status {
	case "valid-connect", "setup-only":
		return "setup-ok"
	case "valid-query":
		return "query-confirmed"
	default:
		return status
	}
}

func printProbeRackSlotTable(res *client.RackSlotProbeResult, variance bool, results []*client.RackSlotProbeResult, opts probeDisplayOpts) {
	fmt.Printf("Target: %s\n", res.Address)
	if len(results) > 1 {
		printProbeRepeatSummary(results, variance)
	}
	mode := "default (setup accepted = valid)"
	if opts.Strict {
		confirm := opts.Confirm
		if confirm == "" {
			confirm = "any"
		}
		mode = fmt.Sprintf("strict (confirm=%s)", confirm)
		if opts.Retries > 0 {
			mode += fmt.Sprintf(", retries=%d", opts.Retries)
		}
	}
	fmt.Printf("Mode: %s\n\n", mode)
	showAll := opts.ShowAll
	candidates := res.Candidates
	strict := opts.Strict
	const maxExamplesPerStatus = 5
	if !showAll && len(candidates) > 15 {
		// Collapsed: summary by status + first N examples per status (use display status for grouping when strict)
		statusCounts := make(map[string]int)
		statusExamples := make(map[string][]client.RackSlotCandidate)
		for _, c := range candidates {
			s := probeStatusDisplay(string(c.Status), strict)
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
				disp := probeStatusDisplay(string(c.Status), strict)
				_, _ = fmt.Fprintf(os.Stdout, "%-4d  %-4d  %-6s  %-13s  %-3d  %-11s  %-10s  %s\n",
					c.Rack, c.Slot, c.Stage, disp, c.PDUSize, confirmedBy, c.Confidence, errStr)
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
			disp := probeStatusDisplay(string(c.Status), strict)
			_, _ = fmt.Fprintf(os.Stdout, "%-4d  %-4d  %-6s  %-13s  %-3d  %-11s  %-10s  %s\n",
				c.Rack, c.Slot, c.Stage, disp, c.PDUSize, confirmedBy, c.Confidence, errStr)
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
	collapsed := !showAll && len(candidates) > 15
	if len(res.Valid) > 0 {
		fmt.Println()
		if collapsed {
			fmt.Printf("Setup-accepted combinations: %d total (use --show-all or --format json for full details).\n", len(res.Valid))
		} else {
			if opts.Strict {
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
	}
	printProbeInterpretation(res, true, variance)
}

// printProbeRepeatSummary prints "Repeat (N runs):" with min-max and optional variance line (IMPROVEMENT_PLAN 5.1).
func printProbeRepeatSummary(results []*client.RackSlotProbeResult, variance bool) {
	if len(results) < 2 {
		return
	}
	minSA, maxSA := results[0].SetupAccepted, results[0].SetupAccepted
	minCQ, maxCQ := results[0].ConfirmedByQuery, results[0].ConfirmedByQuery
	minTCP, maxTCP := results[0].TCPOnly, results[0].TCPOnly
	minF, maxF := results[0].Flaky, results[0].Flaky
	for _, r := range results[1:] {
		if r.SetupAccepted < minSA {
			minSA = r.SetupAccepted
		}
		if r.SetupAccepted > maxSA {
			maxSA = r.SetupAccepted
		}
		if r.ConfirmedByQuery < minCQ {
			minCQ = r.ConfirmedByQuery
		}
		if r.ConfirmedByQuery > maxCQ {
			maxCQ = r.ConfirmedByQuery
		}
		if r.TCPOnly < minTCP {
			minTCP = r.TCPOnly
		}
		if r.TCPOnly > maxTCP {
			maxTCP = r.TCPOnly
		}
		if r.Flaky < minF {
			minF = r.Flaky
		}
		if r.Flaky > maxF {
			maxF = r.Flaky
		}
	}
	fmt.Printf("Repeat (%d runs):\n", len(results))
	if minSA == maxSA {
		fmt.Printf("- Setup accepted: %d\n", maxSA)
	} else {
		fmt.Printf("- Setup accepted: %d–%d\n", minSA, maxSA)
	}
	if minCQ == maxCQ {
		fmt.Printf("- Confirmed by query: %d\n", maxCQ)
	} else {
		fmt.Printf("- Confirmed by query: %d–%d\n", minCQ, maxCQ)
	}
	if minTCP == maxTCP {
		fmt.Printf("- TCP-only: %d\n", maxTCP)
	} else {
		fmt.Printf("- TCP-only: %d–%d\n", minTCP, maxTCP)
	}
	if maxF > 0 {
		if minF == maxF {
			fmt.Printf("- Flaky: %d\n", maxF)
		} else {
			fmt.Printf("- Flaky: %d–%d\n", minF, maxF)
		}
	}
	if variance {
		fmt.Println("- Results varied across runs; endpoint may be rate-sensitive or unstable.")
	}
	fmt.Println()
}

// includeRecommendedTuple when true prints the recommended/suggested tuple in the interpretation block.
func printProbeInterpretation(res *client.RackSlotProbeResult, includeRecommendedTuple bool, variance bool) {
	fmt.Println()
	fmt.Println("Interpretation:")
	if variance {
		fmt.Println("- Results varied across runs; endpoint may be rate-sensitive or unstable.")
	}
	if res.SetupAccepted == 0 && res.TCPOnly == 0 {
		fmt.Println("- No successful TCP or setup; target unreachable or not speaking S7.")
		return
	}
	if includeRecommendedTuple && len(res.Valid) > 0 {
		first := res.Valid[0]
		fmt.Printf("- Suggested default tuple: rack=%d slot=%d\n", first.Rack, first.Slot)
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

// printProbeSummaryOnly prints only summary and interpretation; no full candidate list (IMPROVEMENT_PLAN 1.1).
func printProbeSummaryOnly(res *client.RackSlotProbeResult, variance bool, results []*client.RackSlotProbeResult, _ probeDisplayOpts) {
	fmt.Printf("Target: %s\n", res.Address)
	if len(results) > 1 {
		printProbeRepeatSummary(results, variance)
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
		fmt.Println("Recommended connection tuple:")
		first := res.Valid[0]
		fmt.Printf("- rack=%d slot=%d\n", first.Rack, first.Slot)
		fmt.Printf("Setup-accepted combinations: %d total (use --show-all or --format json for full list).\n", len(res.Valid))
	}
	printProbeInterpretation(res, false, variance) // interpretation text only; tuple already printed above
}

func printProbeRackSlotText(res *client.RackSlotProbeResult, variance bool, results []*client.RackSlotProbeResult, opts probeDisplayOpts) {
	if len(results) > 1 {
		printProbeRepeatSummary(results, variance)
	}
	fmt.Printf("target=%s candidates=%d valid=%d setup_accepted=%d confirmed_by_query=%d tcp_only=%d\n",
		res.Address, len(res.Candidates), len(res.Valid), res.SetupAccepted, res.ConfirmedByQuery, res.TCPOnly)
	strict := opts.Strict
	for _, c := range res.Candidates {
		disp := probeStatusDisplay(string(c.Status), strict)
		_, _ = fmt.Fprintf(os.Stdout, "  rack=%d slot=%d stage=%s status=%s pdu=%d confirmed_by=%s confidence=%s error=%s\n",
			c.Rack, c.Slot, c.Stage, disp, c.PDUSize, c.ConfirmedBy, c.Confidence, c.Error)
	}
}

// printProbeResultJSON outputs probe result in target/request/result envelope (snake_case).
func printProbeResultJSON(res *client.RackSlotProbeResult, variance bool, results []*client.RackSlotProbeResult, cfg *config.ProbeRackSlotConfig, opts probeDisplayOpts) {
	port := cfg.Port
	if port == 0 {
		port = 102
	}
	target := map[string]interface{}{"ip": cfg.IP, "port": port}
	request := map[string]interface{}{
		"rack_min": cfg.RackMin,
		"rack_max": cfg.RackMax,
		"slot_min": cfg.SlotMin,
		"slot_max": cfg.SlotMax,
		"strict":   cfg.Strict,
		"confirm":  opts.Confirm,
	}

	strict := opts.Strict
	candidates := make([]map[string]interface{}, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		candidates = append(candidates, map[string]interface{}{
			"rack":         c.Rack,
			"slot":         c.Slot,
			"stage":        string(c.Stage),
			"status":       probeStatusDisplay(string(c.Status), strict),
			"pdu_size":     c.PDUSize,
			"confirmed_by": string(c.ConfirmedBy),
			"confidence":   string(c.Confidence),
			"error":        c.Error,
		})
	}
	valid := make([]map[string]interface{}, 0, len(res.Valid))
	for _, c := range res.Valid {
		valid = append(valid, map[string]interface{}{
			"rack":         c.Rack,
			"slot":         c.Slot,
			"stage":        string(c.Stage),
			"status":       probeStatusDisplay(string(c.Status), strict),
			"pdu_size":     c.PDUSize,
			"confirmed_by": string(c.ConfirmedBy),
			"confidence":   string(c.Confidence),
			"error":        c.Error,
		})
	}
	resultObj := map[string]interface{}{
		"candidates":         candidates,
		"valid":              valid,
		"setup_accepted":     res.SetupAccepted,
		"confirmed_by_query": res.ConfirmedByQuery,
		"tcp_only":           res.TCPOnly,
		"flaky":              res.Flaky,
	}
	if len(results) > 1 {
		resultObj["repeat_runs"] = len(results)
		resultObj["results_varied"] = variance
	}

	out := map[string]interface{}{
		"target":  target,
		"request": request,
		"result":  resultObj,
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}
