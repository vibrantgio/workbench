package main

import (
	"image"
	stdcolor "image/color"

	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/imageseed"
)

// ImageLoaded reports a picture read, decoded and reduced to its seed
// candidates. It carries the preview the window paints rather than the
// original: the original may be twenty megapixels, and nothing after this
// point needs a pixel of it.
type ImageLoaded struct {
	Path       string
	Preview    *image.NRGBA
	Candidates []imageseed.Candidate
}

// ImageRejected reports a drop that produced no candidates, with the reason
// in the words the window shows. It is a message rather than a command
// error: a file the user dropped by mistake must not end the application.
type ImageRejected struct {
	Path   string
	Reason string
}

// SelectCandidate chooses one of the extracted seeds by its position in the
// row. Emitted by a click on a swatch.
type SelectCandidate struct {
	Index int
}

// SelectBase chooses the syntax palette code is coloured from, by its
// position in the base selector. Emitted by a click on one of its rows.
//
// It carries the appearance the row was clicked under, because that is what
// the choice is for: the sun's list sets the light palette and the moon's the
// dark one. The row knows which list it is on; the reducer, which never sees a
// palette, does not.
type SelectBase struct {
	Index int
	Dark  bool
}

// KeepSeed asks for what is on screen to outlast the window: the chosen
// candidate and the chosen syntax bases are written to the kept-theme file,
// where every application that adopts a brand looks for one. Emitted by a
// click on the keep affordance.
type KeepSeed struct{}

// SeedKept reports what is now in that file.
type SeedKept struct {
	Seed  stdcolor.NRGBA
	Bases highlight.BasePair
}

// KeepFailed reports a keep that did not happen, with the reason in the
// words the window shows. Like a rejected drop it is a message and not a
// command error: a full disk is not a reason to close the window on
// somebody mid-decision.
type KeepFailed struct {
	Reason string
}

// SetScheme puts the window on one side of the light/dark pair and keeps it
// there. It carries the side to move to rather than a "flip it" instruction,
// because which side is showing depends on a palette the reducer never sees:
// the switch knows what it is drawn on, so it says where to go.
type SetScheme struct {
	Dark bool
}
