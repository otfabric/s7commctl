package s7commctl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/s7comm/client"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var diagnosticConfig = config.DefaultDiagnosticConfig()

var diagnosticCmd = &cobra.Command{
	Use:   "diagnostic",
	Short: "Read diagnostic buffer from S7 device",
	RunE:  runDiagnostic,
}

func init() {
	config.RegisterFlags(diagnosticCmd, &diagnosticConfig)
	config.LoadFromEnv(&diagnosticConfig)
	rootCmd.AddCommand(diagnosticCmd)
}

func runDiagnostic(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(diagnosticConfig.IP); err != nil {
		return err
	}
	if err := validate.ValidateRackSlot(diagnosticConfig.Rack, diagnosticConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(diagnosticConfig.IP, diagnosticConfig.Quiet, "verify authorization and network exposure", diagnosticConfig.NoExposureWarning)

	opts := diagnosticConfig.ToClientOptions()
	c := client.New(diagnosticConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		diagnosticFailJSON(diagnosticConfig.Format, fmt.Sprintf("connect failed: %v", err))
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	diag, err := c.ReadDiagBuffer(cmd.Context())
	if err != nil {
		diagnosticFailJSON(diagnosticConfig.Format, fmt.Sprintf("read diagnostic buffer failed: %v", err))
		return fmt.Errorf("read diagnostic buffer failed: %w", err)
	}

	cpuState, _ := c.GetCPUState(cmd.Context())
	protection, _ := c.GetProtectionLevel(cmd.Context())

	if diagnosticConfig.Quiet {
		_, _ = fmt.Fprintln(os.Stdout, cpuState.String())
		return nil
	}

	if diagnosticConfig.Format == output.FormatJSON {
		payload := map[string]interface{}{
			"cpu_state":        cpuState.String(),
			"protection_level": protection.String(),
			"diagnostic":       diag,
		}
		output.PrintJSON(payload)
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "CPU State: %s\nProtection: %s\n\n", cpuState.String(), protection.String())

	_, _ = fmt.Fprint(os.Stdout, output.FormatDiagBuffer(diag, diagnosticConfig.Format))
	return nil
}

// diagnosticFailJSON prints a structured error envelope to stdout and exits 1 when format is json (IMPROVEMENT_PLAN 2.3).
func diagnosticFailJSON(format, errMsg string) {
	if strings.ToLower(format) != output.FormatJSON {
		return
	}
	port := diagnosticConfig.Port
	if port == 0 {
		port = 102
	}
	out := map[string]interface{}{
		"status": "error",
		"error":  errMsg,
		"target": map[string]interface{}{
			"ip":   diagnosticConfig.IP,
			"port": port,
			"rack": diagnosticConfig.Rack,
			"slot": diagnosticConfig.Slot,
		},
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
	os.Exit(1)
}
