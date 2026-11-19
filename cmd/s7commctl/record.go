// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/otfabric/s7commctl/internal/app/validate"
	"github.com/spf13/cobra"
)

type recordConfig struct {
	config.DeviceConfig
	Area     string `flag:"area" env:"S7COMMCTL_AREA" desc:"Memory area: DB, I, Q, M, T, C, DATA_RECORD, S7200AO, IEC_*_200"`
	DBNumber int    `flag:"db" env:"S7COMMCTL_DB" desc:"Data block number"`
	Offset   int    `flag:"offset" env:"S7COMMCTL_OFFSET" desc:"Byte offset (on T and C: element number)"`
	Size     int    `flag:"size" env:"S7COMMCTL_SIZE" desc:"Number of bytes per sample (on T and C: elements of two bytes)"`
	Type     string `flag:"type" env:"S7COMMCTL_TYPE" desc:"Data type"`
	EveryMS  int    `flag:"every-ms" env:"S7COMMCTL_EVERY_MS" desc:"Sampling interval in milliseconds"`
	Count    int    `flag:"count" env:"S7COMMCTL_COUNT" desc:"Number of samples (0 = run until interrupted)"`
	Output   string `flag:"output" env:"S7COMMCTL_OUTPUT" desc:"Output file path"`
	Format   string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: jsonl, hex"`

	MaxFailures int `flag:"max-failures" env:"S7COMMCTL_MAX_FAILURES" desc:"Stop after this many consecutive samples fail with a transport error or timeout (0 = never stop)"`
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
	MaxFailures:  5,
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
		return ExitUsage(err)
	}
	if !recConfig.UseExplicitTSAP() && !recConfig.AutoRack {
		if err := validate.ValidateRackSlot(recConfig.Rack, recConfig.Slot); err != nil {
			return ExitUsage(err)
		}
	}
	warnIfPublicTarget(recConfig.IP, recConfig.Quiet, "verify authorization and network exposure", recConfig.NoExposureWarning)
	if recConfig.Size <= 0 {
		return ExitUsage(fmt.Errorf("size must be > 0"))
	}
	if recConfig.EveryMS <= 0 {
		return ExitUsage(fmt.Errorf("every-ms must be > 0"))
	}
	if recConfig.MaxFailures < 0 {
		return ExitUsage(fmt.Errorf("max-failures must be >= 0"))
	}

	area, err := validate.ValidateArea(recConfig.Area)
	if err != nil {
		return ExitUsage(err)
	}

	opts, err := recConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}
	c := client.New(recConfig.IP, opts...)
	if err := c.Connect(cmd.Context()); err != nil {
		return ExitConnect(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	// Opened, and an existing file emptied, only once there is a connection to
	// record from.
	f, err := os.OpenFile(filepath.Clean(recConfig.Output), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return ExitRuntime(fmt.Errorf("open output file: %w", err))
	}
	defer func() { _ = f.Close() }()

	addr := model.Address{Area: area, DBNumber: recConfig.DBNumber, Start: recConfig.Offset, Size: recConfig.Size}
	ticker := time.NewTicker(time.Duration(recConfig.EveryMS) * time.Millisecond)
	defer ticker.Stop()

	// A sample fails when its status is not success. Consecutive failures with a
	// transport error or timeout mean the connection is lost: nothing
	// reconnects, so --max-failures of them end the run.
	samples, failed, lost := 0, 0, 0
	var last *client.ReadResult
	for recConfig.Count == 0 || samples < recConfig.Count {
		select {
		case <-cmd.Context().Done():
			return nil
		case <-ticker.C:
			result, err := c.ReadArea(cmd.Context(), addr)
			if err != nil {
				return ExitRuntime(fmt.Errorf("record read failed: %w", err))
			}
			if err := writeSample(f, recConfig.Format, recConfig.Type, samples, result); err != nil {
				return ExitRuntime(fmt.Errorf("write sample failed: %w", err))
			}
			samples++
			if cmd.Context().Err() != nil {
				return nil // interrupted: the sample in flight is no failure
			}
			if result.OK() {
				lost = 0
				continue
			}
			failed++
			last = result
			if result.Status != client.ReadStatusTransportErr && result.Status != client.ReadStatusTimeout {
				lost = 0
				continue
			}
			lost++
			if recConfig.MaxFailures > 0 && lost >= recConfig.MaxFailures {
				printRecordSummary(samples, failed)
				return &ExitError{Code: readStatusCode(result.Status), Err: fmt.Errorf("record stopped: the connection to %s was lost (%d consecutive samples failed; the last: %s)",
					recConfig.IP, lost, sampleOutcome(result))}
			}
		}
	}

	printRecordSummary(samples, failed)
	if failed > 0 {
		return ExitOperation(fmt.Errorf("%d of %d samples failed; the last: %s", failed, samples, sampleOutcome(last)))
	}
	return nil
}

func printRecordSummary(samples, failed int) {
	_, _ = fmt.Fprintf(os.Stdout, "Recorded %d samples (%d failed) to %s\n", samples, failed, recConfig.Output)
}

// sampleOutcome is the status of a failed sample and what went wrong.
func sampleOutcome(r *client.ReadResult) string {
	return fmt.Sprintf("%s (%v)", r.Status, r.Err())
}

func writeSample(w io.Writer, format, dataType string, sampleIdx int, result *client.ReadResult) error {
	data := []byte{}
	status := ""
	if result != nil {
		data = result.Data
		status = string(result.Status)
	}
	switch format {
	case "hex":
		_, err := fmt.Fprintln(w, hex.EncodeToString(data))
		return err
	case "jsonl":
		fallthrough
	default:
		record := map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
			"sample":    sampleIdx,
			"status":    status,
			"raw_hex":   hex.EncodeToString(data),
		}
		if result != nil {
			record["requested_length"] = result.RequestedLength
			record["returned_length"] = result.ReturnedLength
			if result.Message != "" {
				record["message"] = result.Message
			}
		}
		if dataType != "raw" && len(data) > 0 {
			record["decoded"] = outputValue(data, dataType)
		}
		line, _ := json.Marshal(record)
		_, err := fmt.Fprintln(w, string(line))
		return err
	}
}

func outputValue(data []byte, dataType string) any {
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
