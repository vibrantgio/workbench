package main

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"

	"github.com/reactivego/rx"

	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"
	"github.com/vibrantgio/textdraw"
)

// buildLayers returns the layer-builder the spectrum window renders.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// themed carries one theme emission's palette plus the 961 icon widgets
// prebuilt in that theme's glyph colour. Prebuilding is cheap —
// raster.Widget only decodes the viewBox up front and rasterises lazily,
// caching per size — and it means a keystroke re-filters prebuilt widgets
// instead of reconstructing them.
type themed struct {
	palette Palette
	icons   []layout.Widget
}

// ContentLayer renders the page: search field over the filtered icon grid.
//
// The two stateful widgets deliberately live at subscription scope, OUTSIDE
// the per-emission Map (llm.txt rule 2): the grid's scroll position, and the
// search field — a prism TextField whose editor state is Defer-scoped inside
// the component, subscribed exactly once by the CombineLatest3 below.
// Constructing either per emission would reset scroll or typing on every
// keystroke.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	shaper := text.NewShaper(text.WithCollection(style.FontFaces()))
	grid := &layout.List{Axis: layout.Vertical}

	search := input.TextField(th, input.TextFieldProps{
		Placeholder: "Search icons…",
		Description: "search icons by name",
		OnChange: func(gtx layout.Context, text string) {
			mvu.MessageOp{Message: SetQuery{Text: text}}.Add(gtx.Ops)
		},
	})

	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(t.Color, func(c tokens.ColorTokens) themed {
			p := PaletteFrom(c)
			widgets := make([]layout.Widget, len(IconTable))
			for i, icon := range IconTable {
				w, err := raster.Widget(icon.Data, IconSize, IconSize, raster.WithColors(p.Icon))
				if err != nil {
					panic(fmt.Sprintf("icon %s: %v", icon.Name, err))
				}
				widgets[i] = w
			}
			return themed{palette: p, icons: widgets}
		})
	})

	return rx.Map(rx.CombineLatest3(themes, search, modelObs),
		func(next rx.Tuple3[themed, layout.Widget, Model]) layout.Widget {
			return Page(shaper, next.First, next.Second, next.Third, grid)
		})
}

// Page stacks the search field above the grid of icons matching the query.
func Page(shaper *text.Shaper, t themed, search layout.Widget, model Model, grid *layout.List) layout.Widget {
	visible := FilterIcons(model.Query)
	gridWidget := Grid(shaper, t, visible, model.Query, grid)
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(Padding).Layout(gtx, search)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(Padding).Layout(gtx, gridWidget)
			}),
		)
	}
}

// Grid lays the visible icons out in rows of as many fixed-size cells as fit
// the width, scrolled by the subscription-scoped list state. Each cell shows
// the glyph with its exported name captioned underneath.
func Grid(shaper *text.Shaper, t themed, visible []int, query string, grid *layout.List) layout.Widget {
	p := t.palette
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max

		if len(visible) == 0 {
			notice := fmt.Sprintf("No icons match %q", query)
			textdraw.FillText(gtx, shaper, style.H6, image.Rectangle{Max: size}, 0.5, 0.5, p.Muted, notice)
			return layout.Dimensions{Size: size}
		}

		cellW, cellH := gtx.Dp(CellW), gtx.Dp(CellH)
		iconPx := gtx.Dp(IconSize)
		cols := max(1, size.X/cellW)
		rows := (len(visible) + cols - 1) / cols

		return grid.Layout(gtx, rows, func(gtx layout.Context, row int) layout.Dimensions {
			for col := 0; col < cols; col++ {
				i := row*cols + col
				if i >= len(visible) {
					break
				}
				icon := visible[i]
				cell := image.Rect(col*cellW, 0, (col+1)*cellW, cellH)

				cl := clip.Rect(cell).Push(gtx.Ops)

				// Glyph centred in the cell's upper part.
				off := op.Offset(image.Pt(cell.Min.X+(cellW-iconPx)/2, gtx.Dp(8))).Push(gtx.Ops)
				cgtx := gtx
				cgtx.Constraints = layout.Exact(image.Pt(iconPx, iconPx))
				t.icons[icon](cgtx)
				off.Pop()

				// Name captioned below the glyph.
				captionRect := image.Rect(cell.Min.X, gtx.Dp(8)+iconPx+gtx.Dp(4), cell.Max.X, cellH)
				textdraw.FillText(gtx, shaper, Caption, captionRect, 0.5, 0.0, p.Text, IconTable[icon].Name)

				cl.Pop()
			}
			return layout.Dimensions{Size: image.Pt(size.X, cellH)}
		})
	}
}
