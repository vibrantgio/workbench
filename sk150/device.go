package main

// The Modbus RTU client for the XY-SK150 buck-boost converter, and the
// register map this app reads and writes. The SK150 is the display-limited
// sibling of the XY6020L; the map below is the XY-SK family layout confirmed
// against the live device (fw 136): voltages carry 2 decimals, currents 3
// (the XY6020L's carry 2 — do not copy its scaling), power 1.
//
// Labels on the device connector are from the master's perspective and the
// unit is configured for 9600 baud (register 0x19 = 0; a change applies only
// after a power cycle).

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const (
	slaveAddr = 1
	baudRate  = 9600

	regVSet    = 0x0000 // 0.01 V, R/W
	regISet    = 0x0001 // 0.001 A, R/W
	regVOut    = 0x0002 // 0.01 V
	regIOut    = 0x0003 // 0.001 A
	regPower   = 0x0004 // 0.1 W
	regVIn     = 0x0005 // 0.01 V
	regAhLo    = 0x0006 // mAh, low 16 bits
	regAhHi    = 0x0007
	regWhLo    = 0x0008 // mWh, low 16 bits
	regWhHi    = 0x0009
	regOutH    = 0x000A
	regOutM    = 0x000B
	regOutS    = 0x000C
	regTIn     = 0x000D // 0.1 degree
	regProtect = 0x0010 // 0 = normal; write 0 to clear a trip
	regCVCC    = 0x0011 // 0 = CV, 1 = CC
	regOnOff   = 0x0012 // 0 = off, 1 = on, R/W

	regLVP = 0x0052 // 0.01 V input low-voltage protection
	regOVP = 0x0053 // 0.01 V over-voltage protection
	regOCP = 0x0054 // 0.001 A over-current protection
	regOPP = 0x0055 // 0.1 W over-power protection

	// regExtract recalls a memory group: writing 1..9 copies that group
	// into the live M0 set (writing 0 has no effect); reading it reports
	// the last recalled group.
	regExtract = 0x001D

	// Device settings (XY-SK series; confirmed present on the SK150).
	regLock      = 0x000F // key lock 0/1
	regTempUnit  = 0x0013 // 0 = °C, 1 = °F
	regBacklight = 0x0014 // 0..5
	regSleep     = 0x0015 // screen-off minutes, 0 = never
	regBeeper    = 0x001C // 0/1
	regMPPTOn    = 0x001F // MPPT (solar input tracking) 0/1
	regMPPTPct   = 0x0020 // MPPT set point, percent of open-circuit voltage
	regBatCutoff = 0x0021 // battery-full cutoff current, 0.001 A, 0 = off
	regCPOn      = 0x0022 // constant-power mode 0/1
	regCPWatts   = 0x0023 // constant-power set point, 0.1 W
	// 0x0025 is the factory-reset trigger: never written by this app.

	deviceBlockOff = 0x000F // one read covers regLock..regCPWatts
	deviceBlockLen = 0x0023 - 0x000F + 1

	// Offsets inside a memory group (M0 live at 0x50, Mn at 0x50+n*0x10).
	offVSet = 0  // 0.01 V
	offISet = 1  // 0.001 A
	offLVP  = 2  // 0.01 V
	offOVP  = 3  // 0.01 V
	offOCP  = 4  // 0.001 A
	offOPP  = 5  // 0.1 W
	offOHPH = 6  // max output time, hours (0 = off with minutes 0)
	offOHPM = 7  // max output time, minutes
	offOAHL = 8  // max output charge, mAh, low word (0 = off)
	offOAHH = 9  // high word
	offOWHL = 10 // max output energy, 10 mWh units, low word (0 = off)
	offOWHH = 11 // high word
	offOTP  = 12 // over-temperature protection, whole degrees
	offINI  = 13 // output on at power-up 0/1

	monitorBlockLen  = 0x14 // registers 0x00..0x13 in one read
	settingsBlockOff = 0x0050
	settingsBlockLen = 14 // the live M0 data set, 0x50..0x5D

	// The ten preset memory groups M0..M9: M0 is the live set at 0x50,
	// the stored groups follow at a 0x10 stride (M3 = 0x50 + 3*0x10).
	presetBase   = 0x0050
	presetStride = 0x0010
	presetRegs   = 14
)

// presetReg returns the address of one register inside memory group n.
func presetReg(n int, off uint16) uint16 {
	return presetBase + uint16(n)*presetStride + off
}

// protectNames maps regProtect values to the trip they report.
var protectNames = []string{"", "OVP", "OCP", "OPP", "LVP", "OAH", "OHP", "OTP", "OEP", "OWH", "ICP"}

// protectLVP is the regProtect value of an input low-voltage trip — the one
// trip a Clear cannot reset: it re-arms while the cutoff is above the input
// and heals itself once it is not.
const protectLVP = 4

// ProtectName returns the short name of a protection status value.
func ProtectName(v int) string {
	if v > 0 && v < len(protectNames) {
		return protectNames[v]
	}
	return fmt.Sprintf("code %d", v)
}

// Reading is one poll of the live monitor block.
type Reading struct {
	VSet, ISet float64 // programmed setpoints, V / A
	VOut, IOut float64 // live output, V / A
	Power      float64 // live output power, W
	VIn        float64 // input voltage, V
	MilliAh    uint32  // accumulated charge since power-up
	MilliWh    uint32  // accumulated energy since power-up
	Hours      int     // output-on time
	Mins       int
	Secs       int
	TempIn     float64 // internal temperature
	CC         bool    // true = constant-current, false = constant-voltage
	On         bool    // output enabled
	Protect    int     // 0 = normal, else a protection trip (ProtectName)
}

// Preset is one memory group (M0..M9), all fourteen registers: a full
// operating profile the device stores and recalls. M0 is the live set.
type Preset struct {
	VSet    float64 // V
	ISet    float64 // A
	LVP     float64 // input low-voltage cutoff, V
	OVP     float64 // over-voltage protection, V
	OCP     float64 // over-current protection, A
	OPP     float64 // over-power protection, W
	OHPH    int     // max output time, hours (with OHPM 0 = no limit)
	OHPM    int     // max output time, minutes
	OAH     uint32  // max output charge, mAh (0 = no limit)
	OWH     uint32  // max output energy, 10 mWh units (0 = no limit)
	OTP     float64 // over-temperature protection, degrees
	PowerOn bool    // output on at power-up
}

// Settings is the live group (M0) — the same shape as a stored preset.
type Settings = Preset

// DeviceSettings are the unit-wide settings outside the memory groups.
type DeviceSettings struct {
	KeyLock       bool
	Fahrenheit    bool
	Backlight     int // 0..5
	SleepMinutes  int // 0 = never
	Beeper        bool
	Group         int // last recalled memory group
	MPPTOn        bool
	MPPTPercent   int     // set point as % of open-circuit input voltage
	BatteryCutoff float64 // A, 0 = off
	CPOn          bool
	CPWatts       float64
}

// decodePreset unpacks the fourteen registers of a memory group.
func decodePreset(r []uint16) Preset {
	return Preset{
		VSet:    float64(r[offVSet]) / 100,
		ISet:    float64(r[offISet]) / 1000,
		LVP:     float64(r[offLVP]) / 100,
		OVP:     float64(r[offOVP]) / 100,
		OCP:     float64(r[offOCP]) / 1000,
		OPP:     float64(r[offOPP]) / 10,
		OHPH:    int(r[offOHPH]),
		OHPM:    int(r[offOHPM]),
		OAH:     uint32(r[offOAHH])<<16 | uint32(r[offOAHL]),
		OWH:     uint32(r[offOWHH])<<16 | uint32(r[offOWHL]),
		OTP:     float64(r[offOTP]),
		PowerOn: r[offINI] == 1,
	}
}

// encodePreset packs a memory group for a write-multiple transaction.
func encodePreset(p Preset) []uint16 {
	r := make([]uint16, presetRegs)
	r[offVSet] = scaled(p.VSet, 100)
	r[offISet] = scaled(p.ISet, 1000)
	r[offLVP] = scaled(p.LVP, 100)
	r[offOVP] = scaled(p.OVP, 100)
	r[offOCP] = scaled(p.OCP, 1000)
	r[offOPP] = scaled(p.OPP, 10)
	r[offOHPH] = uint16(p.OHPH)
	r[offOHPM] = uint16(p.OHPM)
	r[offOAHL] = uint16(p.OAH & 0xFFFF)
	r[offOAHH] = uint16(p.OAH >> 16)
	r[offOWHL] = uint16(p.OWH & 0xFFFF)
	r[offOWHH] = uint16(p.OWH >> 16)
	r[offOTP] = uint16(p.OTP)
	if p.PowerOn {
		r[offINI] = 1
	}
	return r
}

// decodeDeviceSettings unpacks the regLock..regCPWatts block.
func decodeDeviceSettings(r []uint16) DeviceSettings {
	at := func(reg uint16) uint16 { return r[reg-deviceBlockOff] }
	return DeviceSettings{
		KeyLock:       at(regLock) == 1,
		Fahrenheit:    at(regTempUnit) == 1,
		Backlight:     int(at(regBacklight)),
		SleepMinutes:  int(at(regSleep)),
		Beeper:        at(regBeeper) == 1,
		Group:         int(at(regExtract)),
		MPPTOn:        at(regMPPTOn) == 1,
		MPPTPercent:   int(at(regMPPTPct)),
		BatteryCutoff: float64(at(regBatCutoff)) / 1000,
		CPOn:          at(regCPOn) == 1,
		CPWatts:       float64(at(regCPWatts)) / 10,
	}
}

// Bus is what the command layer talks to: the real serial device, or the
// demo-mode simulator.
type Bus interface {
	Poll() (Reading, error)
	ReadSettings() (Settings, error)
	ReadPreset(n int) (Preset, error)
	ReadDeviceSettings() (DeviceSettings, error)
	WriteReg(reg, val uint16) error
	WriteRegs(start uint16, vals []uint16) error
}

// Device is the serial Modbus client. One instance serves the whole app;
// the mutex serialises the poll chain against write commands.
type Device struct {
	mu   sync.Mutex
	port serial.Port
	path string
}

// device is the real hardware client; activeBus is what commands use — the
// device by default, the simulator once demo mode starts. Commands run
// concurrently, so the swap goes through the RWMutex accessors.
var (
	device    = &Device{}
	busMu     sync.RWMutex
	activeBus Bus = device
)

func currentBus() Bus {
	busMu.RLock()
	defer busMu.RUnlock()
	return activeBus
}

func setBus(b Bus) {
	busMu.Lock()
	defer busMu.Unlock()
	activeBus = b
}

// humane rewrites a transport error as something a person at the bench can
// act on; the raw text is kept only for the unrecognised remainder.
func humane(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no serial adapter"):
		return "no USB serial adapter found — plug the cable in"
	case errors.Is(err, fs.ErrNotExist) || strings.Contains(msg, "no such file"):
		return "the serial adapter disappeared — check the USB cable"
	case errors.Is(err, fs.ErrPermission) || strings.Contains(msg, "permission denied") || strings.Contains(msg, "access is denied"):
		if runtime.GOOS == "linux" {
			return "no permission for the serial port — add your user to the dialout group"
		}
		return "no access to the serial port — another program may be using it"
	case strings.Contains(msg, "timeout"):
		return "the SK150 isn't answering — check its power and the wiring"
	case strings.Contains(msg, "bad crc"):
		return "garbled data from the device — check the wiring"
	case strings.Contains(msg, "device exception"):
		return "the device refused that request"
	case strings.Contains(msg, "busy"):
		return "the serial port is in use by another program"
	default:
		return "connection problem — " + err.Error()
	}
}

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

// ftdiVID is the USB vendor ID of FTDI, whose adapter ships with the SK150.
const ftdiVID = "0403"

// adapterGlobs is the fallback when port enumeration yields nothing: Linux's
// stable udev names, then macOS's callout devices (FTDI shows up as
// cu.usbserial-<id>, CDC-ACM adapters as cu.usbmodem*). Windows ports are
// COMn names rather than files, so only enumeration finds them.
var adapterGlobs = []string{
	"/dev/serial/by-id/*",
	"/dev/cu.usbserial*",
	"/dev/cu.usbmodem*",
}

// findAdapter returns the USB serial adapter to talk to, preferring an FTDI
// one; empty when none is plugged in. The OS port enumeration works on
// Linux, macOS and Windows alike; the path globs are a safety net for a
// system where enumeration fails.
func findAdapter() string {
	if ports, err := enumerator.GetDetailedPortsList(); err == nil {
		first := ""
		for _, p := range ports {
			if !p.IsUSB {
				continue
			}
			if strings.EqualFold(p.VID, ftdiVID) {
				return p.Name
			}
			if first == "" {
				first = p.Name
			}
		}
		if first != "" {
			return first
		}
	}
	for _, g := range adapterGlobs {
		matches, _ := filepath.Glob(g)
		if len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		for _, m := range matches {
			if strings.Contains(m, "FTDI") {
				return m
			}
		}
		return matches[0]
	}
	return ""
}

// open connects to the FTDI adapter (any USB serial adapter if none says
// FTDI). Callers hold d.mu.
func (d *Device) open() error {
	if d.port != nil {
		return nil
	}
	path := d.path
	if path == "" {
		path = findAdapter()
	}
	if path == "" {
		return fmt.Errorf("no serial adapter found")
	}
	p, err := serial.Open(path, &serial.Mode{BaudRate: baudRate})
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	p.SetReadTimeout(100 * time.Millisecond)
	d.port, d.path = p, path
	return nil
}

// close drops the port so the next transact reopens; callers hold d.mu.
func (d *Device) close() {
	if d.port != nil {
		d.port.Close()
		d.port = nil
	}
}

// transact writes one request frame and collects respLen response bytes,
// verifying the trailing CRC.
func (d *Device) transact(req []byte, respLen int) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.open(); err != nil {
		return nil, err
	}
	d.port.ResetInputBuffer()
	if _, err := d.port.Write(req); err != nil {
		d.close()
		return nil, err
	}
	buf := make([]byte, 0, respLen)
	tmp := make([]byte, respLen)
	deadline := time.Now().Add(800 * time.Millisecond)
	for len(buf) < respLen && time.Now().Before(deadline) {
		n, err := d.port.Read(tmp)
		if err != nil {
			d.close()
			return nil, err
		}
		buf = append(buf, tmp[:n]...)
	}
	if len(buf) < respLen {
		return nil, fmt.Errorf("timeout (%d of %d bytes)", len(buf), respLen)
	}
	if len(buf) >= 3 && buf[1]&0x80 != 0 {
		return nil, fmt.Errorf("device exception 0x%02x", buf[2])
	}
	if crc16(buf[:respLen-2]) != binary.LittleEndian.Uint16(buf[respLen-2:respLen]) {
		return nil, fmt.Errorf("bad CRC")
	}
	return buf, nil
}

// ReadRegs reads count holding registers starting at start (function 0x03).
func (d *Device) ReadRegs(start, count uint16) ([]uint16, error) {
	req := make([]byte, 6, 8)
	req[0], req[1] = slaveAddr, 0x03
	binary.BigEndian.PutUint16(req[2:], start)
	binary.BigEndian.PutUint16(req[4:], count)
	req = binary.LittleEndian.AppendUint16(req, crc16(req))
	resp, err := d.transact(req, 5+2*int(count))
	if err != nil {
		return nil, err
	}
	regs := make([]uint16, count)
	for i := range regs {
		regs[i] = binary.BigEndian.Uint16(resp[3+2*i:])
	}
	return regs, nil
}

// WriteReg writes one holding register (function 0x06).
func (d *Device) WriteReg(reg, val uint16) error {
	req := make([]byte, 6, 8)
	req[0], req[1] = slaveAddr, 0x06
	binary.BigEndian.PutUint16(req[2:], reg)
	binary.BigEndian.PutUint16(req[4:], val)
	req = binary.LittleEndian.AppendUint16(req, crc16(req))
	_, err := d.transact(req, 8)
	return err
}

// WriteRegs writes consecutive holding registers (function 0x10) — one
// transaction for a whole memory group or a 32-bit pair.
func (d *Device) WriteRegs(start uint16, vals []uint16) error {
	n := len(vals)
	req := make([]byte, 7, 9+2*n)
	req[0], req[1] = slaveAddr, 0x10
	binary.BigEndian.PutUint16(req[2:], start)
	binary.BigEndian.PutUint16(req[4:], uint16(n))
	req[6] = byte(2 * n)
	for _, v := range vals {
		req = binary.BigEndian.AppendUint16(req, v)
	}
	req = binary.LittleEndian.AppendUint16(req, crc16(req))
	_, err := d.transact(req, 8)
	return err
}

// Poll reads the whole monitor block in one transaction.
func (d *Device) Poll() (Reading, error) {
	r, err := d.ReadRegs(regVSet, monitorBlockLen)
	if err != nil {
		return Reading{}, err
	}
	return Reading{
		VSet:    float64(r[regVSet]) / 100,
		ISet:    float64(r[regISet]) / 1000,
		VOut:    float64(r[regVOut]) / 100,
		IOut:    float64(r[regIOut]) / 1000,
		Power:   float64(r[regPower]) / 10,
		VIn:     float64(r[regVIn]) / 100,
		MilliAh: uint32(r[regAhHi])<<16 | uint32(r[regAhLo]),
		MilliWh: uint32(r[regWhHi])<<16 | uint32(r[regWhLo]),
		Hours:   int(r[regOutH]),
		Mins:    int(r[regOutM]),
		Secs:    int(r[regOutS]),
		TempIn:  float64(r[regTIn]) / 10,
		CC:      r[regCVCC] == 1,
		On:      r[regOnOff] == 1,
		Protect: int(r[regProtect]),
	}, nil
}

// ReadPreset reads one memory group (0 = the live set).
func (d *Device) ReadPreset(n int) (Preset, error) {
	r, err := d.ReadRegs(presetReg(n, 0), presetRegs)
	if err != nil {
		return Preset{}, err
	}
	return decodePreset(r), nil
}

// ReadSettings reads the live profile: the ACTIVE data group's block. On
// this firmware the active group (register 0x1D) is what the protections,
// limits and power-on flag come from — the 0x50 block is merely group 0's
// storage (verified: a recall changes 0x1D and the working setpoints but
// never the 0x50 block; the power-on flag of the active group governs the
// next boot).
func (d *Device) ReadSettings() (Settings, error) {
	r, err := d.ReadRegs(regExtract, 1)
	if err != nil {
		return Settings{}, err
	}
	n := int(r[0])
	if n > 9 {
		n = 0
	}
	return d.ReadPreset(n)
}

// ReadDeviceSettings reads the unit-wide settings block.
func (d *Device) ReadDeviceSettings() (DeviceSettings, error) {
	r, err := d.ReadRegs(deviceBlockOff, deviceBlockLen)
	if err != nil {
		return DeviceSettings{}, err
	}
	return decodeDeviceSettings(r), nil
}
