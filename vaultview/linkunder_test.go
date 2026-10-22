package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

// hoverNoteSource is the note the bar is read over: one line of prose
// carrying a wikilink and an external link, so the pointer can stand on each
// of them and on neither.
const hoverNoteSource = "See [[Sources]] and [the site](https://example.test/page) for more.\n"

// TestTheBarSaysTheAddressUnderThePointer drives the pointer across a
// rendered note and reads the bar: the wikilink's resolved vault path while
// the pointer is on it, the external link's URL while it is on that, and the
// note's line count back as soon as it is on neither.
func TestTheBarSaysTheAddressUnderThePointer(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	col := tokens.PlatformLight
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Hover.md", hoverNoteSource))
	m.Current = "guide/Hover.md"
	note := m.CurrentNote()
	if note == nil {
		t.Fatal("the fixture model has no current note")
	}

	doc := markdown.NewDocument(note.Blocks)
	style := noteStyle(col, tokens.DefaultTypography)
	// The addresses the page's own resolution makes of what the document
	// reports: the message the window posts carries exactly this.
	var addresses []string
	record := func(_ layout.Context, url string) {
		addresses = append(addresses, linkAddress(m, url))
	}

	size := image.Pt(560, 200)
	r := new(input.Router)
	ops := new(op.Ops)
	frame := func(evs ...event.Event) {
		r.Queue(evs...)
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         ops,
			Source:      r.Source(),
		}
		doc.Layout(gtx, shaper, style)
		r.Frame(ops)
	}
	move := func(x, y int) {
		frame(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(x), float32(y)), Source: pointer.Mouse})
	}

	frame()
	frame()
	// Where the two links landed, read off the shape the frame declares
	// across the first line rather than off widths the faces decide. The
	// reports this probe makes are the probe's own and are thrown away.
	const y = 8
	var runs [][2]int
	on := false
	for x := 0; x < size.X; x++ {
		move(x, y)
		hand := r.Cursor() == pointer.CursorPointer
		switch {
		case hand && !on:
			runs = append(runs, [2]int{x, x})
		case hand:
			runs[len(runs)-1][1] = x
		}
		on = hand
	}
	if len(runs) != 2 {
		t.Fatalf("the line declares %d link runs, want the wikilink and the web link: %v", len(runs), runs)
	}
	move(size.X-1, y)
	style.Text.OnLink = record
	addresses = nil

	mid := func(run [2]int) int { return (run[0] + run[1]) / 2 }
	bar := func() string {
		m2 := m
		for _, a := range addresses {
			m2, _ = Update(m2, LinkUnder{URL: a})
		}
		return statusLine(m2)
	}

	if got, want := statusLine(m), lineCountLabel(note.Lines); got != want {
		t.Fatalf("with the pointer on no link the bar says %q, want %q", got, want)
	}

	move(mid(runs[0]), y)
	if got, want := bar(), "Sources.md"; got != want {
		t.Errorf("over the wikilink the bar says %q, want the note's vault path %q (reports: %v)", got, want, addresses)
	}

	move(mid(runs[1]), y)
	if got, want := bar(), "https://example.test/page"; got != want {
		t.Errorf("over the web link the bar says %q, want its URL %q (reports: %v)", got, want, addresses)
	}

	move(size.X-1, y)
	if got, want := bar(), lineCountLabel(note.Lines); got != want {
		t.Errorf("with the pointer off the links the bar says %q, want the count %q (reports: %v)", got, want, addresses)
	}
}

// TestALandingTakesTheAddressOffTheBar: the note the pointer was on leaves
// the screen when a link is followed, and the link it carried goes with it,
// so the bar reports the new note's length rather than an address that is no
// longer under anything.
func TestALandingTakesTheAddressOffTheBar(t *testing.T) {
	m := scannedModel(t)
	m.Index = treeIndex("x.md", "f.md")
	m = cacheNote(m, fNote())
	m, _ = Update(m, LinkUnder{URL: "f.md"})
	if got := statusLine(m); got != "f.md" {
		t.Fatalf("the bar says %q while the pointer is on the link, want %q", got, "f.md")
	}
	m, _ = Update(m, Navigate{Path: "f.md"})
	if m.LinkUnder != "" {
		t.Errorf("the landing kept the address %q on the model", m.LinkUnder)
	}
	if got, want := statusLine(m), lineCountLabel(m.CurrentNote().Lines); got != want {
		t.Errorf("after the landing the bar says %q, want %q", got, want)
	}
}

// TestTheBarSaysTheAddressTheKeyboardIsOn moves the keyboard through the
// same note and reads the bar: a link reached by Tab says its address where
// a hovered one says it, and the count comes back when the keyboard leaves
// the links — which is what the platform's browsers do.
func TestTheBarSaysTheAddressTheKeyboardIsOn(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	col := tokens.PlatformLight
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Hover.md", hoverNoteSource))
	m.Current = "guide/Hover.md"
	note := m.CurrentNote()
	if note == nil {
		t.Fatal("the fixture model has no current note")
	}

	doc := markdown.NewDocument(note.Blocks)
	style := noteStyle(col, tokens.DefaultTypography)
	var addresses []string
	style.Text.OnLink = func(_ layout.Context, url string) {
		addresses = append(addresses, linkAddress(m, url))
	}

	size := image.Pt(560, 200)
	r := new(input.Router)
	ops := new(op.Ops)
	// A focusable beyond the note, so the keyboard has somewhere to go after
	// the last link instead of wrapping back onto the first.
	var beyond int
	frame := func() {
		ops.Reset()
		gtx := layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         ops,
			Source:      r.Source(),
		}
		doc.Layout(gtx, shaper, style)
		for {
			if _, ok := gtx.Event(key.FocusFilter{Target: &beyond}); !ok {
				break
			}
		}
		event.Op(gtx.Ops, &beyond)
		r.Frame(ops)
	}
	bar := func() string {
		m2 := m
		for _, a := range addresses {
			m2, _ = Update(m2, LinkUnder{URL: a})
		}
		return statusLine(m2)
	}

	frame()
	frame()
	if got, want := bar(), lineCountLabel(note.Lines); got != want {
		t.Fatalf("with the keyboard on no link the bar says %q, want %q", got, want)
	}

	r.MoveFocus(key.FocusForward)
	frame()
	if got, want := bar(), "Sources.md"; got != want {
		t.Errorf("with the keyboard on the wikilink the bar says %q, want the note's vault path %q (reports: %v)", got, want, addresses)
	}

	r.MoveFocus(key.FocusForward)
	frame()
	if got, want := bar(), "https://example.test/page"; got != want {
		t.Errorf("with the keyboard on the web link the bar says %q, want its URL %q (reports: %v)", got, want, addresses)
	}

	r.MoveFocus(key.FocusForward)
	frame()
	if got, want := bar(), lineCountLabel(note.Lines); got != want {
		t.Errorf("with the keyboard off the links the bar says %q, want the count %q (reports: %v)", got, want, addresses)
	}
}

// TestTheBarElidesAnAddressWiderThanItself narrows the bar under an address
// far wider than it and reads what was drawn: the line is shortened in its
// middle rather than at its end, so the start and the end of the address
// both stand.
func TestTheBarElidesAnAddressWiderThanItself(t *testing.T) {
	tok := themeTokens{
		col:    tokens.PlatformLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.DeterministicShaper(),
	}
	const long = "https://example.test/a/considerably/longer/path/than/the/bar/can/hold/page.html"
	m := goldenModel()
	m = cacheNote(m, noteFromSource("guide/Hover.md", hoverNoteSource))
	m.Current = "guide/Hover.md"
	m, _ = Update(m, LinkUnder{URL: long})

	// The same bar at two widths. Narrow, the line cannot be the whole
	// address; wide, it is.
	draw := func(width int) *image.RGBA {
		return golden.Capture(t, image.Pt(width, 24), func(gtx layout.Context) layout.Dimensions {
			return layoutStatusBar(gtx, m, tok, width)
		})
	}
	wide := draw(900)
	narrow := draw(260)
	if sameImage(wide, narrow) {
		t.Fatal("the bar drew the same line at 900 and at 260 wide")
	}

	// The end of the address survives: two addresses sharing everything but
	// their last characters draw different bars at the narrow width.
	m2 := m
	m2, _ = Update(m2, LinkUnder{URL: long[:len(long)-len("page.html")] + "other.htm"})
	other := golden.Capture(t, image.Pt(260, 24), func(gtx layout.Context) layout.Dimensions {
		return layoutStatusBar(gtx, m2, tok, 260)
	})
	if sameImage(narrow, other) {
		t.Error("two addresses differing only at their ends drew the same bar; the end was cut rather than the middle")
	}
}

// sameImage reports whether two captures of the same size hold the same
// pixels everywhere.
func sameImage(a, b *image.RGBA) bool {
	bounds := a.Bounds()
	if bounds != b.Bounds() {
		return false
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				return false
			}
		}
	}
	return true
}
