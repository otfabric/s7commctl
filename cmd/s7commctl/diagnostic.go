package s7commctl

import (
	"fmt"
	"os"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
	"otfabric/s7comm/client"
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
	warnIfPublicTarget(diagnosticConfig.IP, diagnosticConfig.Quiet, "verify authorization and network exposure")

	opts := diagnosticConfig.ToClientOptions()
	c := client.New(diagnosticConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	diag, err := c.ReadDiagBuffer(cmd.Context())
	if err != nil {
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
