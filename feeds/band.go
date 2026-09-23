package main

// The strip across the top of the Feeds window, and the platform's three
// control buttons standing in it.
//
// This window takes the full-size-content treatment (main.go), so the native
// title bar is gone and the regions underneath reach the window's own top
// edge. Two of them do, not one: the rail's panel is set into the leading
// side and the navbar caps the content column beside it. What follows is the
// arithmetic that keeps those two halves reading as one band.

import (
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/pane"
)

// windowBandDp is the depth of that strip across the CONTENT column.
//
// It is the platform's, not a density's. The three control buttons stand a
// measured nineteen dp in from the window's own glass on both axes, so the
// band that holds them centred is nineteen above a fourteen dp circle and
// nineteen below it: 52, which is the band every stored toolbar capture
// measures — 8 px above a 36 px control and 8 below it. patterns/pane states
// that arithmetic once, as BandDp, because the panel's own strip is cut from
// the same inset, and this window reads it rather than restating it.
//
// The rail's half of the strip is the panel's own [pane.StripDp], which is
// this number less a margin at each end: the panel stands one margin inside
// the window's top edge, so the two halves hold one line between them — the
// buttons' centre — rather than one depth.
const windowBandDp = unit.Dp(pane.BandDp)

// windowButtonRun is where the platform's three control buttons stand once
// the native strip is gone: the top-leading corner of the window, which in
// this layout is the top-leading corner of the rail's panel — the buttons
// stand INSIDE the pane, as they do in every stored window whose sidebar is
// one.
//
// The run is patterns/pane's, which states the platform's measured inset
// once: the circles' own edges nineteen dp in from the window's glass on both
// axes, read off Finder, Mail, Notes and Voice Memos. The pane's strip is cut
// to that inset, so the buttons and the strip they stand in cannot drift
// apart. The drag claim asks the window where the run actually ended.
var windowButtonRun = pane.Buttons
