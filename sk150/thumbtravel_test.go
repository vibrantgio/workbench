package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
)

// TestPresetsThumbTravelsByTheMeasuredRule scrolls the Presets tab at Always
// by 10 px a step to its end, in both schemes, in a frame shorter than its
// slots.
//
// The platform's thumb moves by the measured rule: its offset is the thumb's
// length × the distance scrolled ÷ the visible height, and its length does
// not change while the content scrolls. At the end of the content the
// thumb's bottom is at the track's bottom. The slots and the spacers between
// them differ in height, so a bar that sizes the content from the rows in
// view breaks the rule.
func TestPresetsThumbTravelsByTheMeasuredRule(t *testing.T) {
	size := image.Pt(int(winW), 300)
	const step = 10
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			m := goldenModel()
			m.Screen = "presets"
			m.HavePresets = true
			w := windowFrameBars(t, tc.c, m, postMessage, desktop.ScrollBarsAlways)
			var (
				ops op.Ops
				r   gioinput.Router
			)
			frame := func() {
				ops.Reset()
				w(layout.Context{
					Constraints: layout.Exact(size),
					Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
					Ops:         &ops,
					Source:      r.Source(),
				})
				r.Frame(&ops)
			}
			shot := func() *image.RGBA {
				return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
					gtx.Source = r.Source()
					return w(gtx)
				})
			}

			bar := themed{platform: tc.c, scrollBars: desktop.ScrollBarsAlways}.listBar()
			pad := int(bar.TrackPadding)
			x := size.X - int(bar.Width())/2
			fill := pageFill(tc.c)
			// thumb returns the thumb's top and bottom down the lane's
			// middle column: the lines drawn at least half as far from the
			// fill as the line drawn farthest from it. The track is drawn
			// at a low coverage, so only the thumb passes.
			thumb := func(img *image.RGBA) (top, bot int) {
				dist := make([]int, size.Y)
				most := 0
				for y := range dist {
					c := at(img, x, y)
					d := max(absDiff(c.R, fill.R), absDiff(c.G, fill.G), absDiff(c.B, fill.B))
					dist[y] = d
					most = max(most, d)
				}
				top, bot = -1, -1
				for y, d := range dist {
					if most > 0 && 2*d >= most {
						if top < 0 {
							top = y
						}
						bot = y + 1
					}
				}
				return top, bot
			}

			frame()
			frame()
			prev := shot()
			top0, bot0 := thumb(prev)
			if top0 < 0 {
				t.Fatal("at Always a resting frame drew no thumb in the lane")
			}
			length := bot0 - top0
			// The lane runs from the track's padding above the resting
			// thumb to the frame's bottom; the visible height is the lane's.
			listTop := top0 - pad
			visible := size.Y - listTop
			t.Logf("rest: thumb y %d–%d, length %d, visible height %d", top0, bot0, length, visible)

			mid := f32.Pt(float32(size.X)/2, float32(size.Y-40))
			r.Queue(pointer.Event{Kind: pointer.Move, Position: mid, Source: pointer.Mouse})
			scrolled := 0
			var top, bot int
			for i := 0; ; i++ {
				if i > 100 {
					t.Fatal("the content did not reach its end after 100 steps")
				}
				r.Queue(pointer.Event{Kind: pointer.Scroll, Position: mid, Source: pointer.Mouse, Scroll: f32.Pt(0, step)})
				frame()
				frame()
				img := shot()
				if golden.PixelDiff(prev, img) == 0 {
					break
				}
				prev = img
				top, bot = thumb(img)
				scrolled += step
				if l := bot - top; l < length-1 || l > length+1 {
					t.Errorf("after %d px the thumb is %d px long; at rest it is %d px, and its length does not change", scrolled, l, length)
				}
				// The last step that moved the content can move it by less
				// than a whole step, so its offset is checked at the end.
				want := float64(length) * float64(scrolled) / float64(visible)
				if got := float64(top - top0); (got < want-1 || got > want+1) && top+length < size.Y-pad {
					t.Errorf("after %d px the thumb moved %v px; the rule gives %d × %d ÷ %d = %.1f", scrolled, got, length, scrolled, visible, want)
				}
			}
			if bot != size.Y-pad {
				t.Errorf("at the end of the content the thumb's bottom is at y %d; the track's bottom is at y %d", bot, size.Y-pad)
			}
		})
	}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
