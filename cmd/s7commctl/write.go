package s7commctl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/otfabric/s7comm/client"
	"github.com/otfabric/s7comm/model"

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
		return err
	}
	if err := validate.ValidateRackSlot(writeConfig.Rack, writeConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(writeConfig.IP, writeConfig.Quiet, "writes on exposed PLCs are high risk", writeConfig.NoExposureWarning)

	if !writeConfig.Dangerous {
		return fmt.Errorf("write operations require --dangerous flag")
	}

	if !writeConfig.Force {
		if !confirmWrite(os.Stdin) {
			return fmt.Errorf("write aborted by user")
		}
	}

	area, err := validate.ValidateArea(writeConfig.Area)
	if err != nil {
		return err
	}

	data, err := validate.ParseTypedValue(writeConfig.Value, writeConfig.Type)
	if err != nil {
		return fmt.Errorf("invalid value: %w", err)
	}

	opts := writeConfig.ToClientOptions()
	c := client.New(writeConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	addr := model.Address{
		Area:     area,
		DBNumber: writeConfig.DBNumber,
		Start:    writeConfig.Offset,
		Size:     len(data),
	}

	if err := c.WriteArea(cmd.Context(), addr, data); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if writeConfig.Quiet {
		return nil
	}

	if strings.EqualFold(writeConfig.Format, "json") {
		payload := map[string]interface{}{
			"ok":      true,
			"area":    area.String(),
			"db":      writeConfig.DBNumber,
			"offset":  writeConfig.Offset,
			"written": len(data),
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(b))
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "Successfully wrote %d bytes to %s%d.%d\n", len(data), area.String(), writeConfig.DBNumber, writeConfig.Offset)
	return nil
}

func confirmWrite(in io.Reader) bool {
	fmt.Fprint(os.Stderr, "WARNING: this operation writes to PLC memory. Type 'yes' to continue: ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(line), "yes")
}
