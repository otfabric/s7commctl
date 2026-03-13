package s7commctl

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/otfabric/s7comm/client"
	"github.com/otfabric/s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

type recordConfig struct {
	config.DeviceConfig
	Area     string `flag:"area" env:"S7COMMCTL_AREA" desc:"Memory area: DB, I, Q, M"`
	DBNumber int    `flag:"db" env:"S7COMMCTL_DB" desc:"Data block number"`
	Offset   int    `flag:"offset" env:"S7COMMCTL_OFFSET" desc:"Byte offset"`
	Size     int    `flag:"size" env:"S7COMMCTL_SIZE" desc:"Number of bytes per sample"`
	Type     string `flag:"type" env:"S7COMMCTL_TYPE" desc:"Data type"`
	EveryMS  int    `flag:"every-ms" env:"S7COMMCTL_EVERY_MS" desc:"Sampling interval in milliseconds"`
	Count    int    `flag:"count" env:"S7COMMCTL_COUNT" desc:"Number of samples (0 = run until interrupted)"`
	Output   string `flag:"output" env:"S7COMMCTL_OUTPUT" desc:"Output file path"`
	Format   string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: jsonl, hex"`
}

var recConfig = recordConfig{
	DeviceConfig: config.DefaultDeviceConfig(),
	Area:         "DB",
	DBNumber:     1,
	Offset:       0,
	Size:         1,
	Type:         "raw",
	EveryMS:      1000,
	Count:        0,
	Output:       "record.jsonl",
	Format:       "jsonl",
}

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record periodic S7 reads to a local file",
	RunE:  runRecord,
}

func init() {
	config.RegisterFlags(recordCmd, &recConfig)
	config.LoadFromEnv(&recConfig)
	rootCmd.AddCommand(recordCmd)
}

func runRecord(cmd *cobra.Command, args []string) error {
	if err := validate.ValidateIP(recConfig.IP); err != nil {
		return err
	}
	if err := validate.ValidateRackSlot(recConfig.Rack, recConfig.Slot); err != nil {
		return err
	}
	warnIfPublicTarget(recConfig.IP, recConfig.Quiet, "verify authorization and network exposure", recConfig.NoExposureWarning)
	if recConfig.Size <= 0 {
		return fmt.Errorf("size must be > 0")
	}
	if recConfig.EveryMS <= 0 {
		return fmt.Errorf("every-ms must be > 0")
	}

	area, err := validate.ValidateArea(recConfig.Area)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(recConfig.Output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer func() { _ = f.Close() }()

	c := client.New(recConfig.IP, recConfig.ToClientOptions()...)
	if err := c.Connect(cmd.Context()); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer func() { _ = c.Close() }()

	addr := model.Address{Area: area, DBNumber: recConfig.DBNumber, Start: recConfig.Offset, Size: recConfig.Size}
	ticker := time.NewTicker(time.Duration(recConfig.EveryMS) * time.Millisecond)
	defer ticker.Stop()

	samples := 0
	for recConfig.Count == 0 || samples < recConfig.Count {
		select {
		case <-cmd.Context().Done():
			return nil
		case <-ticker.C:
			result, err := c.ReadArea(cmd.Context(), addr)
			if err != nil {
				return fmt.Errorf("record read failed: %w", err)
			}
			if err := writeSample(f, samples, result); err != nil {
				return err
			}
			samples++
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "Recorded %d samples to %s\n", samples, recConfig.Output)
	return nil
}

func writeSample(f *os.File, sampleIdx int, result *client.ReadResult) error {
	data := []byte{}
	status := ""
	if result != nil {
		data = result.Data
		status = string(result.Status)
	}
	switch recConfig.Format {
	case "hex":
		_, err := fmt.Fprintln(f, hex.EncodeToString(data))
		return err
	case "jsonl":
		fallthrough
	default:
		record := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"sample":    sampleIdx,
			"status":    status,
			"raw_hex":   hex.EncodeToString(data),
		}
		if result != nil {
			record["requested_length"] = result.RequestedLength
			record["returned_length"] = result.ReturnedLength
			if result.Error != "" {
				record["error"] = result.Error
			}
		}
		if recConfig.Type != "raw" && len(data) > 0 {
			record["decoded"] = outputValue(data, recConfig.Type)
		}
		line, _ := json.Marshal(record)
		_, err := fmt.Fprintln(f, string(line))
		return err
	}
}

func outputValue(data []byte, dataType string) interface{} {
	switch dataType {
	case "bool":
		return model.DecodeBool(data, 0)
	case "int":
		if len(data) >= 2 {
			return model.DecodeInt(data[:2])
		}
	case "dint":
		if len(data) >= 4 {
			return model.DecodeDInt(data[:4])
		}
	case "real":
		if len(data) >= 4 {
			return model.DecodeReal(data[:4])
		}
	case "string":
		return model.DecodeString(data)
	}
	return nil
}
