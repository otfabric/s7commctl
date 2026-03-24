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

	"github.com/otfabric/go-s7comm/client"

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
		return ExitUsage(err)
	}
	warnIfPublicTarget(compareReadConfig.IP, false, "verify authorization and network exposure", compareReadConfig.NoExposureWarning)
	candidates, err := parseCandidates(compareReadConfig.Candidates)
	if err != nil {
		return ExitUsage(fmt.Errorf("candidates: %w", err))
	}
	if len(candidates) == 0 {
		return ExitUsage(fmt.Errorf("at least one candidate required (e.g. --candidates 0/0,0/1)"))
	}
	area, err := validate.ValidateArea(compareReadConfig.Area)
	if err != nil {
		return ExitUsage(err)
	}
	if compareReadConfig.Size <= 0 {
		return ExitUsage(fmt.Errorf("size must be > 0"))
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
			return ExitRuntime(fmt.Errorf("compare read failed: %w", err))
		}
		results = append(results, result)
		if i < repeat-1 && interval > 0 {
			time.Sleep(interval)
		}
	}
	result := results[len(results)-1]

	sameAsBaseline := computeSameAsBaseline(result)
	stableByCandidate, dataStableAcrossRuns := compareDataStable(results)
	if repeat < 2 {
		stableByCandidate = nil
		dataStableAcrossRuns = true
	}

	if strings.ToLower(compareReadConfig.Format) == output.FormatJSON {
		printCompareReadJSON(result, sameAsBaseline, repeat, dataStableAcrossRuns, stableByCandidate)
		return nil
	}

	fmt.Printf("Target: %s\n", result.Request.Address)
	fmt.Printf("Read: area=%s offset=%d size=%d\n", compareReadConfig.Area, result.Request.Offset, result.Request.Size)
	fmt.Printf("Rack/slot-insensitive: %v\n", result.RackSlotInsensitive)
	if repeat > 1 {
		fmt.Printf("Repeat runs: %d; data stable across runs: %v\n", repeat, dataStableAcrossRuns)
		if len(stableByCandidate) > 0 {
			fmt.Printf("Stable by candidate: %v\n", stableByCandidate)
		}
	}
	fmt.Println()
	fmt.Println("Rack  Slot  Status         Returned  SameAsBaseline  Error")
	fmt.Println("----  ----  -------------  --------  --------------  -----")
	for i, bc := range result.ByCandidate {
		errStr := bc.Result.Message
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

// candidateResult holds the comparable outcome for one candidate in a run.
type candidateResult struct {
	status string
	length int
	data   []byte
}

// compareDataStable returns per-candidate stability across runs, keyed by rack/slot so ordering is irrelevant.
func compareDataStable(results []*client.CompareReadResult) (stableByCandidate []bool, allStable bool) {
	if len(results) < 2 {
		return nil, true
	}
	// Build map key -> result for first run
	keyToResult := func(res *client.CompareReadResult) map[string]candidateResult {
		m := make(map[string]candidateResult, len(res.ByCandidate))
		for _, bc := range res.ByCandidate {
			key := fmt.Sprintf("%d/%d", bc.Rack, bc.Slot)
			m[key] = candidateResult{
				status: string(bc.Result.Status),
				length: bc.Result.ReturnedLength,
				data:   bytes.Clone(bc.Result.Data),
			}
		}
		return m
	}
	firstMap := keyToResult(results[0])
	stableByKey := make(map[string]bool, len(firstMap))
	for key := range firstMap {
		stable := true
		ref := firstMap[key]
		for _, res := range results[1:] {
			runMap := keyToResult(res)
			cur, ok := runMap[key]
			if !ok || cur.status != ref.status || cur.length != ref.length || !bytes.Equal(cur.data, ref.data) {
				stable = false
				break
			}
		}
		stableByKey[key] = stable
	}
	// Map back to candidate order for the last result
	res := results[len(results)-1]
	stableByCandidate = make([]bool, len(res.ByCandidate))
	allStable = true
	for i, bc := range res.ByCandidate {
		key := fmt.Sprintf("%d/%d", bc.Rack, bc.Slot)
		stableByCandidate[i] = stableByKey[key]
		if !stableByKey[key] {
			allStable = false
		}
	}
	return stableByCandidate, allStable
}

func printCompareReadJSON(res *client.CompareReadResult, sameAsBaseline []string, repeat int, dataStable bool, stableByCandidate []bool) {
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
		if i < len(stableByCandidate) {
			ent["stable_across_runs"] = stableByCandidate[i]
		}
		if len(bc.Result.Data) > 0 {
			ent["data_hex"] = hex.EncodeToString(bc.Result.Data)
		}
		if bc.Result.Message != "" {
			ent["message"] = bc.Result.Message
		}
		byCandidate = append(byCandidate, ent)
	}
	port := res.Request.Port
	if port == 0 {
		port = 102
	}
	target := map[string]interface{}{"ip": res.Request.Address, "port": port}
	reqMap := map[string]interface{}{
		"area":      res.Request.Area.String(),
		"db_number": res.Request.DBNumber,
		"offset":    res.Request.Offset,
		"size":      res.Request.Size,
	}
	candidates := make([]map[string]interface{}, 0, len(res.Request.Candidates))
	for _, r := range res.Request.Candidates {
		candidates = append(candidates, map[string]interface{}{"rack": r.Rack, "slot": r.Slot})
	}
	reqMap["candidates"] = candidates
	resultObj := map[string]interface{}{
		"rack_slot_insensitive": res.RackSlotInsensitive,
		"by_candidate":          byCandidate,
	}
	if repeat > 1 {
		resultObj["repeat_runs"] = repeat
		resultObj["data_stable_across_runs"] = dataStable
		resultObj["stable_by_candidate"] = stableByCandidate
	}
	out := map[string]interface{}{
		"target":  target,
		"request": reqMap,
		"result":  resultObj,
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
