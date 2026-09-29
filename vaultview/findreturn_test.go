package main

import (
	"testing"

	"gioui.org/io/key"
)

// The find field's shortcut takes the keyboard from whatever holds it, so
// closing the field has to give it back there. These drive both columns
// against one router, as the rail-focus tests do, and read where the keys are
// on the frame after each press: a focus command reaches the router at the
// end of the frame that executed it.

// TestAClosedFindFieldGivesTheKeyboardBackToTheRail is the reader who was
// walking the rail: the shortcut takes them into the field, Escape puts them
// back on the rows they came from, and the arrows go on walking.
func TestAClosedFindFieldGivesTheKeyboardBackToTheRail(t *testing.T) {
	f := newRailFocusFrame(t)
	f.frame() // registers the tags and measures the rows

	f.clickRailRow(0)
	f.frame() // the frame the opened note's document arrives on
	f.frame() // and the frame that reads back what that one did with the keys
	if !f.onRail {
		t.Fatal("the rail does not hold the keyboard after a click on a row; there is nothing to give back")
	}

	f.pressMod(findKey, key.ModShortcut)
	if !f.find.open {
		t.Fatal("the platform's find shortcut did not open the field")
	}
	f.frame()
	if !f.inField {
		t.Fatal("the find field does not hold the keyboard after the shortcut opened it")
	}

	f.press(key.NameEscape)
	f.frame()
	if !f.onRail {
		t.Fatal("Escape left the rail without the keyboard; the field did not give it back to where it came from")
	}
	if f.inNote {
		t.Error("Escape put the keyboard in the note, which is not where the field took it from")
	}

	f.press(key.NameDownArrow)
	if got := f.v.list.Selected(); got != 1 {
		t.Errorf("Down left the rail's selection on row %d, want the second row: the arrows are dead after the field closed", got)
	}
	if !f.onRail {
		t.Error("the rail does not hold the keyboard after Down")
	}
}

// TestAClosedFindFieldLeavesTheKeyboardInTheNote is the other half: a reader
// who was in the document is handed back to the document, and the rail is
// left alone.
func TestAClosedFindFieldLeavesTheKeyboardInTheNote(t *testing.T) {
	f := newRailFocusFrame(t)
	f.frame() // registers the tags

	f.navigate(railFocusPaths[0])
	if !f.inNote {
		t.Fatal("the note column does not hold the keyboard on the first note it shows")
	}

	f.pressMod(findKey, key.ModShortcut)
	if !f.find.open {
		t.Fatal("the platform's find shortcut did not open the field")
	}
	f.frame()
	if !f.inField {
		t.Fatal("the find field does not hold the keyboard after the shortcut opened it")
	}

	f.press(key.NameEscape)
	f.frame()
	if !f.inNote {
		t.Error("Escape left the note without the keyboard")
	}
	if f.onRail {
		t.Error("Escape put the keyboard in the rail, which is not where the field took it from")
	}
}
