package main

// Where the keyboard lands when the ambiguous-link chooser opens, read off
// the rendered frame. The chooser's body is a list of candidate notes, and a
// list at the front of a dialog is a focusable: it takes the keyboard when
// the dialog opens rather than leaving it on the header's close mark, wears
// the halo on its own box and shows its first row selected.

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/vibrantgio/theme/tokens"
)

// chooserFocusModel is the golden window with the chooser raised over it:
// the reader followed a wikilink whose file part matches three notes, so the
// dialog asks which one was meant.
func chooserFocusModel() Model {
	m, _ := Update(goldenModel(), OpenChooser{
		Body: "Colour",
		Candidates: []string{
			"Design/notes/Colour.md",
			"Design/Colour.md",
			"guide/Colour.md",
		},
	})
	return m
}

// liveChooser is the chooser's layer.
func liveChooser(t *testing.T, m Model, c tokens.PlatformColors, shaper *text.Shaper) layout.Widget {
	t.Helper()
	return liveDialog(t, chooserLayer, m, c, shaper)
}

// TestTheChooserOpensOnItsListAndNotOnTheCloseMark reads the opening
// keyboard off the live chooser, in both schemes.
//
// As it opens, the halo stands around the candidate list and its first row
// wears the selection fill; one Down moves that selection one row on; one Tab
// carries the halo off the list and onto the header's close mark, which
// stands above the list rather than below it. So the chooser opens on its
// rows, and the close mark is a Tab away.
func TestTheChooserOpensOnItsListAndNotOnTheCloseMark(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := chooserFocusModel()
	down := key.Event{Name: key.NameDownArrow, State: key.Press}
	tab := key.Event{Name: key.NameTab, State: key.Press}

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			ring := haloRing(tc.colors)
			cursor := tc.colors.SelectedContentBackground

			opened := settledDialog(t, liveChooser(t, m, tc.colors, shaper), tc.colors)
			halo := colorBox(opened, ring)
			if halo.Empty() {
				t.Fatal("the chooser opened with no halo anywhere in the window: nothing holds the keyboard")
			}
			row := colorBox(opened, cursor)
			if row.Empty() {
				t.Fatal("the chooser opened with no row selected: a list holding the keyboard shows which row it stands on")
			}
			if halo.Min.X != row.Min.X-2*haloOutside || halo.Max.X != row.Max.X+2*haloOutside {
				t.Errorf("the halo runs x %d-%d and the selected row x %d-%d: the band is not on the list's own box",
					halo.Min.X, halo.Max.X, row.Min.X, row.Max.X)
			}
			if halo.Min.Y != row.Min.Y-2*haloOutside {
				t.Errorf("the halo starts at y %d and the selected row at y %d: the list did not open on its FIRST row",
					halo.Min.Y, row.Min.Y)
			}
			rows := len(m.ChooserCandidates)
			if got, want := halo.Dy(), haloOutside+rows*chooserRowHDp+haloOutside; got != want {
				t.Errorf("the halo stands %d px tall; the list's box is %d rows of %d and the band %d px past it on each side",
					got, rows, chooserRowHDp, haloOutside)
			}

			readsBandOverFills(t, opened, tc.colors, halo, row, chooserRowHDp)

			walked := settledDialog(t, liveChooser(t, m, tc.colors, shaper), tc.colors, down)
			moved := colorBox(walked, cursor)
			if moved.Max.Y != row.Max.Y+chooserRowHDp {
				t.Errorf("one Down left the selection ending at y %d, from y %d: the arrows do not move the chooser's selection one row",
					moved.Max.Y, row.Max.Y)
			}

			tabbed := settledDialog(t, liveChooser(t, m, tc.colors, shaper), tc.colors, tab)
			next := colorBox(tabbed, ring)
			if next.Empty() {
				t.Fatal("Tab left no halo in the window: the close mark is not in the chooser's keyboard cycle")
			}
			if next.Max.Y > halo.Min.Y {
				t.Errorf("Tab left the halo at y %d-%d rather than above the list at y %d-%d: it did not move off the list onto the close mark",
					next.Min.Y, next.Max.Y, halo.Min.Y, halo.Max.Y)
			}
		})
	}
}

// TestTheChoosersListIsOneFocusTarget reads whether the candidate list
// registers a focus filter per row, by asking the router for the one move Tab
// makes and seeing where the halo lands.
//
// A list is ONE focusable wherever it stands, so the move carries the
// keyboard out of the list and onto the header's close mark, which stands
// above it. A focus filter per row — which is what a widget.Clickable
// registers — would hand the move to a row of the list instead, where nothing
// draws a halo at all.
func TestTheChoosersListIsOneFocusTarget(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := chooserFocusModel()

	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			ring := haloRing(tc.colors)

			d := newDialogDriver(t, liveChooser(t, m, tc.colors, shaper), tc.colors)
			opened := d.capture()
			if opened == nil {
				return // headless unavailable; Capture called t.Skip
			}
			halo := colorBox(opened, ring)
			if halo.Empty() {
				t.Fatal("the chooser opened with no halo anywhere in the window: nothing holds the keyboard")
			}

			d.moveForward()
			next := colorBox(d.capture(), ring)
			if next.Empty() {
				t.Fatal("one forward focus move left no halo in the window: the move landed on a row of the list, so the list is more than one focus target")
			}
			if next.Max.Y > halo.Min.Y {
				t.Errorf("one forward focus move left the halo at y %d-%d rather than above the list at y %d-%d: the list is more than one focus target",
					next.Min.Y, next.Max.Y, halo.Min.Y, halo.Max.Y)
			}
		})
	}
}
