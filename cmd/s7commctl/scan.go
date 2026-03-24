package s7commctl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var scanReadRangesConfig = config.DefaultScanReadRangesConfig()

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan targets for readable ranges or behavior",
}

var scanReadRangesCmd = &cobra.Command{
	Use:   "read-ranges",
	Short: "Scan an area to find readable address ranges",
	Long:  `Probe an area from start to end by step; consolidate adjacent readable probes into spans. Read-only.`,
	RunE:  runScanReadRanges,
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.AddCommand(scanReadRangesCmd)
	config.RegisterFlags(scanReadRangesCmd, &scanReadRangesConfig)
	config.LoadFromEnv(&scanReadRangesConfig)
}

func runScanReadRanges(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(scanReadRangesConfig.IP); err != nil {
		return ExitUsage(err)
	}
	warnIfPublicTarget(scanReadRangesConfig.IP, scanReadRangesConfig.Quiet, "verify authorization and network exposure", scanReadRangesConfig.NoExposureWarning)
	if !scanReadRangesConfig.UseExplicitTSAP() && !scanReadRangesConfig.AutoRack {
		if err := validate.ValidateRackSlot(scanReadRangesConfig.Rack, scanReadRangesConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	area, err := validate.ValidateArea(scanReadRangesConfig.Area)
	if err != nil {
		return ExitUsage(err)
	}
	if scanReadRangesConfig.End <= scanReadRangesConfig.Start {
		return ExitUsage(fmt.Errorf("end must be > start"))
	}
	if scanReadRangesConfig.ProbeSize <= 0 {
		return ExitUsage(fmt.Errorf("probe-size must be > 0"))
	}

	opts, err := scanReadRangesConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(scanReadRangesConfig.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
		return ExitRuntime(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	req := client.RangeProbeRequest{
		Area:        area,
		DBNumber:    scanReadRangesConfig.DBNumber,
		Start:       scanReadRangesConfig.Start,
		End:         scanReadRangesConfig.End,
		Step:        scanReadRangesConfig.Step,
		ProbeSize:   scanReadRangesConfig.ProbeSize,
		Retries:     scanReadRangesConfig.Retries,
		RetryDelay:  time.Duration(scanReadRangesConfig.RetryDelayMS) * time.Millisecond,
		Repeat:      scanReadRangesConfig.Repeat,
		Parallelism: 0,
	}
	if req.Step == 0 {
		req.Step = req.ProbeSize
	}

	result, err := c.ProbeReadableRanges(cmd.Context(), req)
	if err != nil {
		return ExitRuntime(fmt.Errorf("scan failed: %w", err))
	}

	format := strings.ToLower(scanReadRangesConfig.Format)
	switch format {
	case output.FormatJSON:
		printScanReadRangesJSON(result)
	case "table":
		if scanReadRangesConfig.SummaryOnly {
			printScanReadRangesSpans(result)
		} else {
			printScanReadRangesTable(result)
		}
	default:
		printScanReadRangesSpans(result)
	}
	return nil
}

// spanHuman returns human-friendly "bytes start-end" (inclusive) for [start..end).
func spanHuman(start, end int) string {
	if end <= start {
		return fmt.Sprintf("bytes %d-%d", start, start)
	}
	return fmt.Sprintf("bytes %d-%d", start, end-1)
}

func printScanReadRangesSpans(res *client.RangeProbeResult) {
	fmt.Printf("Area: %s", res.Area.String())
	if res.Area == model.AreaDB {
		fmt.Printf(" DB%d", res.DBNumber)
	}
	fmt.Println()
	if len(res.Summary.ReadableSpans) > 0 {
		fmt.Println("Readable spans:")
		for _, s := range res.Summary.ReadableSpans {
			fmt.Printf("  [%d..%d) %s  status=%s\n", s.Start, s.End, spanHuman(s.Start, s.End), s.Status)
		}
	}
	if len(res.Summary.EmptySpans) > 0 {
		fmt.Println("Empty spans:")
		for _, s := range res.Summary.EmptySpans {
			fmt.Printf("  [%d..%d) %s\n", s.Start, s.End, spanHuman(s.Start, s.End))
		}
	}
	// REQUIREMENTS #12: split short-read into "Partial spans", rest into "Failed spans"
	var partialSpans, failedSpans []client.ReadableSpan
	for _, s := range res.Summary.FailedSpans {
		if string(s.Status) == "short-read" {
			partialSpans = append(partialSpans, s)
		} else {
			failedSpans = append(failedSpans, s)
		}
	}
	if len(partialSpans) > 0 {
		fmt.Println("Partial spans (short-read):")
		for _, s := range partialSpans {
			fmt.Printf("  [%d..%d) %s  status=%s\n", s.Start, s.End, spanHuman(s.Start, s.End), s.Status)
		}
	}
	if len(failedSpans) > 0 {
		fmt.Println("Failed spans:")
		for _, s := range failedSpans {
			fmt.Printf("  [%d..%d) %s  status=%s\n", s.Start, s.End, spanHuman(s.Start, s.End), s.Status)
		}
	}
	if len(res.Summary.InconclusiveSpans) > 0 {
		fmt.Println("Inconclusive spans:")
		for _, s := range res.Summary.InconclusiveSpans {
			fmt.Printf("  [%d..%d) %s\n", s.Start, s.End, spanHuman(s.Start, s.End))
		}
	}
}

func printScanReadRangesTable(res *client.RangeProbeResult) {
	fmt.Println("Offset  Status         Returned  Error")
	fmt.Println("------  -------------  --------  -----")
	for _, p := range res.Probes {
		errStr := ""
		if p.Result.Message != "" {
			errStr = p.Result.Message
			if len(errStr) > 30 {
				errStr = errStr[:27] + "..."
			}
		}
		_, _ = fmt.Fprintf(os.Stdout, "%-6d  %-13s  %-8d  %s\n",
			p.Offset, p.Result.Status, p.Result.ReturnedLength, errStr)
	}
	fmt.Println()
	printScanReadRangesSpans(res)
}

// spanToMap returns a map for JSON span output.
func spanToMap(s client.ReadableSpan) map[string]interface{} {
	m := map[string]interface{}{"start": s.Start, "end": s.End}
	if s.Status != "" {
		m["status"] = string(s.Status)
	}
	return m
}

// printScanReadRangesJSON outputs a stable CLI schema with snake_case and area as string (IMPROVEMENT_PLAN 2.5).
func printScanReadRangesJSON(res *client.RangeProbeResult) {
	port := scanReadRangesConfig.Port
	if port == 0 {
		port = 102
	}
	target := map[string]interface{}{
		"ip":   scanReadRangesConfig.IP,
		"port": port,
		"rack": scanReadRangesConfig.Rack,
		"slot": scanReadRangesConfig.Slot,
	}
	req := map[string]interface{}{
		"area":       res.Area.String(),
		"start":      scanReadRangesConfig.Start,
		"end":        scanReadRangesConfig.End,
		"step":       scanReadRangesConfig.Step,
		"probe_size": scanReadRangesConfig.ProbeSize,
	}
	if res.Area == model.AreaDB {
		req["db_number"] = res.DBNumber
	}
	var partialSpans, failedSpans []client.ReadableSpan
	for _, s := range res.Summary.FailedSpans {
		if string(s.Status) == "short-read" {
			partialSpans = append(partialSpans, s)
		} else {
			failedSpans = append(failedSpans, s)
		}
	}
	readable := make([]map[string]interface{}, 0, len(res.Summary.ReadableSpans))
	for _, s := range res.Summary.ReadableSpans {
		readable = append(readable, spanToMap(s))
	}
	empty := make([]map[string]interface{}, 0, len(res.Summary.EmptySpans))
	for _, s := range res.Summary.EmptySpans {
		empty = append(empty, spanToMap(s))
	}
	partial := make([]map[string]interface{}, 0, len(partialSpans))
	for _, s := range partialSpans {
		partial = append(partial, spanToMap(s))
	}
	failed := make([]map[string]interface{}, 0, len(failedSpans))
	for _, s := range failedSpans {
		failed = append(failed, spanToMap(s))
	}
	inconclusive := make([]map[string]interface{}, 0, len(res.Summary.InconclusiveSpans))
	for _, s := range res.Summary.InconclusiveSpans {
		inconclusive = append(inconclusive, spanToMap(s))
	}
	summary := map[string]interface{}{
		"readable_count":     len(res.Summary.ReadableSpans),
		"empty_count":        len(res.Summary.EmptySpans),
		"partial_count":      len(partialSpans),
		"failed_count":       len(failedSpans),
		"inconclusive_count": len(res.Summary.InconclusiveSpans),
	}
	spans := map[string]interface{}{
		"readable":     readable,
		"empty":        empty,
		"partial":      partial,
		"failed":       failed,
		"inconclusive": inconclusive,
	}
	out := map[string]interface{}{
		"target":  target,
		"request": req,
		"summary": summary,
		"spans":   spans,
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}
