// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/go-s7comm/client"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var blockInfoConfig = config.DefaultBlocksConfig()
var blockUploadConfig = config.DefaultBlocksConfig()

var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "Block operations",
}

var blockInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get block metadata",
	RunE:  runBlockInfo,
}

var blockUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload block contents",
	RunE:  runBlockUpload,
}

func init() {
	config.RegisterFlags(blockInfoCmd, &blockInfoConfig)
	config.LoadFromEnv(&blockInfoConfig)
	config.RegisterFlags(blockUploadCmd, &blockUploadConfig)
	config.LoadFromEnv(&blockUploadConfig)

	blockCmd.AddCommand(blockInfoCmd)
	blockCmd.AddCommand(blockUploadCmd)
	rootCmd.AddCommand(blockCmd)
}

func runBlockInfo(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(blockInfoConfig.IP); err != nil {
		return ExitUsage(err)
	}
	if !blockInfoConfig.UseExplicitTSAP() && !blockInfoConfig.AutoRack {
		if err := validate.ValidateRackSlot(blockInfoConfig.Rack, blockInfoConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(blockInfoConfig.IP, blockInfoConfig.Quiet, "verify authorization and network exposure", blockInfoConfig.NoExposureWarning)
	if blockInfoConfig.Number <= 0 {
		return ExitUsage(fmt.Errorf("--number must be > 0"))
	}

	bt, err := validate.ValidateBlockType(blockInfoConfig.BlockType)
	if err != nil {
		return ExitUsage(err)
	}

	opts, err := blockInfoConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(blockInfoConfig.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
		return ExitRuntime(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	info, err := c.GetBlockInfo(cmd.Context(), bt, blockInfoConfig.Number)
	if err != nil {
		return ExitRuntime(fmt.Errorf("block info failed: %w", err))
	}

	if blockInfoConfig.Quiet {
		_, _ = fmt.Fprintf(os.Stdout, "%s%d\n", bt.String(), blockInfoConfig.Number)
		return nil
	}

	switch strings.ToLower(blockInfoConfig.Format) {
	case "json":
		port := blockInfoConfig.Port
		if port == 0 {
			port = 102
		}
		reqMeta := blockInfoConfig.ConnectionRequestMeta()
		reqMeta["type"] = blockInfoConfig.BlockType
		reqMeta["number"] = blockInfoConfig.Number
		out := map[string]interface{}{
			"target":  map[string]interface{}{"ip": blockInfoConfig.IP, "port": port},
			"request": reqMeta,
			"result": map[string]interface{}{
				"type":        info.Type.String(),
				"number":      info.Number,
				"load_memory": info.LoadMemory,
				"local_data":  info.LocalData,
				"mc7_size":    info.MC7Size,
			},
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(b))
	case "table":
		_, _ = fmt.Fprintln(os.Stdout, "┌────────────┬─────────┐")
		_, _ = fmt.Fprintln(os.Stdout, "│ Field      │ Value   │")
		_, _ = fmt.Fprintln(os.Stdout, "├────────────┼─────────┤")
		_, _ = fmt.Fprintf(os.Stdout, "│ Type       │ %-7s │\n", info.Type.String())
		_, _ = fmt.Fprintf(os.Stdout, "│ Number     │ %-7d │\n", info.Number)
		_, _ = fmt.Fprintf(os.Stdout, "│ LoadMemory │ %-7d │\n", info.LoadMemory)
		_, _ = fmt.Fprintf(os.Stdout, "│ LocalData  │ %-7d │\n", info.LocalData)
		_, _ = fmt.Fprintf(os.Stdout, "│ MC7Size    │ %-7d │\n", info.MC7Size)
		_, _ = fmt.Fprintln(os.Stdout, "└────────────┴─────────┘")
	default:
		_, _ = fmt.Fprintf(os.Stdout, "%s%d load=%d local=%d mc7=%d\n", info.Type.String(), info.Number, info.LoadMemory, info.LocalData, info.MC7Size)
	}
	return nil
}

func runBlockUpload(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(blockUploadConfig.IP); err != nil {
		return ExitUsage(err)
	}
	if !blockUploadConfig.UseExplicitTSAP() && !blockUploadConfig.AutoRack {
		if err := validate.ValidateRackSlot(blockUploadConfig.Rack, blockUploadConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(blockUploadConfig.IP, blockUploadConfig.Quiet, "verify authorization and network exposure", blockUploadConfig.NoExposureWarning)
	if blockUploadConfig.Number <= 0 {
		return ExitUsage(fmt.Errorf("--number must be > 0"))
	}

	bt, err := validate.ValidateBlockType(blockUploadConfig.BlockType)
	if err != nil {
		return ExitUsage(err)
	}

	opts, err := blockUploadConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(blockUploadConfig.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
		return ExitRuntime(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	data, err := c.UploadBlock(cmd.Context(), bt, blockUploadConfig.Number)
	if err != nil {
		return ExitRuntime(fmt.Errorf("block upload failed: %w", err))
	}

	if blockUploadConfig.Quiet {
		_, _ = fmt.Fprintf(os.Stdout, "%s%d\n", bt.String(), blockUploadConfig.Number)
		return nil
	}

	switch strings.ToLower(blockUploadConfig.Format) {
	case "json":
		port := blockUploadConfig.Port
		if port == 0 {
			port = 102
		}
		reqMeta := blockUploadConfig.ConnectionRequestMeta()
		reqMeta["type"] = blockUploadConfig.BlockType
		reqMeta["number"] = blockUploadConfig.Number
		out := map[string]interface{}{
			"target":  map[string]interface{}{"ip": blockUploadConfig.IP, "port": port},
			"request": reqMeta,
			"result": map[string]interface{}{
				"type":   bt.String(),
				"number": blockUploadConfig.Number,
				"size":   len(data.Data),
				"hex":    hex.EncodeToString(data.Data),
			},
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(b))
		return nil
	case "hex":
		_, _ = fmt.Fprintln(os.Stdout, hex.EncodeToString(data.Data))
		return nil
	case "table":
		_, _ = fmt.Fprintln(os.Stdout, "┌──────┬────────┬──────┐")
		_, _ = fmt.Fprintln(os.Stdout, "│ Type │ Number │ Size │")
		_, _ = fmt.Fprintln(os.Stdout, "├──────┼────────┼──────┤")
		_, _ = fmt.Fprintf(os.Stdout, "│ %-4s │ %-6d │ %-4d │\n", bt.String(), blockUploadConfig.Number, len(data.Data))
		_, _ = fmt.Fprintln(os.Stdout, "└──────┴────────┴──────┘")
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s%d size=%d\n", bt.String(), blockUploadConfig.Number, len(data.Data))
	_, _ = fmt.Fprintln(os.Stdout, hex.EncodeToString(data.Data))
	return nil
}
