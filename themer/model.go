package main

import (
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// Model is the whole application state: where the theme colour on screen came
// from, the picture it may have come out of, and what the kept-theme file
// currently holds.
type Model struct {
	// Preview is the dropped picture shrunk to something a window can
	// paint every frame; nil until an image loads.
	Preview *image.NRGBA
	// Name is the dropped file's base name, shown beside the picture.
	Name string
	// Candidates are the colours the picture offers, most prominent first.
	Candidates []imageseed.Candidate
	// Selected indexes Candidates. It is 0 on a fresh extraction, so the
	// leading colour is the one a drop lands on.
	Selected int
	// From says which of the three ways of choosing the colour on screen
	// came from.
	From Choice
	// Typed is the colour written into the field, and Text is what was
	// written. Text is kept beside the colour because a field being typed
	// into holds runs that are not yet a colour, and the caption says so.
	Typed stdcolor.NRGBA
	Text  string
	// DragOver is true while a file drag hovers over the window, and is
	// what the drop zone highlights on.
	DragOver bool
	// Scheme is which side of the platform's pair the preview is drawn in.
	// FollowOS until the switch is pressed, and the window's own answer from
	// then on: a theme colour has to be seen on both sides, and waiting for
	// the desktop to change its mind is not a way to do that. It moves the
	// preview alone — the window itself follows the setting, like every
	// other application.
	Scheme Scheme
	// Problem describes the last thing that did not work — a drop that
	// became no colours (an unreadable file, a format nothing here decodes,
	// a picture with no opaque pixels), or a theme that could not be kept.
	// Empty when the last thing worked.
	Problem string
	// KeepPath is the file a kept theme is written to, resolved once at
	// startup. Empty when this machine has no config directory to put it
	// in, which is the one way keeping can fail before it is tried.
	KeepPath string
	// Platform is the accent colour the platform reports — blue while macOS
	// is on Multicolour. It is read off the appearance stream and kept
	// current, so what is offered is the setting in force rather than the
	// one in force when the window opened. A zero alpha means this platform
	// reports none.
	Platform stdcolor.NRGBA
	// Kept is the colour that file currently holds: read from it at startup
	// and replaced by every keep that succeeds, so the window can say
	// whether what is on screen is what would come back. A zero alpha means
	// no colour is kept.
	Kept stdcolor.NRGBA
	// KeptFollows is that file saying the theme colour follows the system,
	// which is the one thing it can hold that is not a colour.
	KeptFollows bool
}

// Choice names where the theme colour on screen came from.
type Choice int

const (
	// FromPlatform is the platform's own accent colour. It is the default,
	// and it is what a brand that follows the system keeps: the colour goes
	// on changing under a window that has chosen it.
	FromPlatform Choice = iota
	// FromImage is one of the colours a picture offered.
	FromImage
	// FromHex is a colour written into the field.
	FromHex
)

// Follows reports whether the theme colour on screen is the platform's own,
// which is what the kept file records as following the system.
func (m Model) Follows() bool { return m.From == FromPlatform }

// Color is the theme colour on screen, and whether there is one. A platform
// reporting no accent, an empty field and a picture that offered nothing all
// answer that there is not, and the window previews the platform's set
// untouched.
func (m Model) Color() (stdcolor.NRGBA, bool) {
	switch m.From {
	case FromImage:
		if m.Selected < 0 || m.Selected >= len(m.Candidates) {
			return stdcolor.NRGBA{}, false
		}
		return m.Candidates[m.Selected].Color, true
	case FromHex:
		return m.Typed, m.Typed.A != 0
	}
	return m.Platform, m.Platform.A != 0
}

// Scheme names which side of the platform's pair the preview shows, and
// whether that is the window's decision or the desktop's.
type Scheme int

const (
	// FollowOS takes the side the desktop is set to.
	FollowOS Scheme = iota
	// ShowLight and ShowDark override it, for as long as the window is open.
	ShowLight
	ShowDark
)

// Dark reports which side the preview is drawn on, given the platform set the
// window itself is wearing. The desktop decides until the switch is pressed.
func (m Model) Dark(live tokens.PlatformColors) bool {
	switch m.Scheme {
	case ShowLight:
		return false
	case ShowDark:
		return true
	}
	return isDark(live)
}

// IsKept reports whether what is on screen is what is already in the
// kept-theme file — the colour, or the standing instruction to follow the
// system. It is the difference between an affordance offering something and
// one confirming it.
func (m Model) IsKept() bool {
	if m.Follows() {
		// The file holds no colour to compare, so what is compared is the
		// one thing it does hold: that it follows too.
		return m.KeptFollows
	}
	col, ok := m.Color()
	return ok && m.Kept.A != 0 && m.Kept == col
}

// Init returns the model the window starts from — where a kept theme is
// written and what is already there — plus the load of a picture named on the
// command line if there is one, the same command a drop runs, so a path
// argument and a dragged file reach the application identically.
//
// A machine with no config directory to write to is not a reason to refuse to
// start: the path stays empty, the window works, and the one thing it cannot
// do says so when it is asked.
func Init() (Model, mvu.Command) {
	m := Model{}
	kept := brand.Brand{}
	if path, err := brand.Path(); err == nil {
		m.KeepPath = path
		kept = brand.KeptFrom(path)
	}
	m.Kept, m.KeptFollows = kept.Seed, kept.FollowSystem
	if kept.Chosen() {
		// A window opens on the colour that was kept, so the first frame
		// previews the theme every other application is already wearing.
		m.From, m.Typed, m.Text = FromHex, kept.Seed, hexOf(kept.Seed)
	}
	if len(os.Args) > 1 {
		return m, LoadImage(os.Args[1])
	}
	return m, mvu.DoNothing()
}

// KeepSource is what the kept file records about where the colour came from.
// It is provenance and nothing reads it back as input, so it says the picture
// where there is one and nothing at all where the colour was written by hand.
func (m Model) KeepSource() string {
	if m.From == FromImage {
		return m.Name
	}
	return ""
}

// parseHex reads what was written into the field as a colour: #rrggbb, in
// either case, with the mark optional. Anything else is not yet a colour —
// a field halfway through being typed into is the ordinary state, not an
// error — and comes back false.
func parseHex(s string) (stdcolor.NRGBA, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 6 {
		return stdcolor.NRGBA{}, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return stdcolor.NRGBA{}, false
	}
	return stdcolor.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}, true
}

// shortName is what the window shows for a loaded picture: the file's own
// name without the directory it happened to sit in.
func shortName(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
