package main

// promptfield_test.go pins the prompt field to CG4.12's ruling: a field
// states the surface it stands on rather than leaving its interior to
// coincide with the window's own plane by accident. The prompt field is
// built here through the same production constructor view.go calls
// (input.TextField, with Surface: promptFieldSurface), over a platform set
// whose ControlBackground differs from both its WindowBackground and its
// TextBackground, so a fill that only landed right because two platform
// names happened to match would show up as a failure.

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// promptFieldSize is the box the prompt field is captured in: wide enough
// that its trailing end is clear of the placeholder, and one row tall.
var promptFieldSize = image.Pt(320, 44)

// promptFieldFill renders the prompt field over live and returns its own
// interior, read at the trailing end of the row, clear of the placeholder
// and the rounded corners.
func promptFieldFill(t *testing.T, live tokens.PlatformColors) color.RGBA {
	t.Helper()
	th := theme.Default()
	th.Platform = rx.Of(live)

	widgets := make(chan layout.Widget, 1)
	sub := input.TextField(rx.Of(th), input.TextFieldProps{
		Placeholder: "Send a message",
		Description: "chat prompt",
		Surface:     promptFieldSurface,
		Submit:      true,
	}).Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
		if err != nil || done || w == nil {
			return
		}
		select {
		case widgets <- w:
		default:
		}
	})
	defer sub.Unsubscribe()

	var field layout.Widget
	select {
	case field = <-widgets:
	case <-time.After(2 * time.Second):
		t.Fatal("the prompt field emitted nothing to lay out — it is waiting on a stream that never carried it a value")
	}
	img := golden.Capture(t, promptFieldSize, func(gtx layout.Context) layout.Dimensions {
		return field(gtx)
	})
	return img.RGBAAt(promptFieldSize.X-14, promptFieldSize.Y/2)
}

// sameRGB reports whether a captured pixel is the colour named, ignoring
// alpha: every colour this field is handed is opaque.
func sameRGB(got color.RGBA, want color.NRGBA) bool {
	return got.R == want.R && got.G == want.G && got.B == want.B
}

// TestThePromptFieldStatesTheSurfaceItStandsOn: the prompt field is handed
// Surface: promptFieldSurface, so its interior follows ControlBackground —
// the transcript's own plane — even on a platform set where that no longer
// coincides with the window's own plane or the platform's text background.
func TestThePromptFieldStatesTheSurfaceItStandsOn(t *testing.T) {
	live := tokens.PlatformLight
	live.ControlBackground = color.NRGBA{R: 0x11, G: 0x66, B: 0xaa, A: 0xff}

	got := promptFieldFill(t, live)
	if !sameRGB(got, live.ControlBackground) {
		t.Fatalf("the prompt field's interior is %v, want the stated surface (ControlBackground) %v — Surface is not reaching the field", got, live.ControlBackground)
	}
	if sameRGB(got, live.TextBackground) {
		t.Fatalf("the prompt field's interior is the platform's text background %v — it is standing there only because the two names used to coincide, not because Surface states it", live.TextBackground)
	}
	if sameRGB(got, live.WindowBackground) {
		t.Fatalf("the prompt field's interior is the window's own plane %v — Surface is falling back to the default instead of stating ControlBackground", live.WindowBackground)
	}
}
