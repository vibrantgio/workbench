package main

// The demo-mode device: a simulated SK150 behind the same Bus surface as
// the hardware. It keeps the memory groups and device settings as RAW
// registers and decodes them like the real client does, so whatever the
// app writes is read back exactly — including the fields it has just
// learned. On top: enough physics to make every screen worth looking at (a
// breathing load that crosses CV/CC on its own, temperature that follows
// dissipation, accumulating counters) and the unit's own habits (LVP
// re-trips while the condition persists and heals itself — output back on —
// once it passes, V-SET above OVP trips instead of
// clamping, a recall drops the output, the charge/energy/time/temperature
// limits trip, a battery-full cutoff ends the charge, constant-power mode
// caps the load).

import (
	"math"
	"sync"
	"time"
)

type simDevice struct {
	mu     sync.Mutex
	last   time.Time
	phase  float64
	groups [10][presetRegs]uint16 // M0 .. M9, raw storage
	dev    [deviceBlockLen]uint16 // regLock .. regCPWatts, raw
	vset   uint16                 // the working setpoints, 0x00/0x01
	iset   uint16

	on         bool
	protect    int
	cc         bool
	vin, temp  float64
	vout, iout float64
	mah, mwh   float64
	onFor      time.Duration
}

func newSimDevice() *simDevice {
	s := &simDevice{last: time.Now(), vin: 19.9, temp: 26, on: true}
	factory := Preset{VSet: 5, ISet: 7.1, LVP: 5.5, OVP: 38, OCP: 7.2, OPP: 150, OTP: 95}
	for n := range s.groups {
		copy(s.groups[n][:], encodePreset(factory))
	}
	copy(s.groups[0][:], encodePreset(factory.With(FVSet, 12)))
	copy(s.groups[8][:], encodePreset(factory.With(FVSet, 12)))
	copy(s.groups[9][:], encodePreset(factory.With(FVSet, 24)))
	s.dev[regBacklight-deviceBlockOff] = 5
	s.dev[regBeeper-deviceBlockOff] = 1
	s.dev[regMPPTPct-deviceBlockOff] = 80
	s.dev[regExtract-deviceBlockOff] = 8
	s.vset, s.iset = s.groups[8][offVSet], s.groups[8][offISet]
	return s
}

// active is the data group in force — the live profile, like the firmware.
func (s *simDevice) active() int { return int(s.dev[regExtract-deviceBlockOff]) % 10 }

// live is the active group's profile with the working setpoints in place.
func (s *simDevice) live() Preset {
	p := decodePreset(s.groups[s.active()][:])
	p.VSet, p.ISet = float64(s.vset)/100, float64(s.iset)/1000
	return p
}
func (s *simDevice) devSettings() DeviceSettings { return decodeDeviceSettings(s.dev[:]) }

// step advances the simulation to now; callers hold s.mu.
func (s *simDevice) step() {
	now := time.Now()
	dt := now.Sub(s.last).Seconds()
	s.last = now
	if dt <= 0 {
		return
	}
	if dt > 5 {
		dt = 5
	}
	s.phase += dt
	s.vin = 19.9 + 0.05*math.Sin(s.phase/7)
	live, dev := s.live(), s.devSettings()

	// The unit's protection habits: judged only while the output runs
	// (verified: an LVP above the input does not trip while idle); LVP
	// re-arms while the condition persists; the budget limits trip once
	// exceeded.
	// The LVP trip heals itself (verified on the hardware, 2026-09-02):
	// the moment the cutoff is back below the input the error clears and
	// the output resumes on its own.
	if s.protect == 4 && live.LVP <= s.vin {
		s.protect = 0
		s.on = true
	}

	if s.protect == 0 && s.on {
		switch {
		case live.LVP > s.vin:
			s.protect = 4
		case live.OAH > 0 && s.mah >= float64(live.OAH):
			s.protect = 5
		case (live.OHPH > 0 || live.OHPM > 0) &&
			s.onFor >= time.Duration(live.OHPH)*time.Hour+time.Duration(live.OHPM)*time.Minute:
			s.protect = 6
		case live.OTP > 0 && s.temp >= live.OTP:
			s.protect = 7
		case live.OWH > 0 && s.mwh/10 >= float64(live.OWH):
			s.protect = 9
		}
		if s.protect != 0 {
			s.on = false
		}
	}

	if !s.on || s.protect != 0 {
		s.vout, s.iout, s.cc = 0, 0, false
		s.temp += (26 - s.temp) * dt / 60
		return
	}

	// A wandering load resistance: the demo drifts between CV and CC.
	r := 2.4 + 1.6*math.Sin(s.phase/19)
	if load := live.VSet / r; load >= live.ISet {
		s.cc, s.iout, s.vout = true, live.ISet, live.ISet*r
	} else {
		s.cc, s.vout, s.iout = false, live.VSet, load
	}
	if dev.CPOn && dev.CPWatts > 0 && s.vout*s.iout > dev.CPWatts {
		s.iout = dev.CPWatts / s.vout
	}
	s.iout = math.Max(0, s.iout+0.002*math.Sin(s.phase*3))
	if dev.BatteryCutoff > 0 && !s.cc && s.iout < dev.BatteryCutoff && s.onFor > 10*time.Second {
		s.on = false // battery full: the charge ends
	}
	p := s.vout * s.iout
	s.temp += ((26 + p/6) - s.temp) * dt / 45
	s.mah += s.iout * dt / 3.6
	s.mwh += p * dt / 3.6
	s.onFor += time.Duration(dt * float64(time.Second))
}

func (s *simDevice) Poll() (Reading, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.step()
	live := s.live()
	secs := int(s.onFor.Seconds())
	return Reading{
		VSet: live.VSet, ISet: live.ISet,
		VOut: s.vout, IOut: s.iout, Power: s.vout * s.iout,
		VIn:     s.vin,
		MilliAh: uint32(s.mah), MilliWh: uint32(s.mwh),
		Hours: secs / 3600, Mins: secs / 60 % 60, Secs: secs % 60,
		TempIn: s.temp, CC: s.cc, On: s.on, Protect: s.protect,
	}, nil
}

func (s *simDevice) ReadSettings() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return decodePreset(s.groups[s.active()][:]), nil
}

func (s *simDevice) ReadPreset(n int) (Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return decodePreset(s.groups[n][:]), nil
}

func (s *simDevice) ReadDeviceSettings() (DeviceSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.devSettings(), nil
}

func (s *simDevice) WriteRegs(start uint16, vals []uint16) error {
	for i, v := range vals {
		if err := s.WriteReg(start+uint16(i), v); err != nil {
			return err
		}
	}
	return nil
}

func (s *simDevice) WriteReg(reg, val uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.step()
	switch {
	case reg == regVSet:
		// The real firmware trips OVP on a too-high programming request
		// rather than clamping it.
		if float64(val)/100 > s.live().OVP {
			s.protect, s.on = 1, false
		} else {
			s.vset = val
		}
	case reg == regISet:
		s.iset = val
	case reg == regOnOff:
		s.on = val == 1 && s.protect == 0
	case reg == regProtect:
		// Like the hardware (verified): clearing a trip zeroes the on-time
		// clock, so an OHP trip does not re-fire at once, and leaves the
		// output off whatever the power-on flag says.
		if val == 0 {
			s.protect = 0
			s.onFor = 0
		}
	case reg == regExtract:
		// Like the hardware (verified): a recall loads only the group's
		// voltage and current setpoints and drops the output — the live
		// protections and power-on flag stay as they are.
		if n := int(val); isGroup(n) {
			s.vset, s.iset = s.groups[n][offVSet], s.groups[n][offISet]
			s.on = false
			s.dev[regExtract-deviceBlockOff] = val
		}
	case reg >= deviceBlockOff && reg < deviceBlockOff+deviceBlockLen:
		s.dev[reg-deviceBlockOff] = val
	case reg >= presetBase && reg < presetBase+10*presetStride:
		n := int(reg-presetBase) / int(presetStride)
		if off := int(reg-presetBase) % int(presetStride); off < presetRegs {
			s.groups[n][off] = val
		}
	}
	return nil
}
