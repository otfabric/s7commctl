package s7commctl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/s7comm/client"
	"github.com/otfabric/s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var listBlocksConfig = config.DefaultBlocksConfig()
var listDBConfig = config.DefaultBlocksConfig()

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List S7 entities",
}

var listBlocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "List program blocks",
	RunE:  runListBlocks,
}

var listDBCmd = &cobra.Command{
	Use:   "db",
	Short: "List DB blocks",
	RunE:  runListDB,
}

var listAreasCmd = &cobra.Command{
	Use:   "areas",
	Short: "List supported memory areas",
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintln(os.Stdout, "DB\nI\nQ\nM\nT\nC")
	},
}

func init() {
	config.RegisterFlags(listBlocksCmd, &listBlocksConfig)
	config.LoadFromEnv(&listBlocksConfig)
	config.RegisterFlags(listDBCmd, &listDBConfig)
	config.LoadFromEnv(&listDBConfig)

	listCmd.AddCommand(listBlocksCmd)
	listCmd.AddCommand(listDBCmd)
	listCmd.AddCommand(listAreasCmd)
	rootCmd.AddCommand(listCmd)
}

func runListBlocks(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(listBlocksConfig.IP); err != nil {
		return err
	}
	if err := validate.ValidateRackSlot(listBlocksConfig.Rack, listBlocksConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(listBlocksConfig.IP, listBlocksConfig.Quiet, "verify authorization and network exposure", listBlocksConfig.NoExposureWarning)

	opts := listBlocksConfig.ToClientOptions()
	c := client.New(listBlocksConfig.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
		listBlocksFailJSON(listBlocksConfig.Format, fmt.Sprintf("connect failed: %v", err))
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	var blocks []model.BlockInfo
	var err error
	if listBlocksConfig.BlockType != "" {
		bt, e := validate.ValidateBlockType(listBlocksConfig.BlockType)
		if e != nil {
			return e
		}
		blocks, err = c.ListBlocks(cmd.Context(), bt)
	} else {
		blocks, err = c.ListAllBlocks(cmd.Context())
	}
	if err != nil {
		listBlocksFailJSON(listBlocksConfig.Format, fmt.Sprintf("list blocks failed: %v", err))
		return fmt.Errorf("list blocks failed: %w", err)
	}
	if blocks == nil {
		blocks = []model.BlockInfo{}
	}

	_, _ = fmt.Fprint(os.Stdout, output.FormatBlocks(blocks, listBlocksConfig.Format))
	return nil
}

// listBlocksFailJSON prints structured error JSON and exits when format is json (REQUIREMENTS #17).
func listBlocksFailJSON(format, errMsg string) {
	if strings.ToLower(format) != output.FormatJSON {
		return
	}
	out := map[string]string{"status": "error", "error": errMsg}
	data, _ := json.Marshal(out)
	_, _ = fmt.Fprintln(os.Stdout, string(data))
	os.Exit(1)
}

func runListDB(cmd *cobra.Command, args []string) error {
	listDBConfig.BlockType = "DB"
	if listDBConfig.Format == "" {
		listDBConfig.Format = "table"
	}
	listBlocksConfig = listDBConfig
	return runListBlocks(cmd, args)
}
