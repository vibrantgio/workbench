package main

// Modbus RTU framing, shared by both sides of the proxy: the host frames
// arriving over USB and the device replies arriving over the UART. Nothing
// here touches hardware, so it is unit-tested with the plain Go toolchain.

import "encoding/binary"

const (
	fnReadRegs   = 0x03
	fnWriteReg   = 0x06
	fnWriteRegs  = 0x10
	exceptionLen = 5   // addr, fn|0x80, code, crc
	maxFrame     = 256 // RTU frames are at most 256 bytes
	unknownLen   = -1  // the length cannot be told from what is known

	regSlaveAddr = 0x0018
	regBaud      = 0x0019 // SK-family baud selector, applied after a power cycle
)

func crc16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// crcOK reports whether the frame ends in its own CRC.
func crcOK(f []byte) bool {
	n := len(f)
	return n >= 4 && crc16(f[:n-2]) == binary.LittleEndian.Uint16(f[n-2:])
}

// requestLen returns the full length of the request frame whose first bytes
// are buf, or unknownLen while more bytes are needed to tell. A function
// the SK150 does not speak is reported as unknownLen too; the framer then
// falls back to a gap timeout.
func requestLen(buf []byte) int {
	if len(buf) < 2 {
		return unknownLen
	}
	switch buf[1] {
	case fnReadRegs, fnWriteReg:
		return 8
	case fnWriteRegs:
		if len(buf) < 7 {
			return unknownLen
		}
		return 9 + int(buf[6])
	}
	return unknownLen
}

// responseLen returns the length of the reply the device owes for req, given
// the reply bytes seen so far: exceptions are five bytes whatever the
// function; a broadcast (address 0) gets no reply at all, reported as 0.
func responseLen(req, got []byte) int {
	if req[0] == 0 {
		return 0
	}
	if len(got) >= 2 && got[1]&0x80 != 0 {
		return exceptionLen
	}
	switch req[1] {
	case fnReadRegs:
		return 5 + 2*int(binary.BigEndian.Uint16(req[4:]))
	case fnWriteReg, fnWriteRegs:
		return 8
	}
	return unknownLen
}

// writesReg reports whether the request writes register reg, and the value
// it writes: a single-register write, or a multi-register write whose range
// covers it.
func writesReg(req []byte, reg uint16) (uint16, bool) {
	if len(req) < 8 {
		return 0, false
	}
	start := binary.BigEndian.Uint16(req[2:])
	switch req[1] {
	case fnWriteReg:
		if start == reg {
			return binary.BigEndian.Uint16(req[4:]), true
		}
	case fnWriteRegs:
		count := binary.BigEndian.Uint16(req[4:])
		if reg >= start && reg < start+count {
			i := 7 + 2*int(reg-start)
			if i+2 <= len(req)-2 {
				return binary.BigEndian.Uint16(req[i:]), true
			}
		}
	}
	return 0, false
}

// isSuccess reports whether resp is a normal (non-exception) reply to req.
func isSuccess(req, resp []byte) bool {
	return len(resp) >= 2 && resp[0] == req[0] && resp[1] == req[1] && crcOK(resp)
}

// readRegsRequest builds a function 0x03 request frame.
func readRegsRequest(addr byte, start, count uint16) []byte {
	req := make([]byte, 6, 8)
	req[0], req[1] = addr, fnReadRegs
	binary.BigEndian.PutUint16(req[2:], start)
	binary.BigEndian.PutUint16(req[4:], count)
	return binary.LittleEndian.AppendUint16(req, crc16(req))
}

// framer assembles request frames from the byte stream a host sends over
// USB, where the RTU silent-interval rule cannot be relied on. A frame is
// recognised the moment the bytes hold one: the length its function code
// implies, ending in a good CRC, at any offset — so stray bytes ahead of it
// (the tail of a request a previous host session abandoned) are skipped
// without waiting. A function whose length is not known is only complete
// once the host falls silent; see flush.
type framer struct {
	buf [maxFrame]byte
	n   int
}

// push adds one byte and returns a complete, CRC-valid frame if that byte
// finished one. The returned slice is only valid until the next push.
func (f *framer) push(b byte) []byte {
	if f.n == maxFrame {
		f.n = 0
	}
	f.buf[f.n] = b
	f.n++
	for i := 0; i+4 <= f.n; i++ {
		fr := f.buf[i:f.n]
		if want := requestLen(fr); want != unknownLen && len(fr) == want && crcOK(fr) {
			f.n = 0
			return fr
		}
	}
	return nil
}

// flush is the gap-timeout path: it returns the CRC-valid frame the buffer
// ends in, if any, and clears the buffer either way. Only functions whose
// length push cannot tell reach here; anything else is leftovers.
func (f *framer) flush() []byte {
	n := f.n
	f.n = 0
	for i := 0; i+4 <= n; i++ {
		if crcOK(f.buf[i:n]) {
			return f.buf[i:n]
		}
	}
	return nil
}

func (f *framer) pending() bool { return f.n > 0 }
