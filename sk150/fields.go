package main

// The numeric settings the UI writes, as data: each Field is either a slot
// in a memory group (the Limits tab writes it in M0, the preset editor in
// Mn) or a device-wide register. Row captions, placeholders, ranges,
// scaling and display formats all come from this table, so a new setting
// is one line here plus one row in the view.

import (
	"fmt"
	"math"
)

type Field int

const (
	FVSet Field = iota
	FISet
	FLVP
	FOVP
	FOCP
	FOPP
	FOTP
	FOHPH
	FOHPM
	FOAH // entered in Ah, stored as a 32-bit mAh pair
	FOWH // entered in Wh, stored as a 32-bit 10 mWh pair
	FMPPTPct
	FBatCutoff
	FCPWatts
	FBacklight
	FSleep
	fieldCount
)

type fieldSpec struct {
	Key      string  // widget slot key
	Label    string  // row caption
	Short    string  // the device's own name, for compact grids
	Unit     string  // field placeholder
	Lo, Hi   float64 // accepted range in user units
	Scale    float64 // raw = round(value × Scale)
	Format   string  // "now" display of the user-unit value
	GroupOff int     // offset within a memory group, or -1 for a device register
	Reg      uint16  // the device register when GroupOff is -1
	Wide     bool    // a 32-bit pair at GroupOff (low word) and GroupOff+1
}

var fieldSpecs = [fieldCount]fieldSpec{
	FVSet:      {"vset", "Voltage setpoint", "V-SET", "volts", 0, 36, 100, "%.2f V", offVSet, 0, false},
	FISet:      {"iset", "Current limit", "I-SET", "amps", 0, 7.1, 1000, "%.3f A", offISet, 0, false},
	FLVP:       {"lvp", "Input cutoff LVP", "LVP", "volts", 0, 36, 100, "%.2f V", offLVP, 0, false},
	FOVP:       {"ovp", "Over-voltage OVP", "OVP", "volts", 0, 38, 100, "%.2f V", offOVP, 0, false},
	FOCP:       {"ocp", "Over-current OCP", "OCP", "amps", 0, 7.2, 1000, "%.3f A", offOCP, 0, false},
	FOPP:       {"opp", "Over-power OPP", "OPP", "watts", 0, 150, 10, "%.1f W", offOPP, 0, false},
	FOTP:       {"otp", "Over-temperature OTP", "OTP", "degrees", 0, 120, 1, "%.0f°", offOTP, 0, false},
	FOHPH:      {"ohph", "Max output time, hours", "OHP h", "hours", 0, 999, 1, "%.0f h", offOHPH, 0, false},
	FOHPM:      {"ohpm", "Max output time, minutes", "OHP m", "minutes", 0, 59, 1, "%.0f min", offOHPM, 0, false},
	FOAH:       {"oah", "Max output charge", "OAH", "Ah (0 = off)", 0, 4000, 1000, "%.3f Ah", offOAHL, 0, true},
	FOWH:       {"owh", "Max output energy", "OWH", "Wh (0 = off)", 0, 40000, 100, "%.2f Wh", offOWHL, 0, true},
	FMPPTPct:   {"mppt", "MPPT set point", "PPT %", "% of Voc", 0, 100, 1, "%.0f %%", -1, regMPPTPct, false},
	FBatCutoff: {"batcut", "Battery-full cutoff", "bTF", "amps", 0, 7.1, 1000, "%.3f A", -1, regBatCutoff, false},
	FCPWatts:   {"cpw", "Constant power set point", "CP W", "watts", 0, 150, 10, "%.1f W", -1, regCPWatts, false},
	FBacklight: {"bled", "Backlight level", "b-L", "0–5", 0, 5, 1, "%.0f", -1, regBacklight, false},
	FSleep:     {"sleep", "Screen sleep", "SLP", "minutes", 0, 999, 1, "%.0f min", -1, regSleep, false},
}

// presetFields lists the group fields in editor order.
var presetFields = []Field{FVSet, FISet, FLVP, FOVP, FOCP, FOPP, FOTP, FOHPH, FOHPM, FOAH, FOWH}

// limitFields is the Limits tab's order: protections first, then the
// output budget limits.
var limitFields = []Field{FLVP, FOVP, FOCP, FOPP, FOTP, FOHPH, FOHPM, FOAH, FOWH}

// deviceFields is the Device tab's numeric rows.
var deviceFields = []Field{FMPPTPct, FBatCutoff, FCPWatts, FBacklight, FSleep}

func (f Field) spec() fieldSpec { return fieldSpecs[f] }

// Now formats the field's current value for the "now …" column.
func (f Field) Now(v float64) string { return "now " + fmt.Sprintf(f.spec().Format, v) }

// Value reads a group field in user units.
func (p Preset) Value(f Field) float64 {
	switch f {
	case FVSet:
		return p.VSet
	case FISet:
		return p.ISet
	case FLVP:
		return p.LVP
	case FOVP:
		return p.OVP
	case FOCP:
		return p.OCP
	case FOPP:
		return p.OPP
	case FOTP:
		return p.OTP
	case FOHPH:
		return float64(p.OHPH)
	case FOHPM:
		return float64(p.OHPM)
	case FOAH:
		return float64(p.OAH) / 1000
	case FOWH:
		return float64(p.OWH) / 100
	}
	return 0
}

// With returns the group with one field replaced (user units).
func (p Preset) With(f Field, v float64) Preset {
	switch f {
	case FVSet:
		p.VSet = v
	case FISet:
		p.ISet = v
	case FLVP:
		p.LVP = v
	case FOVP:
		p.OVP = v
	case FOCP:
		p.OCP = v
	case FOPP:
		p.OPP = v
	case FOTP:
		p.OTP = v
	case FOHPH:
		p.OHPH = int(v)
	case FOHPM:
		p.OHPM = int(v)
	case FOAH:
		p.OAH = uint32(math.Round(v * 1000))
	case FOWH:
		p.OWH = uint32(math.Round(v * 100))
	}
	return p
}

// Value reads a device field in user units.
func (d DeviceSettings) Value(f Field) float64 {
	switch f {
	case FMPPTPct:
		return float64(d.MPPTPercent)
	case FBatCutoff:
		return d.BatteryCutoff
	case FCPWatts:
		return d.CPWatts
	case FBacklight:
		return float64(d.Backlight)
	case FSleep:
		return float64(d.SleepMinutes)
	}
	return 0
}

// Switch identifies one boolean setting the toggle switches drive.
type Switch int

const (
	SwMPPT Switch = iota
	SwCP
	SwBeeper
	SwFahrenheit
	SwLock
	SwPowerOn       // the live group's output-on-at-power-up
	SwPresetPowerOn // the same flag of the preset being edited
	switchCount
)

type switchSpec struct {
	Key   string
	Label string
	Short string // the device's own menu name
}

var switchSpecs = [switchCount]switchSpec{
	SwMPPT:          {"sw.mppt", "MPPT solar tracking", "PPT"},
	SwCP:            {"sw.cp", "Constant power mode", "CP"},
	SwBeeper:        {"sw.beep", "Beeper", "bEP"},
	SwFahrenheit:    {"sw.fahr", "Temperatures in °F", "C-F"},
	SwLock:          {"sw.lock", "Key lock", "LOCK"},
	SwPowerOn:       {"sw.ini", "Output on at power-up", "POn"},
	SwPresetPowerOn: {"sw.pini", "Output on at power-up", "POn"},
}

// state reads the switch's current value off the model.
func (s Switch) state(m Model) bool {
	switch s {
	case SwMPPT:
		return m.D.MPPTOn
	case SwCP:
		return m.D.CPOn
	case SwBeeper:
		return m.D.Beeper
	case SwFahrenheit:
		return m.D.Fahrenheit
	case SwLock:
		return m.D.KeyLock
	case SwPowerOn:
		return m.S.PowerOn
	case SwPresetPowerOn:
		if isGroup(m.EditPreset) {
			return m.Presets[m.EditPreset].PowerOn
		}
	}
	return false
}
