package main

import (
	"image"
	"reflect"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/breadcrumb"
	"github.com/vibrantgio/mvu"
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

func newTrailPad(shaper *text.Shaper, colors tokens.PlatformColors) *trailPad {
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

// trailWidth measures the natural width of a row of these titles: loose
// constraints, so the answer is the row's own and not the window's.
func trailWidth(shaper *text.Shaper, titles ...string) int {
	segs := make([]breadcrumb.Segment, len(titles))
	for i, l := range titles {
		segs[i] = breadcrumb.Segment{Key: l, Title: l}
	}
	row := breadcrumb.NewTrail(shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
		tokens.PlatformLight, tokens.Spacing, tokens.DefaultTypography.TitleSmall)
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(1<<14, 1<<14)},
		Ops:         new(op.Ops),
	}
	return row(gtx, segs).Size.X
}

// centreOf answers where the middle of segment i of this trail is, derived
// from measured widths rather than from restated spacing: the separator
// costs whatever two titles cost beyond the two titles alone.
func centreOf(shaper *text.Shaper, i int, places []place) int {
	sep := trailWidth(shaper, "x", "x") - 2*trailWidth(shaper, "x")
	x := 0
	for j := 0; j < i; j++ {
		x += trailWidth(shaper, places[j].title) + sep
	}
	return x + trailWidth(shaper, places[i].title)/2
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
		{"picker directory trail", dirPlaces("/vaults/Second Brain/guide", place{title: "Macintosh HD", path: "/"})},
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
				pad := newTrailPad(shaper, tokens.PlatformLight)
				pad.frame(segs)
				if pad.dims.Size.X == 0 || pad.dims.Size.Y == 0 {
					t.Fatalf("the trail drew nothing: %v", pad.dims)
				}
				pad.clickAt(centreOf(shaper, i, tc.places), segs)

				if i == len(tc.places)-1 {
					if clicked != "" {
						t.Errorf("the current location %q navigated to %q; it is where the reader already is",
							want.title, clicked)
					}
					continue
				}
				if clicked != want.path {
					t.Errorf("a click on %q went to %q, want %q", want.title, clicked, want.path)
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

	before := []place{{title: "guide", path: "guide"}, {title: "notes", path: "guide/notes"},
		{title: "Reading list", path: "guide/notes/Reading list.md"}}
	// The reader clicked "notes" and the model moved on: another branch now
	// stands in the position "notes" stood in.
	after := []place{{title: "guide", path: "guide"}, {title: "drafts", path: "guide/drafts"},
		{title: "Sketch", path: "guide/drafts/Sketch.md"}}

	pad := newTrailPad(shaper, tokens.PlatformLight)
	pad.frame(trailSegments(before, click))
	pad.clickAt(centreOf(shaper, 1, before), trailSegments(after, click))

	if clicked != "guide/notes" {
		t.Errorf("the click on %q opened %q, want %q", before[1].title, clicked, before[1].path)
	}
}

// bandTrailFrame lays the whole window out against a real input.Router and
// records what the toolbar posts, so a press on a place in its trail is read
// back as the message the running window would reduce. The toolbar is the
// window's own, at the width it has beside a standing rail, so what answers
// the press is the composition and not a row built to be pressed.
type bandTrailFrame struct {
	t      *testing.T
	f      *frameState
	r      input.Router
	ops    op.Ops
	m      Model
	tok    themeTokens
	rowW   int
	posted []mvu.Message
}

func newBandTrailFrame(t *testing.T, m Model, rowW int) *bandTrailFrame {
	t.Helper()
	b := &bandTrailFrame{t: t, f: newFrameState(defaultArrangement()), m: m, tok: goldenTokens(), rowW: rowW}
	b.f.leading = func() unit.Dp { return goldenLeading }
	b.f.post = func(_ layout.Context, msg mvu.Message) { b.posted = append(b.posted, msg) }
	return b
}

func (b *bandTrailFrame) frame() {
	b.ops.Reset()
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(b.rowW, windowH)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &b.ops,
		Source:      b.r.Source(),
	}
	b.f.layout(gtx, b.m, b.tok, paneSlot(nil), fillSlot, nil)
	b.r.Frame(&b.ops)
}

// bandTrailLeading is where the toolbar's trail begins inside the content
// column: the column's own edge inset, the window's navigation pair and the
// measured gap after it.
var bandTrailLeading = noteInsetDp + 2*railToggleWidthDp + 1 + toolbarTitleGapDp

// pressPlace presses the middle of the trail's i-th place and answers what
// the row posted for it. The press is reported on the frame after the one
// that drew the place, which is the frame this draws second.
func (b *bandTrailFrame) pressPlace(i int, places []place) []mvu.Message {
	b.t.Helper()
	b.frame()
	at := f32.Pt(float32(b.f.geom.ContentX+bandTrailLeading+centreOf(b.tok.shaper, i, places)), float32(toolbarDepthDp)/2)
	b.posted = nil
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	b.frame()
	return b.posted
}

// bandTrailModel is the golden window's model with a note two folders deep,
// so the band's trail carries a vault, two folders and the note itself.
func bandTrailModel() Model {
	m := goldenModel()
	m = cacheNote(m, noteFromSource("Design/notes/Colour.md", "# Colour\n\nprose\n"))
	m.Current = "Design/notes/Colour.md"
	return m
}

// TestTheBandsTrailRevealsThePlacePressed drives the chrome row's trail: the
// vault at its head returns the outline to its root, each folder after it
// opens the way down to itself, and the note at the end answers nothing,
// being where the reader already is.
func TestTheBandsTrailRevealsThePlacePressed(t *testing.T) {
	m := bandTrailModel()
	places := notePlaces(m)
	if len(places) != 4 {
		t.Fatalf("the trail carries %d places: %+v; this model is meant to be two folders deep", len(places), places)
	}
	for i, c := range []struct {
		what string
		want mvu.Message
	}{
		{"the vault", RootTree{}},
		{"the first folder", RevealFolder{Dir: "Design"}},
		{"the second folder", RevealFolder{Dir: "Design/notes"}},
		{"the note itself", nil},
	} {
		t.Run(c.what, func(t *testing.T) {
			b := newBandTrailFrame(t, m, windowW)
			posted := b.pressPlace(i, places)
			if c.want == nil {
				if len(posted) != 0 {
					t.Errorf("a press on %s posted %v; it is where the reader already is", c.what, posted)
				}
				return
			}
			if len(posted) != 1 || posted[0] != c.want {
				t.Errorf("a press on %s posted %v, want %v", c.what, posted, c.want)
			}
		})
	}
}

// TestTheBandsTrailElidesTheMiddleWhenNarrow holds the path bar's rule at a
// band too narrow for the whole trail: the first place and the last stand,
// the places between them are dropped behind one ellipsis, and the row fits
// the room the band has for it. The ellipsis answers nothing — it stands for
// several places and there is no one place to go to.
func TestTheBandsTrailElidesTheMiddleWhenNarrow(t *testing.T) {
	m := bandTrailModel()
	places := notePlaces(m)
	tok := goldenTokens()
	f := newFrameState(defaultArrangement())

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(windowW, toolbarDepthDp)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	// The whole trail stands while the band has room for it.
	wide := bandTrailSegments(gtx, tok, m, 1<<14, f.revealPlace)
	if len(wide) != len(places) {
		t.Fatalf("a band with room to spare drew %d of %d places: %v", len(wide), len(places), titlesOf(wide))
	}

	// Then a band one pixel too narrow for the whole trail, read off the
	// row itself: the room the window's own arithmetic leaves the trail
	// there is what the trail is cut to.
	measure := trailMeasure(gtx, tok)
	loose := gtx
	loose.Constraints = layout.Constraints{Max: image.Pt(1<<14, toolbarDepthDp)}
	// What the band owes the controls standing after the trail, read as the
	// row reads it rather than restated here.
	after := 1<<14 - f.bandTrailRoom(loose, m)
	rowW := bandTrailLeading + after + measure(wide) - 1

	child := gtx
	child.Constraints = layout.Constraints{Max: image.Pt(rowW-bandTrailLeading, toolbarDepthDp)}
	room := f.bandTrailRoom(child, m)
	got := bandTrailSegments(child, tok, m, room, f.revealPlace)
	if reflect.DeepEqual(titlesOf(got), titlesOf(wide)) {
		t.Fatalf("a band %d px wide, a pixel short of the whole trail, still drew every place: %v",
			rowW, titlesOf(got))
	}
	// One place was dropped and one ellipsis stands for it, the first place
	// and the last still standing.
	want := []string{places[0].title, trailEllipsis, places[2].title, places[3].title}
	if !reflect.DeepEqual(titlesOf(got), want) {
		t.Fatalf("at a band %d px wide the trail drew %v, want %v", rowW, titlesOf(got), want)
	}
	if w := measure(got); w > room {
		t.Errorf("the elided trail lays out %d px wide in %d px of band", w, room)
	}
	if got[1].OnClick != nil {
		t.Error("the ellipsis answers a press; it stands for several places and goes to none")
	}
	if got[0].OnClick == nil || got[len(got)-1].OnClick != nil {
		t.Error("the elided trail did not keep its first place answering and its last plain")
	}
}

// titlesOf reads the titles a row of segments draws, in order.
func titlesOf(segs []breadcrumb.Segment) []string {
	out := make([]string, len(segs))
	for i, s := range segs {
		out[i] = s.Title
	}
	return out
}
