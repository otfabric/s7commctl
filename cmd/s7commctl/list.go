package s7commctl

import (
	"fmt"
	"os"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
	"otfabric/s7comm/client"
	"otfabric/s7comm/model"
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
	warnIfPublicTarget(listBlocksConfig.IP, listBlocksConfig.Quiet, "verify authorization and network exposure")

	opts := listBlocksConfig.ToClientOptions()
	c := client.New(listBlocksConfig.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
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
		return fmt.Errorf("list blocks failed: %w", err)
	}

	_, _ = fmt.Fprint(os.Stdout, output.FormatBlocks(blocks, listBlocksConfig.Format))
	return nil
}

func runListDB(cmd *cobra.Command, args []string) error {
	listDBConfig.BlockType = "DB"
	if listDBConfig.Format == "" {
		listDBConfig.Format = "table"
	}
	listBlocksConfig = listDBConfig
	return runListBlocks(cmd, args)
}
