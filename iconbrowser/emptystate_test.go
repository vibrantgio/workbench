package main

// The icon grid standing with nothing to show, held against the platform's
// empty state: the line is drawn by patterns/shell in the frame the page
// hands its content region, and it spends the platform's tertiary label
// over the window's own plane.
//
// The frame is read off a render rather than recomputed from the page's
// insets: a probe stands in for the grid, paints the whole frame it is
// given, and the picture says where that frame was.

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// noMatchQuery is a search no icon's name carries, which is the state the
// grid stands empty in, and noMatchLine what the page then says.
const noMatchQuery = "zzzz"

var noMatchLine = fmt.Sprintf("No Icons Match %q", noMatchQuery)

// TestTheGridsEmptyStateCentresInItsFrame holds the line's placement
// against the arrangement the window actually laid out: it centres both
// ways in the frame the page gives the grid, at the measured size a
// content region's empty state is set at, and in the platform's tertiary
// label over the window's plane.
func TestTheGridsEmptyStateCentresInItsFrame(t *testing.T) {
	model := Model{Query: noMatchQuery}
	for _, tc := range windowSchemes {
		t.Run(tc.name, func(t *testing.T) {
			frame := gridFrame(t, tc.c, model)
			if frame.Empty() {
				t.Fatal("the page gave its content region no frame")
			}
			plane := tc.c.WindowBackground
			got := gridTextBox(renderWindow(t, tc.c, noMatchQuery, titleBandDp), frame, plane)
			if got.Empty() {
				t.Fatal("the grid is bare, want the line the empty state draws")
			}

			// What the pattern draws by itself in a frame of this size.
			ref := gridEmptyStateProbe(t, tc.c, frame.Size())
			refBox := gridTextBox(ref, ref.Bounds(), plane)
			if want := refBox.Add(frame.Min); got != want {
				t.Errorf("the grid draws its line at %v, want %v — the frame the line centres in is the grid's own",
					got, want)
			}
			// The box read here is the painted text's own and the line
			// box is what the pattern centres, so the two centres differ
			// by what the descenders reach below the box's middle: a
			// pixel across and two down at this size.
			if c, want := refBox.Min.X+refBox.Max.X, frame.Dx(); abs(c-want) > 2 {
				t.Errorf("the line's text centres at %.1f across, want the grid's %.1f",
					float64(c)/2, float64(want)/2)
			}
			if c, want := refBox.Min.Y+refBox.Max.Y, frame.Dy(); abs(c-want) > 4 {
				t.Errorf("the line's text centres at %.1f down, want the grid's %.1f",
					float64(c)/2, float64(want)/2)
			}

			// In the role a content region's empty state is set in.
			style := tokens.DefaultTypography.HeadlineMedium
			style.LineHeight = 0
			if h, want := got.Dy(), gridLineHeight(t, tc.c, style); h != want {
				t.Errorf("the line stands %d pixels tall, want the %d sp reading's %d",
					h, int(style.Size), want)
			}

			// And it spends the platform's tertiary label over the plane.
			want := vgcolor.Flatten(tc.c.TertiaryLabel, plane)
			if p := gridPlateau(renderWindow(t, tc.c, noMatchQuery, titleBandDp), plane, frame); p != want {
				t.Errorf("the line's text plateaus at #%02x%02x%02x, want TertiaryLabel over the plane, #%02x%02x%02x",
					p.R, p.G, p.B, want.R, want.G, want.B)
			}
		})
	}
}

// gridFrame is the frame the page hands its content region, read off a
// render of the window with a probe standing in for the icon grid: the
// probe paints every pixel of the frame it is given, so the picture says
// where that frame was.
func gridFrame(t *testing.T, c tokens.PlatformColors, model Model) image.Rectangle {
	t.Helper()
	tok := staticThemed(t, c)
	mark := color.NRGBA{R: 0xff, B: 0xff, A: 0xff}
	probe := func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		paint.FillShape(gtx.Ops, mark, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	}
	page := desktop.CapTop(func() unit.Dp { return titleBandDp },
		pageOver(tok, staticSearch(t, c, model.Query), model, len(FilterIcons(model.Query)), probe))
	paintBackdrop := backdrop.Widget(tok.palette.Backdrop)
	img := golden.Capture(t, windowSize, func(gtx layout.Context) layout.Dimensions {
		paintBackdrop(gtx)
		return page(gtx)
	})
	box := image.Rectangle{}
	first := true
	size := img.Bounds().Size()
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			if pixelAt(img, image.Pt(x, y)) != mark {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// gridEmptyStateProbe draws the pattern alone in a frame of the given size
// on the window's own plane, which is the fill the grid stands on.
func gridEmptyStateProbe(t *testing.T, c tokens.PlatformColors, size image.Point) *image.RGBA {
	t.Helper()
	plane := c.WindowBackground
	typo := tokens.DefaultTypography
	return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, plane, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return shell.EmptyState(gtx, typo.DeterministicShaper(), c, typo, plane,
			shell.EmptyStateProps{Line: noMatchLine})
	})
}

// gridLineHeight is how tall the line's painted text stands when it is
// set at a stated style in the colour the empty state spends.
func gridLineHeight(t *testing.T, c tokens.PlatformColors, style tokens.TextStyle) int {
	t.Helper()
	plane := c.WindowBackground
	size := image.Pt(1200, 200)
	img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, plane, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return typeset.Text(gtx, tokens.DefaultTypography.DeterministicShaper(), noMatchLine,
			style, vgcolor.Flatten(c.TertiaryLabel, plane), 1)
	})
	return gridTextBox(img, img.Bounds(), plane).Dy()
}

// gridTextBox is the box the line's painted text occupies inside one
// region of a render: every pixel the region's own fill does not account
// for.
func gridTextBox(img *image.RGBA, region image.Rectangle, fill color.NRGBA) image.Rectangle {
	box := image.Rectangle{}
	first := true
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if pixelAt(img, image.Pt(x, y)) == fill {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// gridPlateau is the painted colour furthest from the fill inside a
// region, which is the colour of the pixels the text covers wholly.
func gridPlateau(img *image.RGBA, fill color.NRGBA, region image.Rectangle) color.NRGBA {
	var found color.NRGBA
	far := -1
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			c := pixelAt(img, image.Pt(x, y))
			d := abs(int(c.R)-int(fill.R)) + abs(int(c.G)-int(fill.G)) + abs(int(c.B)-int(fill.B))
			if d > far {
				far, found = d, c
			}
		}
	}
	return found
}

// abs is the distance of n from nought.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
