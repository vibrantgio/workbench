package main

import "time"

// The message vocabulary. The UI emits the Apply/Set/Toggle messages via
// mvu.MessageOp; the command layer answers with Connected, Polled,
// SettingsRead, PresetsRead, DeviceSettingsRead and Written.

// SetScreen navigates: a name from tabScreens.
type SetScreen struct {
	Screen string
}

// Connected reports the connect command's outcome; empty Err means the
// device answered.
type Connected struct {
	Err string
}

// Polled delivers one reading of the live block, or the transport error.
// At is stamped by the poll command when the reading arrived.
type Polled struct {
	R   Reading
	At  time.Time
	Err string
}

// SettingsRead delivers the live group, or the transport error.
type SettingsRead struct {
	S   Settings
	Err string
}

// PresetsRead delivers all ten memory groups, or the transport error.
type PresetsRead struct {
	Presets [10]Preset
	Err     string
}

// DeviceSettingsRead delivers the unit-wide settings, or the error.
type DeviceSettingsRead struct {
	D   DeviceSettings
	Err string
}

// Written reports a register write's outcome. What names the action for the
// notice line (empty = silent on success); the flags say which blocks to
// re-read.
type Written struct {
	What     string
	Err      string
	Settings bool // the live group changed
	Presets  bool // a stored memory group changed
	Device   bool // the device settings block changed
}

// Notice sets the feedback line without touching the device.
type Notice struct {
	Text string
}

// EnterDemo switches the app onto the simulated device — offered when no
// hardware answers. Restarting the app returns to real hardware.
type EnterDemo struct{}

// EditActivePreset opens the active preset — the settings in force — in
// the editor (the Monitor's Set button).
type EditActivePreset struct{}

// ApplyDevice writes one numeric device setting from a field's text.
type ApplyDevice struct {
	F    Field
	Text string
}

// SetSwitch writes a boolean setting.
type SetSwitch struct {
	S  Switch
	On bool
}

// ConfirmLVP is the override decision's confirm: write the pending cutoff
// even though it will trip protection immediately.
type ConfirmLVP struct{}

// DismissLVP cancels the pending cutoff write.
type DismissLVP struct{}

// ToggleOutput flips the output relay state.
type ToggleOutput struct{}

// ClearProtection resets a tripped protection state.
type ClearProtection struct{}

// RecallPreset makes memory group N (0..9) the live operating set.
type RecallPreset struct{ N int }

// EditPreset opens memory group N (0..9) in the editor; noEdit closes it.
type EditPreset struct{ N int }

// SavePresetEdit writes memory group N as one block, taking each field
// from its text where one was typed and from the stored group otherwise.
// Stay keeps the editor open afterwards (Return in a field); the Save
// button returns to the list.
type SavePresetEdit struct {
	N     int
	Texts map[Field]string
	Stay  bool
}
