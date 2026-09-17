package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/brand"
	"github.com/vibrantgio/theme/imagecolor"
	"github.com/vibrantgio/theme/tokens"
)

// Model is the whole application state: where the theme colour on screen came
// from, the picture it may have come out of, and what the kept-theme file
// currently holds.
type Model struct {
	// Preview is the dropped picture shrunk to something a window can
	// paint every frame; nil until an image loads.
	Preview *image.NRGBA
	// Name is the dropped file's style name, shown beside the picture.
	Name string
	// Candidates are the colours the picture offers, most prominent first.
	Candidates []imagecolor.Candidate
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
	// Scheme is which appearance the window is on. FollowOS until the switch
	// at the top of the window is pressed, and the window's own answer from
	// then on: the two appearances of one theme are two sets of choices, and
	// a person settling a theme has to reach both without waiting for the
	// desktop to change its mind. It moves the whole window — its own plane
	// and every group on it, the style list, and the picture of an
	// application in the preview.
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
	// Styles are the syntax highlighter styles a fence can be coloured from —
	// the ones that ship embedded and the ones read out of the styles folder —
	// in the order the chooser lists them.
	Styles []StyleOption
	// LightAt and DarkAt index Styles, one per appearance: the style code is
	// coloured from under the sun, and the one it is coloured from under the
	// moon. They start on whatever was kept.
	//
	// Two of them because a style is fitted to a background, so the two
	// appearances of one theme are two choices. Picking under the sun moves
	// one and picking under the moon the other; the appearance switch moves
	// neither, and switches which is on offer.
	LightAt, DarkAt int
	// KeptStyles are the syntax styles that file currently holds, one per
	// appearance, resolved the same way the applied pair is. They sit beside
	// Kept because the keep affordance confirms when everything on screen is
	// what is on disk, and the styles are part of everything.
	KeptStyles highlight.StylePair
	// Mono is the typeface fenced code wears. Empty is Roboto Mono, the
	// default; the one other name this window applies is "JetBrains Mono".
	Mono string
	// KeptMono is the typeface that file currently holds, empty for Roboto
	// Mono, so the keep affordance can say whether the face on screen is the
	// one on disk.
	KeptMono string
}

// StyleOption is one row of the style chooser: a style's name, whether
// it came out of the styles folder rather than shipping embedded, and which
// appearances it is offered under. Where a style came from changes nothing
// about how it is used; it is worth showing because it is the difference
// between a name somebody recognises and one they put there themselves.
type StyleOption struct {
	Name  string
	Added bool
	// Light and Dark are measured off the style's own background, once, when
	// the list is built — the answer cannot change while the window is open,
	// and measuring seventy-four backgrounds per frame to learn it would be a
	// waste of a frame. A style fitted to no background of its own carries
	// both.
	Light bool
	Dark  bool
}

// Suits reports whether this style is one to offer under the appearance on
// screen.
func (o StyleOption) Suits(dark bool) bool {
	if dark {
		return o.Dark
	}
	return o.Light
}

// StyleAt is the row of Styles the given appearance is coloured from.
func (m Model) StyleAt(dark bool) int {
	if dark {
		return m.DarkAt
	}
	return m.LightAt
}

// Style is the highlighter style code is coloured from under one appearance. An
// index out of range — no styles at all, which no build has — falls back to
// the highlighter's own default for that appearance, because there is always
// a style.
func (m Model) Style(dark bool) string {
	at := m.StyleAt(dark)
	if at < 0 || at >= len(m.Styles) {
		return highlight.DefaultStyles().Style(dark)
	}
	return m.Styles[at].Name
}

// AppliedStyles is the pair on screen: what code is coloured from under each
// appearance. It is what the sample is drawn through, what is kept, and what
// is compared with the file to say whether what is on screen is what is on
// disk.
func (m Model) AppliedStyles() highlight.StylePair {
	return highlight.StylePair{Light: m.Style(false), Dark: m.Style(true)}
}

// VisibleStyles are the rows the chooser lists under the appearance on screen,
// as indices into Styles: every style fitted to that appearance, and nothing
// else.
//
// The applied style is always among them. Each appearance has its own choice,
// picked from its own list, so the list a person is looking at always holds
// the name their code is coloured with — flipping the scheme swaps the list
// and the applied style together, in one frame, rather than leaving a name
// marked on the half it was not fitted for.
func (m Model) VisibleStyles(dark bool) []int {
	out := make([]int, 0, len(m.Styles))
	for i, b := range m.Styles {
		if b.Suits(dark) {
			out = append(out, i)
		}
	}
	return out
}

// AppliedMono is the typeface name the sample wears: JetBrains Mono when that
// is selected, Roboto Mono otherwise.
func (m Model) AppliedMono() string {
	if m.Mono == tokens.CodeFaceJetBrains {
		return tokens.CodeFaceJetBrains
	}
	return tokens.CodeFaceRoboto
}

// keepMono is what keeping writes: "JetBrains Mono", or empty for Roboto
// Mono. Empty is how the file spells the default, so a file without the key
// and a file that chose Roboto Mono come back the same way.
func (m Model) keepMono() string {
	if m.AppliedMono() == tokens.CodeFaceJetBrains {
		return tokens.CodeFaceJetBrains
	}
	return ""
}

// styleNames is every highlighter style there is, in the order this window lists
// them: by name, the way somebody scanning for one reads names.
//
// It is not the byte order the names arrive in, and the difference is not
// academic. An underscore sorts under every letter, so a name that carries
// one lands ahead of every name that continues with a letter past that point
// — which puts hr_high_contrast in front of hrdark, two rows up from where a
// reader running down the h's for "hrdark" would stop looking. Separators are
// not what a name is looked up by, so they are left out of the comparison
// entirely and the raw name breaks the tie, which keeps the order total and
// the list the same list on every machine.
func styleNames() []string {
	names := highlight.Styles()
	slices.SortStableFunc(names, func(a, b string) int {
		if c := strings.Compare(lookupKey(a), lookupKey(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return names
}

// lookupKey is a style name reduced to what it is looked up by: its letters
// and digits, folded to one case.
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

// styleOptions is every highlighter style on offer, in the order the window lists
// them, marked with where each came from and which appearance it was fitted
// to.
func styleOptions() []StyleOption {
	names := styleNames()
	out := make([]StyleOption, len(names))
	for i, n := range names {
		out[i] = StyleOption{
			Name:  n,
			Added: highlight.Loaded(n),
			Light: highlight.StyleSuits(n, false),
			Dark:  highlight.StyleSuits(n, true),
		}
	}
	return out
}

// styleIndex finds a style by name, and falls back to the position of the
// appearance's own default rather than to zero: position zero is whatever
// sorted first, which is a style nobody chose.
func styleIndex(styles []StyleOption, name string, dark bool) int {
	for i, b := range styles {
		if b.Name == name {
			return i
		}
	}
	fallback := highlight.DefaultStyles().Style(dark)
	for i, b := range styles {
		if b.Name == fallback {
			return i
		}
	}
	return 0
}

// skippedSentence says which files in the styles folder did not load and why.
// One file is named with its reason; more than one names the first and counts
// the rest, because the caption is a line and not a log.
func skippedSentence(skipped []highlight.Skipped) string {
	if len(skipped) == 1 {
		return "style not loaded — " + skipped[0].String()
	}
	return fmt.Sprintf("%d styles not loaded — %s, and %d more",
		len(skipped), skipped[0], len(skipped)-1)
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

// Scheme names which side of the platform's pair the syntax style group is
// offering names for, and whether that is the window's decision or the
// desktop's.
type Scheme int

const (
	// FollowOS takes the side the desktop is set to.
	FollowOS Scheme = iota
	// ShowLight and ShowDark override it, for as long as the window is open.
	ShowLight
	ShowDark
)

// Dark reports which side the syntax style group is set for, given the
// platform set the window itself is wearing. The desktop decides until the
// switch is pressed.
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
// system, and both syntax styles and the code face besides, since all of them
// are written and all of them come back. It is the difference between an
// affordance offering something and one confirming it.
//
// Both members of the pair count, including the one the appearance on screen
// is not showing: the file holds the pair, so a style picked under the moon
// and then left behind a flip to the sun is still an unkept change.
func (m Model) IsKept() bool {
	if m.KeptStyles != m.AppliedStyles() || m.KeptMono != m.keepMono() {
		return false
	}
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
	// The styles folder is read here, once, before the first frame. It is a
	// handful of small files in a directory most people do not have, and
	// reading it later would mean a chooser that grows a row while somebody
	// is reading it. A file in it that will not parse is named in the caption
	// rather than thrown: the other styles loaded, and the window is still
	// worth looking at.
	if dir, err := brand.StylesDir(); err == nil {
		if _, skipped := highlight.LoadDir(dir); len(skipped) > 0 {
			m.Problem = skippedSentence(skipped)
		}
	}
	m.Styles = styleOptions()
	kept := brand.Brand{}
	if path, err := brand.Path(); err == nil {
		m.KeepPath = path
		kept = brand.KeptFrom(path)
	}
	m = m.adoptKept(kept)
	if kept.Chosen() {
		// A window opens on the colour that was kept, so the first frame
		// previews the theme every other application is already wearing.
		m.From, m.Typed, m.Text = FromHex, kept.ThemeColor, hexOf(kept.ThemeColor)
	}
	if len(os.Args) > 1 {
		return m, LoadImage(os.Args[1])
	}
	return m, mvu.DoNothing()
}

// adoptKept folds what is already in the kept-theme file into the model: the
// colour it holds, the syntax styles it names resolved against what this build
// can actually draw, and the code face it names.
//
// A style name nothing resolves — a style whose file has left the folder, one
// written by a build that had it — opens the window on the default rather
// than on whatever sorted first, because the default is what everything else
// showing that file's theme will use. So does a name fitted to the appearance
// it is not kept for, which is what a file naming one style with no appearance
// attached comes back as: the name stands on the half it was measured to
// belong on, and the other half opens on its own default.
//
// Unknown, empty, or "Roboto Mono" all open on Roboto Mono. Only JetBrains
// Mono is a selection this window can restore.
func (m Model) adoptKept(kept brand.Brand) Model {
	m.Kept, m.KeptFollows = kept.ThemeColor, kept.FollowSystem
	m.KeptStyles = highlight.StylesOrDefault(kept.Style.Names())
	m.LightAt = styleIndex(m.Styles, m.KeptStyles.Light, false)
	m.DarkAt = styleIndex(m.Styles, m.KeptStyles.Dark, true)
	m.Mono = ""
	if kept.Mono == tokens.CodeFaceJetBrains {
		m.Mono = tokens.CodeFaceJetBrains
	}
	m.KeptMono = m.Mono
	return m
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
