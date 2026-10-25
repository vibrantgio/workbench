package main

// The proxy: the RP2040 is the SK150's only Modbus master, and the host on
// USB talks to the RP2040 as if it were the SK150. Every host frame is
// forwarded unchanged and the device's reply is relayed unchanged, so the
// desktop app sees exactly the device it already knows; a device that stays
// silent leaves the host to time out, as it would on the FTDI cable.
//
// The one thing the proxy interprets is a write to the baud register: the
// SK150 adopts a new baud only at its next power cycle, so the link
// remembers what it was told, and once the device stops answering it
// re-probes, trying that speed first.
//
// Everything in this file is hardware-free (the port interface is all it
// needs) so the logic runs under the plain Go test tool; main.go binds it
// to the RP2040's UART, USB and LED.

import (
	"errors"
	"sync"
	"time"
)

// port is the byte-level serial surface TinyGo's UART and USB CDC share.
type port interface {
	Buffered() int
	ReadByte() (byte, error)
	Write(p []byte) (int, error)
}

// baudRates is the SK-family encoding of the baud register: 0 is 9600
// (confirmed on the SK150), 6 is 115200 (the XY6020L documentation) and
// the unit's own menu offers options 0..8. The rows in between and above are the
// classic nine-entry serial list those two fixed points sit in; the probe
// tries every entry anyway, so a wrong row costs a little probe time, not
// the link.
var baudRates = [...]uint32{9600, 14400, 19200, 38400, 56000, 57600, 115200, 128000, 256000}

const (
	defaultBaud       = 9600 // the SK150 as shipped
	defaultAddr       = 1
	missesBeforeProbe = 3
)

// Timings are variables so the tests can shrink them.
var (
	replyWait  = 500 * time.Millisecond // first byte of a reply (the SK150 takes tens of ms)
	gapWait    = 20 * time.Millisecond  // silence that ends a reply of unknown length
	probeWait  = 200 * time.Millisecond // reply wait while probing baud rates
	probeEvery = 10 * time.Second       // rate limit on re-probing a silent device
	pollSleep  = time.Millisecond
)

var errTimeout = errors.New("no reply")

// link is the SK150 side of the proxy: the sole owner of the UART. The
// mutex serialises the host's requests against whatever else the firmware
// grows to run on the bus.
type link struct {
	mu      sync.Mutex
	port    port
	setBaud func(uint32)
	busy    func(bool) // activity indicator, may be nil

	baud    uint32 // the speed the UART runs at now
	pending uint32 // the speed the host told the device to adopt, 0 = none
	addr    byte   // the slave address the host uses, for probing
	misses  int    // consecutive unanswered requests
	probed  time.Time
}

func newLink(p port, setBaud func(uint32)) *link {
	l := &link{port: p, setBaud: setBaud, addr: defaultAddr}
	l.use(defaultBaud)
	return l
}

func (l *link) use(baud uint32) {
	l.baud = baud
	l.setBaud(baud)
}

// Transact forwards one host request to the device and returns its reply.
// A broadcast returns nil, nil. errTimeout means the device stayed silent.
func (l *link) Transact(req []byte) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if req[0] != 0 {
		l.addr = req[0]
	}
	resp, err := l.exchange(req, replyWait)
	switch {
	case err == nil:
		l.misses = 0
		if v, ok := writesReg(req, regBaud); ok && isSuccess(req, resp) && int(v) < len(baudRates) {
			l.pending = baudRates[v]
			logs("baud register set: next power cycle ", utoa(l.pending))
		}
	case errors.Is(err, errTimeout):
		l.misses++
		if l.misses >= missesBeforeProbe && time.Since(l.probed) > probeEvery {
			l.probe()
		}
	}
	return resp, err
}

// exchange writes req and collects the reply: as many bytes as the request
// implies, or, for a function whose reply length is unknown, until the line
// falls silent. Callers hold l.mu.
func (l *link) exchange(req []byte, wait time.Duration) ([]byte, error) {
	if l.busy != nil {
		l.busy(true)
		defer l.busy(false)
	}
	l.drain()
	if _, err := l.port.Write(req); err != nil {
		return nil, err
	}
	if responseLen(req, nil) == 0 {
		return nil, nil // broadcast: nothing to wait for
	}
	var buf [maxFrame]byte
	n := 0
	deadline := time.Now().Add(wait)
	for {
		if l.port.Buffered() == 0 {
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(pollSleep)
			continue
		}
		b, err := l.port.ReadByte()
		if err != nil {
			break
		}
		if n < maxFrame {
			buf[n] = b
			n++
		}
		if want := responseLen(req, buf[:n]); want != unknownLen && n >= want {
			n = want
			break
		}
		deadline = time.Now().Add(gapWait)
	}
	if n == 0 {
		return nil, errTimeout
	}
	out := make([]byte, n)
	copy(out, buf[:n])
	return out, nil
}

// drain discards stale bytes waiting on the UART before a new request.
func (l *link) drain() {
	for l.port.Buffered() > 0 {
		l.port.ReadByte()
	}
}

// Probe finds the speed the device listens at, trying the speed it was
// told to adopt first, then the current one, then the whole table. It
// returns whether the device answered. Each candidate gets two tries: the
// first frame after a speed change can be spoiled by a half-received byte.
func (l *link) Probe() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.probe()
}

func (l *link) probe() bool {
	l.probed = time.Now()
	candidates := make([]uint32, 0, len(baudRates)+2)
	if l.pending != 0 {
		candidates = append(candidates, l.pending)
	}
	candidates = append(candidates, l.baud)
	candidates = append(candidates, baudRates[:]...)
	req := readRegsRequest(l.addr, regBaud, 1)
	for _, b := range candidates {
		l.use(b)
		for try := 0; try < 2; try++ {
			resp, err := l.exchange(req, probeWait)
			if err == nil && len(resp) == 7 && isSuccess(req, resp) {
				l.misses = 0
				l.pending = 0
				logs("device found at ", utoa(b))
				return true
			}
		}
	}
	logs("device not answering at any baud rate")
	l.use(defaultBaud)
	return false
}

// proxy is the host side: it frames the bytes arriving over USB, runs each
// request through the link, and relays the reply.
type proxy struct {
	host port
	link *link
	f    framer
	last time.Time // when the last host byte arrived
}

func newProxy(host port, l *link) *proxy { return &proxy{host: host, link: l} }

// hostGap is the silence after which a partial host frame of unknown
// length is taken as complete (USB delivers a frame in one packet, so any
// real gap is far shorter than this).
const hostGap = 100 * time.Millisecond

// step handles the bytes waiting from the host and reports whether it did
// anything; the main loop sleeps briefly when it did not.
func (p *proxy) step() bool {
	if p.host.Buffered() == 0 {
		if p.f.pending() && time.Since(p.last) > hostGap {
			if req := p.f.flush(); req != nil {
				p.handle(req)
			}
			return true
		}
		return false
	}
	for p.host.Buffered() > 0 {
		b, err := p.host.ReadByte()
		if err != nil {
			break
		}
		p.last = time.Now()
		if req := p.f.push(b); req != nil {
			p.handle(req)
		}
	}
	return true
}

func (p *proxy) handle(req []byte) {
	resp, err := p.link.Transact(req)
	if err != nil {
		logs("device: ", err.Error())
		return
	}
	if resp != nil {
		p.host.Write(resp)
	}
}

// logTo receives diagnostics; nil discards them. It must never be the USB
// port, which carries Modbus.
var logTo port

func logs(parts ...string) {
	if logTo == nil {
		return
	}
	for _, s := range parts {
		logTo.Write([]byte(s))
	}
	logTo.Write([]byte("\r\n"))
}

// utoa is a strconv-free integer formatter; fmt would bloat the firmware.
func utoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
