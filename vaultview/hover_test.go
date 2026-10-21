package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

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
	style.Text.OnLinkHover = record
	addresses = nil

	mid := func(run [2]int) int { return (run[0] + run[1]) / 2 }
	bar := func() string {
		m2 := m
		for _, a := range addresses {
			m2, _ = Update(m2, HoverLink{URL: a})
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
	m, _ = Update(m, HoverLink{URL: "f.md"})
	if got := statusLine(m); got != "f.md" {
		t.Fatalf("the bar says %q while the pointer is on the link, want %q", got, "f.md")
	}
	m, _ = Update(m, Navigate{Path: "f.md"})
	if m.HoveredLink != "" {
		t.Errorf("the landing kept the address %q on the model", m.HoveredLink)
	}
	if got, want := statusLine(m), lineCountLabel(m.CurrentNote().Lines); got != want {
		t.Errorf("after the landing the bar says %q, want %q", got, want)
	}
}
