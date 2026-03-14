package s7commctl

import (
	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/spf13/cobra"
)

var blocksConfig = config.DefaultBlocksConfig()

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "List or inspect program blocks",
	Long:  "Alias/shortcut for listing blocks. Use \"list blocks\" for the same behavior under the list namespace.",
	RunE:  runBlocks,
}

func init() {
	config.RegisterFlags(blocksCmd, &blocksConfig)
	config.LoadFromEnv(&blocksConfig)
	rootCmd.AddCommand(blocksCmd)
}

// runBlocks delegates to runListBlocksWithConfig so blocks and list blocks share identical behavior (including JSON error envelope).
func runBlocks(cmd *cobra.Command, args []string) error {
	return runListBlocksWithConfig(cmd, args, blocksConfig)
}
