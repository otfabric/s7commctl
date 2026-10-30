// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/output"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

var fingerprintConfig = config.DefaultFingerprintConfig()

var fingerprintCmd = &cobra.Command{
	Use:   "fingerprint",
	Short: "Fingerprint an S7 device",
	RunE:  runFingerprint,
}

func init() {
	config.RegisterFlags(fingerprintCmd, &fingerprintConfig)
	config.LoadFromEnv(&fingerprintConfig)
	rootCmd.AddCommand(fingerprintCmd)
}

func runFingerprint(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(fingerprintConfig.IP); err != nil {
		return ExitUsage(err)
	}
	if !fingerprintConfig.UseExplicitTSAP() && !fingerprintConfig.AutoRack {
		if err := validate.ValidateRackSlot(fingerprintConfig.Rack, fingerprintConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(fingerprintConfig.IP, fingerprintConfig.Quiet, "verify authorization and network exposure", fingerprintConfig.NoExposureWarning)

	opts, err := fingerprintConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(fingerprintConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return ExitConnect(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	info, err := c.Identify(cmd.Context())
	if err != nil {
		return ExitRuntime(fmt.Errorf("identify failed: %w", err))
	}

	connInfo := c.ConnectionInfo()
	protection, _ := c.GetProtectionLevel(cmd.Context())
	readRes, readErr := c.ReadMerkers(cmd.Context(), 0, 1)
	_, uploadProbeErr := c.ListBlocks(cmd.Context(), model.BlockDB)

	view := &output.FingerprintView{
		CPUType:            info.CPUType,
		CPUGeneration:      classifyCPUGeneration(info.OrderNumber, info.CPUType),
		OrderNumber:        info.OrderNumber,
		FirmwareVer:        info.FWVersion,
		ModuleName:         info.ModuleName,
		PDUSize:            connInfo.PDUSize,
		MaxAmqCalling:      connInfo.MaxAmqCalling,
		MaxAmqCalled:       connInfo.MaxAmqCalled,
		DetectedRack:       connInfo.Rack,
		DetectedSlot:       connInfo.Slot,
		AcceptedTSAP:       fmt.Sprint(connInfo.RemoteTSAP),
		Protection:         protection.String(),
		SupportsRead:       readErr == nil && readRes != nil && readRes.OK(),
		WriteLikelyAllowed: protection == model.ProtectionNone,
		SupportsSZL:        true,
		UploadProbeOK:      uploadProbeErr == nil,
	}

	if fingerprintConfig.Quiet {
		_, _ = fmt.Fprintln(os.Stdout, view.CPUType)
		return nil
	}

	if strings.ToLower(fingerprintConfig.Format) == output.FormatJSON {
		port := fingerprintConfig.Port
		if port == 0 {
			port = 102
		}
		req := fingerprintConfig.ConnectionRequestMeta()
		out := map[string]interface{}{
			"target":  map[string]interface{}{"ip": fingerprintConfig.IP, "port": port},
			"request": req,
			"result":  output.FingerprintResultMap(view),
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(data))
		return nil
	}
	_, _ = fmt.Fprint(os.Stdout, output.FormatFingerprint(view, fingerprintConfig.Format))
	return nil
}

func classifyCPUGeneration(orderNumber, cpuType string) int {
	candidate := strings.ToUpper(orderNumber + " " + cpuType)
	switch {
	case strings.Contains(candidate, "1500"):
		return 1500
	case strings.Contains(candidate, "1200"):
		return 1200
	case strings.Contains(candidate, "400"):
		return 400
	case strings.Contains(candidate, "300"):
		return 300
	default:
		return 0
	}
}
