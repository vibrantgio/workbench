package main

import (
	"image"
	stdcolor "image/color"

	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/imagecolor"
)

// ImageLoaded reports a picture read, decoded and reduced to the colours it
// offers. It carries the preview the window paints rather than the original:
// the original may be twenty megapixels, and nothing after this point needs a
// pixel of it.
type ImageLoaded struct {
	Path       string
	Preview    *image.NRGBA
	Candidates []imagecolor.Candidate
}

// ImageRejected reports a drop that produced no colours, with the reason in
// the words the window shows. It is a message rather than a command error: a
// file the user dropped by mistake must not end the application.
type ImageRejected struct {
	Path   string
	Reason string
}

// SelectCandidate makes one of a picture's colours the theme colour, by its
// position in the row. Emitted by a click on a swatch.
type SelectCandidate struct {
	Index int
}

// FollowSystem makes the platform's own accent colour the theme colour, which
// is what a kept theme records as following the system. Emitted by a click on
// the cell at the leading end of the row.
//
// It is not a candidate index because it is not a candidate: the colour it
// chooses is whatever the platform is set to at the moment it is drawn, and
// it goes on changing under a window that has chosen it.
type FollowSystem struct{}

// PlatformColorChanged reports the accent colour the platform now says an
// application should paint itself with — blue while macOS is on Multicolour.
// Emitted by the appearance stream the window subscribes, once at startup and
// again on every change.
//
// A zero alpha means this platform reports no such colour, which is how a
// window on a desktop that publishes none learns not to offer the choice.
type PlatformColorChanged struct {
	Color stdcolor.NRGBA
}

// HexTyped carries what is currently written in the colour field. Emitted on
// every change, because a field that only answered on a key nobody knows to
// press would be a field that does nothing.
type HexTyped struct {
	Text string
}

// SelectStyle chooses the highlighter style code is coloured from, by its
// position in the chooser's list. Emitted by a click on one of its rows.
//
// It carries the appearance the row was clicked under, because that is what
// the choice is for: the sun's list sets the light style and the moon's the
// dark one. The row knows which list it is on; the reducer, which never sees
// a style, does not.
type SelectStyle struct {
	Index int
	Dark  bool
}

// SelectMono chooses the typeface fenced code wears. Emitted by a click on
// one of the two names beside the sample. The name is one of the two the
// plate offers; anything else is ignored.
type SelectMono struct {
	Name string
}

// KeepColor asks for the theme colour on screen to outlast the window: it is
// written to the kept-theme file, where every application that adopts a brand
// looks for one. Emitted by a click on the keep affordance.
type KeepColor struct{}

// ColorKept reports what is now in that file: the colour, the syntax styles and
// the code face. Follows is the file holding the instruction to follow the
// system rather than a colour, in which case ThemeColor is the zero colour.
type ColorKept struct {
	ThemeColor stdcolor.NRGBA
	Follows    bool
	Styles     highlight.StylePair
	Mono       string
}

// KeepFailed reports a keep that did not happen, with the reason in the words
// the window shows. Like a rejected drop it is a message and not a command
// error: a full disk is not a reason to close the window on somebody
// mid-decision.
type KeepFailed struct {
	Reason string
}

// SetScheme puts the syntax style group on one side of the platform's pair and
// keeps it there. It carries the side to move to rather than a "flip it"
// instruction, because which side is showing depends on a set the reducer
// never sees: the switch knows what it is drawn on, so it says where to go.
type SetScheme struct {
	Dark bool
}
