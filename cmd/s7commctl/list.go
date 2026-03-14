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
		_, _ = fmt.Fprintln(os.Stdout, "DB\nI\nQ\nM\nT\nC\nDATA_RECORD\nS7200AO\nIEC_COUNTER_200\nIEC_TIMER_200")
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
	return runListBlocksWithConfig(cmd, args, listBlocksConfig)
}

// runListBlocksWithConfig runs list blocks with the given config (avoids mutating global listBlocksConfig).
func runListBlocksWithConfig(cmd *cobra.Command, _ []string, cfg config.BlocksConfig) error {
	if err := validate.ValidateIP(cfg.IP); err != nil {
		return ExitUsage(err)
	}
	if !cfg.UseExplicitTSAP() && !cfg.AutoRack {
		if err := validate.ValidateRackSlot(cfg.Rack, cfg.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(cfg.IP, cfg.Quiet, "verify authorization and network exposure", cfg.NoExposureWarning)

	opts, err := cfg.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(cfg.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
		listBlocksErrorJSON(cfg, "connect failed", err)
		return ExitRuntime(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	var blocks []model.BlockInfo
	if cfg.BlockType != "" {
		bt, e := validate.ValidateBlockType(cfg.BlockType)
		if e != nil {
			return ExitUsage(e)
		}
		blocks, err = c.ListBlocks(cmd.Context(), bt)
	} else {
		blocks, err = c.ListAllBlocks(cmd.Context())
	}
	if err != nil {
		listBlocksErrorJSON(cfg, "list blocks failed", err)
		return ExitRuntime(fmt.Errorf("list blocks failed: %w", err))
	}
	if blocks == nil {
		blocks = []model.BlockInfo{}
	}

	_, _ = fmt.Fprint(os.Stdout, output.FormatBlocks(blocks, cfg.Format))
	return nil
}

// listBlocksErrorJSON prints structured error JSON to stdout when format is json (caller should then return ExitRuntime).
func listBlocksErrorJSON(cfg config.BlocksConfig, prefix string, err error) {
	if strings.ToLower(cfg.Format) != output.FormatJSON {
		return
	}
	msg := prefix
	if err != nil {
		msg = prefix + ": " + err.Error()
	}
	port := cfg.Port
	if port == 0 {
		port = 102
	}
	reqMeta := cfg.ConnectionRequestMeta()
	reqMeta["block_type"] = cfg.BlockType
	target := map[string]interface{}{"ip": cfg.IP, "port": port}
	out := map[string]interface{}{
		"target":  target,
		"request": reqMeta,
		"error":   map[string]string{"code": "runtime", "message": msg},
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}

func runListDB(cmd *cobra.Command, args []string) error {
	cfg := listDBConfig
	cfg.BlockType = "DB"
	if cfg.Format == "" {
		cfg.Format = "table"
	}
	return runListBlocksWithConfig(cmd, args, cfg)
}
