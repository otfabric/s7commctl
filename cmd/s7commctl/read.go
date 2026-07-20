// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var readConfig = config.DefaultReadConfig()

var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Read data from an S7 device",
	RunE:  runRead,
}

func init() {
	config.RegisterFlags(readCmd, &readConfig)
	config.LoadFromEnv(&readConfig)
	rootCmd.AddCommand(readCmd)
}

func runRead(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(readConfig.IP); err != nil {
		return ExitUsage(err)
	}
	if !readConfig.UseExplicitTSAP() && !readConfig.AutoRack {
		if err := validate.ValidateRackSlot(readConfig.Rack, readConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(readConfig.IP, readConfig.Quiet, "verify authorization and network exposure", readConfig.NoExposureWarning)

	area, err := validate.ValidateArea(readConfig.Area)
	if err != nil {
		return ExitUsage(err)
	}

	opts, err := readConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(readConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return ExitRuntime(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	addr := model.Address{
		Area:     area,
		DBNumber: readConfig.DBNumber,
		Start:    readConfig.Offset,
		Size:     readConfig.Size,
	}

	result, err := c.ReadArea(cmd.Context(), addr)
	if err != nil {
		return ExitRuntime(fmt.Errorf("read failed: %w", err))
	}

	if readConfig.Quiet {
		if result != nil && len(result.Data) > 0 {
			_, _ = fmt.Fprintln(os.Stdout, hex.EncodeToString(result.Data))
		}
		return nil
	}

	port := readConfig.Port
	if port == 0 {
		port = 102
	}
	meta := &output.ReadResultMeta{
		IP: readConfig.IP, Port: port,
		Area: readConfig.Area, DBNumber: readConfig.DBNumber, Offset: readConfig.Offset, RequestedSize: readConfig.Size,
		ConnectionRequest: readConfig.ConnectionRequestMeta(),
	}
	_, _ = fmt.Fprint(os.Stdout, output.FormatReadResult(result, readConfig.Type, readConfig.Format, meta))
	if result != nil && result.Err() != nil && readConfig.StrictRead {
		return ExitOperation(result.Err())
	}
	return nil
}
