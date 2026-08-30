package main

// The strip across the top of the Feeds window, and the platform's three
// control buttons standing in it.
//
// This window takes the full-size-content treatment (main.go), so the native
// title bar is gone and the regions underneath reach the window's own top
// edge. Two of them do, not one: the sidebar caps the leading side and the
// navbar caps the content region beside it. What follows is the arithmetic
// that keeps those two halves reading as a single band.

import (
	"gioui.org/unit"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/tokens"
)

// windowBandDp is the depth of that strip, the same on both sides of the
// sidebar's trailing edge.
//
// The two halves of a strip that crosses a seam may wear their own fills —
// here they wear the same one, since both regions are furniture — but not
// their own depths. A strip 52 dp deep on one side of the seam and 40 on the
// other is a step in the window's top edge rather than a band with a seam
// through it.
//
// The right half's depth is not this app's to choose. patterns/shell pins the
// navbar slot to shell.NavbarHeight, so that number IS the band, and the
// sidebar holds the same depth open on the other side of the seam by calling
// the same export rather than restating its arithmetic.
// TestTheWindowsTopStripIsOneBand measures both halves off a rendered frame
// at both densities, so a drift between them is caught where it would show.
func windowBandDp(d tokens.Density) unit.Dp {
	return shell.NavbarHeight(d)
}

// windowButtonRun is where the platform's three control buttons stand once
// the native strip is gone: the top-leading corner of the window, which in
// this layout is the top-leading corner of the sidebar.
//
// desktop.ButtonRunIn derives the whole run from the band's height alone —
// the platform centres the buttons in whatever band a window gives them and
// sets their leading inset equal to their top inset — so at a 52 dp band that
// is 19 dp in, 19 dp down, 14 dp across and 79 dp to the far edge of the
// third circle. Nothing else in this file needs to know those numbers; the
// sidebar keeps the whole band clear rather than dodging the run, and the
// drag claim asks the window where the run actually ended.
//
// The density is stated rather than read because the placement is a one-off
// call made before the window's first frame, when no theme emission has
// arrived yet. Comfortable is what the live theme emits and never stops
// emitting (theme/system), so it is also what the navbar beside the buttons
// is pinned at; a window that learned to switch density at runtime would have
// to re-place them on the change, and this app has no such control.
var windowButtonRun = desktop.ButtonRunIn(windowBandDp(tokens.Comfortable))
