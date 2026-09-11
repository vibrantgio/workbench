package main

// Shared headless test helpers.

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/tabs"
	"github.com/vibrantgio/theme/tokens"
)

// staticTabs builds the whole strip from the per-tab static renderers,
// each cell's content in the shell's own contentSlot — the composition
// the app draws, minus the streams. It walks tabPages, so a tab added to
// the shell and forgotten here does not compile rather than quietly
// dropping out of the seam test and the review captures.
func staticTabs(
	shaper *text.Shaper,
	guide []byte,
	st outlineState,
	c tokens.PlatformColors,
	typo tokens.Typography,
) []tabs.Tab {
	out := make([]tabs.Tab, len(tabPages))
	for i, page := range tabPages {
		var content layout.Widget
		switch page {
		case pageDocs:
			content = renderDocsTab(shaper, guide, st, c, typo)
		case pageTheme:
			content = renderThemeTab(shaper, c, typo)
		default:
			content = renderGroupTab(shaper, tabGroups[page], c, typo)
		}
		out[i] = tabs.Tab{Label: tabLabels[i], Content: contentSlot(content)}
	}
	return out
}

// scene fills the frame with a background colour and lays the
// layout.Widget over it, so captures have a deterministic fill.
func scene(w layout.Widget, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// drawOnce lays a layout.Widget out for one frame at the given size and
// returns its dimensions; one that fails to compose panics or reports zero.
func drawOnce(t *testing.T, size image.Point, w layout.Widget) layout.Dimensions {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	return w(gtx)
}
