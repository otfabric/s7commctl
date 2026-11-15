// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
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

	area, err := validate.ValidateArea(writeConfig.Area)
	if err != nil {
		return ExitUsage(err)
	}
	plan, err := planWrite(writeConfig, area, cmd.Flags().Changed("bit"))
	if err != nil {
		return ExitUsage(err)
	}
	opts, err := writeConfig.ToClientOptions()
	if err != nil {
		return ExitUsage(err)
	}

	if !writeConfig.Force {
		if !confirmWrite(os.Stdin, writePrompt(writeConfig, area, plan)) {
			return ExitUsage(fmt.Errorf("write aborted by user"))
		}
	}

	c := client.New(writeConfig.IP, opts...)

	if err := c.Connect(cmd.Context()); err != nil {
		return ExitConnect(fmt.Errorf("connect failed: %w", err))
	}
	defer func() { _ = c.Close() }()

	if plan.str {
		if err := checkStringFits(cmd.Context(), c, area, writeConfig, len(plan.data)-1); err != nil {
			return err
		}
	}
	if plan.bit >= 0 {
		err = c.WriteBit(cmd.Context(), model.BitAddress{
			Area:       area,
			DBNumber:   writeConfig.DBNumber,
			ByteOffset: plan.start,
			BitOffset:  plan.bit,
		}, plan.bitValue)
	} else {
		err = c.WriteArea(cmd.Context(), model.Address{
			Area:     area,
			DBNumber: writeConfig.DBNumber,
			Start:    plan.start,
			Size:     len(plan.data),
		}, plan.data)
	}
	if err != nil {
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
		if plan.bit >= 0 {
			reqMeta["bit"] = plan.bit
		}
		if len(writeConfig.Value) <= maxValuePreview {
			reqMeta["value"] = writeConfig.Value
		} else {
			reqMeta["value"] = writeConfig.Value[:maxValuePreview] + "..."
			reqMeta["value_truncated"] = true
		}
		out := map[string]any{
			"target":  map[string]any{"ip": writeConfig.IP, "port": port},
			"request": reqMeta,
			"result":  map[string]any{"ok": true, "written": plan.size()},
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		_, _ = fmt.Fprintln(os.Stdout, string(b))
		return nil
	}

	if plan.bit >= 0 {
		_, _ = fmt.Fprintf(os.Stdout, "Successfully wrote 1 bit to %s\n", plan.address)
		return nil
	}
	_, _ = fmt.Fprintf(os.Stdout, "Successfully wrote %d bytes to %s%s\n", len(plan.data), plan.address, plan.noteSuffix())
	return nil
}

// maxValuePreview is how much of --value the JSON request and the prompt show.
const maxValuePreview = 128

// maxStringChars is the most characters an S7 STRING holds: its length bytes
// are one octet each, and 255 is not a valid maximum.
const maxStringChars = 254

// writePlan is what write sends: the octets of data from byte start, or, with
// bit 0-7, that one bit of byte start.
type writePlan struct {
	start    int
	data     []byte
	bit      int // -1 for a byte write
	bitValue bool
	address  string // the address written, as output names it
	note     string // what the octets are, when that is not plain
	str      bool   // --type string: the STRING's declared maximum is read first
}

// size is the number of octets written, 1 for a bit write.
func (p writePlan) size() int {
	if p.bit >= 0 {
		return 1
	}
	return len(p.data)
}

func (p writePlan) noteSuffix() string {
	if p.note == "" {
		return ""
	}
	return " (" + p.note + ")"
}

// planWrite turns the flags into what write sends. bitSet reports whether
// --bit was given.
func planWrite(cfg config.WriteConfig, area model.Area, bitSet bool) (writePlan, error) {
	typ := strings.ToLower(cfg.Type)
	if bitSet {
		if typ != "bool" {
			return writePlan{}, fmt.Errorf("--bit applies to --type bool only")
		}
		if cfg.Bit < 0 || cfg.Bit > 7 {
			return writePlan{}, fmt.Errorf("--bit must be 0-7, got %d", cfg.Bit)
		}
		v, err := strconv.ParseBool(cfg.Value)
		if err != nil {
			return writePlan{}, fmt.Errorf("invalid value: %w", err)
		}
		return writePlan{
			start:    cfg.Offset,
			bit:      cfg.Bit,
			bitValue: v,
			address:  formatBitAddress(area, cfg.DBNumber, cfg.Offset, cfg.Bit),
		}, nil
	}
	switch typ {
	case "bool":
		return writePlan{}, fmt.Errorf("--type bool requires --bit 0-7, the bit of the byte at --offset to write; --type raw writes a whole byte")
	case "string":
		if n := len(cfg.Value); n > maxStringChars {
			return writePlan{}, fmt.Errorf("the value has %d octets; an S7 STRING holds at most %d", n, maxStringChars)
		}
	}

	data, err := validate.ParseTypedValue(cfg.Value, cfg.Type)
	if err != nil {
		return writePlan{}, fmt.Errorf("invalid value: %w", err)
	}
	plan := writePlan{start: cfg.Offset, data: data, bit: -1}
	if typ == "string" {
		// Byte 0 of an S7 STRING is its maximum length, which the PLC program
		// declares. The write reads it, leaves it and writes the current length
		// (byte 1) and the characters. A negative offset is left for the client
		// to refuse.
		if cfg.Offset >= 0 {
			plan.start = cfg.Offset + 1
		}
		plan.data = data[1:]
		plan.note = "current length and characters of the STRING at " + formatWriteAddress(area, cfg.DBNumber, cfg.Offset)
		plan.str = true
	}
	plan.address = formatWriteAddress(area, cfg.DBNumber, plan.start)
	return plan, nil
}

// checkStringFits reads byte 0 of the STRING at cfg.Offset, the maximum length
// the PLC program declares, and refuses a value of n octets that is longer.
// A read that fails stops the write with its error.
func checkStringFits(ctx context.Context, c *client.Client, area model.Area, cfg config.WriteConfig, n int) error {
	at := formatWriteAddress(area, cfg.DBNumber, cfg.Offset)
	res, err := c.ReadArea(ctx, model.Address{Area: area, DBNumber: cfg.DBNumber, Start: cfg.Offset, Size: 1})
	code := codeProtocol
	if err == nil {
		code = readStatusCode(res.Status)
		err = res.Err()
	}
	if err != nil {
		return &ExitError{Code: classify(err, code), Err: fmt.Errorf("write failed: read the maximum length of the STRING at %s: %w", at, err)}
	}
	if limit := int(res.Data[0]); n > limit {
		return ExitUsage(fmt.Errorf("the value has %d octets and the STRING at %s holds at most %d (its byte 0); nothing was written", n, at, limit))
	}
	return nil
}

// formatWriteAddress returns the address of the byte at offset (e.g. DB1.DBB0,
// MB100, IB0, QB4), or the timer or counter (T20, C3).
func formatWriteAddress(area model.Area, dbNumber, offset int) string {
	switch area {
	case model.AreaDB:
		return fmt.Sprintf("DB%d.DBB%d", dbNumber, offset)
	case model.AreaMerkers:
		return fmt.Sprintf("MB%d", offset)
	case model.AreaInputs:
		return fmt.Sprintf("IB%d", offset)
	case model.AreaOutputs:
		return fmt.Sprintf("QB%d", offset)
	case model.AreaTimer:
		return fmt.Sprintf("T%d", offset)
	case model.AreaCounter:
		return fmt.Sprintf("C%d", offset)
	default:
		return fmt.Sprintf("%s %d", area.String(), offset)
	}
}

// formatBitAddress returns the address of bit bit of the byte at offset (e.g.
// DB1.DBX4.3, M10.0, I0.1, Q0.3).
func formatBitAddress(area model.Area, dbNumber, offset, bit int) string {
	switch area {
	case model.AreaDB:
		return fmt.Sprintf("DB%d.DBX%d.%d", dbNumber, offset, bit)
	case model.AreaMerkers, model.AreaInputs, model.AreaOutputs:
		return fmt.Sprintf("%s%d.%d", area.String(), offset, bit)
	default:
		return fmt.Sprintf("%s %d.%d", area.String(), offset, bit)
	}
}

// writePrompt is the confirmation question: what is written where, on which
// PLC and connection.
func writePrompt(cfg config.WriteConfig, area model.Area, plan writePlan) string {
	what := fmt.Sprintf("%d bytes", len(plan.data))
	if plan.bit >= 0 {
		what = "1 bit"
	}
	value := cfg.Value
	if len(value) > maxValuePreview {
		value = value[:maxValuePreview] + "..."
	}
	if strings.EqualFold(cfg.Type, "string") {
		value = strconv.Quote(value)
	}
	where := "area " + strings.ToUpper(cfg.Area)
	if area == model.AreaDB {
		where += fmt.Sprintf(", DB %d", cfg.DBNumber)
	}
	where += fmt.Sprintf(", offset %d", plan.start)
	if plan.bit >= 0 {
		where += fmt.Sprintf(", bit %d", plan.bit)
	}
	if plan.note != "" {
		where += "; " + plan.note
	}
	port := cfg.Port
	if port == 0 {
		port = 102
	}
	conn := fmt.Sprintf("rack %d slot %d", cfg.Rack, cfg.Slot)
	switch {
	case cfg.UseExplicitTSAP():
		conn = fmt.Sprintf("local TSAP %s, remote TSAP %s", cfg.LocalTSAP, cfg.RemoteTSAP)
	case cfg.AutoRack:
		conn = "rack and slot detected automatically"
	}
	return fmt.Sprintf("WARNING: this operation writes to PLC memory: %s (%s %s) to %s (%s) on %s port %d, %s. Type 'yes' to continue: ",
		what, strings.ToLower(cfg.Type), value, plan.address, where, cfg.IP, port, conn)
}

func confirmWrite(in io.Reader, prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "confirmation input unavailable")
		return false
	}
	return strings.EqualFold(strings.TrimSpace(line), "yes")
}
