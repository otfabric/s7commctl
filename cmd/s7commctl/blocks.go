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

var blocksConfig = config.DefaultBlocksConfig()

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "List or inspect program blocks",
	RunE:  runBlocks,
}

func init() {
	config.RegisterFlags(blocksCmd, &blocksConfig)
	config.LoadFromEnv(&blocksConfig)
	rootCmd.AddCommand(blocksCmd)
}

func runBlocks(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(blocksConfig.IP); err != nil {
		return err
	}
	if err := validate.ValidateRackSlot(blocksConfig.Rack, blocksConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(blocksConfig.IP, blocksConfig.Quiet, "verify authorization and network exposure")

	opts := blocksConfig.ToClientOptions()
	c := client.New(blocksConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	var blocks []model.BlockInfo
	var err error

	if blocksConfig.BlockType != "" {
		bt, err := validate.ValidateBlockType(blocksConfig.BlockType)
		if err != nil {
			return err
		}
		blocks, err = c.ListBlocks(cmd.Context(), bt)
		if err != nil {
			return fmt.Errorf("list blocks failed: %w", err)
		}
	} else {
		blocks, err = c.ListAllBlocks(cmd.Context())
		if err != nil {
			return fmt.Errorf("list blocks failed: %w", err)
		}
	}

	if blocksConfig.Quiet {
		for _, b := range blocks {
			_, _ = fmt.Fprintf(os.Stdout, "%s%d\n", b.Type.String(), b.Number)
		}
		return nil
	}

	_, _ = fmt.Fprint(os.Stdout, output.FormatBlocks(blocks, blocksConfig.Format))
	return nil
}
