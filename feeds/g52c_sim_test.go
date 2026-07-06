// g52c_sim_test.go verifies the G5.2c live behaviours headlessly, at the
// pixel level, against the REAL composed shell — the same widget tree the
// running app renders. The mvu message loop itself (click → MessageOp →
// collector → Update) is exercised by mvu's own collector tests and was
// proven live by GX.10; here the messages are applied to the model directly
// and the assertions are on rendered output:
//
//   - SelectArticle populates the detail (right) pane,
//   - SelectTab swaps the pane's content (Reader / Raw / Comments),
//   - ToggleShare paints the Share popover and CloseShare restores the
//     pre-open frame exactly,
//   - hovering the Unread ("•") header shows the tooltip after its delay,
//     driven through a real gioui.org/io/input.Router.
package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/tooltip"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// awaitStableWidget drains the emission channel until it has been silent for
// quiet, returning the LAST widget seen. Model changes fan out through
// CombineLatest chains (and the pagination SwitchMap re-subscription), so a
// single send can produce several intermediate emissions; pixel assertions
// must run against the settled one.
func awaitStableWidget(t *testing.T, emissions <-chan layout.Widget, what string) layout.Widget {
	t.Helper()
	const quiet = 150 * time.Millisecond
	var last layout.Widget
	deadline := time.Now().Add(2 * time.Second)
	for {
		select {
		case w := <-emissions:
			last = w
		case <-time.After(quiet):
			if last != nil {
				return last
			}
			if time.Now().After(deadline) {
				t.Fatalf("no widget emission after %s", what)
				return nil
			}
		}
	}
}

// regionDiff counts differing pixels between a and b inside r.
func regionDiff(a, b *image.RGBA, r image.Rectangle) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	r = r.Intersect(a.Bounds())
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			off := (y-a.Bounds().Min.Y)*a.Stride + (x-a.Bounds().Min.X)*4
			if a.Pix[off] != b.Pix[off] ||
				a.Pix[off+1] != b.Pix[off+1] ||
				a.Pix[off+2] != b.Pix[off+2] ||
				a.Pix[off+3] != b.Pix[off+3] {
				n++
			}
		}
	}
	return n
}

// rightPaneRegion is a conservative window inside the detail pane: the main
// area starts after the 192 dp sidebar, the split sits at ratio 0.6 of the
// remaining 1008 px (≈ x 799 at PxPerDp 1), and the navbar occupies the top
// 64 px.
var rightPaneRegion = image.Rect(810, 80, shellCanvasW-10, shellCanvasH-20)

// TestG52cDetailPopoverStatesHeadless renders the real shell at six model
// states and asserts the pixel-level deltas the G5.2c Measurable describes.
func TestG52cDetailPopoverStatesHeadless(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	send, modelObs := rx.Subject[Model](0, 1, 256)
	layer := feedsShellLayer(rx.Of(theme.Default()), shaper, modelObs)

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	}, rx.Goroutine)
	defer sub.Unsubscribe()

	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	size := image.Pt(shellCanvasW, shellCanvasH)
	snap := func(what string) *image.RGBA {
		w := awaitStableWidget(t, emissions, what)
		img := capture(t, size, scene(w, bg))
		if img == nil {
			t.Skip("headless rendering unavailable")
		}
		return img
	}

	m := initialModel()
	send.Next(m)
	placeholder := snap("initial model")

	// Clicking an article (its SelectArticle message) populates the pane.
	m, _ = Update(m, SelectArticle{Article: "go-blog-01"})
	send.Next(m)
	reader := snap("SelectArticle")
	if n := regionDiff(placeholder, reader, rightPaneRegion); n <= 0 {
		t.Errorf("detail pane unchanged after SelectArticle (diff=%d); pane did not populate", n)
	}

	// Switching tabs swaps the pane content: Raw renders the same body in
	// Go Mono, Comments renders the placeholder list.
	m, _ = Update(m, SelectTab{Idx: tabRaw})
	send.Next(m)
	raw := snap("SelectTab(Raw)")
	if n := regionDiff(reader, raw, rightPaneRegion); n <= 0 {
		t.Errorf("detail pane unchanged after SelectTab(Raw) (diff=%d); tabs did not swap", n)
	}

	m, _ = Update(m, SelectTab{Idx: tabComments})
	send.Next(m)
	comments := snap("SelectTab(Comments)")
	if n := regionDiff(raw, comments, rightPaneRegion); n <= 0 {
		t.Errorf("detail pane unchanged after SelectTab(Comments) (diff=%d); tabs did not swap", n)
	}

	// ToggleShare paints the popover; CloseShare must restore the pre-open
	// frame EXACTLY (the widgets are pure functions of model + theme).
	m, _ = Update(m, ToggleShare{})
	send.Next(m)
	shareOpen := snap("ToggleShare")
	if n := pixelDiff(comments, shareOpen); n <= 0 {
		t.Errorf("frame unchanged after ToggleShare (diff=%d); popover did not open", n)
	}

	m, _ = Update(m, CloseShare{})
	send.Next(m)
	shareClosed := snap("CloseShare")
	if n := pixelDiff(comments, shareClosed); n != 0 {
		t.Errorf("frame after CloseShare differs from pre-open frame by %d pixel(s); popover did not dismiss cleanly", n)
	}
}

// Tooltip-harness canvas: a stand-in table pane the size of the real one.
const (
	tipCanvasW = 800
	tipCanvasH = 400
)

// TestUnreadTooltipHoverHeadless drives the Unread-header tooltip through a
// real input.Router: a pointer.Move into the header cell, the show delay
// elapsing via gtx.Now, then a pixel assertion that the surface painted
// below the header. This is the same overlay composition articlesMain
// builds (overlayUnreadTooltip + tooltip.Tooltip).
func TestUnreadTooltipHoverHeadless(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	tipObs := tooltip.Tooltip(rx.Of(theme.Default()), tooltip.Props{
		Text:      "Unread",
		Placement: tooltip.Bottom,
		Shaper:    shaper,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
	})
	tip, err := collectOne(tipObs)
	if err != nil || tip == nil {
		t.Fatalf("tooltip widget: %v", err)
	}

	stubTable := func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, tokens.DefaultLight.Surface, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	overlay := overlayUnreadTooltip(stubTable, tip)

	size := image.Pt(tipCanvasW, tipCanvasH)
	r := new(gioinput.Router)
	t0 := time.Unix(100, 0)
	frame := func(now time.Time) {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(size),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Now:         now,
			Source:      r.Source(),
		}
		overlay(gtx)
		r.Frame(ops)
	}

	// Frame 1 registers the hover area; then a Move into the header cell
	// (the trailing 96×44 px), frame 2 records hover entry at t0, frame 3
	// past the delay flips the tooltip shown.
	frame(t0)
	r.Queue(pointer.Event{
		Kind:     pointer.Move,
		Position: f32.Pt(tipCanvasW-unreadColWDp/2, tableHeaderHDp/2),
		Source:   pointer.Mouse,
	})
	frame(t0)
	tShown := t0.Add(tooltip.DefaultDelay + time.Millisecond)
	frame(tShown)

	// Render the hovered state and a never-hovered baseline; the surface
	// must appear below the header, around the trigger's centre line.
	renderAt := func(now time.Time, src gioinput.Source, w layout.Widget) *image.RGBA {
		img := captureAt(t, size, w, now, src)
		if img == nil {
			t.Skip("headless rendering unavailable")
		}
		return img
	}
	hovered := renderAt(tShown.Add(time.Millisecond), r.Source(), overlay)

	freshTip, err := collectOne(tooltip.Tooltip(rx.Of(theme.Default()), tooltip.Props{
		Text:      "Unread",
		Placement: tooltip.Bottom,
		Shaper:    shaper,
		Trigger: func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
	}))
	if err != nil || freshTip == nil {
		t.Fatalf("baseline tooltip widget: %v", err)
	}
	baseline := renderAt(t0, gioinput.Source{}, overlayUnreadTooltip(stubTable, freshTip))

	surfaceRegion := image.Rect(tipCanvasW-3*unreadColWDp, tableHeaderHDp, tipCanvasW, tableHeaderHDp+80)
	if n := regionDiff(baseline, hovered, surfaceRegion); n <= 0 {
		t.Errorf("no pixels changed below the • header after hover + delay (diff=%d); tooltip did not show", n)
	}
}

// captureAt mirrors capture() but threads Now and an input Source into the
// layout context, which the live tooltip path needs.
func captureAt(t *testing.T, size image.Point, draw layout.Widget, now time.Time, src gioinput.Source) *image.RGBA {
	t.Helper()
	return captureCtx(t, size, func(gtx layout.Context) layout.Dimensions {
		gtx.Now = now
		gtx.Source = src
		paint.FillShape(gtx.Ops, color.NRGBA{R: 240, G: 240, B: 240, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return draw(gtx)
	})
}

func captureCtx(t *testing.T, size image.Point, draw layout.Widget) *image.RGBA {
	t.Helper()
	return capture(t, size, draw)
}
