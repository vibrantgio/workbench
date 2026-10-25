package main

import (
	"bytes"
	"testing"
)

// The two frames from the XY6020L Modbus documentation's worked example.
var (
	docRequest  = []byte{0x01, 0x03, 0x00, 0x02, 0x00, 0x02, 0x65, 0xCB}
	docResponse = []byte{0x01, 0x03, 0x04, 0x01, 0xF4, 0x05, 0xDC, 0xB8, 0xF4}
)

func TestCRC(t *testing.T) {
	if !crcOK(docRequest) || !crcOK(docResponse) {
		t.Fatal("documented frames must verify")
	}
	bad := append([]byte(nil), docRequest...)
	bad[3]++
	if crcOK(bad) {
		t.Fatal("a corrupted frame must not verify")
	}
	if got := readRegsRequest(1, 2, 2); !bytes.Equal(got, docRequest) {
		t.Fatalf("readRegsRequest = % X, want % X", got, docRequest)
	}
}

func TestRequestLen(t *testing.T) {
	if got := requestLen(docRequest[:1]); got != unknownLen {
		t.Fatalf("one byte: %d", got)
	}
	if got := requestLen(docRequest[:2]); got != 8 {
		t.Fatalf("read: %d", got)
	}
	multi := []byte{1, 0x10, 0, 0x50, 0, 14, 28}
	if got := requestLen(multi[:6]); got != unknownLen {
		t.Fatalf("write-multiple without byte count: %d", got)
	}
	if got := requestLen(multi); got != 37 {
		t.Fatalf("write-multiple of 14 registers: %d", got)
	}
	if got := requestLen([]byte{1, 0x11}); got != unknownLen {
		t.Fatalf("unknown function: %d", got)
	}
}

func TestResponseLen(t *testing.T) {
	if got := responseLen(docRequest, nil); got != 9 {
		t.Fatalf("read of two registers: %d", got)
	}
	if got := responseLen(docRequest, []byte{1, 0x83}); got != exceptionLen {
		t.Fatalf("exception: %d", got)
	}
	bcast := []byte{0, 0x06, 0, 0x12, 0, 1, 0, 0}
	if got := responseLen(bcast, nil); got != 0 {
		t.Fatalf("broadcast: %d", got)
	}
	single := []byte{1, 0x06, 0, 0x12, 0, 1, 0, 0}
	if got := responseLen(single, nil); got != 8 {
		t.Fatalf("write single: %d", got)
	}
}

func writeReq(reg, val uint16) []byte {
	req := []byte{1, fnWriteReg, byte(reg >> 8), byte(reg), byte(val >> 8), byte(val)}
	c := crc16(req)
	return append(req, byte(c), byte(c>>8))
}

func writeMultiReq(start uint16, vals ...uint16) []byte {
	req := []byte{1, fnWriteRegs, byte(start >> 8), byte(start), 0, byte(len(vals)), byte(2 * len(vals))}
	for _, v := range vals {
		req = append(req, byte(v>>8), byte(v))
	}
	c := crc16(req)
	return append(req, byte(c), byte(c>>8))
}

func TestWritesReg(t *testing.T) {
	if v, ok := writesReg(writeReq(regBaud, 6), regBaud); !ok || v != 6 {
		t.Fatalf("single write: %d %v", v, ok)
	}
	if _, ok := writesReg(writeReq(regSlaveAddr, 6), regBaud); ok {
		t.Fatal("a write elsewhere must not match")
	}
	if v, ok := writesReg(writeMultiReq(regSlaveAddr, 1, 3, 7), regBaud); !ok || v != 3 {
		t.Fatalf("multi write covering the register: %d %v", v, ok)
	}
	if _, ok := writesReg(writeMultiReq(0x50, 1, 2), regBaud); ok {
		t.Fatal("a multi write elsewhere must not match")
	}
	if _, ok := writesReg(docRequest, regBaud); ok {
		t.Fatal("a read must not match")
	}
}

func feed(f *framer, data []byte) (frames [][]byte) {
	for _, b := range data {
		if fr := f.push(b); fr != nil {
			frames = append(frames, append([]byte(nil), fr...))
		}
	}
	return frames
}

func TestFramer(t *testing.T) {
	var f framer
	if got := feed(&f, docRequest); len(got) != 1 || !bytes.Equal(got[0], docRequest) {
		t.Fatalf("clean frame: %v", got)
	}

	// Leading garbage is slid out; the frame after it is still found.
	noisy := append([]byte{0x00, 0xFF, 0x01}, docRequest...)
	if got := feed(&f, noisy); len(got) != 1 || !bytes.Equal(got[0], docRequest) {
		t.Fatalf("frame after garbage: %v", got)
	}

	// Two back-to-back frames of different shapes.
	multi := writeMultiReq(0x50, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14)
	stream := append(append([]byte(nil), multi...), docRequest...)
	got := feed(&f, stream)
	if len(got) != 2 || !bytes.Equal(got[0], multi) || !bytes.Equal(got[1], docRequest) {
		t.Fatalf("two frames: %v", got)
	}

	// An unknown function is left buffered and comes out on flush.
	odd := []byte{1, 0x11}
	c := crc16(odd)
	odd = append(odd, byte(c), byte(c>>8))
	if got := feed(&f, odd); len(got) != 0 || !f.pending() {
		t.Fatalf("unknown function must wait for a gap: %v", got)
	}
	if got := f.flush(); !bytes.Equal(got, odd) {
		t.Fatalf("flush = % X", got)
	}
	if f.flush() != nil || f.pending() {
		t.Fatal("flush must clear the buffer")
	}

	// Leftovers ahead of an unknown-function frame are skipped at the gap
	// too, and pure leftovers yield nothing.
	feed(&f, append([]byte{0x03, 0x06}, odd...))
	if got := f.flush(); !bytes.Equal(got, odd) {
		t.Fatalf("flush past leftovers = % X", got)
	}
	feed(&f, []byte{0x01, 0x10, 0x00})
	if got := f.flush(); got != nil {
		t.Fatalf("leftovers flushed as % X", got)
	}
}
