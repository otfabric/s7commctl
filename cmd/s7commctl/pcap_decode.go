// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/otfabric/s7commctl/internal/app/config"
	"github.com/spf13/cobra"
)

type pcapDecodeConfig struct {
	File   string `flag:"file" env:"S7COMMCTL_PCAP_FILE" desc:"Path to .pcap file"`
	Format string `flag:"format" env:"S7COMMCTL_FORMAT" desc:"Output format: text, jsonl"`
	Port   int    `flag:"port" env:"S7COMMCTL_PORT" desc:"TCP port filter"`
}

var pdcConfig = pcapDecodeConfig{
	Format: "text",
	Port:   102,
}

var pcapDecodeCmd = &cobra.Command{
	Use:   "pcap-decode",
	Short: "Decode S7 frames from an offline pcap file",
	Long:  "Best-effort decoder for clean Ethernet/IPv4/TCP captures. Does not perform full TCP stream reassembly (no sequence, retransmit, or out-of-order handling).",
	RunE:  runPcapDecode,
}

func init() {
	config.RegisterFlags(pcapDecodeCmd, &pdcConfig)
	config.LoadFromEnv(&pdcConfig)
	_ = pcapDecodeCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(pcapDecodeCmd)
}

func runPcapDecode(cmd *cobra.Command, args []string) error {
	f, err := os.Open(filepath.Clean(pdcConfig.File))
	if err != nil {
		return ExitRuntime(fmt.Errorf("open pcap: %w", err))
	}
	defer func() { _ = f.Close() }()

	pr, err := newPCAPReader(f)
	if err != nil {
		return ExitRuntime(err)
	}
	if err := decodePCAP(pr, os.Stdout, pdcConfig.Format, pdcConfig.Port); err != nil {
		return ExitRuntime(err)
	}
	return nil
}

// decodePCAP decodes the TPKT frames of a capture while it reads it, one
// packet at a time, and writes one event per frame to w. Each direction of a
// connection is its own byte stream with its own tpktStream. port, when above
// 0, selects the packets from or to that TCP port.
func decodePCAP(pr *pcapReader, w io.Writer, format string, port int) error {
	streams := map[string]*tpktStream{}
	for {
		pkt, err := pr.next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		meta, payload, ok := parseEthernetIPv4TCP(pkt)
		if !ok {
			continue
		}
		if port > 0 && meta.SrcPort != port && meta.DstPort != port {
			continue
		}
		if len(payload) == 0 {
			continue
		}

		source := fmt.Sprintf("%s:%d>%s:%d", meta.SrcIP, meta.SrcPort, meta.DstIP, meta.DstPort)
		s := streams[source]
		if s == nil {
			s = &tpktStream{}
			streams[source] = s
		}
		s.write(payload)
		for {
			frame, ok := s.next()
			if !ok {
				break
			}
			summary, derr := inspectTPKTFrame(frame)
			if werr := writeFrameEvent(w, format, source, summary, derr); werr != nil {
				return werr
			}
		}
	}
}

// pcapMaxPacketLength is the largest captured length libpcap accepts (MAXIMUM_SNAPLEN).
const pcapMaxPacketLength = 262144

type tcpMeta struct {
	SrcIP   string
	DstIP   string
	SrcPort int
	DstPort int
}

// pcapReader reads the packets of a classic pcap file one at a time.
type pcapReader struct {
	r     io.Reader
	order binary.ByteOrder
	hdr   [16]byte
	buf   []byte // holds the packet next returned
}

// newPCAPReader reads the global header of a pcap file from r.
func newPCAPReader(r io.Reader) (*pcapReader, error) {
	gh := make([]byte, 24)
	if _, err := io.ReadFull(r, gh); err != nil {
		return nil, fmt.Errorf("read pcap global header: %w", err)
	}

	// The writer stores 0xa1b2c3d4 in its native order: read back little-endian,
	// it comes out as written for a little-endian file and swapped otherwise.
	magic := binary.LittleEndian.Uint32(gh[0:4])
	var order binary.ByteOrder
	switch magic {
	case 0xa1b2c3d4:
		order = binary.LittleEndian
	case 0xd4c3b2a1:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("unsupported pcap magic: 0x%08x", magic)
	}

	network := order.Uint32(gh[20:24])
	if network != 1 {
		return nil, fmt.Errorf("unsupported pcap linktype %d (need ethernet)", network)
	}
	return &pcapReader{r: r, order: order}, nil
}

// next returns the next packet with captured data, or io.EOF at the end of the
// capture; a capture that ends inside a packet header ends there too. The
// packet is valid until the following call: the reader reuses its buffer, so
// reading a capture holds one packet, at most pcapMaxPacketLength octets.
func (p *pcapReader) next() ([]byte, error) {
	for {
		_, err := io.ReadFull(p.r, p.hdr[:])
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, io.EOF
		}
		if err != nil {
			return nil, fmt.Errorf("read pcap packet header: %w", err)
		}
		incl := p.order.Uint32(p.hdr[8:12])
		if incl == 0 {
			continue
		}
		if incl > pcapMaxPacketLength {
			return nil, fmt.Errorf("pcap packet length %d exceeds %d", incl, pcapMaxPacketLength)
		}
		n := int(incl) // at most pcapMaxPacketLength
		if cap(p.buf) < n {
			p.buf = make([]byte, min(max(n, 2*cap(p.buf)), pcapMaxPacketLength))
		}
		pkt := p.buf[:n]
		if _, err := io.ReadFull(p.r, pkt); err != nil {
			return nil, fmt.Errorf("read pcap packet payload: %w", err)
		}
		return pkt, nil
	}
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
	// A fragment (more fragments set, or an offset) is not a whole TCP segment.
	if binary.BigEndian.Uint16(ip[6:8])&0x3FFF != 0 {
		return tcpMeta{}, nil, false
	}
	// The datagram ends where its total length says: what follows in the
	// captured frame (Ethernet padding, a trailer) is not TCP data. A total
	// length of 0 is what a capture of a segmentation-offload packet holds; the
	// captured length counts then.
	if totalLen := int(binary.BigEndian.Uint16(ip[2:4])); totalLen != 0 {
		if totalLen < ihl || totalLen > len(ip) {
			return tcpMeta{}, nil, false
		}
		ip = ip[:totalLen]
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

// popTPKTFrame pops one TPKT frame from buf.
// Returns (frame, remainder, true) or (nil, buf, false) if no complete frame.
// frame and remainder are parts of buf, not copies.
func popTPKTFrame(buf []byte) ([]byte, []byte, bool) {
	for len(buf) >= tpktHeaderLength {
		if buf[0] != tpktVersion {
			buf = buf[1:]
			continue
		}
		totalLen := int(binary.BigEndian.Uint16(buf[2:4]))
		if totalLen < tpktMinPacketLength || totalLen > tpktMaxPacketLength {
			buf = buf[1:]
			continue
		}
		if len(buf) < totalLen {
			return nil, buf, false
		}
		return buf[:totalLen:totalLen], buf[totalLen:], true
	}
	return nil, buf, false
}

// tpktStream reassembles the TPKT frames of one direction of a TCP
// connection. Its pending octets are buf[off:]: next consumes them by moving
// off, write drops the consumed ones once per packet before it appends. After
// next has returned every complete frame, fewer than tpktMaxPacketLength
// octets are pending (an incomplete frame), so the buffer holds at most one
// TPKT maximum plus one packet.
type tpktStream struct {
	buf []byte
	off int
}

// write appends the payload of one packet.
func (s *tpktStream) write(payload []byte) {
	if s.off > 0 {
		n := copy(s.buf, s.buf[s.off:])
		s.buf = s.buf[:n]
		s.off = 0
	}
	s.buf = append(s.buf, payload...)
}

// next returns the next complete frame. The frame is valid until the next
// write.
func (s *tpktStream) next() ([]byte, bool) {
	frame, rest, ok := popTPKTFrame(s.buf[s.off:])
	s.off = len(s.buf) - len(rest)
	return frame, ok
}
