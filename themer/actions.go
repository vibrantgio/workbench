package main

import (
	"image"

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

// SetScheme puts the window on one side of the light/dark pair and keeps it
// there. It carries the side to move to rather than a "flip it" instruction,
// because which side is showing depends on a palette the reducer never sees:
// the switch knows what it is drawn on, so it says where to go.
type SetScheme struct {
	Dark bool
}
