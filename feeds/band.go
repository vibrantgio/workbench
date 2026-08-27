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
	"github.com/vibrantgio/theme/tokens"
)

// windowBandDp is the depth of that strip, the same on both sides of the
// sidebar's trailing edge.
//
// ADR-021 R6 lets the two halves of a strip that crosses a seam wear their
// own fills — here they happen to wear the same one, since both regions are
// furniture — but not their own depths. A strip 52 dp deep on one side of the
// seam and 40 on the other is not a band with a seam through it; it is a step
// in the window's top edge, which is the defect the same arrangement in
// mindchat was built to avoid.
//
// The right half's depth is not this app's to choose. patterns/shell pins the
// navbar slot to the density's bar height, so that number IS the band and
// this restates its rule — ControlHeight + 2·PaddingY, which is 52 dp
// Comfortable and 40 dp Compact — so the sidebar can hold the same depth open
// on the other side of the seam. The rule is restated rather than imported
// because the shell keeps it unexported; TestTheWindowsTopStripIsOneBand
// measures both halves off a rendered frame at both densities, so a drift in
// either rule is caught where it would show rather than where it was written.
func windowBandDp(d tokens.Density) unit.Dp {
	return unit.Dp(d.ControlHeight + 2*d.PaddingY)
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
