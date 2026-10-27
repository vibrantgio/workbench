package main

import (
	"image"
	stdcolor "image/color"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/paragraph"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/dialog"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// windowSize is the window at the size it opens at.
var windowSize = image.Pt(windowW, windowH)

// underDialog is what the dialog layer is captured over: a colour that no
// part of the dialog draws, so the surface is the one run of the window's
// background in the capture.
var underDialog = stdcolor.NRGBA{R: 0xd0, G: 0x40, B: 0x40, A: 0xff}

// liveDialog subscribes the dialog layer for a model on the light set and
// returns the first thing it emits to lay out.
func liveDialog(t *testing.T, m Model, onClose func(gtx layout.Context)) layout.Widget {
	t.Helper()
	th := theme.Default()
	th.Platform = rx.Of(tokens.PlatformLight)
	th.Typography = rx.Of(tokens.DefaultTypography)
	got := make(chan layout.Widget, 1)
	sub := DialogLayer(rx.Of(th), rx.Of(m), nil, onClose).Subscribe(rx.GoroutineContext(),
		func(w layout.Widget, err error, done bool) {
			if err != nil || done || w == nil {
				return
			}
			select {
			case got <- w:
			default:
			}
		})
	t.Cleanup(sub.Unsubscribe)
	select {
	case w := <-got:
		return w
	case <-time.After(2 * time.Second):
		t.Fatal("the dialog layer emitted nothing to lay out")
	}
	return nil
}

// dialogParts captures w over [underDialog] and returns the capture, the
// dialog's surface and the close mark in the surface's header. The surface
// is the run of the window's background, and the mark is the run of other
// colours in the surface's top trailing corner.
func dialogParts(t *testing.T, w layout.Widget) (img *image.RGBA, surface, mark image.Rectangle) {
	t.Helper()
	img = golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
		fillRect(gtx, image.Rectangle{Max: windowSize}, underDialog)
		return w(gtx)
	})
	fill := tokens.PlatformLight.WindowBackground
	for y := 0; y < windowSize.Y; y++ {
		for x := 0; x < windowSize.X; x++ {
			if is(img.RGBAAt(x, y), fill) {
				surface = surface.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if surface.Empty() {
		t.Fatal("no dialog surface in the capture")
	}
	corner := image.Rect(surface.Max.X-48, surface.Min.Y, surface.Max.X, surface.Min.Y+48)
	for y := corner.Min.Y; y < corner.Max.Y; y++ {
		for x := corner.Min.X; x < corner.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if !is(c, fill) && c.R < 0x80 {
				mark = mark.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return img, surface, mark
}

// driver lays w out on a router of its own, one frame per call.
func driver(w layout.Widget) (*input.Router, func()) {
	r := new(input.Router)
	ops := new(op.Ops)
	return r, func() {
		ops.Reset()
		w(layout.Context{Ops: ops, Source: r.Source(), Constraints: layout.Exact(windowSize)})
		r.Frame(ops)
	}
}

// press queues a press and a release of the main mouse button at pt.
func press(r *input.Router, pt image.Point) {
	at := f32.Pt(float32(pt.X)+0.5, float32(pt.Y)+0.5)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: at, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
}

// TestAClickOnTheThemeColourOpensTheDialog: the pointer over the theme
// colour's swatch reaches the swatch's handler, whose click sends
// [OpenDialog]. With the model that message makes, the window draws the
// dialog, and the dialog's body holds the text.
//
// A test without a window cannot read the message a click adds, so the test
// proves the handler by hover and gives the reducer the message.
func TestAClickOnTheThemeColourOpensTheDialog(t *testing.T) {
	m := goldenModel(t, false)
	bar := new(topClicks)
	r, frame := driver(staticPage(m, tokens.PlatformLight, newCodeState(), desktop.ScrollBarsWhenScrolling, bar))
	frame()
	frame()
	y := colourBoxTop() + int(ColourBoxH)/2
	hit := false
	for x := windowW / 2; x < windowW && !hit; x++ {
		r.Queue(pointer.Event{Kind: pointer.Move, Position: f32.Pt(float32(x)+0.5, float32(y)+0.5), Source: pointer.Mouse})
		frame()
		hit = bar.swatch.Hovered()
	}
	if !hit {
		t.Fatal("no point of the Theme colour group's row reaches the swatch's handler")
	}

	opened := ReduceModel(m, OpenDialog{})
	if !opened.DialogOpen {
		t.Fatal("OpenDialog left the dialog closed")
	}
	if DialogText != "Drop an image on this window to take the theme colour from it." {
		t.Errorf("the dialog's text is %q", DialogText)
	}
	closed := liveDialog(t, m, nil)
	shut := golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
		fillRect(gtx, image.Rectangle{Max: windowSize}, underDialog)
		return closed(gtx)
	})
	if !is(shut.RGBAAt(windowW/2, windowH/2), underDialog) {
		t.Error("the dialog layer draws over the window while the dialog is closed")
	}

	img, surface, mark := dialogParts(t, liveDialog(t, opened, nil))
	if mark.Empty() {
		t.Error("the dialog's header has no close")
	}
	// The body is under the header: the text is the one thing drawn there.
	body := image.Rect(surface.Min.X, surface.Min.Y+48, surface.Max.X, surface.Max.Y)
	drawn := 0
	for y := body.Min.Y; y < body.Max.Y; y++ {
		for x := body.Min.X; x < body.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R < 0x80 {
				drawn++
			}
		}
	}
	if drawn < 200 {
		t.Errorf("the dialog's body draws %d dark pixels; the text draws more", drawn)
	}
}

// TestTheDialogClosesByItsCloseEscapeAndAClickOutside: each of the three
// ways out calls the dialog's close once.
func TestTheDialogClosesByItsCloseEscapeAndAClickOutside(t *testing.T) {
	m := ReduceModel(goldenModel(t, false), OpenDialog{})
	_, surface, mark := dialogParts(t, liveDialog(t, m, nil))
	if mark.Empty() {
		t.Fatal("no close mark to press")
	}
	for _, c := range []struct {
		name string
		act  func(r *input.Router)
	}{
		{"its close", func(r *input.Router) {
			press(r, image.Pt((mark.Min.X+mark.Max.X)/2, (mark.Min.Y+mark.Max.Y)/2))
		}},
		{"Escape", func(r *input.Router) {
			r.Queue(key.Event{Name: key.NameEscape, State: key.Press})
		}},
		{"a click outside it", func(r *input.Router) {
			press(r, image.Pt(surface.Min.X/2, surface.Min.Y/2))
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			closed := 0
			r, frame := driver(liveDialog(t, m, func(layout.Context) { closed++ }))
			// The first frame registers the dialog, the second gives it the
			// keyboard.
			frame()
			frame()
			c.act(r)
			frame()
			if closed != 1 {
				t.Errorf("the dialog's close ran %d times, want 1", closed)
			}
		})
	}
	if got := ReduceModel(m, CloseDialog{}); got.DialogOpen {
		t.Error("CloseDialog left the dialog open")
	}
}

// TestADropWhileTheDialogIsOpenSetsTheColourAndClosesIt drives the message
// the window's drop handling sends, and then the message the picture's read
// sends back.
func TestADropWhileTheDialogIsOpenSetsTheColourAndClosesIt(t *testing.T) {
	m := ReduceModel(ReduceModel(judging(), HexTyped{Text: "#e8112d"}), OpenDialog{})
	m, _ = Update(m, desktop.FilesDropped{Paths: []string{"/pictures/scene.png"}})
	if m.DialogOpen {
		t.Error("the dialog is still open after the drop")
	}
	candidates := dropped(t).Candidates
	m = ReduceModel(m, ImageLoaded{Path: "/pictures/scene.png", Candidates: candidates})
	col, ok := m.Color()
	if m.From != FromImage || !ok || col != candidates[0].Color {
		t.Errorf("the theme colour is %v from %v, want the picture's leading %v", col, m.From, candidates[0].Color)
	}
	if m.DialogOpen {
		t.Error("the picture's colours opened the dialog again")
	}
}

// TestTheKeyboardGoesBackWhenTheDialogCloses: the tag that held the keyboard
// before the dialog opened holds it again on the frame after the dialog
// closes.
func TestTheKeyboardGoesBackWhenTheDialogCloses(t *testing.T) {
	var field, closeTag int
	keys := new(keyboardReturn)
	keys.watch(&field)
	open, steal := false, false
	r := new(input.Router)
	ops := new(op.Ops)
	frame := func() {
		ops.Reset()
		gtx := layout.Context{Ops: ops, Source: r.Source(), Constraints: layout.Exact(windowSize)}
		keys.frame(gtx, open)
		tags := []event.Tag{&field}
		if open {
			tags = append(tags, &closeTag)
		}
		for i, tag := range tags {
			area := clip.Rect(image.Rect(0, i*20, 20, i*20+20)).Push(ops)
			event.Op(ops, tag)
			area.Pop()
			for {
				if _, ok := gtx.Event(key.FocusFilter{Target: tag}); !ok {
					break
				}
			}
		}
		if steal {
			gtx.Execute(key.FocusCmd{Tag: &closeTag})
			steal = false
		}
		r.Frame(ops)
	}
	frame()
	r.Source().Execute(key.FocusCmd{Tag: &field})
	frame()
	if !r.Source().Focused(&field) {
		t.Fatal("the field does not hold the keyboard before the dialog opens")
	}
	open, steal = true, true
	frame()
	frame()
	if !r.Source().Focused(&closeTag) {
		t.Fatal("the dialog did not take the keyboard")
	}
	open = false
	frame()
	frame()
	if !r.Source().Focused(&field) {
		t.Error("the keyboard did not go back to the field when the dialog closed")
	}
}

// TestTheColoursTabWithTheDialogOpen stores the Colours tab with the dialog
// open, in both appearances, at the size the window opens at.
//
// Regenerate with:
//
//	go test ./ -run TestTheColoursTabWithTheDialogOpen -golden.update
func TestTheColoursTabWithTheDialogOpen(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  tokens.PlatformColors
		dark bool
	}{{"light", tokens.PlatformLight, false}, {"dark", tokens.PlatformDark, true}} {
		t.Run(tc.name, func(t *testing.T) {
			m := ReduceModel(goldenModel(t, tc.dark), OpenDialog{})
			golden.Compare(t, "colours-dialog-"+tc.name, dialogPage(t, m, tc.set))
		})
	}
}

// dialogPage renders the page for m on c with the dialog's still render over
// it.
func dialogPage(t *testing.T, m Model, c tokens.PlatformColors) *image.RGBA {
	t.Helper()
	page := staticPage(m, c, newCodeState(), desktop.ScrollBarsWhenScrolling, new(topClicks))
	win := WindowSet(c, m)
	ty := pinned()
	typo := tokens.DefaultTypography
	box := dialog.Render(ty.Shaper, dialog.Props{
		Title: DialogTitle,
		Body:  DialogBody(win, typo.SystemFont, ty.Shaper, paragraph.NewState()),
	}, m.DialogOpen, win, tokens.Spacing, tokens.Radius, typo.Title2, tokens.Comfortable)
	return golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
		fillRect(gtx, image.Rectangle{Max: windowSize}, win.WindowBackground)
		page(gtx)
		return box(gtx)
	})
}
