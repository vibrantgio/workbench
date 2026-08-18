package main

import (
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// Model is the whole application state: the picture last dropped, the seed
// candidates extracted from it, which of them is chosen, and whether a drag
// is hovering over the window right now.
type Model struct {
	// Preview is the dropped picture shrunk to something a window can
	// paint every frame; nil until an image loads.
	Preview *image.NRGBA
	// Name is the dropped file's base name, shown beside the picture.
	Name string
	// Candidates are the extracted seeds, most prominent first.
	Candidates []imageseed.Candidate
	// Selected indexes Candidates. It is 0 on a fresh extraction, so the
	// leading candidate is the one the window shows itself in.
	Selected int
	// DragOver is true while a file drag hovers over the window, and is
	// what the drop zone highlights on.
	DragOver bool
	// Scheme is which side of the light/dark pair the window draws in.
	// FollowOS until the switch in the window is pressed, and the window's
	// own answer from then on: judging a seed means seeing both sides of it,
	// and waiting for the desktop to change its mind is not a way to do that.
	Scheme Scheme
	// Problem describes the last drop that did not become candidates —
	// an unreadable file, a format nothing here decodes, a picture with no
	// opaque pixels. Empty when the last drop worked.
	Problem string
}

// Scheme names which side of a light/dark pair is shown, and whether that is
// the window's decision or the desktop's.
type Scheme int

const (
	// FollowOS takes the side the desktop is set to.
	FollowOS Scheme = iota
	// ShowLight and ShowDark override it, in the window, for as long as it
	// is open.
	ShowLight
	ShowDark
)

// Dark reports whether the window is drawing the dark side, given the palette
// the OS handed over. The OS decides until the window overrides it.
func (m Model) Dark(os tokens.ColorTokens) bool {
	switch m.Scheme {
	case ShowLight:
		return false
	case ShowDark:
		return true
	}
	return isDark(os)
}

// Seed returns the chosen candidate's colour, and whether there is one. No
// candidates, or an index no longer in range, means no seed and the window
// stays in the OS palette.
func (m Model) Seed() (stdcolor.NRGBA, bool) {
	if m.Selected < 0 || m.Selected >= len(m.Candidates) {
		return stdcolor.NRGBA{}, false
	}
	return m.Candidates[m.Selected].Color, true
}

// Init returns the empty model, plus the load of a picture named on the
// command line if there is one — the same command a drop runs, so a path
// argument and a dragged file reach the application identically.
func Init() (Model, mvu.Command) {
	if len(os.Args) > 1 {
		return Model{}, LoadImage(os.Args[1])
	}
	return Model{}, mvu.DoNothing()
}

// shortName is what the window shows for a loaded picture: the file's own
// name without the directory it happened to sit in.
func shortName(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
