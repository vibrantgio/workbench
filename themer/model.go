package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	// Opened is the seed the window's own theme was built from, before the
	// first frame: the brand that was kept when it opened, and the zero
	// colour when nothing was. The desktop sends one side of that theme per
	// frame and never the other, so this is what the other side is derived
	// from while nothing is chosen.
	//
	// It is deliberately not Kept. Keeping replaces what is on disk and moves
	// that field; the stream this window flips was built once and goes on
	// deriving the pair it was built with, so a window that kept a colour and
	// then went back to the styles is still wearing the one it opened in.
	Opened stdcolor.NRGBA
	// KeptBases are the syntax bases that file currently holds, one per
	// appearance, resolved the same way the applied pair is. They sit
	// beside Kept for the same reason: the keep affordance confirms when
	// everything on screen is what is on disk, and the bases are now part
	// of everything.
	KeptBases highlight.BasePair
	// Bases are the syntax palettes a fence can be coloured from — the
	// ones that ship embedded and the ones read out of the styles folder —
	// in the order the selector lists them.
	Bases []BaseOption
	// Styles are those same palettes offered the other way round: as seeds,
	// one card each, in the order the grid lays them out. Every style with a
	// colour in it is here, embedded and folder-loaded alike.
	Styles []StyleCard
	// Style names the style the theme on screen was adopted from, and is
	// empty when the seed came out of a picture. It is what tells the window
	// to stand the style's own colours where the photograph would be, and
	// nothing else turns on it: a seed from a style is a seed.
	Style string
	// Mono is the typeface fenced code wears. Empty is Roboto Mono, the
	// default; the one other name this window applies is "JetBrains Mono".
	// It starts on whatever was kept.
	Mono string
	// KeptMono is the typeface that file currently holds, empty for
	// Roboto Mono, so the keep affordance can say whether the face on
	// screen is the one on disk.
	KeptMono string
	// LightAt and DarkAt index Bases, one per appearance: the palette the
	// code is coloured from under the sun, and the one it is coloured from
	// under the moon. They start on whatever was kept, so a window opens
	// showing the code the way the last one left it.
	//
	// Two of them because a syntax palette is fitted to a ground, so the
	// two appearances of one theme are two choices. Picking under the sun
	// moves one and picking under the moon the other; the scheme control
	// moves neither, and switches which is applied.
	LightAt, DarkAt int
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

// BaseAt is the row of Bases the given appearance is coloured from.
func (m Model) BaseAt(dark bool) int {
	if dark {
		return m.DarkAt
	}
	return m.LightAt
}

// Base is the syntax palette the code is coloured from under one appearance.
// An index out of range — no styles at all, which no build has — falls back to
// the highlighter's own default for that appearance, because there is always a
// base.
func (m Model) Base(dark bool) string {
	at := m.BaseAt(dark)
	if at < 0 || at >= len(m.Bases) {
		return highlight.DefaultBases().Base(dark)
	}
	return m.Bases[at].Name
}

// AppliedBases is the pair on screen: what the code is coloured from under
// each appearance. It is what the window draws through, what it keeps, and
// what it compares with the file to say whether the theme on screen is the one
// on disk.
func (m Model) AppliedBases() highlight.BasePair {
	return highlight.BasePair{Light: m.Base(false), Dark: m.Base(true)}
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
	m.Styles = styleCards()
	kept := brand.Brand{}
	if path, err := brand.Path(); err == nil {
		m.KeepPath = path
		kept = brand.KeptFrom(path)
	}
	m = m.adoptKept(kept)
	// The window's theme was built from this same file a moment ago, in the
	// call that opened the window, and it is built once. Recording the colour
	// it was built from is what lets the window show the other side of that
	// theme before anything has been chosen — the side the desktop, which
	// only ever sends the one it is set to, never hands over.
	m.Opened = kept.Seed
	if len(os.Args) > 1 {
		return m, LoadImage(os.Args[1])
	}
	return m, mvu.DoNothing()
}

// adoptKept folds what is already in the kept-theme file into the model: the
// colour it holds, and the syntax bases it names resolved against what this
// build can actually draw. A name nothing resolves — a style whose
// file has left the folder, one written by a build that had it — opens the
// window on the default rather than on whatever sorted first, because the
// default is what everything else showing that file's theme will use. So does
// a name fitted to the appearance it is not kept for, which is what a file
// naming one base with no appearance attached comes back as: the name stands
// on the half it was measured to belong on, and the other half opens on its
// own default.
func (m Model) adoptKept(kept brand.Brand) Model {
	m.Kept = kept.Seed
	m.KeptBases = highlight.BasesOrDefault(kept.Base.Names())
	m.LightAt = baseIndex(m.Bases, m.KeptBases.Light, false)
	m.DarkAt = baseIndex(m.Bases, m.KeptBases.Dark, true)
	// Unknown, empty, or "Roboto Mono" all open on Roboto Mono. Only
	// JetBrains Mono is a selection this window can restore.
	m.Mono = ""
	if kept.Mono == tokens.CodeFaceJetBrains {
		m.Mono = tokens.CodeFaceJetBrains
	}
	m.KeptMono = m.Mono
	return m
}

// AppliedMono is the typeface name the specimen wears: JetBrains Mono
// when that is selected, Roboto Mono otherwise.
func (m Model) AppliedMono() string {
	if m.Mono == tokens.CodeFaceJetBrains {
		return tokens.CodeFaceJetBrains
	}
	return tokens.CodeFaceRoboto
}

// keepMono is what Keep writes: "JetBrains Mono", or empty for Roboto
// Mono. Empty is how the file spells the default, so a file without the
// key and a file that chose Roboto Mono come back the same way.
func (m Model) keepMono() string {
	if m.AppliedMono() == tokens.CodeFaceJetBrains {
		return tokens.CodeFaceJetBrains
	}
	return ""
}

// styleNames is every syntax palette there is, in the order this window lists
// them: by name, the way somebody scanning for one reads names.
//
// It is not the byte order the names arrive in, and the difference is not
// academic. An underscore sorts under every letter, so a name that carries one
// lands ahead of every name that continues with a letter past that point —
// which puts hr_high_contrast in front of hrdark, two rows up from where a
// reader running down the h's for "hrdark" would stop looking. Separators are
// not what a name is looked up by, so they are left out of the comparison
// entirely and the raw name breaks the tie, which keeps the order total and
// the list the same list on every machine.
//
// Both places the styles are offered — the grid of cards and the list beside
// the code — read this, because two orderings of one set inside one window is
// the same defect as no ordering at all.
func styleNames() []string {
	names := highlight.Bases()
	slices.SortStableFunc(names, func(a, b string) int {
		if c := strings.Compare(lookupKey(a), lookupKey(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return names
}

// lookupKey is a style name reduced to what it is looked up by: its letters and
// digits, folded to one case.
func lookupKey(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(name) {
		if r != '-' && r != '_' && r != ' ' && r != '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// baseOptions is every syntax palette on offer, in the order the window lists
// them, marked with where each came from and which appearance it was fitted to.
func baseOptions() []BaseOption {
	names := styleNames()
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
// as indices into Bases: every base fitted to that appearance, and nothing
// else.
//
// The applied base is always among them, and there is no case to make an
// exception for. Each appearance has its own choice, picked from its own list,
// so the list a person is looking at always holds the name their code is
// coloured with — flipping the scheme swaps the list and the applied base
// together, in one frame, rather than leaving a name marked on the half it was
// not fitted for.
func (m Model) VisibleBases(dark bool) []int {
	out := make([]int, 0, len(m.Bases))
	for i, b := range m.Bases {
		if b.Suits(dark) {
			out = append(out, i)
		}
	}
	return out
}

// VisibleStyles are the cards the grid lays out under the appearance on
// screen, as indices into Styles: every style fitted to that appearance, and
// nothing else.
//
// One state and not a second control, for the reason the base list has one:
// a filter with a switch of its own could be set to disagree with the window,
// which would mean offering styles for the scheme nobody is looking at — and
// on this grid it would be worse than on that list, since a click here is what
// puts the window in a scheme's theme in the first place.
func (m Model) VisibleStyles(dark bool) []int {
	out := make([]int, 0, len(m.Styles))
	for i, s := range m.Styles {
		if s.Suits(dark) {
			out = append(out, i)
		}
	}
	return out
}

// baseIndex finds a base by name, and falls back to the position of the
// appearance's own default rather than to zero: position zero is whatever
// sorted first, which is a style nobody chose.
func baseIndex(bases []BaseOption, name string, dark bool) int {
	for i, b := range bases {
		if b.Name == name {
			return i
		}
	}
	fallback := highlight.DefaultBases().Base(dark)
	for i, b := range bases {
		if b.Name == fallback {
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
// kept-theme file — the colour and both syntax bases, since all of them are
// written and all of them come back. It is the difference between an
// affordance offering something and one confirming it.
//
// Both members count, including the one the appearance on screen is not
// showing: the file holds the pair, so a base picked under the moon and then
// left behind a flip to the sun is still an unkept change.
func (m Model) SeedIsKept() bool {
	seed, ok := m.Seed()
	return ok && m.Kept.A != 0 && m.Kept == seed && m.KeptBases == m.AppliedBases() && m.KeptMono == m.keepMono()
}

// shortName is what the window shows for a loaded picture: the file's own
// name without the directory it happened to sit in.
func shortName(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
