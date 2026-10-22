package main

// The toolbar's Share: a bordered button at the trailing end of the share
// over the article, titled with the platform's share symbol in the toolbar's
// own label colour. A toolbar action is a button — never a word in the accent
// and never a link — so what is held here is the button's own box, the colour
// its symbol reads in, that no accent stands at its site, and that a press on
// it posts the popover's message.

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// shareButtonSite is where the window stands the button, stated from the
// numbers the window and the platform give rather than read off a render:
// the toolbar's own trailing gutter, the room the share reserves for the
// button, and the toolbar control's measured height centred in the band.
func shareButtonSite() image.Rectangle {
	var (
		trail = int(shell.PlatformToolbar.Trail)
		depth = int(shell.ToolbarDepth)
		high  = int(tokens.Comfortable.ToolbarControlHeight)
		top   = (depth - high) / 2
	)
	return image.Rect(windowSize.X-trail-shareCapDp, top, windowSize.X-trail, top+high)
}

// TestTheShareButtonWearsTheToolbarsLabelColour reads the symbol off a render
// of the window: the colour furthest from the fill the button carries is the
// toolbar label over that fill, which is what every other symbol in a
// toolbar control reads in.
func TestTheShareButtonWearsTheToolbarsLabelColour(t *testing.T) {
	site := shareButtonSite()
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			if img == nil {
				t.Skip("no capture backend")
			}
			fill := vgcolor.Flatten(tc.c.ToolbarControlFill, tc.c.ControlBackground)
			want := vgcolor.Flatten(tc.c.ToolbarLabel, fill)
			if got := emptyStatePlateau(img, fill, site); got != want {
				t.Errorf("the Share button's symbol plateaus at %v, want the toolbar label over its own fill, %v", got, want)
			}
		})
	}
}

// TestTheShareButtonDrawsNoAccent is the other half of the ruling: the button
// carries no blue at all, where the word it replaced was drawn in the accent
// from end to end. The window's own blue sweep names every place this window
// has a blue to draw, and the button's site is not among them; this reads the
// site itself so the claim stands on its own.
func TestTheShareButtonDrawsNoAccent(t *testing.T) {
	site := shareButtonSite()
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := renderWindow(t, tc.c)
			if img == nil {
				t.Skip("no capture backend")
			}
			for y := site.Min.Y; y < site.Max.Y; y++ {
				for x := site.Min.X; x < site.Max.X; x++ {
					if c := at(img, x, y); isBlue(c) || sameColour(c, tc.c.ControlAccent) {
						t.Fatalf("the Share button draws %v at (%d,%d); a toolbar action is a button in the label colour and never a word in the accent", c, x, y)
					}
				}
			}
		})
	}
}

// sameColour answers whether two painted colours are the same, alpha aside: a
// render carries the flattened result, so an accent with alpha of its own is
// compared on what it paints.
func sameColour(a, b color.NRGBA) bool { return a.R == b.R && a.G == b.G && a.B == b.B }

// shareButtonFrame drives the live Share control — sharePopover's own anchor,
// with its own clickable — through a real input.Router, in the box the
// window's overlay hands it: the toolbar row less its trailing gutter.
type shareButtonFrame struct {
	ops    op.Ops
	r      gioinput.Router
	w      layout.Widget
	size   image.Point
	posted []mvu.Message
}

func newShareButtonFrame(t *testing.T) *shareButtonFrame {
	t.Helper()
	b := &shareButtonFrame{
		size: image.Pt(windowSize.X-int(shell.PlatformToolbar.Trail), int(shell.ToolbarDepth)),
	}
	obs := sharePopover(
		rx.Of(golden.StaticTheme(tokens.PlatformLight)),
		rx.Of(false),
		popover.NewArbiter(),
		func(_ layout.Context, msg mvu.Message) { b.posted = append(b.posted, msg) },
	)
	sub := obs.Subscribe(rx.GoroutineContext(), func(next layout.Widget, err error, done bool) {
		if !done && next != nil {
			b.w = next
		}
	})
	t.Cleanup(func() { sub.Unsubscribe() })
	sub.Wait()
	if b.w == nil {
		t.Fatal("the Share control emitted nothing at all")
	}
	return b
}

func (b *shareButtonFrame) frame() {
	b.ops.Reset()
	b.w(layout.Context{
		Constraints: layout.Exact(b.size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &b.ops,
		Source:      b.r.Source(),
	})
	b.r.Frame(&b.ops)
}

// click presses and releases at x on the band's centre line and lays out the
// frame that takes it, then one more so the clickable's answer is drained.
func (b *shareButtonFrame) click(x int) []mvu.Message {
	b.posted = nil
	at := f32.Pt(float32(x)+0.5, float32(b.size.Y)/2)
	b.r.Queue(
		pointer.Event{Kind: pointer.Press, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
		pointer.Event{Kind: pointer.Release, Position: at, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary},
	)
	b.frame()
	b.frame()
	return b.posted
}

// TestTheShareButtonTakesTheToolbarControlsOwnBox walks the band and asks
// where the button answers a press. What it answers over is the toolbar
// control's box — the 24 dp mark with the platform's measured clear room on
// each side — standing on the toolbar's own trailing gutter. A bare mark
// would answer over 24 and a word over whatever it happened to set to.
func TestTheShareButtonTakesTheToolbarControlsOwnBox(t *testing.T) {
	b := newShareButtonFrame(t)
	b.frame()
	b.frame()

	lo, hi := -1, -1
	for x := b.size.X - 2*shareCapDp; x < b.size.X; x++ {
		if !postedOneOf(b.click(x), ToggleShare{}) {
			continue
		}
		if lo < 0 {
			lo = x
		}
		hi = x
	}
	if lo < 0 {
		t.Fatal("the Share button answers nowhere in the share over the article")
	}
	if w := hi - lo + 1; w != shareCapDp {
		t.Errorf("the Share button answers over %d dp, want the toolbar control's own %d", w, shareCapDp)
	}
	if hi != b.size.X-1 {
		t.Errorf("the Share button ends %d dp before the room's trailing edge, want it standing on the toolbar's own gutter", b.size.X-1-hi)
	}
}

// TestTheShareButtonPostsToggleShare is the button's one act: a press on it
// posts the message that opens the popover, which is what the word it
// replaced posted.
func TestTheShareButtonPostsToggleShare(t *testing.T) {
	b := newShareButtonFrame(t)
	b.frame()
	b.frame()

	site := shareButtonSite()
	mid := b.size.X - shareCapDp/2
	if got := b.click(mid); !postedOneOf(got, ToggleShare{}) {
		t.Errorf("a press on the Share button at %d posted %v, want a main.ToggleShare", mid, got)
	}
	if got, want := b.size.X-shareCapDp, site.Min.X; got != want {
		t.Errorf("the button the press reached begins at %d in the room the overlay gives it, which is window column %d; the site read off the window is %d",
			got, got, want)
	}
}
