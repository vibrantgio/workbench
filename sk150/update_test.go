package main

import "testing"

// deviceOnGroup is a model whose device block names M3 as the group in
// force, so EditActivePreset has a group to open.
func deviceOnGroup(n int) Model {
	m, _ := Init()
	m.HaveD = true
	m.D.Group = n
	m.HavePresets = true
	return m
}

// apply folds one message into the model, discarding the command: these
// tests read state, not effects.
func apply(m Model, msg any) Model {
	next, _ := Update(m, msg)
	return next
}

// TestLeavingTheEditorShowsTheListOfMemorySlots pins the result the Presets
// tab must give: whichever route opened the editor, moving to another screen
// cancels the edit, so returning to Presets shows the list.
func TestLeavingTheEditorShowsTheListOfMemorySlots(t *testing.T) {
	for _, tc := range []struct {
		name string
		from string // the tab the route is taken from
		open any
		want int
	}{
		{"Set on the Monitor", "monitor", EditActivePreset{}, 3},
		{"Edit on a slot", "presets", EditPreset{N: 7}, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := deviceOnGroup(3)
			m.Screen = tc.from
			m = apply(m, tc.open)
			if m.EditPreset != tc.want {
				t.Fatalf("editor opened on M%d, want M%d", m.EditPreset, tc.want)
			}
			if m.Screen != "presets" {
				t.Fatalf("editor left the screen on %q, want presets", m.Screen)
			}
			clears := m.EditClears

			m = apply(m, SetScreen{Screen: "monitor"})
			if m.EditPreset != noEdit {
				t.Errorf("leaving the tab kept M%d open, want the editor closed", m.EditPreset)
			}
			if m.EditClears != clears+1 {
				t.Errorf("EditClears is %d, want %d: what was typed is not dropped", m.EditClears, clears+1)
			}

			m = apply(m, SetScreen{Screen: "presets"})
			if m.EditPreset != noEdit {
				t.Errorf("the Presets tab reopened M%d, want the list of memory slots", m.EditPreset)
			}
			if m.EditClears != clears+1 {
				t.Errorf("returning to the list cleared again: EditClears is %d, want %d", m.EditClears, clears+1)
			}
		})
	}
}

// TestSetOnTheMonitorOpensTheEditor keeps the one route from the Monitor: the
// Set button opens the group in force on the Presets tab.
func TestSetOnTheMonitorOpensTheEditor(t *testing.T) {
	m := deviceOnGroup(5)
	m.Screen = "monitor"

	m = apply(m, EditActivePreset{})
	if m.Screen != "presets" {
		t.Errorf("Set left the app on %q, want presets", m.Screen)
	}
	if m.EditPreset != 5 {
		t.Errorf("Set opened M%d, want the active group M5", m.EditPreset)
	}
}
