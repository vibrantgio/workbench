package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/vibrantgio/mvu"
)

const (
	pollInterval = 400 * time.Millisecond

	// The chart history window: samples older than historyKeep fall off,
	// and the buffer never exceeds historyCap regardless of clock jumps.
	historyKeep = 15 * time.Minute
	historyCap  = 2400
)

// startScreen is the tab the app opens on; startDemo starts against the
// simulator instead of the hardware. Both may be set by main.go from the
// command line.
var (
	startScreen     = "monitor"
	startDemo       = false
	startEditPreset = noEdit
)

// noEdit is the EditPreset value while no memory group is open.
const noEdit = -1

// isGroup reports whether n names one of the ten memory groups M0..M9.
func isGroup(n int) bool { return n >= 0 && n <= 9 }

// Init seeds the model and starts the connect attempt.
func Init() (Model, mvu.Command) {
	return Model{Screen: startScreen, Demo: startDemo, EditPreset: startEditPreset, Status: "connecting…"}, connectCmd(0)
}

// statusOnline is the header line for a healthy link.
func statusOnline(m Model) string {
	// Kept short: the header centers the trip badge in the space after the
	// title block, and a long status line would push that centre off.
	if m.Demo {
		return "demo mode"
	}
	return "online"
}

// Update folds messages into the next model. The poll loop is a
// self-scheduling command chain: Connected starts it, every Polled returns
// the next poll. Writes run as their own commands; the device client's mutex
// serialises them against the chain.
func Update(m Model, message mvu.Message) (Model, mvu.Command) {
	switch msg := message.(type) {
	case SetScreen:
		m.Screen = msg.Screen
		return m, mvu.DoNothing()

	case Connected:
		if msg.Err != "" {
			m.Online = false
			m.Status = msg.Err
			return m, connectCmd(2 * time.Second)
		}
		m.Online = true
		m.Status = statusOnline(m)
		return m, mvu.DoConcurrent(readSettingsCmd(), readPresetsCmd(), readDeviceSettingsCmd(), pollCmd(0))

	case Polled:
		if msg.Err != "" {
			m.Online = false
			m.Status = msg.Err
			return m, pollCmd(time.Second)
		}
		m.R, m.HaveR = msg.R, true
		m.History = appendSample(m.History, Sample{At: msg.At, V: msg.R.VOut, I: msg.R.IOut})
		m.Online = true
		m.Status = statusOnline(m)
		m.PollCount++
		// Slow refresh of what the panel can change behind the app's back:
		// the active profile and device block every ~4 s, all presets
		// every ~16 s (ten reads — kept rare to leave the bus to polling).
		cmds := []mvu.Command{pollCmd(pollInterval)}
		if m.PollCount%10 == 0 {
			cmds = append(cmds, readSettingsCmd(), readDeviceSettingsCmd())
		}
		if m.PollCount%40 == 0 {
			cmds = append(cmds, readPresetsCmd())
		}
		if len(cmds) == 1 {
			return m, cmds[0]
		}
		return m, mvu.DoConcurrent(cmds...)

	case PresetsRead:
		if msg.Err != "" {
			m.Notice = "reading the presets failed: " + msg.Err
			return m, mvu.DoNothing()
		}
		m.Presets, m.HavePresets = msg.Presets, true
		return m, mvu.DoNothing()

	case SettingsRead:
		if msg.Err != "" {
			m.Notice = "reading the limits failed: " + msg.Err
			return m, mvu.DoNothing()
		}
		m.S, m.HaveS = msg.S, true
		return m, mvu.DoNothing()

	case DeviceSettingsRead:
		if msg.Err != "" {
			m.Notice = "reading the device settings failed: " + msg.Err
			return m, mvu.DoNothing()
		}
		m.D, m.HaveD = msg.D, true
		return m, mvu.DoNothing()

	case Written:
		if msg.Err != "" {
			what := msg.What
			if what == "" {
				what = "write"
			}
			m.Notice = what + " failed: " + msg.Err
			return m, mvu.DoNothing()
		}
		if msg.What != "" {
			m.Notice = msg.What + " ✓"
		}
		var after []mvu.Command
		if msg.Settings {
			after = append(after, readSettingsCmd())
		}
		if msg.Presets {
			after = append(after, readPresetsCmd())
		}
		if msg.Device {
			after = append(after, readDeviceSettingsCmd())
		}
		switch len(after) {
		case 0:
			return m, mvu.DoNothing()
		case 1:
			return m, after[0]
		default:
			return m, mvu.DoConcurrent(after...)
		}

	case Notice:
		m.Notice = msg.Text
		return m, mvu.DoNothing()

	case EnterDemo:
		if m.Demo {
			return m, mvu.DoNothing()
		}
		// Swap the bus to the simulator in the command world; the pending
		// connect retry (always armed while offline) then finds it within
		// two seconds and starts the normal chains against it.
		m.Demo = true
		m.Status = "starting demo…"
		return m, mvu.Do(func() (mvu.Message, error) {
			setBus(newSimDevice())
			return Notice{Text: "demo mode — no hardware is being touched"}, nil
		})

	case ToggleOutput:
		// The device refuses the ON command while a protection is tripped
		// — say so rather than letting the press die silently.
		if !m.R.On && m.R.Protect != 0 {
			if m.R.Protect == protectLVP {
				m.Notice = "output blocked by LVP — lower the input cutoff below the input to recover"
			} else {
				m.Notice = "output blocked by the " + ProtectName(m.R.Protect) + " trip — press Clear first"
			}
			return m, mvu.DoNothing()
		}
		// Silent on success: the header cluster already shows the new
		// state within a poll cycle.
		target := uint16(1)
		if m.R.On {
			target = 0
		}
		return m, writeCmd("", regOnOff, target, wflags{})

	case ClearProtection:
		return m, writeCmd("protection cleared", regProtect, 0, wflags{})

	case RecallPreset:
		if !isGroup(msg.N) {
			return m, mvu.DoNothing()
		}
		return m, writeCmd(fmt.Sprintf("preset M%d recalled — setpoints loaded, output off", msg.N),
			regExtract, uint16(msg.N), wflags{settings: true, device: true})

	case EditPreset:
		if !isGroup(msg.N) {
			msg.N = noEdit
		}
		if msg.N == noEdit && m.EditPreset != noEdit {
			m.EditClears++ // cancelled: drop what was typed
		}
		m.EditPreset = msg.N
		return m, mvu.DoNothing()

	case EditActivePreset:
		m.Screen = "presets"
		m.EditPreset = activeGroup(m)
		return m, mvu.DoNothing()

	case SavePresetEdit:
		if !isGroup(msg.N) || !m.HavePresets {
			return m, mvu.DoNothing()
		}
		p := m.Presets[msg.N]
		live := msg.N == activeGroup(m)
		for _, f := range presetFields {
			text := strings.TrimSpace(msg.Texts[f])
			if text == "" {
				continue // blank keeps the stored value
			}
			spec := f.spec()
			v, err := parseValue(text, spec.Lo, spec.Hi)
			if err != nil {
				m.Notice = strings.ToLower(spec.Label) + ": " + err.Error()
				return m, mvu.DoNothing()
			}
			p = p.With(f, v)
		}
		if p.VSet > p.OVP {
			m.Notice = fmt.Sprintf("voltage %.2f V exceeds OVP %.2f V — raise OVP too", p.VSet, p.OVP)
			return m, mvu.DoNothing()
		}
		if p.ISet > p.OCP {
			m.Notice = fmt.Sprintf("current %.3f A exceeds OCP %.3f A — raise OCP too", p.ISet, p.OCP)
			return m, mvu.DoNothing()
		}
		// An input cutoff above the live input, written into the profile
		// in force, trips LVP as soon as the output runs: ask first.
		if live && m.HaveR && p.LVP > m.R.VIn && p.LVP != m.S.LVP {
			m.LVPConfirmOpen = true
			m.LVPPending, m.LVPPendingVIn = p.LVP, m.R.VIn
			m.LVPPendingSet, m.LVPPendingN, m.LVPPendingStay = p, msg.N, msg.Stay
			return m, mvu.DoNothing()
		}
		// Saved: the typed values are on their way, so the fields empty;
		// back to the list unless Return in a field asked to stay.
		m.EditClears++
		if !msg.Stay {
			m.EditPreset = noEdit
		}
		return m, savePresetCmd(msg.N, p, live)

	case ApplyDevice:
		spec := msg.F.spec()
		v, err := parseValue(msg.Text, spec.Lo, spec.Hi)
		if err != nil {
			m.Notice = strings.ToLower(spec.Label) + ": " + err.Error()
			return m, mvu.DoNothing()
		}
		return m, writeFieldCmd(msg.F, -1, v, false)

	case SetSwitch:
		reg, fl, label, ok := switchTarget(msg.S, m)
		if !ok {
			return m, mvu.DoNothing()
		}
		val, state := uint16(0), "off"
		if msg.On {
			val, state = 1, "on"
		}
		return m, writeCmd(strings.ToLower(label)+" "+state, reg, val, fl)

	case ConfirmLVP:
		if !m.LVPConfirmOpen {
			return m, mvu.DoNothing()
		}
		m.LVPConfirmOpen = false
		if m.EditPreset == m.LVPPendingN {
			m.EditClears++
			if !m.LVPPendingStay {
				m.EditPreset = noEdit
			}
		}
		return m, savePresetCmd(m.LVPPendingN, m.LVPPendingSet, m.LVPPendingN == activeGroup(m))

	case DismissLVP:
		m.LVPConfirmOpen = false
		m.Notice = "input cutoff change cancelled"
		return m, mvu.DoNothing()

	default:
		return m, mvu.DoNothing()
	}
}

// activeGroup is the data group the unit is running on — the live profile
// — as last read from register 0x1D (group 0 until the device block lands).
func activeGroup(m Model) int {
	if m.HaveD && isGroup(m.D.Group) {
		return m.D.Group
	}
	return 0
}

// switchTarget maps a boolean setting to its register and the blocks a
// write invalidates.
func switchTarget(s Switch, m Model) (reg uint16, fl wflags, label string, ok bool) {
	label = switchSpecs[s].Label
	switch s {
	case SwMPPT:
		return regMPPTOn, wflags{device: true}, label, true
	case SwCP:
		return regCPOn, wflags{device: true}, label, true
	case SwBeeper:
		return regBeeper, wflags{device: true}, label, true
	case SwFahrenheit:
		return regTempUnit, wflags{device: true}, label, true
	case SwLock:
		return regLock, wflags{device: true}, label, true
	case SwPowerOn:
		return presetReg(activeGroup(m), offINI), wflags{settings: true, presets: true}, label, true
	case SwPresetPowerOn:
		if isGroup(m.EditPreset) {
			return presetReg(m.EditPreset, offINI), wflags{presets: true},
				fmt.Sprintf("M%d %s", m.EditPreset, label), true
		}
	}
	return 0, wflags{}, label, false
}

func parseValue(text string, lo, hi float64) (float64, error) {
	t := strings.TrimSpace(strings.ReplaceAll(text, ",", "."))
	if t == "" {
		return 0, fmt.Errorf("enter a value")
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return 0, fmt.Errorf("not a number")
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("out of range %g–%g", lo, hi)
	}
	return v, nil
}

func scaled(v, scale float64) uint16 {
	return uint16(math.Round(v * scale))
}

// appendSample appends one point and trims the window, pure: it drops
// samples older than historyKeep relative to the NEWEST sample (never the
// wall clock) and caps the length.
func appendSample(hist []Sample, s Sample) []Sample {
	next := make([]Sample, len(hist), len(hist)+1)
	copy(next, hist)
	next = append(next, s)
	cut := 0
	oldest := s.At.Add(-historyKeep)
	for cut < len(next) && next[cut].At.Before(oldest) {
		cut++
	}
	if over := len(next) - cut - historyCap; over > 0 {
		cut += over
	}
	return next[cut:]
}

// wflags says which blocks a write invalidates, so Written can re-read them.
type wflags struct {
	settings, presets, device bool
}

func connectCmd(after time.Duration) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		time.Sleep(after)
		if _, err := currentBus().Poll(); err != nil {
			return Connected{Err: humane(err)}, nil
		}
		return Connected{}, nil
	})
}

func pollCmd(after time.Duration) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		time.Sleep(after)
		r, err := currentBus().Poll()
		if err != nil {
			return Polled{Err: humane(err)}, nil
		}
		return Polled{R: r, At: time.Now()}, nil
	})
}

func readSettingsCmd() mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		s, err := currentBus().ReadSettings()
		if err != nil {
			return SettingsRead{Err: humane(err)}, nil
		}
		return SettingsRead{S: s}, nil
	})
}

func readPresetsCmd() mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		var ps [10]Preset
		for n := range ps {
			p, err := currentBus().ReadPreset(n)
			if err != nil {
				return PresetsRead{Err: humane(err)}, nil
			}
			ps[n] = p
		}
		return PresetsRead{Presets: ps}, nil
	})
}

func readDeviceSettingsCmd() mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		d, err := currentBus().ReadDeviceSettings()
		if err != nil {
			return DeviceSettingsRead{Err: humane(err)}, nil
		}
		return DeviceSettingsRead{D: d}, nil
	})
}

func writeCmd(what string, reg, val uint16, fl wflags) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		err := currentBus().WriteReg(reg, val)
		return Written{What: what, Err: humane(err), Settings: fl.settings, Presets: fl.presets, Device: fl.device}, nil
	})
}

// writeFieldCmd writes one numeric field in user units: into memory group
// `group` for group fields, or to its device register. live says the group
// is the active one — the profile in force — so the live blocks are
// re-read afterwards, and a voltage/current setpoint also goes to the
// working registers 0x00/0x01 first (on this firmware the group block is
// storage, not a mirror of the working setpoints).
func writeFieldCmd(f Field, group int, v float64, live bool) mvu.Command {
	spec := f.spec()
	what := fmt.Sprintf("%s set %s", strings.ToLower(spec.Label), fmt.Sprintf(spec.Format, v))
	var reg uint16
	var fl wflags
	switch {
	case spec.GroupOff < 0:
		reg, fl = spec.Reg, wflags{device: true}
	case live:
		reg, fl = presetReg(group, uint16(spec.GroupOff)), wflags{settings: true, presets: true}
	default:
		reg, fl = presetReg(group, uint16(spec.GroupOff)), wflags{presets: true}
		what = fmt.Sprintf("M%d %s", group, what)
	}
	raw := uint32(math.Round(v * spec.Scale))
	var also uint16
	if live && (f == FVSet || f == FISet) {
		also, reg = reg, regVSet
		if f == FISet {
			reg = regISet
		}
	}
	return mvu.Do(func() (mvu.Message, error) {
		var err error
		switch {
		case spec.Wide:
			err = currentBus().WriteRegs(reg, []uint16{uint16(raw & 0xFFFF), uint16(raw >> 16)})
		default:
			err = currentBus().WriteReg(reg, uint16(raw))
			if err == nil && also != 0 {
				err = currentBus().WriteReg(also, uint16(raw))
			}
		}
		return Written{What: what, Err: humane(err), Settings: fl.settings, Presets: fl.presets, Device: fl.device}, nil
	})
}

// savePresetCmd writes a whole memory group in one transaction. Verified
// on the hardware: when the group is the ACTIVE one the firmware mirrors
// its block into the working registers at once (0x00 followed a block
// write within 0.3 s), so the block write alone takes effect — no separate
// working-register write is needed. live says it is that group, so the
// live profile is re-read afterwards.
func savePresetCmd(n int, p Preset, live bool) mvu.Command {
	what := fmt.Sprintf("preset M%d saved", n)
	if live {
		what = fmt.Sprintf("preset M%d applied", n)
	}
	return mvu.Do(func() (mvu.Message, error) {
		err := currentBus().WriteRegs(presetReg(n, 0), encodePreset(p))
		return Written{What: what, Err: humane(err), Presets: true, Settings: live}, nil
	})
}
