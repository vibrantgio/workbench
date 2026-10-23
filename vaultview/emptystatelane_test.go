package main

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// TestEmptyStatesStandOutsideAStandingBarsLane holds each region of this
// window that draws an empty state and has a bar against the reader's
// setting: where the region's bar stands, the frame its line centres in is
// the one the rows would occupy, short of the bar's lane on the trailing
// side, so the line's centre stands half a lane further in than where the
// bar fades; up and down it does not move. The lane is read off the bar
// each region draws its rows with.
func TestEmptyStatesStandOutsideAStandingBarsLane(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	paneSize := image.Pt(320, 300)
	regions := []struct {
		name string
		size image.Point
		fill func(tokens.PlatformColors) color.NRGBA
		lane func(themeTokens) int
		draw func(gtx layout.Context, tok themeTokens) layout.Dimensions
	}{
		{
			name: "the content column",
			size: noteFrameSize,
			fill: func(c tokens.PlatformColors) color.NRGBA { return c.TextBackground },
			lane: func(tok themeTokens) int { return int(noteScrollBar(tok).Width()) },
			draw: func(gtx layout.Context, tok themeTokens) layout.Dimensions {
				paint.FillShape(gtx.Ops, tok.col.TextBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return noteEmptyState(gtx, tok, "No Notes")
			},
		},
		{
			name: "the outline pane",
			size: paneSize,
			fill: chromeSurface,
			lane: func(tok themeTokens) int { return int(asideIndicator(tok).Width()) },
			draw: func(gtx layout.Context, tok themeTokens) layout.Dimensions {
				paint.FillShape(gtx.Ops, chromeSurface(tok.col), clip.Rect{Max: gtx.Constraints.Max}.Op())
				return newAsideView(&docCursor{}).outlinePane(gtx, tok, nil)
			},
		},
		{
			name: "the backlinks pane",
			size: paneSize,
			fill: chromeSurface,
			lane: func(tok themeTokens) int { return int(asideIndicator(tok).Width()) },
			draw: func(gtx layout.Context, tok themeTokens) layout.Dimensions {
				paint.FillShape(gtx.Ops, chromeSurface(tok.col), clip.Rect{Max: gtx.Constraints.Max}.Op())
				return newAsideView(&docCursor{}).backlinkPane(gtx, tok, nil)
			},
		},
		{
			name: "the rail rows",
			size: paneSize,
			fill: chromeSurface,
			lane: func(tok themeTokens) int { return int(asideIndicator(tok).Width()) },
			draw: func(gtx layout.Context, tok themeTokens) layout.Dimensions {
				paint.FillShape(gtx.Ops, chromeSurface(tok.col), clip.Rect{Max: gtx.Constraints.Max}.Op())
				v := &treeView{list: list.NewState()}
				return v.rows(gtx, Model{Screen: screenVault, Filter: "zzzz"}, tok)
			},
		},
	}
	for _, tc := range themeCases {
		for _, r := range regions {
			t.Run(tc.name+"/"+r.name, func(t *testing.T) {
				box := func(bars desktop.ScrollBarSetting) image.Rectangle {
					tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
						sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper, scrollBars: bars}
					fill := r.fill(tc.colors)
					img := golden.Capture(t, r.size, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints = layout.Exact(r.size)
						return r.draw(gtx, tok)
					})
					b := paneGlyphBox(img, img.Bounds(), fill)
					if b.Empty() {
						t.Fatalf("at %v %s is bare, want the line the empty state draws", bars, r.name)
					}
					return b
				}
				fading, standing := box(desktop.ScrollBarsWhenScrolling), box(desktop.ScrollBarsAlways)
				lane := r.lane(themeTokens{col: tc.colors, scrollBars: desktop.ScrollBarsAlways})
				if lane <= 0 {
					t.Fatalf("the standing bar's lane is %d px, want a lane", lane)
				}
				moved := (fading.Min.X + fading.Max.X) - (standing.Min.X + standing.Max.X)
				if abs(moved-lane) > 2 {
					t.Errorf("at Always the line's centre moves %.1f px in, want half the %d px lane, %.1f",
						float64(moved)/2, lane, float64(lane)/2)
				}
				if fading.Min.Y != standing.Min.Y || fading.Max.Y != standing.Max.Y {
					t.Errorf("at Always the line stands at y %d–%d, want %d–%d as where the bar fades",
						standing.Min.Y, standing.Max.Y, fading.Min.Y, fading.Max.Y)
				}
			})
		}
	}
}
