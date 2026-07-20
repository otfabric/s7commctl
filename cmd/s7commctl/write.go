// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var writeConfig = config.DefaultWriteConfig()

var writeCmd = &cobra.Command{
	Use:   "write",
	Short: "Write data to an S7 device",
	RunE:  runWrite,
}

func init() {
	config.RegisterFlags(writeCmd, &writeConfig)
	config.LoadFromEnv(&writeConfig)
	rootCmd.AddCommand(writeCmd)
}

func runWrite(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(writeConfig.IP); err != nil {
		return ExitUsage(err)
	}
	if !writeConfig.UseExplicitTSAP() && !writeConfig.AutoRack {
		if err := validate.ValidateRackSlot(writeConfig.Rack, writeConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(writeConfig.IP, writeConfig.Quiet, "writes on exposed PLCs are high risk", writeConfig.NoExposureWarning)

	if !writeConfig.Dangerous {
		return ExitUsage(fmt.Errorf("write operations require --dangerous flag"))
	}

	if !writeConfig.Force {
		if !confirmWrite(os.Stdin) {
			return ExitOperation(fmt.Errorf("write aborted by user"))
		}
	}

	area, err := validate.ValidateArea(writeConfig.Area)
	if err != nil {
		return ExitUsage(err)
	}

	data, err := validate.ParseTypedValue(writeConfig.Value, writeConfig.Type)
	if err != nil {
		return ExitUsage(fmt.Errorf("invalid value: %w", err))
	}

	opts, err := writeConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(writeConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return ExitRuntime(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	addr := model.Address{
		Area:     area,
		DBNumber: writeConfig.DBNumber,
		Start:    writeConfig.Offset,
		Size:     len(data),
	}

	if err := c.WriteArea(cmd.Context(), addr, data); err != nil {
		return ExitRuntime(fmt.Errorf("write failed: %w", err))
	}

	if writeConfig.Quiet {
		return nil
	}

	if strings.EqualFold(writeConfig.Format, "json") {
		port := writeConfig.Port
		if port == 0 {
			port = 102
		}
		reqMeta := writeConfig.ConnectionRequestMeta()
		reqMeta["area"] = area.String()
		reqMeta["db"] = writeConfig.DBNumber
		reqMeta["offset"] = writeConfig.Offset
		const maxValuePreview = 128
		if len(writeConfig.Value) <= maxValuePreview {
			reqMeta["value"] = writeConfig.Value
		} else {
			reqMeta["value"] = writeConfig.Value[:maxValuePreview] + "..."
			reqMeta["value_truncated"] = true
		}
		out := map[string]interface{}{
			"target":  map[string]interface{}{"ip": writeConfig.IP, "port": port},
			"request": reqMeta,
			"result":  map[string]interface{}{"ok": true, "written": len(data)},
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(b))
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "Successfully wrote %d bytes to %s\n", len(data), formatWriteAddress(area, writeConfig.DBNumber, writeConfig.Offset))
	return nil
}

// formatWriteAddress returns a human-readable address (e.g. DB1.DBX0, M100, I0, Q4, T20, C3).
func formatWriteAddress(area model.Area, dbNumber, offset int) string {
	switch area {
	case model.AreaDB:
		return fmt.Sprintf("DB%d.DBX%d", dbNumber, offset)
	case model.AreaMerkers:
		return fmt.Sprintf("M%d", offset)
	case model.AreaInputs:
		return fmt.Sprintf("I%d", offset)
	case model.AreaOutputs:
		return fmt.Sprintf("Q%d", offset)
	case model.AreaTimer:
		return fmt.Sprintf("T%d", offset)
	case model.AreaCounter:
		return fmt.Sprintf("C%d", offset)
	default:
		return fmt.Sprintf("%s %d", area.String(), offset)
	}
}

func confirmWrite(in io.Reader) bool {
	fmt.Fprint(os.Stderr, "WARNING: this operation writes to PLC memory. Type 'yes' to continue: ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "confirmation input unavailable")
		return false
	}
	return strings.EqualFold(strings.TrimSpace(line), "yes")
}
