// SPDX-License-Identifier: MIT

package s7commctl

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// readPCAPPackets reads every packet of a capture with a pcapReader, each
// packet copied out of the reader's buffer.
func readPCAPPackets(r io.Reader) ([][]byte, error) {
	pr, err := newPCAPReader(r)
	if err != nil {
		return nil, err
	}
	var packets [][]byte
	for {
		pkt, err := pr.next()
		if errors.Is(err, io.EOF) {
			return packets, nil
		}
		if err != nil {
			return nil, err
		}
		packets = append(packets, bytes.Clone(pkt))
	}
}

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

// TestPopTPKTFrame_ResynchronisesOnBadVersion: a capture can start inside a frame.
// Bytes behind a wrong version are noise, not a length to wait for.
func TestPopTPKTFrame_ResynchronisesOnBadVersion(t *testing.T) {
	// A plausible length behind a wrong version byte must not swallow the real frame.
	good := []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80}
	buf := append([]byte{0x09, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00}, good...)
	frame, rest, ok := popTPKTFrame(buf)
	if !ok || !bytes.Equal(frame, good) || len(rest) != 0 {
		t.Fatalf("frame=% X rest=% X ok=%v", frame, rest, ok)
	}
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

// padTo appends zero octets to pkt up to n, as a NIC pads a frame below the
// Ethernet minimum.
func padTo(pkt []byte, n int) []byte {
	return append(pkt, make([]byte, max(n-len(pkt), 0))...)
}

// TestParseEthernetIPv4TCP_PayloadEndsWithTheDatagram: octets after the IPv4
// total length are not TCP data.
func TestParseEthernetIPv4TCP_PayloadEndsWithTheDatagram(t *testing.T) {
	data := []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80}
	for _, tc := range []struct {
		name string
		pkt  []byte
		want []byte
	}{
		{"pure ACK padded to 60 octets", padTo(buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, nil), 60), nil},
		{"short segment padded to 60 octets", padTo(buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, data[:3]), 60), data[:3]},
		{"captured frame check sequence", append(buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, data), 0xDE, 0xAD, 0xBE, 0xEF), data},
		{"no trailer", buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, data), data},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, payload, ok := parseEthernetIPv4TCP(tc.pkt)
			if !ok {
				t.Fatal("not parsed")
			}
			if !bytes.Equal(payload, tc.want) {
				t.Fatalf("payload = % X, want % X", payload, tc.want)
			}
		})
	}
}

// TestParseEthernetIPv4TCP_TotalLength: a total length inside the headers or
// past the captured octets is refused; 0 (a segmentation-offload capture)
// leaves the captured length.
func TestParseEthernetIPv4TCP_TotalLength(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	valid := buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, data) // total length 44
	withTotal := func(n uint16) []byte {
		b := append([]byte(nil), valid...)
		binary.BigEndian.PutUint16(b[16:18], n)
		return b
	}
	for _, tc := range []struct {
		name  string
		total uint16
		ok    bool
		want  []byte
	}{
		{"zero: captured length", 0, true, data},
		{"one", 1, false, nil},
		{"below the IP header", 19, false, nil},
		{"IP header only", 20, false, nil}, // no room for the TCP header
		{"one octet of TCP header", 21, false, nil},
		{"headers only", 40, true, nil},
		{"one data octet", 41, true, data[:1]},
		{"exact", 44, true, data},
		{"one past the capture", 45, false, nil},
		{"maximum", 0xFFFF, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, payload, ok := parseEthernetIPv4TCP(withTotal(tc.total))
			if ok != tc.ok || !bytes.Equal(payload, tc.want) {
				t.Fatalf("payload = % X, ok = %v; want % X, %v", payload, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestParseEthernetIPv4TCP_Fragments: only a whole datagram is a TCP segment.
func TestParseEthernetIPv4TCP_Fragments(t *testing.T) {
	valid := buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, []byte{1, 2, 3, 4})
	withFlags := func(v uint16) []byte {
		b := append([]byte(nil), valid...)
		binary.BigEndian.PutUint16(b[20:22], v)
		return b
	}
	for _, tc := range []struct {
		name  string
		flags uint16
		ok    bool
	}{
		{"none", 0x0000, true},
		{"don't fragment", 0x4000, true},
		{"more fragments", 0x2000, false},
		{"offset 1", 0x0001, false},
		{"last fragment", 0x00B9, false},
		{"largest offset", 0x1FFF, false},
		{"don't fragment and an offset", 0x4001, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := parseEthernetIPv4TCP(withFlags(tc.flags)); ok != tc.ok {
				t.Fatalf("flags 0x%04X: ok = %v, want %v", tc.flags, ok, tc.ok)
			}
		})
	}
}

// TestPcapDecode_PaddedPacketInsideAFrame: a padded pure ACK between the two
// segments of a frame leaves the frame as it was sent.
func TestPcapDecode_PaddedPacketInsideAFrame(t *testing.T) {
	frame := s7ReadRequestFrame(9)
	for _, cut := range []int{5, 17} { // inside the COTP header; behind the S7 header
		path := writePCAP(t,
			buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, frame[:cut]),
			padTo(buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, nil), 60),
			buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, frame[cut:]),
		)
		r := runCLI(t, "pcap-decode", "--file", path).ok(t)
		if n := strings.Count(r.stdout, "\n"); n != 1 || !strings.Contains(r.stdout, "fn=0x04 (Read Var)") || strings.Contains(r.stdout, "error") {
			t.Fatalf("cut at %d: want one Read Var frame without error, got:\n%s", cut, r.stdout)
		}
	}
}

// TestPcapDecode_FramesBeforeADamagedPacketArePrinted: the capture is decoded
// as it is read, so what came before a damaged packet is reported.
func TestPcapDecode_FramesBeforeADamagedPacketArePrinted(t *testing.T) {
	blob := buildPCAP([][]byte{buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, s7ReadRequestFrame(9))})
	blob = append(blob, 0, 0, 0, 0, 0, 0, 0, 0, 0x40, 0, 0, 0, 0x40, 0, 0, 0, 1, 2, 3) // 64 octets announced, 3 present
	path := filepath.Join(t.TempDir(), "capture.pcap")
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		t.Fatal(err)
	}
	runCLI(t, "pcap-decode", "--file", path).wantFailure(t, "read pcap packet payload").has(t, "fn=0x04 (Read Var)")
}

// totalAlloc returns the octets fn allocates on the heap.
func totalAlloc(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestPcapDecode_AllocationIsBoundedByThePacket: decoding a capture holds one
// packet and one stream buffer, so what it allocates follows the largest
// packet, not the size of the file.
func TestPcapDecode_AllocationIsBoundedByThePacket(t *testing.T) {
	const payloadLen = 0xFFFF - 40 // the largest TCP payload an IPv4 total length allows
	const packets = 128
	frame := tpktFrame(append([]byte{0x02, 0xF0, 0x80}, make([]byte, tpktMaxPacketLength-7)...))
	stream := bytes.Repeat(frame, packets*payloadLen/len(frame)+1)
	pkts := make([][]byte, 0, packets)
	for i := range packets {
		pkts = append(pkts, buildEtherIPv4TCP(t, "10.0.0.1", "10.0.0.2", 45000, 102, stream[i*payloadLen:(i+1)*payloadLen]))
	}
	path := writePCAP(t, pkts...)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	var r cliResult
	got := totalAlloc(func() { r = runCLI(t, "pcap-decode", "--file", path, "--format", "jsonl") })
	r.ok(t)
	if n := strings.Count(r.stdout, "\n"); n != packets*payloadLen/len(frame) {
		t.Fatalf("decoded %d frames, want %d", n, packets*payloadLen/len(frame))
	}
	largest := uint64(len(pkts[0]))
	t.Logf("capture of %d octets: %d octets allocated", fi.Size(), got)
	if budget := 24 * largest; got > budget {
		t.Fatalf("decoding a %d-octet capture allocated %d octets; the budget is %d (24 packets of %d)", fi.Size(), got, budget, largest)
	}
}

// TestPopTPKTFrame_AllocationIsLinear: popping the frames of a segment does not
// copy what is left of it.
func TestPopTPKTFrame_AllocationIsLinear(t *testing.T) {
	small := []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80}
	segment := bytes.Repeat(small, 32*1024) // 224 KiB
	frames := 0
	got := totalAlloc(func() {
		buf := segment
		for {
			_, rest, ok := popTPKTFrame(buf)
			if !ok {
				break
			}
			frames++
			buf = rest
		}
	})
	t.Logf("%d frames popped: %d octets allocated", frames, got)
	if frames != 32*1024 {
		t.Fatalf("popped %d frames, want %d", frames, 32*1024)
	}
	if budget := uint64(2 * len(segment)); got > budget {
		t.Fatalf("popping %d frames of a %d-octet segment allocated %d octets, budget %d", frames, len(segment), got, budget)
	}
}

// TestTPKTStream_PendingIsBounded: after next has returned every complete
// frame, less than one TPKT maximum is pending, whatever the packets were.
func TestTPKTStream_PendingIsBounded(t *testing.T) {
	big := tpktFrame(append([]byte{0x02, 0xF0, 0x80}, make([]byte, tpktMaxPacketLength-7)...))
	small := []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80}
	var stream []byte
	for i := range 8 {
		stream = append(stream, big...)
		stream = append(stream, bytes.Repeat(small, i*100)...)
		stream = append(stream, 0xAA) // noise to resynchronise on
	}
	want := 8 + 100*(0+1+2+3+4+5+6+7)
	for _, size := range []int{1, 7, 1460, 65495, pcapMaxPacketLength} {
		t.Run("packets of "+strconv.Itoa(size), func(t *testing.T) {
			var s tpktStream
			frames := 0
			for off := 0; off < len(stream); off += size {
				payload := stream[off:min(off+size, len(stream))]
				s.write(payload)
				if n := len(s.buf) - s.off; n >= tpktMaxPacketLength+len(payload) {
					t.Fatalf("%d octets pending after a packet of %d", n, len(payload))
				}
				for {
					frame, ok := s.next()
					if !ok {
						break
					}
					if _, err := peelTPKT(frame); err != nil {
						t.Fatalf("frame %d: %v", frames, err)
					}
					frames++
				}
				if n := len(s.buf) - s.off; n >= tpktMaxPacketLength {
					t.Fatalf("%d octets pending after every frame was taken", n)
				}
			}
			if frames != want {
				t.Fatalf("packets of %d octets: %d frames, want %d", size, frames, want)
			}
		})
	}
}
