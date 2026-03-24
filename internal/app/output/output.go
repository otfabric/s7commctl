// Package output provides CLI output formatters.
package output

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/otfabric/go-s7comm/client"
	"github.com/otfabric/go-s7comm/model"
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

// DeviceInfoResultMap returns the result object for identify (for use in target/request/result envelope).
func DeviceInfoResultMap(info *model.DeviceInfo) map[string]interface{} {
	if info == nil {
		return nil
	}
	return map[string]interface{}{
		"order_number":  strOrNull(info.OrderNumber),
		"serial_number": strOrNull(info.SerialNumber),
		"module_name":   strOrNull(info.ModuleName),
		"cpu_type":      strOrNull(info.CPUType),
		"fw_version":    strOrNull(info.FWVersion),
	}
}

func FormatDeviceInfo(info *model.DeviceInfo, format string) string {
	if info == nil {
		return ""
	}
	switch format {
	case FormatJSON:
		m := DeviceInfoResultMap(info)
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
		order, module, cpu, fw := info.OrderNumber, info.ModuleName, info.CPUType, info.FWVersion
		if order == "" {
			order = "N/A"
		}
		if module == "" {
			module = "N/A"
		}
		if cpu == "" {
			cpu = "N/A"
		}
		if fw == "" {
			fw = "N/A"
		}
		return fmt.Sprintf("Order: %s, Module: %s, CPU: %s, FW: %s\n", order, module, cpu, fw)
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
		out := make([]map[string]interface{}, 0, len(blocks))
		for _, b := range blocks {
			out = append(out, map[string]interface{}{
				"type":        b.Type.String(),
				"number":      b.Number,
				"language":    b.Language.String(),
				"flags":       b.Flags,
				"load_memory": b.LoadMemory,
				"local_data":  b.LocalData,
				"mc7_size":    b.MC7Size,
				"sbb_length":  b.SBBLength,
				"author":      strOrNull(b.Author),
				"family":      strOrNull(b.Family),
				"name":        strOrNull(b.Name),
				"version":     strOrNull(b.Version),
				"check_sum":   b.CheckSum,
			})
		}
		data, _ := json.MarshalIndent(out, "", "  ")
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

// FingerprintView is the CLI view of a device fingerprint. Uses WriteLikelyAllowed and UploadProbeOK
// instead of library naming to reflect inferred semantics (not direct capability probes).
type FingerprintView struct {
	CPUType            string
	CPUGeneration      int
	OrderNumber        string
	FirmwareVer        string
	ModuleName         string
	PDUSize            int
	MaxAmqCalling      int
	MaxAmqCalled       int
	DetectedRack       int
	DetectedSlot       int
	AcceptedTSAP       string
	Protection         string
	SupportsRead       bool
	WriteLikelyAllowed bool
	SupportsSZL        bool
	UploadProbeOK      bool
	SupportsPI         bool
}

// FingerprintResultMap returns the fingerprint result object (for use in target/request/result envelope).
func FingerprintResultMap(view *FingerprintView) map[string]interface{} {
	if view == nil {
		return nil
	}
	return map[string]interface{}{
		"cpu_type":             strOrNull(view.CPUType),
		"cpu_generation":       view.CPUGeneration,
		"order_number":         strOrNull(view.OrderNumber),
		"firmware_ver":         strOrNull(view.FirmwareVer),
		"module_name":          strOrNull(view.ModuleName),
		"pdu_size":             view.PDUSize,
		"max_amq_calling":      view.MaxAmqCalling,
		"max_amq_called":       view.MaxAmqCalled,
		"detected_rack":        view.DetectedRack,
		"detected_slot":        view.DetectedSlot,
		"accepted_tsap":        view.AcceptedTSAP,
		"protection":           view.Protection,
		"supports_read":        view.SupportsRead,
		"write_likely_allowed": view.WriteLikelyAllowed,
		"supports_szl":         view.SupportsSZL,
		"upload_probe_ok":      view.UploadProbeOK,
		"supports_pi":          view.SupportsPI,
	}
}

func FormatFingerprint(view *FingerprintView, format string) string {
	if view == nil {
		return "null"
	}
	switch format {
	case FormatJSON:
		m := FingerprintResultMap(view)
		data, _ := json.MarshalIndent(m, "", "  ")
		return string(data)
	case FormatTable:
		var sb strings.Builder
		sb.WriteString("┌─────────────────────────────────────────────────┐\n")
		sb.WriteString("│               S7 Device Fingerprint             │\n")
		sb.WriteString("├─────────────────┬───────────────────────────────┤\n")
		writeRow(&sb, "CPU Type", view.CPUType)
		writeRow(&sb, "Order Number", view.OrderNumber)
		writeRow(&sb, "Firmware", view.FirmwareVer)
		writeRow(&sb, "PDU Size", fmt.Sprintf("%d", view.PDUSize))
		writeRow(&sb, "Protection", view.Protection)
		sb.WriteString("└─────────────────┴───────────────────────────────┘\n")
		return sb.String()
	default:
		return fmt.Sprintf("CPU: %s, PDU: %d\n", view.CPUType, view.PDUSize)
	}
}

// DiagBufferResultMap returns the diagnostic buffer result object (for use in target/request/result envelope).
func DiagBufferResultMap(diag *model.DiagBuffer) map[string]interface{} {
	if diag == nil {
		return nil
	}
	entries := make([]map[string]interface{}, 0, len(diag.Entries))
	for _, e := range diag.Entries {
		entries = append(entries, map[string]interface{}{
			"timestamp":   strOrNull(e.Timestamp),
			"event_id":    e.EventID,
			"event_class": e.EventClass,
			"priority":    e.Priority,
			"description": strOrNull(e.Description),
			"info1":       e.Info1,
			"info2":       e.Info2,
		})
	}
	return map[string]interface{}{
		"total_count": diag.TotalCount,
		"entries":     entries,
	}
}

func FormatDiagBuffer(diag *model.DiagBuffer, format string) string {
	switch format {
	case FormatJSON:
		if diag == nil {
			return "null"
		}
		out := DiagBufferResultMap(diag)
		data, _ := json.MarshalIndent(out, "", "  ")
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
// ConnectionRequest, when set, is included as request.connection (e.g. from config.ConnectionRequestMeta()).
type ReadResultMeta struct {
	IP                string
	Port              int
	Rack              int
	Slot              int
	Area              string
	DBNumber          int
	Offset            int
	RequestedSize     int
	ConnectionRequest map[string]interface{} // optional; connection_mode and mode-specific params
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
		if result.Message != "" {
			statusDetail = status + " — " + result.Message
		}
		warnings := result.Warnings
		if warnings == nil {
			warnings = []string{}
		}
		res := map[string]interface{}{
			"status":           status,
			"status_detail":    statusDetail,
			"requested_length": result.RequestedLength,
			"returned_length":  result.ReturnedLength,
			"raw_hex":          hex.EncodeToString(data),
			"warnings":         warnings,
		}
		if result.Message != "" {
			res["message"] = result.Message
		}
		if len(data) > 0 && strings.ToLower(dataType) != "raw" {
			if decoded := decodeValuesForJSON(data, dataType); len(decoded) > 0 {
				res["decoded"] = decoded
			}
		}
		out := map[string]interface{}{"result": res}
		if meta != nil {
			target := map[string]interface{}{"ip": meta.IP, "port": meta.Port}
			request := make(map[string]interface{})
			if len(meta.ConnectionRequest) > 0 {
				request["connection"] = meta.ConnectionRequest
			}
			addr := map[string]interface{}{"area": meta.Area, "offset": meta.Offset, "size": meta.RequestedSize}
			if strings.ToUpper(meta.Area) == "DB" {
				addr["db"] = meta.DBNumber
			}
			request["address"] = addr
			out["target"] = target
			out["request"] = request
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
		if result.Message != "" {
			fmt.Fprintf(&sb, "Message: %s\n", result.Message)
		}
		if len(data) > 0 {
			sb.WriteString("\n")
			sb.WriteString(FormatReadData(data, dataType, format))
		}
		return sb.String()
	}
}

// decodeValuesForJSON returns decoded values for JSON result.decoded (one value per element for typed reads).
func decodeValuesForJSON(data []byte, dataType string) []interface{} {
	var out []interface{}
	switch strings.ToLower(dataType) {
	case "bool":
		for i := 0; i < len(data)*8 && i < 64; i++ {
			out = append(out, model.DecodeBool(data, i))
		}
	case "byte":
		for _, b := range data {
			out = append(out, int(b))
		}
	case "int":
		for i := 0; i+2 <= len(data); i += 2 {
			out = append(out, model.DecodeInt(data[i:i+2]))
		}
	case "word":
		for i := 0; i+2 <= len(data); i += 2 {
			out = append(out, model.DecodeWord(data[i:i+2]))
		}
	case "dint":
		for i := 0; i+4 <= len(data); i += 4 {
			out = append(out, model.DecodeDInt(data[i:i+4]))
		}
	case "dword":
		for i := 0; i+4 <= len(data); i += 4 {
			out = append(out, model.DecodeDWord(data[i:i+4]))
		}
	case "real":
		for i := 0; i+4 <= len(data); i += 4 {
			out = append(out, model.DecodeReal(data[i:i+4]))
		}
	case "string":
		if s := model.DecodeString(data); s != "" {
			out = append(out, s)
		}
	}
	return out
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
