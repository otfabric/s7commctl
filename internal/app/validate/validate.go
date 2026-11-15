// SPDX-License-Identifier: MIT

// Package validate provides input validation for CLI.
package validate

import (
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/otfabric/go-s7comm/model"
	"github.com/otfabric/go-s7comm/wire"
)

func ValidateIP(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP address is required")
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}

// IsPublicIP reports whether an IP is globally routable (not RFC1918/loopback/link-local).
func IsPublicIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() || parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast() {
		return false
	}

	v4 := parsed.To4()
	if v4 == nil {
		return !parsed.IsPrivate()
	}

	if v4[0] == 10 {
		return false
	}
	if v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31 {
		return false
	}
	if v4[0] == 192 && v4[1] == 168 {
		return false
	}

	return true
}

// nonPublicPrefixes are the ranges IsPublicIP reports as not public.
var nonPublicPrefixes = func() []netip.Prefix {
	out := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("224.0.0.0/24"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("fc00::/7"),
	}
	for flags := range 16 { // link-local multicast: ffX2::/16
		out = append(out, netip.PrefixFrom(netip.AddrFrom16([16]byte{0xff, byte(flags<<4 | 2)}), 16))
	}
	return out
}()

// CIDRHasPublicIP reports whether the range cidr holds an address IsPublicIP
// reports as public. It is false for a range ValidateCIDR refuses.
func CIDRHasPublicIP(cidr string) bool {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	addr, _ := netip.AddrFromSlice(n.IP)
	bits, _ := n.Mask.Size()
	if addr.Is4In6() && bits >= 96 {
		addr, bits = addr.Unmap(), bits-96
	}
	p := netip.PrefixFrom(addr, bits)
	for _, np := range nonPublicPrefixes {
		if np.Bits() <= p.Bits() && np.Contains(p.Addr()) {
			return false
		}
	}
	return true
}

func ValidateCIDR(cidr string) error {
	if cidr == "" {
		return fmt.Errorf("CIDR range is required")
	}
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %s", cidr)
	}
	return nil
}

func ValidateRackSlot(rack, slot int) error {
	if rack < 0 || rack > 7 {
		return fmt.Errorf("rack must be 0-7")
	}
	if slot < 0 || slot > 31 {
		return fmt.Errorf("slot must be 0-31")
	}
	return nil
}

func ValidateArea(area string) (model.Area, error) {
	switch strings.ToUpper(area) {
	case "I", "INPUT", "INPUTS":
		return model.AreaInputs, nil
	case "Q", "OUTPUT", "OUTPUTS":
		return model.AreaOutputs, nil
	case "M", "MERKER", "MERKERS":
		return model.AreaMerkers, nil
	case "DB":
		return model.AreaDB, nil
	case "C", "COUNTER":
		return model.AreaCounter, nil
	case "T", "TIMER":
		return model.AreaTimer, nil
	case "DATA_RECORD", "DR":
		return model.Area(wire.AreaDataRecord), nil
	case "S7200AO":
		return model.Area(wire.AreaS7200AO), nil
	case "IEC_COUNTER_200", "IEC_C200":
		return model.Area(wire.AreaIECCounter200), nil
	case "IEC_TIMER_200", "IEC_T200":
		return model.Area(wire.AreaIECTimer200), nil
	default:
		return 0, fmt.Errorf("unknown area: %s", area)
	}
}

func ValidateBlockType(bt string) (model.BlockType, error) {
	switch strings.ToUpper(bt) {
	case "OB":
		return model.BlockOB, nil
	case "DB":
		return model.BlockDB, nil
	case "FC":
		return model.BlockFC, nil
	case "FB":
		return model.BlockFB, nil
	case "SFC":
		return model.BlockSFC, nil
	case "SFB":
		return model.BlockSFB, nil
	default:
		return 0, fmt.Errorf("unknown block type: %s", bt)
	}
}

func ParseHexBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.ReplaceAll(s, " ", "")
	if len(s)%2 != 0 {
		s = "0" + s
	}
	if !regexp.MustCompile(`^[0-9a-fA-F]*$`).MatchString(s) {
		return nil, fmt.Errorf("invalid hex")
	}
	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		val, _ := strconv.ParseUint(s[i:i+2], 16, 8)
		result[i/2] = byte(val)
	}
	return result, nil
}

func ParseTypedValue(value, dataType string) ([]byte, error) {
	switch strings.ToLower(dataType) {
	case "raw":
		return ParseHexBytes(value)
	case "bool":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		if b {
			return []byte{1}, nil
		}
		return []byte{0}, nil
	case "int":
		v, err := strconv.ParseInt(value, 0, 16)
		if err != nil {
			return nil, err
		}
		return model.EncodeInt(int16(v)), nil
	case "dint":
		v, err := strconv.ParseInt(value, 0, 32)
		if err != nil {
			return nil, err
		}
		return model.EncodeDInt(int32(v)), nil
	case "real":
		v, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, err
		}
		return model.EncodeReal(float32(v)), nil
	case "string":
		return model.EncodeString(value, len(value)+2), nil
	default:
		return nil, fmt.Errorf("unsupported type: %s", dataType)
	}
}
