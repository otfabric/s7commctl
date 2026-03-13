package s7commctl

import (
	"fmt"
	"os"

	"otfabric/s7comm/client"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var identifyConfig = config.DefaultIdentifyConfig()

var identifyCmd = &cobra.Command{
	Use:   "identify",
	Short: "Identify an S7 device",
	RunE:  runIdentify,
}

func init() {
	config.RegisterFlags(identifyCmd, &identifyConfig)
	config.LoadFromEnv(&identifyConfig)
	rootCmd.AddCommand(identifyCmd)
}

func runIdentify(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(identifyConfig.IP); err != nil {
		return err
	}
	if err := validate.ValidateRackSlot(identifyConfig.Rack, identifyConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(identifyConfig.IP, identifyConfig.Quiet, "verify authorization and network exposure")

	opts := identifyConfig.ToClientOptions()
	c := client.New(identifyConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	info, err := c.Identify(cmd.Context())
	if err != nil {
		return fmt.Errorf("identify failed: %w", err)
	}

	if identifyConfig.Quiet {
		if info.CPUType != "" {
			_, _ = fmt.Fprintln(os.Stdout, info.CPUType)
		} else {
			_, _ = fmt.Fprintln(os.Stdout, info.OrderNumber)
		}
		return nil
	}

	_, _ = fmt.Fprint(os.Stdout, output.FormatDeviceInfo(info, identifyConfig.Format))
	return nil
}
