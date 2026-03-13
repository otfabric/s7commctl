package s7commctl

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
	"otfabric/s7comm/client"
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
		return err
	}
	if err := validate.ValidateRackSlot(blockInfoConfig.Rack, blockInfoConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(blockInfoConfig.IP, blockInfoConfig.Quiet, "verify authorization and network exposure")
	if blockInfoConfig.Number <= 0 {
		return fmt.Errorf("--number must be > 0")
	}

	bt, err := validate.ValidateBlockType(blockInfoConfig.BlockType)
	if err != nil {
		return err
	}

	c := client.New(blockInfoConfig.IP, blockInfoConfig.ToClientOptions()...)
	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	info, err := c.GetBlockInfo(cmd.Context(), bt, blockInfoConfig.Number)
	if err != nil {
		return fmt.Errorf("block info failed: %w", err)
	}

	if blockInfoConfig.Quiet {
		_, _ = fmt.Fprintf(os.Stdout, "%s%d\n", bt.String(), blockInfoConfig.Number)
		return nil
	}

	switch strings.ToLower(blockInfoConfig.Format) {
	case "json":
		b, _ := json.MarshalIndent(info, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(b))
	case "table":
		_, _ = fmt.Fprintln(os.Stdout, "Field       Value")
		_, _ = fmt.Fprintln(os.Stdout, "----------- -----")
		_, _ = fmt.Fprintf(os.Stdout, "Type        %s\n", info.Type.String())
		_, _ = fmt.Fprintf(os.Stdout, "Number      %d\n", info.Number)
		_, _ = fmt.Fprintf(os.Stdout, "LoadMemory  %d\n", info.LoadMemory)
		_, _ = fmt.Fprintf(os.Stdout, "LocalData   %d\n", info.LocalData)
		_, _ = fmt.Fprintf(os.Stdout, "MC7Size     %d\n", info.MC7Size)
	default:
		_, _ = fmt.Fprintf(os.Stdout, "%s%d load=%d local=%d mc7=%d\n", info.Type.String(), info.Number, info.LoadMemory, info.LocalData, info.MC7Size)
	}
	return nil
}

func runBlockUpload(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(blockUploadConfig.IP); err != nil {
		return err
	}
	if err := validate.ValidateRackSlot(blockUploadConfig.Rack, blockUploadConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(blockUploadConfig.IP, blockUploadConfig.Quiet, "verify authorization and network exposure")
	if blockUploadConfig.Number <= 0 {
		return fmt.Errorf("--number must be > 0")
	}

	bt, err := validate.ValidateBlockType(blockUploadConfig.BlockType)
	if err != nil {
		return err
	}

	c := client.New(blockUploadConfig.IP, blockUploadConfig.ToClientOptions()...)
	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	data, err := c.UploadBlock(cmd.Context(), bt, blockUploadConfig.Number)
	if err != nil {
		return fmt.Errorf("block upload failed: %w", err)
	}

	if blockUploadConfig.Quiet {
		_, _ = fmt.Fprintf(os.Stdout, "%s%d\n", bt.String(), blockUploadConfig.Number)
		return nil
	}

	switch strings.ToLower(blockUploadConfig.Format) {
	case "json":
		result := map[string]interface{}{
			"type":   bt.String(),
			"number": blockUploadConfig.Number,
			"size":   len(data.Data),
			"hex":    hex.EncodeToString(data.Data),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(b))
		return nil
	case "hex":
		_, _ = fmt.Fprintln(os.Stdout, hex.EncodeToString(data.Data))
		return nil
	case "table":
		_, _ = fmt.Fprintln(os.Stdout, "Type Number Size")
		_, _ = fmt.Fprintf(os.Stdout, "%s %d %d\n", bt.String(), blockUploadConfig.Number, len(data.Data))
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s%d size=%d\n", bt.String(), blockUploadConfig.Number, len(data.Data))
	_, _ = fmt.Fprintln(os.Stdout, hex.EncodeToString(data.Data))
	return nil
}
