// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPopTPKTFrame(t *testing.T) {
	buf := []byte{
		0x00,
		0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80,
		0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80,
	}

	f1, rest, ok := popTPKTFrame(buf)
	if !ok {
		t.Fatal("expected first frame")
	}
	if len(f1) != 7 || f1[0] != 0x03 {
		t.Fatalf("unexpected first frame: % X", f1)
	}

	f2, rest2, ok := popTPKTFrame(rest)
	if !ok {
		t.Fatal("expected second frame")
	}
	if len(f2) != 7 || f2[0] != 0x03 {
		t.Fatalf("unexpected second frame: % X", f2)
	}
	if len(rest2) != 0 {
		t.Fatalf("expected empty rest, got %d", len(rest2))
	}
}

func TestParseEthernetIPv4TCP(t *testing.T) {
	payload := []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80}
	pkt := buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, payload)

	meta, got, ok := parseEthernetIPv4TCP(pkt)
	if !ok {
		t.Fatal("expected parse success")
	}
	if meta.SrcIP != "10.0.0.1" || meta.DstIP != "10.0.0.2" {
		t.Fatalf("unexpected meta ips: %+v", meta)
	}
	if meta.SrcPort != 45000 || meta.DstPort != 102 {
		t.Fatalf("unexpected meta ports: %+v", meta)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("unexpected payload: % X", got)
	}
}

func TestReadPCAPPackets(t *testing.T) {
	pkt1 := buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 1234, 102, []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80})
	pkt2 := buildEtherIPv4TCP(t, "10.0.0.2", "10.0.0.1", 102, 1234, []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80})
	blob := buildPCAP([][]byte{pkt1, pkt2})

	packets, err := readPCAPPackets(bytes.NewReader(blob))
	if err != nil {
		t.Fatalf("readPCAPPackets failed: %v", err)
	}
	if len(packets) != 2 {
		t.Fatalf("expected 2 packets, got %d", len(packets))
	}
}

// TestReadPCAPPackets_ByteOrder uses the bytes a capture tool writes: the magic
// 0xa1b2c3d4 in the writer's native order, so d4 c3 b2 a1 on little-endian hosts.
func TestReadPCAPPackets_ByteOrder(t *testing.T) {
	frame := []byte{0xAA, 0xBB, 0xCC}
	for _, tc := range []struct {
		name  string
		magic []byte
		order binary.ByteOrder
	}{
		{"little-endian capture", []byte{0xD4, 0xC3, 0xB2, 0xA1}, binary.LittleEndian},
		{"big-endian capture", []byte{0xA1, 0xB2, 0xC3, 0xD4}, binary.BigEndian},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			_, _ = b.Write(tc.magic)
			_, _ = b.Write(make([]byte, 16)) // version, thiszone, sigfigs, snaplen
			_ = binary.Write(&b, tc.order, uint32(1))
			_, _ = b.Write(make([]byte, 8)) // timestamp
			_ = binary.Write(&b, tc.order, uint32(len(frame)))
			_ = binary.Write(&b, tc.order, uint32(len(frame)))
			_, _ = b.Write(frame)

			packets, err := readPCAPPackets(&b)
			if err != nil {
				t.Fatalf("readPCAPPackets: %v", err)
			}
			if len(packets) != 1 || !bytes.Equal(packets[0], frame) {
				t.Fatalf("packets = % X, want one frame % X", packets, frame)
			}
		})
	}
}

// TestReadPCAPPackets_OversizedPacket: the length field of a packet header is
// untrusted input and must be refused before it sizes an allocation.
func TestReadPCAPPackets_OversizedPacket(t *testing.T) {
	blob := buildPCAP(nil)
	hdr := make([]byte, 16)
	binary.LittleEndian.PutUint32(hdr[8:12], 0xFFFFFFFF)
	blob = append(blob, hdr...)

	_, err := readPCAPPackets(bytes.NewReader(blob))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v, want a length error", err)
	}
}

// TestPcapDecode_ReassemblesAcrossPackets: a frame split over two segments survives
// a response and another flow arriving between them.
func TestPcapDecode_ReassemblesAcrossPackets(t *testing.T) {
	frame := s7ReadRequestFrame(9)
	path := writePCAP(t,
		buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, frame[:5]),
		buildEtherIPv4TCP(t, "10.0.0.2", "10.0.0.1", 102, 45000, frame), // other direction, own TCP stream
		buildEtherIPv4TCP(t, "10.0.0.3", "10.0.0.2", 45000, 102, frame), // other flow
		buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, frame[5:]),
	)
	r := runCLI(t, "pcap-decode", "--file", path, "--format", "jsonl").ok(t)
	if n := strings.Count(r.stdout, `"cotp_type":"DT"`); n != 3 {
		t.Fatalf("got %d frames, want all 3 decoded:\n%s", n, r.stdout)
	}
	if strings.Contains(r.stdout, `"error"`) {
		t.Fatalf("a frame failed to decode:\n%s", r.stdout)
	}
}

func writePCAP(t *testing.T, pkts ...[]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.pcap")
	if err := os.WriteFile(path, buildPCAP(pkts), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildPCAP(pkts [][]byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, uint32(0xa1b2c3d4))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(4))
	_ = binary.Write(&b, binary.LittleEndian, int32(0))
	_ = binary.Write(&b, binary.LittleEndian, uint32(0))
	_ = binary.Write(&b, binary.LittleEndian, uint32(65535))
	_ = binary.Write(&b, binary.LittleEndian, uint32(1)) // ethernet

	for _, p := range pkts {
		_ = binary.Write(&b, binary.LittleEndian, uint32(0))
		_ = binary.Write(&b, binary.LittleEndian, uint32(0))
		_ = binary.Write(&b, binary.LittleEndian, uint32(len(p)))
		_ = binary.Write(&b, binary.LittleEndian, uint32(len(p)))
		_, _ = b.Write(p)
	}

	return b.Bytes()
}

func buildEtherIPv4TCP(t *testing.T, srcIP, dstIP string, srcPort, dstPort int, payload []byte) []byte {
	t.Helper()
	eth := make([]byte, 14)
	eth[12] = 0x08
	eth[13] = 0x00

	ip := make([]byte, 20)
	ip[0] = 0x45
	totalLen := 20 + 20 + len(payload)
	binary.BigEndian.PutUint16(ip[2:4], uint16(totalLen))
	ip[8] = 64
	ip[9] = 6
	src := net.ParseIP(srcIP).To4()
	dst := net.ParseIP(dstIP).To4()
	if src == nil || dst == nil {
		t.Fatalf("buildEtherIPv4TCP requires IPv4 addresses: src=%q dst=%q", srcIP, dstIP)
	}
	copy(ip[12:16], src)
	copy(ip[16:20], dst)

	tcp := make([]byte, 20)
	binary.BigEndian.PutUint16(tcp[0:2], uint16(srcPort))
	binary.BigEndian.PutUint16(tcp[2:4], uint16(dstPort))
	tcp[12] = 0x50

	out := append(eth, ip...)
	out = append(out, tcp...)
	out = append(out, payload...)
	return out
}
