package s7commctl

import (
	"context"
	"encoding/json"
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

var profileConfig = config.DefaultProfileConfig()

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Summarize S7 device capabilities and connection profile",
	Long:  `Probe target for valid rack/slot, then run identify and list blocks to summarize setup support, rack/slot discrimination, identify, and block support.`,
	RunE:  runProfile,
}

func init() {
	rootCmd.AddCommand(profileCmd)
	config.RegisterFlags(profileCmd, &profileConfig)
	config.LoadFromEnv(&profileConfig)
}

func runProfile(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(profileConfig.IP); err != nil {
		return err
	}
	warnIfPublicTarget(profileConfig.IP, false, "verify authorization and network exposure", profileConfig.NoExposureWarning)

	ctx := cmd.Context()
	timeout := time.Duration(profileConfig.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout*2) // allow probe + identify + list
	defer cancel()

	// Quick probe: limited rack/slot range
	probeReq := client.DefaultRackSlotProbeRequest(profileConfig.IP)
	probeReq.Port = profileConfig.Port
	if probeReq.Port == 0 {
		probeReq.Port = 102
	}
	probeReq.RackMin, probeReq.RackMax = 0, 1
	probeReq.SlotMin, probeReq.SlotMax = 0, 5
	probeReq.Timeout = timeout
	probeReq.Parallelism = 4

	probeRes, err := client.ProbeRackSlots(ctx, probeReq)
	if err != nil {
		return fmt.Errorf("probe failed: %w", err)
	}

	setupSupport := probeRes.SetupAccepted > 0
	confirmedByQuery := probeRes.ConfirmedByQuery > 0
	var firstValid *client.RackSlotCandidate
	if len(probeRes.Valid) > 0 {
		firstValid = &probeRes.Valid[0]
	}

	// Optional: identify and list blocks using first valid
	var identifyInfo *model.DeviceInfo
	var blockCount int
	var identifyOK, blocksOK bool

	if firstValid != nil {
		opts := []client.Option{
			client.WithRackSlot(firstValid.Rack, firstValid.Slot),
			client.WithTimeout(timeout),
		}
		if profileConfig.Port > 0 {
			opts = append(opts, client.WithPort(profileConfig.Port))
		}
		c := client.New(profileConfig.IP, opts...)
		if err := c.Connect(ctx); err == nil {
			defer func() { _ = c.Close() }()
			if info, err := c.Identify(ctx); err == nil {
				identifyInfo = info
				identifyOK = true
			}
			if blocks, err := c.ListAllBlocks(ctx); err == nil {
				blockCount = len(blocks)
				blocksOK = true
			}
		}
	}

	format := strings.ToLower(profileConfig.Format)
	if format == output.FormatJSON {
		printProfileJSON(profileConfig.IP, profileConfig.Port, setupSupport, confirmedByQuery, firstValid, identifyInfo, identifyOK, blockCount, blocksOK)
		return nil
	}

	// Text summary
	fmt.Printf("Target: %s\n", profileConfig.IP)
	fmt.Printf("Port: %d\n", profileConfig.Port)
	fmt.Println()
	fmt.Println("Setup support:        ", yesNo(setupSupport))
	fmt.Println("Rack/slot confirmed:  ", yesNo(confirmedByQuery))
	if firstValid != nil {
		fmt.Printf("Recommended rack/slot: %d/%d\n", firstValid.Rack, firstValid.Slot)
	} else {
		fmt.Println("Recommended rack/slot: (none)")
	}
	fmt.Println()
	fmt.Println("Identify support:     ", yesNo(identifyOK))
	if identifyInfo != nil {
		if identifyInfo.CPUType != "" {
			fmt.Printf("  CPU type:   %s\n", identifyInfo.CPUType)
		}
		if identifyInfo.OrderNumber != "" {
			fmt.Printf("  Order:      %s\n", identifyInfo.OrderNumber)
		}
		if identifyInfo.ModuleName != "" {
			fmt.Printf("  Module:     %s\n", identifyInfo.ModuleName)
		}
	}
	fmt.Println("Block list support:   ", yesNo(blocksOK))
	if blocksOK {
		fmt.Printf("  Blocks:     %d\n", blockCount)
	}
	fmt.Println()
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func printProfileJSON(ip string, port int, setupSupport, confirmedByQuery bool, firstValid *client.RackSlotCandidate, identifyInfo *model.DeviceInfo, identifyOK bool, blockCount int, blocksOK bool) {
	out := map[string]interface{}{
		"target":              ip,
		"port":                port,
		"setup_support":       setupSupport,
		"rack_slot_confirmed": confirmedByQuery,
		"identify_support":    identifyOK,
		"block_list_support":  blocksOK,
	}
	if firstValid != nil {
		out["recommended_rack"] = firstValid.Rack
		out["recommended_slot"] = firstValid.Slot
	}
	if identifyInfo != nil {
		out["identify"] = map[string]interface{}{
			"cpu_type":    profileStrOrNull(identifyInfo.CPUType),
			"order":       profileStrOrNull(identifyInfo.OrderNumber),
			"module_name": profileStrOrNull(identifyInfo.ModuleName),
		}
	}
	if blocksOK {
		out["block_count"] = blockCount
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}

func profileStrOrNull(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
