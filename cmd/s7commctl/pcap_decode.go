package s7commctl

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/spf13/cobra"
	"otfabric/s7comm/wire"
)

type pcapDecodeConfig struct {
	File   string `flag:"file" env:"S7COMMCTL_PCAP_FILE" desc:"Path to .pcap file"`
	Format string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: text, jsonl"`
	Port   int    `flag:"port" env:"S7COMMCTL_S7_PORT" desc:"TCP port filter"`
}

var pdcConfig = pcapDecodeConfig{
	Format: "text",
	Port:   102,
}

var pcapDecodeCmd = &cobra.Command{
	Use:   "pcap-decode",
	Short: "Decode S7 frames from an offline pcap file",
	RunE:  runPcapDecode,
}

func init() {
	config.RegisterFlags(pcapDecodeCmd, &pdcConfig)
	config.LoadFromEnv(&pdcConfig)
	_ = pcapDecodeCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(pcapDecodeCmd)
}

func runPcapDecode(cmd *cobra.Command, args []string) error {
	f, err := os.Open(pdcConfig.File)
	if err != nil {
		return fmt.Errorf("open pcap: %w", err)
	}
	defer func() { _ = f.Close() }()

	packets, err := readPCAPPackets(f)
	if err != nil {
		return err
	}

	buffers := map[string][]byte{}
	for _, pkt := range packets {
		meta, payload, ok := parseEthernetIPv4TCP(pkt)
		if !ok {
			continue
		}
		if pdcConfig.Port > 0 && meta.SrcPort != pdcConfig.Port && meta.DstPort != pdcConfig.Port {
			continue
		}
		if len(payload) == 0 {
			continue
		}

		flow := flowKey(meta.SrcIP, meta.SrcPort, meta.DstIP, meta.DstPort)
		buffers[flow] = append(buffers[flow], payload...)
		for {
			frame, rest, ok := popTPKTFrame(buffers[flow])
			if !ok {
				buffers[flow] = rest
				break
			}
			buffers[flow] = rest

			summary, derr := wire.InspectFrame(frame)
			source := fmt.Sprintf("%s:%d>%s:%d", meta.SrcIP, meta.SrcPort, meta.DstIP, meta.DstPort)
			if werr := writeFrameEvent(os.Stdout, pdcConfig.Format, source, summary, derr); werr != nil {
				return werr
			}
		}
	}

	return nil
}

type tcpMeta struct {
	SrcIP   string
	DstIP   string
	SrcPort int
	DstPort int
}

func readPCAPPackets(r io.Reader) ([][]byte, error) {
	gh := make([]byte, 24)
	if _, err := io.ReadFull(r, gh); err != nil {
		return nil, fmt.Errorf("read pcap global header: %w", err)
	}

	magic := binary.LittleEndian.Uint32(gh[0:4])
	var order binary.ByteOrder
	switch magic {
	case 0xa1b2c3d4:
		order = binary.BigEndian
	case 0xd4c3b2a1:
		order = binary.LittleEndian
	default:
		return nil, fmt.Errorf("unsupported pcap magic: 0x%08x", magic)
	}

	network := order.Uint32(gh[20:24])
	if network != 1 {
		return nil, fmt.Errorf("unsupported pcap linktype %d (need ethernet)", network)
	}

	var packets [][]byte
	for {
		ph := make([]byte, 16)
		_, err := io.ReadFull(r, ph)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read pcap packet header: %w", err)
		}
		incl := order.Uint32(ph[8:12])
		if incl == 0 {
			continue
		}
		pkt := make([]byte, incl)
		if _, err := io.ReadFull(r, pkt); err != nil {
			return nil, fmt.Errorf("read pcap packet payload: %w", err)
		}
		packets = append(packets, pkt)
	}

	return packets, nil
}

func parseEthernetIPv4TCP(packet []byte) (tcpMeta, []byte, bool) {
	if len(packet) < 14 {
		return tcpMeta{}, nil, false
	}
	etherType := binary.BigEndian.Uint16(packet[12:14])
	if etherType != 0x0800 {
		return tcpMeta{}, nil, false
	}

	ip := packet[14:]
	if len(ip) < 20 {
		return tcpMeta{}, nil, false
	}
	version := ip[0] >> 4
	if version != 4 {
		return tcpMeta{}, nil, false
	}
	ihl := int(ip[0]&0x0F) * 4
	if ihl < 20 || len(ip) < ihl {
		return tcpMeta{}, nil, false
	}
	if ip[9] != 6 {
		return tcpMeta{}, nil, false
	}

	tcp := ip[ihl:]
	if len(tcp) < 20 {
		return tcpMeta{}, nil, false
	}
	off := int((tcp[12] >> 4) * 4)
	if off < 20 || len(tcp) < off {
		return tcpMeta{}, nil, false
	}

	srcIP := net.IPv4(ip[12], ip[13], ip[14], ip[15]).String()
	dstIP := net.IPv4(ip[16], ip[17], ip[18], ip[19]).String()
	srcPort := int(binary.BigEndian.Uint16(tcp[0:2]))
	dstPort := int(binary.BigEndian.Uint16(tcp[2:4]))
	return tcpMeta{SrcIP: srcIP, DstIP: dstIP, SrcPort: srcPort, DstPort: dstPort}, tcp[off:], true
}

func flowKey(src string, srcPort int, dst string, dstPort int) string {
	k1 := fmt.Sprintf("%s:%d-%s:%d", src, srcPort, dst, dstPort)
	k2 := fmt.Sprintf("%s:%d-%s:%d", dst, dstPort, src, srcPort)
	if strings.Compare(k1, k2) <= 0 {
		return k1
	}
	return k2
}

func popTPKTFrame(buf []byte) ([]byte, []byte, bool) {
	for len(buf) >= 4 {
		if buf[0] != 3 {
			buf = buf[1:]
			continue
		}
		length := int(binary.BigEndian.Uint16(buf[2:4]))
		if length < 4 {
			buf = buf[1:]
			continue
		}
		if len(buf) < length {
			return nil, buf, false
		}
		frame := append([]byte(nil), buf[:length]...)
		return frame, bytes.Clone(buf[length:]), true
	}
	return nil, buf, false
}
