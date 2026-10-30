// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/go-s7comm/client"

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
		return ExitUsage(err)
	}
	if !diagnosticConfig.UseExplicitTSAP() && !diagnosticConfig.AutoRack {
		if err := validate.ValidateRackSlot(diagnosticConfig.Rack, diagnosticConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(diagnosticConfig.IP, diagnosticConfig.Quiet, "verify authorization and network exposure", diagnosticConfig.NoExposureWarning)

	opts, err := diagnosticConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(diagnosticConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		diagnosticErrorJSON(diagnosticConfig, "connect failed", err)
		return ExitConnect(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	diag, err := c.ReadDiagBuffer(cmd.Context())
	if err != nil {
		diagnosticErrorJSON(diagnosticConfig, "read diagnostic buffer failed", err)
		return ExitRuntime(fmt.Errorf("read diagnostic buffer failed: %w", err))
	}

	cpuState, _ := c.GetCPUState(cmd.Context())
	protection, _ := c.GetProtectionLevel(cmd.Context())

	if diagnosticConfig.Quiet {
		_, _ = fmt.Fprintln(os.Stdout, cpuState.String())
		return nil
	}

	if strings.ToLower(diagnosticConfig.Format) == output.FormatJSON {
		port := diagnosticConfig.Port
		if port == 0 {
			port = 102
		}
		reqMeta := diagnosticConfig.ConnectionRequestMeta()
		out := map[string]interface{}{
			"target":  map[string]interface{}{"ip": diagnosticConfig.IP, "port": port},
			"request": reqMeta,
			"result": map[string]interface{}{
				"cpu_state":        cpuState.String(),
				"protection_level": protection.String(),
				"diagnostic":       output.DiagBufferResultMap(diag),
			},
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "CPU State: %s\nProtection: %s\n\n", cpuState.String(), protection.String())

	_, _ = fmt.Fprint(os.Stdout, output.FormatDiagBuffer(diag, diagnosticConfig.Format))
	return nil
}

// diagnosticErrorJSON prints a structured error envelope to stdout when format is json (caller should then return ExitRuntime).
func diagnosticErrorJSON(cfg config.DiagnosticConfig, prefix string, err error) {
	if strings.ToLower(cfg.Format) != output.FormatJSON {
		return
	}
	port := cfg.Port
	if port == 0 {
		port = 102
	}
	reqMeta := cfg.ConnectionRequestMeta()
	out := map[string]interface{}{
		"target":  map[string]interface{}{"ip": cfg.IP, "port": port},
		"request": reqMeta,
		"error":   map[string]string{"code": "runtime", "message": prefix + ": " + err.Error()},
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}
