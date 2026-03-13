package s7commctl

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/otfabric/s7comm/client"
	"github.com/otfabric/s7comm/model"

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
		return err
	}
	warnIfPublicTarget(scanReadRangesConfig.IP, scanReadRangesConfig.Quiet, "verify authorization and network exposure", scanReadRangesConfig.NoExposureWarning)
	if err := validate.ValidateRackSlot(scanReadRangesConfig.Rack, scanReadRangesConfig.Slot); err != nil {
		return err
	}
	area, err := validate.ValidateArea(scanReadRangesConfig.Area)
	if err != nil {
		return err
	}
	if scanReadRangesConfig.End <= scanReadRangesConfig.Start {
		return fmt.Errorf("end must be > start")
	}
	if scanReadRangesConfig.ProbeSize <= 0 {
		return fmt.Errorf("probe-size must be > 0")
	}

	opts := scanReadRangesConfig.ToClientOptions()
	c := client.New(scanReadRangesConfig.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
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
		return fmt.Errorf("scan failed: %w", err)
	}

	format := strings.ToLower(scanReadRangesConfig.Format)
	switch format {
	case output.FormatJSON:
		output.PrintJSON(result)
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
		if p.Result.Error != "" {
			errStr = p.Result.Error
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
