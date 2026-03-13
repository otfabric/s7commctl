package s7commctl

import (
	"bytes"
	"encoding/binary"
	"net"
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

func buildPCAP(pkts [][]byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, uint32(0xd4c3b2a1))
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
