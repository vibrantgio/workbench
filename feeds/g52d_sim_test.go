// g52d_sim_test.go verifies the G5.2d CRUD behaviours: a deterministic golden
// of the Add-feed modal (light + dark, with the empty-URL alert banner), plus
// headless pixel checks against the REAL composed shell — the modal opens on
// OpenAddFeed, the alert appears on an empty SubmitFeed, a submitted feed
// appears in the sidebar, and ConfirmDelete removes a sidebar entry. The mvu
// message loop (click → MessageOp → collector → Update) is proven elsewhere;
// here the messages are applied to the model directly and the assertions are
// on rendered output, mirroring g52c_sim_test.go.
package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gesture"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/alert"
	"github.com/vibrantgio/cadence/card"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// modalCanvas is the canvas the Add-feed modal golden draws into.
const (
	modalCanvasW = 600
	modalCanvasH = 480
)

var modalSharpRadius = tokens.RadiusScale{}

// staticAddFeedModalBody assembles the modal Body from the static Render paths
// of the same components the live addFeedModal composes: a card wrapping the
// error alert (shown to capture the empty-submit state in the golden), the URL
// textfield, and the Add button. Sharp radii + the static Render paths keep
// the golden deterministic.
func staticAddFeedModalBody(shaper *text.Shaper, colors tokens.ColorTokens) layout.Widget {
	body := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		gap := gtx.Dp(unit.Dp(addFeedGapDp))
		alertH := gtx.Dp(unit.Dp(addFeedAlertHDp))
		fieldH := gtx.Dp(unit.Dp(addFeedFieldHDp))
		btnH := gtx.Dp(unit.Dp(addFeedBtnHDp))
		y := 0

		al := alert.Render(shaper, alert.Props{Variant: alert.Error, Title: "Feed URL required"},
			colors, tokens.Spacing, modalSharpRadius, tokens.DefaultTypeScale)
		s := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		ag := gtx
		ag.Constraints = layout.Exact(image.Pt(w, alertH))
		al(ag)
		s.Pop()
		y += alertH + gap

		fld := input.Render(shaper, "https://example.com/feed.xml",
			colors, tokens.Spacing, modalSharpRadius, tokens.DefaultTypeScale, input.RenderState{})
		s = op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		fg := gtx
		fg.Constraints = layout.Exact(image.Pt(w, fieldH))
		fld(fg)
		s.Pop()
		y += fieldH + gap

		btn := button.Render(shaper, "Add", colors, tokens.Spacing, modalSharpRadius,
			tokens.DefaultTypeScale, button.RenderState{})
		s = op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		bg := gtx
		bg.Constraints = layout.Exact(image.Pt(w, btnH))
		btn(bg)
		s.Pop()
		y += btnH
		return layout.Dimensions{Size: image.Pt(w, y)}
	}
	return func(gtx layout.Context) layout.Dimensions {
		c := card.Render(card.Props{Body: body}, colors, tokens.Spacing, modalSharpRadius)
		return c(gtx)
	}
}

// TestAddFeedModalGolden renders the Add-feed modal (open, with the alert
// banner) in light and dark token sets.
func TestAddFeedModalGolden(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"add-feed-modal-light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"add-feed-modal-dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := staticAddFeedModalBody(shaper, tc.colors)
			m := modal.Render(shaper, modal.Props{Title: "Add feed", Body: body, Shaper: shaper},
				true, tc.colors, tokens.Spacing, modalSharpRadius, tokens.DefaultTypeScale)
			renderGolden(t, tc.name, image.Pt(modalCanvasW, modalCanvasH), scene(m, tc.bg))
		})
	}
}

// sidebarRegion is a window over the sidebar area (the leading 192 dp at
// PxPerDp 1), below the navbar, where feed entries render.
var sidebarRegion = image.Rect(0, 80, feedsSidebarWidthDp, shellCanvasH-20)

// scrimRegion samples the centre of the window, where an open modal paints its
// scrim + surface over the shell.
var scrimRegion = image.Rect(shellCanvasW/2-200, shellCanvasH/2-150, shellCanvasW/2+200, shellCanvasH/2+150)

// TestG52dCrudStatesHeadless renders the real shell at the CRUD model states
// and asserts the pixel-level deltas the G5.2d Measurable describes:
// OpenAddFeed paints the modal scrim; an empty SubmitFeed paints the alert;
// a non-empty SubmitFeed adds a sidebar entry; ConfirmDelete removes one.
func TestG52dCrudStatesHeadless(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	send, modelObs := rx.Subject[Model](0, 1, 256)
	layer := feedsShellLayer(rx.Of(theme.Default()), shaper, modelObs)

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
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
	closed := snap("initial model")

	// OpenAddFeed paints the modal scrim + surface over the whole window.
	m, _ = Update(m, OpenAddFeed{})
	send.Next(m)
	modalOpen := snap("OpenAddFeed")
	if n := regionDiff(closed, modalOpen, scrimRegion); n <= 0 {
		t.Errorf("window unchanged after OpenAddFeed (diff=%d in scrim region); modal did not open", n)
	}

	// Empty SubmitFeed raises the alert band inside the modal.
	m, _ = Update(m, SubmitFeed{URL: ""})
	send.Next(m)
	withAlert := snap("SubmitFeed(empty)")
	if !m.addFeedError {
		t.Fatal("empty submit did not set addFeedError")
	}
	if n := regionDiff(modalOpen, withAlert, scrimRegion); n <= 0 {
		t.Errorf("modal unchanged after empty submit (diff=%d); alert did not appear", n)
	}

	// Close the modal (back to a baseline) and add a feed.
	m, _ = Update(m, CloseAddFeed{})
	send.Next(m)
	_ = snap("CloseAddFeed")

	m, _ = Update(m, SubmitFeed{URL: "https://added.test/feed.xml"})
	send.Next(m)
	added := snap("SubmitFeed(non-empty)")
	if !hasFeed(m, FeedID("added:https://added.test/feed.xml")) {
		t.Fatal("non-empty submit did not append the feed to the model")
	}
	// The new entry lands in the first (open) group, so the sidebar pixels change.
	if n := regionDiff(closed, added, sidebarRegion); n <= 0 {
		t.Errorf("sidebar unchanged after add (diff=%d); feed entry did not appear", n)
	}

	// ConfirmDelete removes a sidebar entry; the sidebar changes again.
	m, _ = Update(m, ConfirmDelete{Feed: "go-blog"})
	send.Next(m)
	deleted := snap("ConfirmDelete")
	if hasFeed(m, "go-blog") {
		t.Fatal("ConfirmDelete did not remove the feed from the model")
	}
	if n := regionDiff(added, deleted, sidebarRegion); n <= 0 {
		t.Errorf("sidebar unchanged after delete (diff=%d); entry was not removed", n)
	}
}

// TestHoverGutterDoesNotSwallowSelectPress is the regression guard for the
// G5.2d sidebar layout: a gesture.Hover area spans the whole feed row (to
// reveal the trash icon) and is registered UNDER a label-area widget.Clickable
// (the SelectFeed target). gesture.Hover filters only Enter/Leave/Cancel, so a
// press inside the label must still reach the clickable — click-to-select (a
// G5.2a feature) must survive the hover gutter. This drives a real
// input.Router exactly as drawFeedEntryRow composes the two, and asserts the
// underlying clickable registered the click.
func TestHoverGutterDoesNotSwallowSelectPress(t *testing.T) {
	var hover gesture.Hover
	var click widget.Clickable
	const w, h = 160, 28

	got := false
	// row composes exactly as drawFeedEntryRow does: Clicked() is checked
	// FIRST (the app loops over rows calling Clicked before laying them out —
	// widget.Clickable.Layout drains the same events, so the order matters),
	// then the hover area is registered under the label-area clickable.
	row := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if click.Clicked(gtx) {
			got = true
		}
		hover.Update(gtx.Source)
		hc := clip.Rect{Max: size}.Push(gtx.Ops)
		hover.Add(gtx.Ops)
		hc.Pop()
		lg := gtx
		lg.Constraints = layout.Exact(image.Pt(size.X-24, size.Y))
		return click.Layout(lg, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	}

	r := new(gioinput.Router)
	frame := func() {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(image.Pt(w, h)),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Now:         time.Now(),
			Source:      r.Source(),
		}
		row(gtx)
		r.Frame(ops)
	}

	frame() // register tags
	// Press + release inside the LABEL area (well left of the trash gutter).
	at := f32.Pt(40, h/2)
	r.Queue(pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	frame()
	r.Queue(pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary})
	frame() // drains the click
	if !got {
		t.Error("press in the label area did not reach the select clickable; the hover gutter swallowed it")
	}
}

// TestG52dShellReEmitsOnCrudMessages confirms the shell layer re-emits a fresh
// widget for each G5.2d message — the same-frame-repaint guarantee that the
// G5.2c regression test checks for the earlier message set.
func TestG52dShellReEmitsOnCrudMessages(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	send, modelObs := rx.Subject[Model](0, 1, 256)
	layer := feedsShellLayer(rx.Of(theme.Default()), shaper, modelObs)

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	await := func(what string) {
		w := awaitStableWidget(t, emissions, what)
		if w != nil {
			drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
		}
	}

	m := initialModel()
	send.Next(m)
	await("initial")

	for _, step := range []struct {
		name string
		msg  interface{}
	}{
		{"OpenAddFeed", OpenAddFeed{}},
		{"SubmitFeed(empty)", SubmitFeed{URL: ""}},
		{"CloseAddFeed", CloseAddFeed{}},
		{"SubmitFeed(non-empty)", SubmitFeed{URL: "https://x.test/f"}},
		{"ConfirmDelete", ConfirmDelete{Feed: "hn"}},
	} {
		m, _ = Update(m, step.msg)
		send.Next(m)
		await(step.name)
	}
}

// TestToastNotifyRendersInStack closes the verification gap the model-driven
// CRUD tests cannot reach: toast.Notify fires from the submit/confirm view
// callbacks (the package-global side-channel logged in FEEDBACK-G5.2.md),
// so driving Update directly never invokes it. This test exercises the
// actual Notify → package Subject → Stack render path: an empty stack
// renders no pixels, Notify("Feed added") re-emits the stack widget, and
// the rendered frame differs in the toast region.
func TestToastNotifyRendersInStack(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	stackObs := toast.Stack(rx.Of(theme.Default()), toast.Props{
		Position: toast.TopRight,
		Shaper:   shaper,
	})

	emissions := make(chan layout.Widget, 16)
	sub := stackObs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	size := image.Pt(600, 300)
	empty := awaitStableWidget(t, emissions, "seeded empty stack")
	before := capture(t, size, scene(empty, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))
	if before == nil {
		t.Skip("headless rendering unavailable")
	}

	toast.Notify(toast.Success, "Feed added")
	after := awaitStableWidget(t, emissions, "Notify ping")
	got := capture(t, size, scene(after, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))
	if got == nil {
		t.Skip("headless rendering unavailable")
	}
	if n := pixelDiff(before, got); n <= 0 {
		t.Errorf("stack frame unchanged after toast.Notify (diff=%d); toast did not render", n)
	}
}
