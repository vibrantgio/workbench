package main

import (
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/imageseed"
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
	// Problem describes the last drop that did not become candidates —
	// an unreadable file, a format nothing here decodes, a picture with no
	// opaque pixels. Empty when the last drop worked.
	Problem string
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
