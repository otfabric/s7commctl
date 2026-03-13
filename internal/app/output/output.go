// Package output provides CLI output formatters.
package output

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otfabric/s7comm/client"
	"github.com/otfabric/s7comm/model"
)

const (
	FormatJSON  = "json"
	FormatTable = "table"
	FormatText  = "text"
	FormatHex   = "hex"
)

func PrintJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func FormatDeviceInfo(info *model.DeviceInfo, format string) string {
	if info == nil {
		return ""
	}
	switch format {
	case FormatJSON:
		// JSON: use null for unavailable/empty fields (REQUIREMENTS #18)
		m := map[string]interface{}{
			"order_number":  strOrNull(info.OrderNumber),
			"serial_number": strOrNull(info.SerialNumber),
			"module_name":   strOrNull(info.ModuleName),
			"cpu_type":      strOrNull(info.CPUType),
			"fw_version":    strOrNull(info.FWVersion),
		}
		data, _ := json.MarshalIndent(m, "", "  ")
		return string(data)
	case FormatTable:
		var sb strings.Builder
		sb.WriteString("┌─────────────────────────────────────────────────┐\n")
		sb.WriteString("│               S7 Device Information             │\n")
		sb.WriteString("├─────────────────┬───────────────────────────────┤\n")
		writeRowNA(&sb, "Order Number", info.OrderNumber)
		writeRowNA(&sb, "Serial Number", info.SerialNumber)
		writeRowNA(&sb, "Module Name", info.ModuleName)
		writeRowNA(&sb, "CPU Type", info.CPUType)
		writeRowNA(&sb, "FW Version", info.FWVersion)
		sb.WriteString("└─────────────────┴───────────────────────────────┘\n")
		return sb.String()
	default:
		order, module := info.OrderNumber, info.ModuleName
		if order == "" {
			order = "N/A"
		}
		if module == "" {
			module = "N/A"
		}
		return fmt.Sprintf("Order: %s, Module: %s\n", order, module)
	}
}

// strOrNull returns nil for empty s so JSON marshals as null.
func strOrNull(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// writeRowNA uses "N/A" for empty values (identify plain/table).
func writeRowNA(sb *strings.Builder, key, value string) {
	if value == "" {
		value = "N/A"
	}
	if len(value) > 29 {
		value = value[:26] + "..."
	}
	_, _ = fmt.Fprintf(sb, "│ %-15s │ %-29s │\n", key, value)
}

func writeRow(sb *strings.Builder, key, value string) {
	if value == "" {
		value = "-"
	}
	if len(value) > 29 {
		value = value[:26] + "..."
	}
	_, _ = fmt.Fprintf(sb, "│ %-15s │ %-29s │\n", key, value)
}

func FormatBlocks(blocks []model.BlockInfo, format string) string {
	switch format {
	case FormatJSON:
		data, _ := json.MarshalIndent(blocks, "", "  ")
		return string(data)
	case FormatTable:
		var sb strings.Builder
		sb.WriteString("┌──────┬────────┬──────────┬───────┐\n")
		sb.WriteString("│ Type │ Number │ Language │ Flags │\n")
		sb.WriteString("├──────┼────────┼──────────┼───────┤\n")
		for _, b := range blocks {
			_, _ = fmt.Fprintf(&sb, "│ %-4s │ %6d │ %-8s │ 0x%02X  │\n",
				b.Type.String(), b.Number, b.Language.String(), b.Flags)
		}
		sb.WriteString("└──────┴────────┴──────────┴───────┘\n")
		return sb.String()
	default:
		var sb strings.Builder
		for _, b := range blocks {
			_, _ = fmt.Fprintf(&sb, "%s%d\n", b.Type.String(), b.Number)
		}
		return sb.String()
	}
}

func FormatFingerprint(fp *model.Fingerprint, format string) string {
	switch format {
	case FormatJSON:
		data, _ := json.MarshalIndent(fp, "", "  ")
		return string(data)
	case FormatTable:
		var sb strings.Builder
		sb.WriteString("┌─────────────────────────────────────────────────┐\n")
		sb.WriteString("│               S7 Device Fingerprint             │\n")
		sb.WriteString("├─────────────────┬───────────────────────────────┤\n")
		writeRow(&sb, "CPU Type", fp.CPUType)
		writeRow(&sb, "Order Number", fp.OrderNumber)
		writeRow(&sb, "Firmware", fp.FirmwareVer)
		writeRow(&sb, "PDU Size", fmt.Sprintf("%d", fp.PDUSize))
		writeRow(&sb, "Protection", fp.Protection.String())
		sb.WriteString("└─────────────────┴───────────────────────────────┘\n")
		return sb.String()
	default:
		return fmt.Sprintf("CPU: %s, PDU: %d\n", fp.CPUType, fp.PDUSize)
	}
}

func FormatDiagBuffer(diag *model.DiagBuffer, format string) string {
	switch format {
	case FormatJSON:
		data, _ := json.MarshalIndent(diag, "", "  ")
		return string(data)
	case FormatTable:
		var sb strings.Builder
		sb.WriteString("┌──────────┬───────┬──────────┐\n")
		sb.WriteString("│ Event ID │ Class │ Priority │\n")
		sb.WriteString("├──────────┼───────┼──────────┤\n")
		for _, e := range diag.Entries {
			_, _ = fmt.Fprintf(&sb, "│ 0x%04X   │ 0x%02X  │ %8d │\n", e.EventID, e.EventClass, e.Priority)
		}
		sb.WriteString("└──────────┴───────┴──────────┘\n")
		return sb.String()
	default:
		return fmt.Sprintf("%d entries\n", diag.TotalCount)
	}
}

func FormatHexDump(data []byte) string {
	var sb strings.Builder
	for i := 0; i < len(data); i += 16 {
		_, _ = fmt.Fprintf(&sb, "%08X  ", i)
		for j := 0; j < 16; j++ {
			if i+j < len(data) {
				_, _ = fmt.Fprintf(&sb, "%02X ", data[i+j])
			} else {
				sb.WriteString("   ")
			}
			if j == 7 {
				sb.WriteString(" ")
			}
		}
		sb.WriteString(" |")
		for j := 0; j < 16 && i+j < len(data); j++ {
			b := data[i+j]
			if b >= 0x20 && b < 0x7f {
				sb.WriteByte(b)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}
	return sb.String()
}

// ReadResultMeta is optional metadata for FormatReadResult (target/request context).
type ReadResultMeta struct {
	IP            string
	Port          int
	Rack          int
	Slot          int
	Area          string
	DBNumber      int
	Offset        int
	RequestedSize int
}

// FormatReadResult formats a ReadResult with status, requested/returned length, warnings, and data.
func FormatReadResult(result *client.ReadResult, dataType, format string, meta *ReadResultMeta) string {
	if result == nil {
		return "Result: (no result)\n"
	}
	data := result.Data
	status := string(result.Status)
	switch format {
	case FormatJSON:
		statusDetail := status
		if result.Error != "" {
			statusDetail = status + " — " + result.Error
		}
		res := map[string]interface{}{
			"status":           status,
			"status_detail":    statusDetail,
			"requested_length": result.RequestedLength,
			"returned_length":  result.ReturnedLength,
			"raw_hex":          hex.EncodeToString(data),
			"warnings":         result.Warnings,
		}
		if result.Error != "" {
			res["error"] = result.Error
		}
		out := map[string]interface{}{"result": res}
		if meta != nil {
			target := map[string]interface{}{
				"ip":   meta.IP,
				"port": meta.Port,
				"rack": meta.Rack,
				"slot": meta.Slot,
				"area": meta.Area,
			}
			if strings.ToUpper(meta.Area) == "DB" {
				target["db"] = meta.DBNumber
			}
			out["target"] = target
			out["request"] = map[string]interface{}{"offset": meta.Offset, "size": meta.RequestedSize}
		}
		d, _ := json.MarshalIndent(out, "", "  ")
		return string(d)
	default:
		var sb strings.Builder
		// One-line status summary first (REQUIREMENTS: helps humans and pasted logs)
		if meta != nil {
			fmt.Fprintf(&sb, "status=%s requested=%d returned=%d area=%s offset=%d\n\n",
				status, result.RequestedLength, result.ReturnedLength, meta.Area, meta.Offset)
		}
		if meta != nil {
			fmt.Fprintf(&sb, "Area: %s", meta.Area)
			if meta.Area == "DB" {
				fmt.Fprintf(&sb, " DB%d", meta.DBNumber)
			}
			fmt.Fprintf(&sb, "\nOffset: %d\nRequested: %d bytes\n\n", meta.Offset, meta.RequestedSize)
		}
		fmt.Fprintf(&sb, "Result: %s\nReturned: %d bytes\n", status, result.ReturnedLength)
		for _, w := range result.Warnings {
			fmt.Fprintf(&sb, "Warning: %s\n", w)
		}
		if result.Error != "" {
			fmt.Fprintf(&sb, "Error: %s\n", result.Error)
		}
		if len(data) > 0 {
			sb.WriteString("\n")
			sb.WriteString(FormatReadData(data, dataType, format))
		}
		return sb.String()
	}
}

func FormatReadData(data []byte, dataType, format string) string {
	// Interpret data based on type
	var interpreted string
	switch strings.ToLower(dataType) {
	case "bool":
		var vals []string
		for i := 0; i < len(data)*8; i++ {
			vals = append(vals, fmt.Sprintf("bit %d: %v", i, model.DecodeBool(data, i)))
		}
		interpreted = strings.Join(vals, "\n")
	case "byte":
		var vals []string
		for i, b := range data {
			vals = append(vals, fmt.Sprintf("[%d]: %d (0x%02X)", i, b, b))
		}
		interpreted = strings.Join(vals, "\n")
	case "int":
		var vals []string
		for i := 0; i+2 <= len(data); i += 2 {
			vals = append(vals, fmt.Sprintf("[%d]: %d", i/2, model.DecodeInt(data[i:i+2])))
		}
		interpreted = strings.Join(vals, "\n")
	case "word":
		var vals []string
		for i := 0; i+2 <= len(data); i += 2 {
			vals = append(vals, fmt.Sprintf("[%d]: %d (0x%04X)", i/2, model.DecodeWord(data[i:i+2]), model.DecodeWord(data[i:i+2])))
		}
		interpreted = strings.Join(vals, "\n")
	case "dint":
		var vals []string
		for i := 0; i+4 <= len(data); i += 4 {
			vals = append(vals, fmt.Sprintf("[%d]: %d", i/4, model.DecodeDInt(data[i:i+4])))
		}
		interpreted = strings.Join(vals, "\n")
	case "dword":
		var vals []string
		for i := 0; i+4 <= len(data); i += 4 {
			vals = append(vals, fmt.Sprintf("[%d]: %d (0x%08X)", i/4, model.DecodeDWord(data[i:i+4]), model.DecodeDWord(data[i:i+4])))
		}
		interpreted = strings.Join(vals, "\n")
	case "real":
		var vals []string
		for i := 0; i+4 <= len(data); i += 4 {
			vals = append(vals, fmt.Sprintf("[%d]: %g", i/4, model.DecodeReal(data[i:i+4])))
		}
		interpreted = strings.Join(vals, "\n")
	case "string":
		interpreted = fmt.Sprintf("%q", model.DecodeString(data))
	default: // raw
		interpreted = ""
	}

	switch format {
	case FormatJSON:
		result := map[string]interface{}{
			"raw_hex": hex.EncodeToString(data),
			"length":  len(data),
		}
		if interpreted != "" {
			result["values"] = strings.Split(interpreted, "\n")
		}
		d, _ := json.MarshalIndent(result, "", "  ")
		return string(d)
	case FormatHex:
		if interpreted != "" {
			return FormatHexDump(data) + "\nInterpreted as " + dataType + ":\n" + interpreted + "\n"
		}
		return FormatHexDump(data)
	default:
		if interpreted != "" {
			return interpreted + "\n"
		}
		return FormatHexDump(data)
	}
}
