package main

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func init() {
	// The proxy's waits are tuned for a device answering within tens of
	// milliseconds; the fakes answer at once.
	replyWait = 20 * time.Millisecond
	gapWait = 5 * time.Millisecond
	probeWait = 5 * time.Millisecond
	pollSleep = 100 * time.Microsecond
}

// fakeDevice is an SK150 on the far side of a UART: it answers a request
// only when the link runs at its speed, with whatever reply says.
type fakeDevice struct {
	in, out  []byte
	baud     uint32 // what the device listens at
	linkBaud uint32 // what the link was set to
	reply    func(req []byte) []byte
	requests [][]byte
}

func (d *fakeDevice) Buffered() int { return len(d.in) }

func (d *fakeDevice) ReadByte() (byte, error) {
	if len(d.in) == 0 {
		return 0, errors.New("empty")
	}
	b := d.in[0]
	d.in = d.in[1:]
	return b, nil
}

func (d *fakeDevice) Write(p []byte) (int, error) {
	d.out = append(d.out, p...)
	d.requests = append(d.requests, append([]byte(nil), p...))
	if d.linkBaud == d.baud && d.reply != nil {
		d.in = append(d.in, d.reply(p)...)
	}
	return len(p), nil
}

func (d *fakeDevice) setBaud(b uint32) { d.linkBaud = b }

// echoDevice answers every read with zeros and every write with an echo,
// like a well-behaved slave.
func echoDevice(baud uint32) *fakeDevice {
	d := &fakeDevice{baud: baud}
	d.reply = func(req []byte) []byte {
		var resp []byte
		switch req[1] {
		case fnReadRegs:
			count := int(req[5])
			resp = append([]byte{req[0], req[1], byte(2 * count)}, make([]byte, 2*count)...)
		case fnWriteReg, fnWriteRegs:
			resp = append([]byte(nil), req[:6]...)
		default:
			resp = []byte{req[0], req[1] | 0x80, 1}
		}
		c := crc16(resp)
		return append(resp, byte(c), byte(c>>8))
	}
	return d
}

func newTestLink(d *fakeDevice) *link { return newLink(d, d.setBaud) }

func TestTransactRelaysReplies(t *testing.T) {
	d := echoDevice(defaultBaud)
	l := newTestLink(d)
	resp, err := l.Transact(docRequest)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 3, 4, 0, 0, 0, 0}
	if !bytes.Equal(resp[:7], want) || !crcOK(resp) {
		t.Fatalf("resp = % X", resp)
	}
	if !bytes.Equal(d.requests[0], docRequest) {
		t.Fatalf("device saw % X", d.requests[0])
	}

	// An exception is relayed as the five bytes it is.
	odd := []byte{1, 0x11}
	c := crc16(odd)
	odd = append(odd, byte(c), byte(c>>8))
	resp, err = l.Transact(odd)
	if err != nil || len(resp) != exceptionLen || resp[1] != 0x91 {
		t.Fatalf("exception relay: % X %v", resp, err)
	}

	// A broadcast is forwarded and gets no reply.
	bcast := append([]byte(nil), writeReq(0x12, 1)...)
	bcast[0] = 0
	c = crc16(bcast[:6])
	bcast[6], bcast[7] = byte(c), byte(c>>8)
	resp, err = l.Transact(bcast)
	if err != nil || resp != nil || len(d.requests) != 3 {
		t.Fatalf("broadcast: % X %v (%d requests)", resp, err, len(d.requests))
	}
}

func TestTransactTimesOutAndReprobes(t *testing.T) {
	d := echoDevice(115200) // the device is not at the link's default speed
	l := newTestLink(d)
	// Make the rate limit lapse.
	l.probed = time.Now().Add(-2 * probeEvery)

	for i := 0; i < missesBeforeProbe-1; i++ {
		if _, err := l.Transact(docRequest); !errors.Is(err, errTimeout) {
			t.Fatalf("miss %d: %v", i, err)
		}
	}
	if l.baud != defaultBaud {
		t.Fatal("must not re-probe before the miss threshold")
	}
	if _, err := l.Transact(docRequest); !errors.Is(err, errTimeout) {
		t.Fatalf("third miss: %v", err)
	}
	if l.baud != 115200 {
		t.Fatalf("after the third miss the link runs at %d, want 115200", l.baud)
	}
	if resp, err := l.Transact(docRequest); err != nil || len(resp) != 9 {
		t.Fatalf("after re-probe: % X %v", resp, err)
	}
	if l.misses != 0 {
		t.Fatal("a reply resets the miss count")
	}
}

func TestBaudWriteIsRememberedAndTriedFirst(t *testing.T) {
	d := echoDevice(defaultBaud)
	l := newTestLink(d)
	if _, err := l.Transact(writeReq(regBaud, 6)); err != nil {
		t.Fatal(err)
	}
	if l.pending != 115200 {
		t.Fatalf("pending = %d, want 115200", l.pending)
	}
	// The write also reaches the device untouched.
	if !bytes.Equal(d.requests[0], writeReq(regBaud, 6)) {
		t.Fatalf("device saw % X", d.requests[0])
	}

	// The device power-cycles into its new speed; the probe tries the
	// remembered speed first, so it is the only request at that speed.
	d.baud = 115200
	d.requests = nil
	l.probed = time.Now().Add(-2 * probeEvery)
	for i := 0; i < missesBeforeProbe; i++ {
		l.Transact(docRequest)
	}
	if l.baud != 115200 || l.pending != 0 {
		t.Fatalf("baud %d pending %d", l.baud, l.pending)
	}
	// Three misses, then the probe's first request found it.
	if len(d.requests) != missesBeforeProbe+1 {
		t.Fatalf("%d requests on the wire, want %d", len(d.requests), missesBeforeProbe+1)
	}

	// A multi-register write that covers the register counts too, with an
	// out-of-table value ignored.
	if _, err := l.Transact(writeMultiReq(regSlaveAddr, 1, 3)); err != nil {
		t.Fatal(err)
	}
	if l.pending != 38400 {
		t.Fatalf("pending = %d, want 38400", l.pending)
	}
	l.Transact(writeReq(regBaud, 200))
	if l.pending != 38400 {
		t.Fatalf("an unknown value must be ignored, pending = %d", l.pending)
	}
}

func TestProbeUsesHostAddress(t *testing.T) {
	d := echoDevice(defaultBaud)
	d.reply = func(req []byte) []byte {
		if req[0] != 7 {
			return nil
		}
		resp := []byte{7, 3, 2, 0, 0}
		c := crc16(resp)
		return append(resp, byte(c), byte(c>>8))
	}
	l := newTestLink(d)
	req := append([]byte(nil), docRequest...)
	req[0] = 7
	c := crc16(req[:6])
	req[6], req[7] = byte(c), byte(c>>8)
	l.Transact(req)
	if !l.Probe() {
		t.Fatal("probe must use the address the host talks to")
	}
}

func TestProbeGivesUp(t *testing.T) {
	d := &fakeDevice{baud: 1} // answers nothing
	l := newTestLink(d)
	if l.Probe() {
		t.Fatal("silent device")
	}
	if l.baud != defaultBaud {
		t.Fatalf("falls back to %d, got %d", defaultBaud, l.baud)
	}
}

func TestProxyStep(t *testing.T) {
	d := echoDevice(defaultBaud)
	host := &fakeDevice{baud: 1, linkBaud: 1} // no reply function: a plain pipe
	p := newProxy(host, newTestLink(d))

	if p.step() {
		t.Fatal("nothing to do on an idle host")
	}
	host.in = append(host.in, docRequest...)
	if !p.step() {
		t.Fatal("bytes waiting")
	}
	if len(host.out) != 9 || !crcOK(host.out) {
		t.Fatalf("host got % X", host.out)
	}

	// A frame of unknown length is completed by the gap timeout.
	host.out = nil
	odd := []byte{1, 0x11}
	c := crc16(odd)
	host.in = append(host.in, odd...)
	host.in = append(host.in, byte(c), byte(c>>8))
	p.step()
	if len(host.out) != 0 {
		t.Fatal("unknown function must wait for the gap")
	}
	p.last = time.Now().Add(-2 * hostGap)
	p.step()
	if len(host.out) != exceptionLen {
		t.Fatalf("host got % X", host.out)
	}
}

func TestUtoa(t *testing.T) {
	for v, want := range map[uint32]string{0: "0", 7: "7", 9600: "9600", 115200: "115200"} {
		if got := utoa(v); got != want {
			t.Fatalf("utoa(%d) = %q", v, got)
		}
	}
}
