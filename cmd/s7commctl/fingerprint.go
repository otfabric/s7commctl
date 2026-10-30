// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
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

var (
	// cpuModelRE finds the model number in "CPU 315-2 PN/DP", "S7-1200", "CPU1214C".
	cpuModelRE = regexp.MustCompile(`\b(?:CPU|S7)[ -]*(\d{3,4})`)
	// orderFamilyRE finds the digits after the Siemens article prefix: 6ES7 315-2EH14-0AB0.
	orderFamilyRE = regexp.MustCompile(`\b6ES7[ -]*(\d{2})`)
)

// classifyCPUGeneration returns 300, 400, 1200 or 1500, or 0 when unknown. The
// model number decides first. In order numbers 6ES7 21x is left unknown: S7-200
// and S7-1200 both use it.
func classifyCPUGeneration(orderNumber, cpuType string) int {
	for _, text := range []string{cpuType, orderNumber} {
		m := cpuModelRE.FindStringSubmatch(strings.ToUpper(text))
		if m == nil {
			continue
		}
		switch model, _ := strconv.Atoi(m[1]); {
		case model >= 1500 && model < 1600:
			return 1500
		case model >= 1200 && model < 1300:
			return 1200
		case model >= 400 && model < 500:
			return 400
		case model >= 300 && model < 400:
			return 300
		}
	}
	if m := orderFamilyRE.FindStringSubmatch(strings.ToUpper(orderNumber)); m != nil {
		switch m[1] {
		case "51":
			return 1500
		case "41":
			return 400
		case "31":
			return 300
		}
	}
	return 0
}
