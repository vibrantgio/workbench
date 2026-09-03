package main

import "time"

// Sample is one point of the chart history: what the output measured at one
// poll. The command layer stamps At; the reducer never reads the clock.
type Sample struct {
	At   time.Time
	V, I float64
}

// Model is the single application state. Every mutation flows through
// Update; the poll chain and the write commands deliver their results as
// messages, never by touching this directly.
type Model struct {
	// Screen selects which tab is on screen: "monitor", "presets" or
	// "device".
	Screen string

	// Status is the connection line in the header: "connecting…",
	// "online", or the last transport error in plain words.
	Status string

	// Online is true while polls are succeeding.
	Online bool

	// PollCount counts successful polls; the slower re-reads of the
	// profile, device settings and presets hang off it so edits made on
	// the unit's own panel show up in the app.
	PollCount int

	// Demo is true while the app talks to the simulated device instead of
	// hardware (entered via the demo button or the "demo" argument; a
	// restart returns to real hardware).
	Demo bool

	// R is the latest poll of the live block; HaveR reports whether one
	// has arrived yet.
	R     Reading
	HaveR bool

	// S is the live group (M0): the setpoints, protections and limits in
	// force; HaveS likewise.
	S     Settings
	HaveS bool

	// D is the unit-wide settings block; HaveD likewise.
	D     DeviceSettings
	HaveD bool

	// History is the rolling chart window, oldest first — appended per
	// poll, trimmed by count and age in the reducer.
	History []Sample

	// Presets holds memory groups M0..M9; HavePresets reports whether the
	// read after connect has landed.
	Presets     [10]Preset
	HavePresets bool

	// EditPreset is the memory group open in the preset editor (0..9), or
	// noEdit while the Presets tab shows the list.
	EditPreset int
	// EditClears counts the moments the editor's fields should be emptied:
	// a save accepted, or the editor closed. The view compares it frame to
	// frame; the fields' text lives in the view, not here.
	EditClears int

	// The LVP override decision: setting the input cutoff above the live
	// input voltage trips protection and cuts the output the moment it is
	// written, so that write waits here for explicit confirmation.
	// LVPPending is the value awaiting the decision and LVPPendingVIn the
	// input voltage it was judged against (snapshotted so the dialog text
	// holds still while polls continue).
	LVPConfirmOpen bool
	LVPPending     float64
	LVPPendingVIn  float64
	LVPPendingSet  Preset // the whole preset write waiting on the decision
	LVPPendingN    int    // the memory group it goes to
	LVPPendingStay bool   // whether the editor stays open once written

	// Notice is the last action's feedback line ("voltage set ✓",
	// "current: not a number", …).
	Notice string
}
