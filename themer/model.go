package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"

	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/brand"
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
	// Problem describes the last thing that did not work — a drop that
	// became no candidates (an unreadable file, a format nothing here
	// decodes, a picture with no opaque pixels), or a theme that could not
	// be kept. Empty when the last thing worked.
	Problem string
	// KeepPath is the file a kept theme is written to, resolved once at
	// startup. Empty when this machine has no config directory to put it
	// in, which is the one way keeping can fail before it is tried.
	KeepPath string
	// Kept is the colour that file currently holds: read from it at
	// startup and replaced by every keep that succeeds, so the window can
	// say whether what is on screen is what would come back. A zero alpha
	// means nothing has been kept.
	Kept stdcolor.NRGBA
	// KeptBase is the syntax base that file currently holds, resolved the
	// same way Base is. It sits beside Kept for the same reason: the keep
	// affordance confirms when everything on screen is what is on disk,
	// and the base is now part of everything.
	KeptBase string
	// Bases are the syntax palettes a fence can be coloured from — the
	// ones that ship embedded and the ones read out of the styles folder —
	// in the order the selector lists them.
	Bases []BaseOption
	// BaseAt indexes Bases. It starts on whatever was kept, so a window
	// opens showing the code the way the last one left it.
	BaseAt int
}

// BaseOption is one row of the base selector: a syntax palette's name, whether
// it came out of the styles folder rather than shipping embedded, and which
// appearances it is offered under. Where a style came from changes nothing
// about how it is used; it is worth showing because it is the difference
// between a name somebody recognises and one they put there themselves.
type BaseOption struct {
	Name  string
	Added bool
	// Light and Dark are measured off the style's own ground, once, when the
	// list is built — the answer cannot change while the window is open, and
	// measuring seventy-four backgrounds per frame to learn that would be a
	// waste of a frame. A style fitted to no ground of its own carries both.
	Light bool
	Dark  bool
}

// Suits reports whether this base is one to offer under the appearance on
// screen.
func (o BaseOption) Suits(dark bool) bool {
	if dark {
		return o.Dark
	}
	return o.Light
}

// Base is the syntax palette the code on screen is coloured from. An index
// out of range — no styles at all, which no build has — falls back to the
// highlighter's own default, because there is always a base.
func (m Model) Base() string {
	if m.BaseAt < 0 || m.BaseAt >= len(m.Bases) {
		return highlight.DefaultBase
	}
	return m.Bases[m.BaseAt].Name
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

// Init returns the model the window starts from — where a kept theme is
// written, which colour and syntax base are already there, and every base
// there is to choose from — plus the load of a picture named on the command
// line if there is one, the same command a drop runs, so a path argument and
// a dragged file reach the application identically.
//
// A machine with no config directory to write to is not a reason to refuse
// to start: the path stays empty, the window works, and the one thing it
// cannot do says so when it is asked.
//
// The styles folder is read here, once, before the first frame. It is a
// handful of small files in a directory most people do not have, and reading
// it later would mean a selector that grows a row while somebody is reading
// it. A file in it that will not parse is named in the caption rather than
// thrown: the other styles loaded, and the window is still worth looking at.
func Init() (Model, mvu.Command) {
	m := Model{}
	if dir, err := brand.StylesDir(); err == nil {
		if _, skipped := highlight.LoadDir(dir); len(skipped) > 0 {
			m.Problem = skippedSentence(skipped)
		}
	}
	m.Bases = baseOptions()
	kept := brand.Brand{}
	if path, err := brand.Path(); err == nil {
		m.KeepPath = path
		kept = brand.KeptFrom(path)
	}
	m = m.adoptKept(kept)
	if len(os.Args) > 1 {
		return m, LoadImage(os.Args[1])
	}
	return m, mvu.DoNothing()
}

// adoptKept folds what is already in the kept-theme file into the model: the
// colour it holds, and the syntax base it names resolved against what this
// build can actually derive from. A name nothing resolves — a style whose
// file has left the folder, one written by a build that had it — opens the
// window on the default rather than on whatever sorted first, because the
// default is what everything else showing that file's theme will use.
func (m Model) adoptKept(kept brand.Brand) Model {
	m.Kept = kept.Seed
	m.KeptBase = highlight.BaseOrDefault(kept.Base)
	m.BaseAt = baseIndex(m.Bases, m.KeptBase)
	return m
}

// baseOptions is every syntax palette on offer, in the order the highlighter
// lists them, marked with where each came from and which appearance it was
// fitted to.
func baseOptions() []BaseOption {
	names := highlight.Bases()
	out := make([]BaseOption, len(names))
	for i, n := range names {
		out[i] = BaseOption{
			Name:  n,
			Added: highlight.Loaded(n),
			Light: highlight.BaseSuits(n, false),
			Dark:  highlight.BaseSuits(n, true),
		}
	}
	return out
}

// VisibleBases are the rows the selector lists under the appearance on screen,
// as indices into Bases: every base fitted to that appearance, and the applied
// one whatever it was fitted to.
//
// The applied base is never taken off the list. Half the names go when the
// scheme flips, and the one the code on screen is actually coloured with can be
// among them — a light base is still light when the moon is showing, and the
// derivation goes on reaching its dark counterpart from that same name. A list
// that dropped it would leave nothing marked, the page coloured by something
// the column no longer admits to, and no way back to it except flipping the
// scheme again. So it stays, in its own sorted place, marked as the choice it
// is. Flipping the scheme changes what the window offers, never what was
// chosen.
func (m Model) VisibleBases(dark bool) []int {
	out := make([]int, 0, len(m.Bases))
	for i, b := range m.Bases {
		if b.Suits(dark) || i == m.BaseAt {
			out = append(out, i)
		}
	}
	return out
}

// baseIndex finds a base by name, and falls back to the default's position
// rather than to zero: position zero is whatever sorted first, which is a
// style nobody chose.
func baseIndex(bases []BaseOption, name string) int {
	for i, b := range bases {
		if b.Name == name {
			return i
		}
	}
	for i, b := range bases {
		if b.Name == highlight.DefaultBase {
			return i
		}
	}
	return 0
}

// skippedSentence says which files in the styles folder did not load and
// why. One file is named with its reason; more than one names the first and
// counts the rest, because the caption is a line and not a log.
func skippedSentence(skipped []highlight.Skipped) string {
	if len(skipped) == 1 {
		return "style not loaded — " + skipped[0].String()
	}
	return fmt.Sprintf("%d styles not loaded — %s, and %d more",
		len(skipped), skipped[0], len(skipped)-1)
}

// SeedIsKept reports whether what is on screen is what is already in the
// kept-theme file — the colour and the syntax base both, since both are
// written and both come back. It is the difference between an affordance
// offering something and one confirming it.
func (m Model) SeedIsKept() bool {
	seed, ok := m.Seed()
	return ok && m.Kept.A != 0 && m.Kept == seed && m.KeptBase == m.Base()
}

// shortName is what the window shows for a loaded picture: the file's own
// name without the directory it happened to sit in.
func shortName(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
