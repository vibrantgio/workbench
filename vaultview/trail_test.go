package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/breadcrumb"
	"github.com/vibrantgio/theme/tokens"
)

// trailPad draws one of the window's trails through a real input router, so
// a click travels the path it travels in the running app: registered by the
// frame that drew the segment, queued at the window, and reported to the
// frame after it.
type trailPad struct {
	row  breadcrumb.TrailLayout
	r    input.Router
	ops  op.Ops
	size image.Point
	dims layout.Dimensions
}

func newTrailPad(shaper *text.Shaper, colors tokens.ColorTokens) *trailPad {
	return &trailPad{
		row: breadcrumb.NewTrail(shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
			colors, tokens.Spacing, tokens.DefaultTypography.TitleSmall),
		size: image.Pt(600, 40),
	}
}

func (p *trailPad) frame(segs []breadcrumb.Segment) {
	p.ops.Reset()
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(p.size),
		Ops:         &p.ops,
		Source:      p.r.Source(),
	}
	p.dims = p.row(gtx, segs)
	p.r.Frame(&p.ops)
}

// clickAt presses and releases at x on the row's mid-height, then draws the
// frame that reports it.
func (p *trailPad) clickAt(x int, segs []breadcrumb.Segment) {
	at := f32.Pt(float32(x), float32(p.dims.Size.Y)/2)
	p.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	p.frame(segs)
}

// trailWidth measures the natural width of a row of these labels: loose
// constraints, so the answer is the row's own and not the canvas's.
func trailWidth(shaper *text.Shaper, labels ...string) int {
	segs := make([]breadcrumb.Segment, len(labels))
	for i, l := range labels {
		segs[i] = breadcrumb.Segment{Key: l, Label: l}
	}
	row := breadcrumb.NewTrail(shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
		tokens.DefaultLight, tokens.Spacing, tokens.DefaultTypography.TitleSmall)
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(1<<14, 1<<14)},
		Ops:         new(op.Ops),
	}
	return row(gtx, segs).Size.X
}

// centreOf answers where the middle of segment i of this trail is, derived
// from measured widths rather than from restated spacing: the separator
// costs whatever two labels cost beyond the two labels alone.
func centreOf(shaper *text.Shaper, i int, places []place) int {
	sep := trailWidth(shaper, "x", "x") - 2*trailWidth(shaper, "x")
	x := 0
	for j := 0; j < i; j++ {
		x += trailWidth(shaper, places[j].label) + sep
	}
	return x + trailWidth(shaper, places[i].label)/2
}

// TestTrailClicksReachThePlaceClicked drives both of the window's trails —
// the picker's directory path and the note's path inside the vault —
// through the row the vocabulary draws. Every ancestor answers with its own
// place, and the trailing segment, which is where the reader already is,
// answers with nothing.
func TestTrailClicksReachThePlaceClicked(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()

	note := Model{Screen: screenVault, Vault: "/vaults/Second Brain", CurAnchor: -1}
	note = cacheNote(note, &Note{Path: "guide/notes/Reading list.md", Title: "Reading list"})
	note.Current = "guide/notes/Reading list.md"

	for _, tc := range []struct {
		name   string
		places []place
	}{
		{"picker directory trail", dirPlaces("/vaults/Second Brain/guide")},
		{"note path trail", notePlaces(note)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.places) < 2 {
				t.Fatalf("the trail is %d place(s) long; this proves nothing", len(tc.places))
			}
			for i, want := range tc.places {
				clicked := ""
				segs := trailSegments(tc.places, func(path string) func(gtx layout.Context) {
					return func(gtx layout.Context) { clicked = path }
				})
				pad := newTrailPad(shaper, tokens.DefaultLight)
				pad.frame(segs)
				if pad.dims.Size.X == 0 || pad.dims.Size.Y == 0 {
					t.Fatalf("the trail drew nothing: %v", pad.dims)
				}
				pad.clickAt(centreOf(shaper, i, tc.places), segs)

				if i == len(tc.places)-1 {
					if clicked != "" {
						t.Errorf("the current location %q navigated to %q; it is where the reader already is",
							want.label, clicked)
					}
					continue
				}
				if clicked != want.path {
					t.Errorf("a click on %q went to %q, want %q", want.label, clicked, want.path)
				}
			}
		})
	}
}

// TestTrailClickFollowsItsPlaceAcrossAReshuffle is why the segments carry
// their paths as keys. The click is made on one trail and reported on the
// frame that draws another, in which a different place stands where the
// clicked one stood: the folder the reader pressed is the one that opens.
func TestTrailClickFollowsItsPlaceAcrossAReshuffle(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	clicked := ""
	click := func(path string) func(gtx layout.Context) {
		return func(gtx layout.Context) { clicked = path }
	}

	before := []place{{label: "guide", path: "guide"}, {label: "notes", path: "guide/notes"},
		{label: "Reading list", path: "guide/notes/Reading list.md"}}
	// The reader clicked "notes" and the model moved on: another branch now
	// stands in the position "notes" stood in.
	after := []place{{label: "guide", path: "guide"}, {label: "drafts", path: "guide/drafts"},
		{label: "Sketch", path: "guide/drafts/Sketch.md"}}

	pad := newTrailPad(shaper, tokens.DefaultLight)
	pad.frame(trailSegments(before, click))
	pad.clickAt(centreOf(shaper, 1, before), trailSegments(after, click))

	if clicked != "guide/notes" {
		t.Errorf("the click on %q opened %q, want %q", before[1].label, clicked, before[1].path)
	}
}
