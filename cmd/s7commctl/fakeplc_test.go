// SPDX-License-Identifier: MIT

package s7commctl

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/otfabric/go-s7comm/wire"
)

// admission is how the fake PLC treats a connection to one rack/slot.
type admission int

const (
	admitOK      admission = iota // COTP and S7 setup succeed
	admitNoCOTP                   // TCP only: the connection closes on the COTP CR
	admitNoSetup                  // COTP only: S7 setup is refused with a header error
)

// fakeWrite is one Write Var the fake PLC received.
type fakeWrite struct {
	area  byte
	db    int
	start int
	data  []byte
}

// fakeReply is the outcome of a Read Var, an SZL read or a Write Var.
type fakeReply struct {
	ret  byte // item return code; 0 means wire.RetCodeSuccess
	data []byte
}

// fakePLC is an in-process S7 server on a loopback socket. go-s7comm keeps its
// fake in test files, so this one speaks TPKT, COTP class 0 and S7 itself and
// the real client runs against it. Behaviour fields are set before the first
// connection; a nil func selects the default device.
type fakePLC struct {
	ln   net.Listener
	port string
	wg   sync.WaitGroup

	admit    func(rack, slot int) admission
	read     func(area byte, db, start, size int) fakeReply
	szl      func(id, index uint16) fakeReply
	writeRet byte
	upload   [][]byte // chunks of the block; nil refuses the upload

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	writes []fakeWrite
	dials  int
}

// newFakePLC starts a fake PLC that is stopped when the test ends.
func newFakePLC(t *testing.T) *fakePLC {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	p := &fakePLC{ln: ln, conns: map[net.Conn]struct{}{}}
	p.port = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	p.wg.Add(1)
	go p.acceptLoop()
	t.Cleanup(p.stop)
	return p
}

func (p *fakePLC) stop() {
	_ = p.ln.Close()
	p.mu.Lock()
	for c := range p.conns {
		_ = c.Close()
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// args returns the arguments that point command at this PLC.
func (p *fakePLC) args(command ...string) []string {
	return append(command, "--ip", "127.0.0.1", "--port", p.port, "--timeout", "2000")
}

func (p *fakePLC) recordedWrites() []fakeWrite {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]fakeWrite(nil), p.writes...)
}

func (p *fakePLC) acceptLoop() {
	defer p.wg.Done()
	for {
		conn, err := p.ln.Accept()
		if err != nil {
			return
		}
		p.mu.Lock()
		p.conns[conn] = struct{}{}
		p.dials++
		p.mu.Unlock()
		p.wg.Go(func() {
			defer func() {
				_ = conn.Close()
				p.mu.Lock()
				delete(p.conns, conn)
				p.mu.Unlock()
			}()
			p.serve(conn)
		})
	}
}

func (p *fakePLC) serve(conn net.Conn) {
	cr, err := readRawTPKT(conn)
	if err != nil {
		return
	}
	srcRef, rack, slot, ok := parseCR(cr)
	if !ok {
		return
	}
	adm := admitOK
	if p.admit != nil {
		adm = p.admit(rack, slot)
	}
	if adm == admitNoCOTP {
		return
	}
	if err := writeTPKT(conn, connectionConfirm(srcRef)); err != nil {
		return
	}

	uploadPos := 0
	for {
		tsdu, err := readTSDU(conn)
		if err != nil {
			return
		}
		h, rest, err := wire.ParseS7Header(tsdu)
		if err != nil {
			return
		}
		param, data := rest[:h.ParamLength], rest[h.ParamLength:]
		var resp []byte
		switch {
		case h.ROSCTR.IsUserdata():
			resp = p.handleUserdata(h.PDURef, param, data)
		case len(param) == 0:
			return
		case param[0] == wire.FuncSetupComm:
			if adm == admitNoSetup {
				resp = ackData(h.PDURef, 0x81, 0x04, nil, nil)
				break
			}
			setup := []byte{wire.FuncSetupComm, 0, 0, 1, 0, 1, 0, 0}
			binary.BigEndian.PutUint16(setup[6:8], 240)
			resp = ackData(h.PDURef, 0, 0, setup, nil)
		case param[0] == wire.FuncReadVar:
			resp = p.handleRead(h.PDURef, param)
		case param[0] == wire.FuncWriteVar:
			resp = p.handleWrite(h.PDURef, param, data)
		case param[0] == wire.FuncUploadStart:
			if p.upload == nil {
				resp = ackData(h.PDURef, 0x81, 0x04, nil, nil)
				break
			}
			// As a CPU answers: status, 2 bytes, upload ID 7, block length in ASCII.
			resp = ackData(h.PDURef, 0, 0, []byte{wire.FuncUploadStart, 0, 1, 0, 0, 0, 0, 7, 3, '2', '1', '6'}, nil)
		case param[0] == wire.FuncUpload:
			chunk := p.upload[uploadPos]
			uploadPos++
			more := byte(1)
			if uploadPos == len(p.upload) {
				more = 0
			}
			body := []byte{0, 0, 0x00, 0xFB} // segment length in bytes, then two bytes a CPU sends
			binary.BigEndian.PutUint16(body[0:2], uint16(len(chunk)))
			resp = ackData(h.PDURef, 0, 0, []byte{wire.FuncUpload, more}, append(body, chunk...))
		case param[0] == wire.FuncUploadEnd:
			resp = ackData(h.PDURef, 0, 0, []byte{wire.FuncUploadEnd}, nil)
		default:
			return
		}
		if err := writeTPKT(conn, append([]byte{0x02, 0xF0, 0x80}, resp...)); err != nil {
			return
		}
	}
}

func (p *fakePLC) handleRead(ref uint16, param []byte) []byte {
	size, db, area, start := parseS7Any(param[2:14])
	reply := fakeReply{data: patternBytes(start, size)}
	if p.read != nil {
		reply = p.read(area, db, start, size)
	}
	if reply.ret != 0 && reply.ret != wire.RetCodeSuccess {
		return ackData(ref, 0, 0, []byte{wire.FuncReadVar, 1}, []byte{reply.ret, 0, 0, 0})
	}
	item := []byte{wire.RetCodeSuccess, 0x04, 0, 0}
	binary.BigEndian.PutUint16(item[2:4], uint16(len(reply.data)*8))
	return ackData(ref, 0, 0, []byte{wire.FuncReadVar, 1}, append(item, reply.data...))
}

func (p *fakePLC) handleWrite(ref uint16, param, data []byte) []byte {
	size, db, area, start := parseS7Any(param[2:14])
	p.mu.Lock()
	p.writes = append(p.writes, fakeWrite{area: area, db: db, start: start, data: append([]byte(nil), data[4:4+size]...)})
	p.mu.Unlock()
	ret := p.writeRet
	if ret == 0 {
		ret = wire.RetCodeSuccess
	}
	return ackData(ref, 0, 0, []byte{wire.FuncWriteVar, 1}, []byte{ret})
}

// handleUserdata answers a user data request as an S7-300 does: SZL reads
// (CPU functions) and block lists and block info (block functions). The
// test hook p.szl answers all of them; block functions use SZLBlockList
// (index: the block type) and SZLBlockInfo as keys.
func (p *fakePLC) handleUserdata(ref uint16, param, data []byte) []byte {
	group, sub := param[5]&0x0f, param[6]
	var id, index uint16
	switch {
	case group == wire.UserdataGroupCPU && sub == wire.UserdataSZL && len(data) >= 8:
		id, index = binary.BigEndian.Uint16(data[4:6]), binary.BigEndian.Uint16(data[6:8])
	case group == wire.UserdataGroupBlock && sub == wire.UserdataBlockList && len(data) >= 6:
		id, index = wire.SZLBlockList, uint16(data[5])
	case group == wire.UserdataGroupBlock && sub == wire.UserdataBlockInfo && len(data) >= 12:
		// data: ff 09 00 08 '0' <type> <five digits> 'A'; index is the block number
		n, _ := strconv.Atoi(string(data[6:11]))
		id, index = wire.SZLBlockInfo, uint16(n)
	default:
		return userdataAnswer(ref, group, sub, []byte{wire.RetCodeNotAvailable, 0, 0, 0})
	}
	reply := defaultSZL(id, index)
	if p.szl != nil {
		reply = p.szl(id, index)
	}
	if reply.ret != 0 && reply.ret != wire.RetCodeSuccess {
		return userdataAnswer(ref, group, sub, []byte{reply.ret, 0, 0, 0})
	}
	payload := reply.data
	if group == wire.UserdataGroupCPU { // an SZL answer starts with its ID and index
		payload = append(binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, id), index), payload...)
	}
	body := []byte{wire.RetCodeSuccess, 0x09, 0, 0}
	binary.BigEndian.PutUint16(body[2:4], uint16(len(payload)))
	return userdataAnswer(ref, group, sub, append(body, payload...))
}

// userdataAnswer is a one-packet user data response.
func userdataAnswer(ref uint16, group, sub byte, data []byte) []byte {
	param := []byte{0x00, 0x01, 0x12, 0x08, 0x12, 0x80 | group, sub, 0x00, 0x00, 0x00, 0x00, 0x00}
	h := make([]byte, 10, 10+len(param)+len(data))
	h[0], h[1] = 0x32, byte(wire.ROSCTRUserdata)
	binary.BigEndian.PutUint16(h[4:6], ref)
	binary.BigEndian.PutUint16(h[6:8], uint16(len(param)))
	binary.BigEndian.PutUint16(h[8:10], uint16(len(data)))
	return append(append(h, param...), data...)
}

// szlList builds SZL list data: record length, record count, records
// (each padded to the record length).
func szlList(recLen int, recs ...[]byte) []byte {
	out := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, uint16(recLen)), uint16(len(recs)))
	for _, r := range recs {
		rec := make([]byte, recLen)
		copy(rec, r)
		out = append(out, rec...)
	}
	return out
}

// record is an SZL record: its 2-byte index, then text.
func record(index uint16, text string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, index), text...)
}

// componentSZL is an SZL 0x001C list (34-byte records) from index -> text.
func componentSZL(texts map[uint16]string) []byte {
	var recs [][]byte
	for _, i := range []uint16{1, 2, 3, 4, 5, 7, 8} {
		if s, ok := texts[i]; ok {
			recs = append(recs, record(i, s))
		}
	}
	return szlList(34, recs...)
}

// protectionSZL is SZL 0x0232 index 4 with sch_rel = level (Siemens 1..3).
func protectionSZL(level byte) []byte {
	return szlList(40, []byte{0, 4, 0, 1, 0, 0, 0, level, 0, 2})
}

// defaultSZL answers like the CPU 315-2 PN/DP of go-s7comm's recorded
// captures, in RUN at protection level 3 (read/write protected), with DB1
// and DB2 loaded and two diagnostic entries.
func defaultSZL(id, index uint16) fakeReply {
	switch id {
	case wire.SZLModuleID:
		firmware := make([]byte, 28)
		copy(firmware, []byte{0x00, 0x07})
		copy(firmware[24:], []byte{'V', 3, 2, 7})
		return fakeReply{data: szlList(28, record(1, "6ES7 315-2EH14-0AB0"), firmware)}
	case wire.SZLComponentID:
		return fakeReply{data: componentSZL(map[uint16]string{
			1: "S7300/ET200M station_1", 2: "PLC_1", 4: "Original Siemens Equipment",
			5: "S C-B1U393142011", 7: "CPU 315-2 PN/DP", 8: "MMC 4A1AC019",
		})}
	case wire.SZLCPUState:
		return fakeReply{data: szlList(20, []byte{0x51, 0x44, 0xFF, 0x08})}
	case wire.SZLProtectionInfo:
		return fakeReply{data: protectionSZL(3)}
	case wire.SZLDiagBuffer:
		return fakeReply{data: szlList(20, []byte{0x4A, 0x01, 0x02, 0x03}, []byte{0x4B, 0x02, 0x04, 0x05})}
	case wire.SZLBlockList:
		if index == wire.BlockTypeDB {
			return fakeReply{data: []byte{0, 1, 0x22, 0x05, 0, 2, 0x22, 0x05}}
		}
		return fakeReply{}
	case wire.SZLBlockInfo:
		return fakeReply{data: blockInfo(int(index), 100, 20, 80)}
	}
	return fakeReply{ret: 0x0A}
}

// blockInfo is a block info record as an S7-300 answers it.
func blockInfo(number, loadMem, localData, mc7 int) []byte {
	r := make([]byte, 78)
	copy(r, []byte{0x01, 0x00, 0x00, 0x4a, 0x22, 0x00, 'p', 'p', 0x01, 0x01, 0x05, 0x0a})
	binary.BigEndian.PutUint16(r[12:14], uint16(number))
	binary.BigEndian.PutUint32(r[14:18], uint32(loadMem))
	binary.BigEndian.PutUint16(r[38:40], uint16(localData))
	binary.BigEndian.PutUint16(r[40:42], uint16(mc7))
	copy(r[42:], "SIMATIC")
	r[66] = 0x10
	return r
}

// patternBytes is the default memory: every byte holds its own address.
func patternBytes(start, size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(start + i)
	}
	return b
}

func parseS7Any(a []byte) (size, db int, area byte, start int) {
	size = int(binary.BigEndian.Uint16(a[4:6]))
	db = int(binary.BigEndian.Uint16(a[6:8]))
	area = a[8]
	start = (int(a[9])<<16 | int(a[10])<<8 | int(a[11])) / 8
	return size, db, area, start
}

// parseCR extracts the caller's reference and the called rack/slot from a COTP
// connection request.
func parseCR(tpdu []byte) (srcRef uint16, rack, slot int, ok bool) {
	if len(tpdu) < 7 || tpdu[1]&0xF0 != 0xE0 {
		return 0, 0, 0, false
	}
	srcRef = binary.BigEndian.Uint16(tpdu[4:6])
	end := int(tpdu[0]) + 1
	for i := 7; i+2 <= end && end <= len(tpdu); {
		code, n := tpdu[i], int(tpdu[i+1])
		if code == 0xC2 && n == 2 {
			b := tpdu[i+3]
			return srcRef, int(b >> 5), int(b & 0x1F), true
		}
		i += 2 + n
	}
	return srcRef, 0, 0, true
}

// connectionConfirm answers a CR: class 0 with a 1024-byte TPDU.
func connectionConfirm(dstRef uint16) []byte {
	cc := []byte{9, 0xD0, 0, 0, 0, 1, 0, 0xC0, 1, 0x0A}
	binary.BigEndian.PutUint16(cc[2:4], dstRef)
	return cc
}

func ackData(ref uint16, errClass, errCode byte, param, data []byte) []byte {
	h := make([]byte, 12, 12+len(param)+len(data))
	h[0] = 0x32
	h[1] = byte(wire.ROSCTRAckData)
	binary.BigEndian.PutUint16(h[4:6], ref)
	binary.BigEndian.PutUint16(h[6:8], uint16(len(param)))
	binary.BigEndian.PutUint16(h[8:10], uint16(len(data)))
	h[10], h[11] = errClass, errCode
	return append(append(h, param...), data...)
}

// readRawTPKT returns the TPDU of the next TPKT packet.
func readRawTPKT(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[2:4]))
	if hdr[0] != 3 || n < 4 {
		return nil, fmt.Errorf("bad TPKT header % X", hdr)
	}
	tpdu := make([]byte, n-4)
	_, err := io.ReadFull(r, tpdu)
	return tpdu, err
}

// readTSDU reassembles one COTP data unit.
func readTSDU(r io.Reader) ([]byte, error) {
	var tsdu []byte
	for {
		tpdu, err := readRawTPKT(r)
		if err != nil {
			return nil, err
		}
		if len(tpdu) < 3 || tpdu[1] != 0xF0 {
			return nil, fmt.Errorf("not a DT TPDU: % X", tpdu)
		}
		tsdu = append(tsdu, tpdu[3:]...)
		if tpdu[2]&0x80 != 0 {
			return tsdu, nil
		}
	}
}

func writeTPKT(w io.Writer, tpdu []byte) error {
	_, err := w.Write(tpktFrame(tpdu))
	return err
}

// tpktFrame wraps a TPDU in a TPKT header.
func tpktFrame(tpdu []byte) []byte {
	f := make([]byte, 4, 4+len(tpdu))
	f[0] = 3
	binary.BigEndian.PutUint16(f[2:4], uint16(4+len(tpdu)))
	return append(f, tpdu...)
}

// s7ReadRequestFrame is a complete TPKT frame holding an S7 Read Var job.
func s7ReadRequestFrame(ref uint16) []byte {
	req, err := wire.EncodeReadVarRequest(ref, []wire.S7AnyAddress{{Area: 0x84, DBNumber: 1, Start: 0, Size: 2}})
	if err != nil {
		panic(err)
	}
	return tpktFrame(append([]byte{0x02, 0xF0, 0x80}, req...))
}
