package s7commctl

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/s7comm/client"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var compareReadConfig = config.DefaultCompareReadConfig()

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare reads or behavior across rack/slot candidates",
}

var compareReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Run the same read across multiple rack/slot pairs",
	Long:  `Connect to each candidate (rack/slot), perform the same read, report results and whether all returned identical data (rack/slot-insensitive).`,
	RunE:  runCompareRead,
}

func init() {
	rootCmd.AddCommand(compareCmd)
	compareCmd.AddCommand(compareReadCmd)
	config.RegisterFlags(compareReadCmd, &compareReadConfig)
	config.LoadFromEnv(&compareReadConfig)
}

func runCompareRead(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(compareReadConfig.IP); err != nil {
		return err
	}
	warnIfPublicTarget(compareReadConfig.IP, false, "verify authorization and network exposure", compareReadConfig.NoExposureWarning)
	candidates, err := parseCandidates(compareReadConfig.Candidates)
	if err != nil {
		return fmt.Errorf("candidates: %w", err)
	}
	if len(candidates) == 0 {
		return fmt.Errorf("at least one candidate required (e.g. --candidates 0/0,0/1)")
	}
	area, err := validate.ValidateArea(compareReadConfig.Area)
	if err != nil {
		return err
	}
	if compareReadConfig.Size <= 0 {
		return fmt.Errorf("size must be > 0")
	}

	req := client.CompareReadRequest{
		Address:    compareReadConfig.IP,
		Port:       compareReadConfig.Port,
		Candidates: candidates,
		Area:       area,
		DBNumber:   compareReadConfig.DBNumber,
		Offset:     compareReadConfig.Offset,
		Size:       compareReadConfig.Size,
		Timeout:    time.Duration(compareReadConfig.Timeout) * time.Millisecond,
	}
	if req.Port == 0 {
		req.Port = 102
	}

	repeat := compareReadConfig.Repeat
	if repeat < 1 {
		repeat = 1
	}
	interval := time.Duration(compareReadConfig.IntervalMS) * time.Millisecond

	var results []*client.CompareReadResult
	for i := 0; i < repeat; i++ {
		result, err := client.CompareRead(cmd.Context(), req)
		if err != nil {
			return fmt.Errorf("compare read failed: %w", err)
		}
		results = append(results, result)
		if i < repeat-1 && interval > 0 {
			time.Sleep(interval)
		}
	}
	result := results[len(results)-1]

	sameAsBaseline := computeSameAsBaseline(result)
	dataStableAcrossRuns := repeat == 1 || compareDataStable(results)

	if strings.ToLower(compareReadConfig.Format) == output.FormatJSON {
		printCompareReadJSON(result, sameAsBaseline, repeat, dataStableAcrossRuns)
		return nil
	}

	fmt.Printf("Target: %s\n", result.Request.Address)
	fmt.Printf("Read: area=%s offset=%d size=%d\n", compareReadConfig.Area, result.Request.Offset, result.Request.Size)
	fmt.Printf("Rack/slot-insensitive: %v\n", result.RackSlotInsensitive)
	if repeat > 1 {
		fmt.Printf("Repeat runs: %d; data stable across runs: %v\n", repeat, dataStableAcrossRuns)
	}
	fmt.Println()
	fmt.Println("Rack  Slot  Status         Returned  SameAsBaseline  Error")
	fmt.Println("----  ----  -------------  --------  --------------  -----")
	for i, bc := range result.ByCandidate {
		errStr := bc.Result.Error
		if len(errStr) > 30 {
			errStr = errStr[:27] + "..."
		}
		sab := "-"
		if i < len(sameAsBaseline) {
			sab = sameAsBaseline[i]
		}
		_, _ = fmt.Fprintf(os.Stdout, "%-4d  %-4d  %-13s  %-8d  %-14s  %s\n",
			bc.Rack, bc.Slot, bc.Result.Status, bc.Result.ReturnedLength, sab, errStr)
	}
	return nil
}

// computeSameAsBaseline returns one label per candidate: "baseline", "yes", "no", or "-".
func computeSameAsBaseline(res *client.CompareReadResult) []string {
	var baseline []byte
	baselineIdx := -1
	for i, bc := range res.ByCandidate {
		if string(bc.Result.Status) == "success" && len(bc.Result.Data) > 0 {
			baseline = bc.Result.Data
			baselineIdx = i
			break
		}
	}
	out := make([]string, len(res.ByCandidate))
	for i, bc := range res.ByCandidate {
		if string(bc.Result.Status) != "success" {
			out[i] = "-"
			continue
		}
		if i == baselineIdx {
			out[i] = "baseline"
			continue
		}
		if baseline == nil {
			out[i] = "-"
			continue
		}
		if bytes.Equal(bc.Result.Data, baseline) {
			out[i] = "yes"
		} else {
			out[i] = "no"
		}
	}
	return out
}

func compareDataStable(results []*client.CompareReadResult) bool {
	if len(results) < 2 {
		return true
	}
	var first []byte
	for _, bc := range results[0].ByCandidate {
		if string(bc.Result.Status) == "success" && len(bc.Result.Data) > 0 {
			first = bc.Result.Data
			break
		}
	}
	if first == nil {
		return false
	}
	for _, res := range results[1:] {
		var found []byte
		for _, bc := range res.ByCandidate {
			if string(bc.Result.Status) == "success" && len(bc.Result.Data) > 0 {
				found = bc.Result.Data
				break
			}
		}
		if !bytes.Equal(first, found) {
			return false
		}
	}
	return true
}

func printCompareReadJSON(res *client.CompareReadResult, sameAsBaseline []string, repeat int, dataStable bool) {
	sab := sameAsBaseline
	if sab == nil {
		sab = []string{}
	}
	byCandidate := make([]map[string]interface{}, 0, len(res.ByCandidate))
	for i, bc := range res.ByCandidate {
		ent := map[string]interface{}{
			"rack":             bc.Rack,
			"slot":             bc.Slot,
			"status":           string(bc.Result.Status),
			"returned_length":  bc.Result.ReturnedLength,
			"same_as_baseline": "-",
		}
		if i < len(sab) {
			ent["same_as_baseline"] = sab[i]
		}
		if len(bc.Result.Data) > 0 {
			ent["data_hex"] = hex.EncodeToString(bc.Result.Data)
		}
		if bc.Result.Error != "" {
			ent["error"] = bc.Result.Error
		}
		byCandidate = append(byCandidate, ent)
	}
	out := map[string]interface{}{
		"request":               res.Request,
		"rack_slot_insensitive": res.RackSlotInsensitive,
		"by_candidate":          byCandidate,
	}
	if repeat > 1 {
		out["repeat_runs"] = repeat
		out["data_stable_across_runs"] = dataStable
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}

// parseCandidates parses "0/0,0/1,1/2" into []client.RackSlot.
func parseCandidates(s string) ([]client.RackSlot, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var out []client.RackSlot
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "/")
		if idx < 0 {
			return nil, fmt.Errorf("invalid candidate %q (expected rack/slot)", part)
		}
		rack, err1 := strconv.Atoi(strings.TrimSpace(part[:idx]))
		slot, err2 := strconv.Atoi(strings.TrimSpace(part[idx+1:]))
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("invalid candidate %q (expected integers)", part)
		}
		if rack < 0 || rack > 7 || slot < 0 || slot > 31 {
			return nil, fmt.Errorf("candidate %q: rack 0-7, slot 0-31", part)
		}
		out = append(out, client.RackSlot{Rack: rack, Slot: slot})
	}
	return out, nil
}
