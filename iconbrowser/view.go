package main

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// buildLayers returns the layer-builder the theme window renders: the backdrop
// full-bleed to the window's top edge, the title-bar strip included, and the
// page over it starting below that strip.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// themed carries one theme emission's palette and typography plus the 961
// icon widgets prebuilt in that theme's glyph colour. Prebuilding is cheap —
// raster.Widget only decodes the viewBox up front and rasterises lazily,
// caching per size — and it means a keystroke re-filters prebuilt widgets
// instead of reconstructing them.
type themed struct {
	palette Palette
	typ     Type
	icons   []layout.Widget
}

// ContentLayer renders the page: search field over the filtered icon grid,
// held down past the native title-bar strip the window opens the top of itself
// into.
//
// The two stateful widgets deliberately live at subscription scope, OUTSIDE
// the per-emission Map (llms.txt rule 2): the grid's scroll position, and the
// search field — a components TextField whose editor state is Defer-scoped inside
// the component, subscribed exactly once by the CombineLatest3 below.
// Constructing either per emission would reset scroll or typing on every
// keystroke.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	grid := &layout.List{Axis: layout.Vertical}

	search := input.TextField(th, input.TextFieldProps{
		Placeholder: "Search icons…",
		Description: "search icons by name",
		OnChange: func(gtx layout.Context, text string) {
			mvu.MessageOp{Message: SetQuery{Text: text}}.Add(gtx.Ops)
		},
	})

	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Typography),
			func(n rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themed {
				p := PaletteFrom(n.First)
				widgets := make([]layout.Widget, len(IconTable))
				for i, icon := range IconTable {
					w, err := raster.Widget(icon.Data, IconSize, IconSize, raster.WithColors(p.Icon))
					if err != nil {
						panic(fmt.Sprintf("icon %s: %v", icon.Name, err))
					}
					widgets[i] = w
				}
				return themed{palette: p, typ: TypeFrom(n.Second), icons: widgets}
			})
	})

	return underTitleBar(rx.Map(rx.CombineLatest3(themes, search, modelObs),
		func(next rx.Tuple3[themed, layout.Widget, Model]) layout.Widget {
			return Page(next.First, next.Second, next.Third, grid)
		}))
}

// underTitleBar holds the page down past the native title-bar strip the
// full-size-content window opens the top of itself into, and claims that same
// strip for the window's own drag.
//
// The strip carries no fill of its own, and that is R6 satisfied rather than
// skipped: the region this band caps is the window's own ground — the
// Background pin BackdropLayer fills the whole window with — so the region's
// fill reaches the top edge without anything being painted twice. A band drawn
// here would be furniture this window does not have. Nothing in this window is
// chrome: the grid is the content ground, the two section labels are ink on it,
// and the search field is a control standing on it in the page's own vertical
// flow rather than a toolbar over it. Lifting that field into the strip would
// invent the toolbar — and would not fit in one either, since a components
// TextField is a Density.ControlHeight box (36 dp comfortable) and the band the
// window buttons are centred in is 32 (ADR-019).
//
// So what has to clear the platform's three control buttons is the field, and
// the inset is what buys it that clearance: the field is the page's topmost ink
// and the page starts below the strip. TestThePageClearsTheWindowButtons reads
// the result off the frame rather than trusting the arithmetic.
//
// desktop.CapTop is the other half of R6 with the inset: the native drag
// leaves with the native strip, and the strip here carries paint but no widget
// of its own, so without its claim the window could not be moved by its top
// edge at all. The claim is recorded before the page, so every region the page
// declares below it — the search field's editor and its focus catcher, the
// grid's scroll — keeps its own presses; the band and the page do not overlap
// in any case. desktop.TopInset is read at frame time, so away from the
// full-size-content treatment the whole cap is an exact no-op.
func underTitleBar(pageObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(pageObs, func(w layout.Widget) layout.Widget {
		return desktop.CapTop(desktop.TopInset, w)
	})
}

// The two sets the page shows, each under its own label: the design system's
// own marks first, because they are the ones a control should reach for, then
// the Material catalogue everything outside the standard controls comes from.
const (
	MarksHeading     = "Vibrant Gio marks"
	CatalogueHeading = "Material Design icons"
)

// Page stacks the search field over the two labelled sets: the design system's
// own marks, then the grid of Material icons matching the query. The marks
// section is dropped whole when the query matches none of them, so a search
// for a Material glyph is not padded by an empty row.
func Page(t themed, search layout.Widget, model Model, grid *layout.List) layout.Widget {
	visible := FilterIcons(model.Query)
	names := FilterMarks(model.Query)
	gridWidget := Grid(t, visible, model.Query, grid)
	markWidget := MarkGrid(t, names)
	marksHeading := Heading(t, MarksHeading, MarkSizeNote())
	catalogueHeading := Heading(t, CatalogueHeading, fmt.Sprintf("%d icons", len(visible)))
	sides := layout.Inset{Left: Padding, Right: Padding}
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 5)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(Padding).Layout(gtx, search)
		}))
		if len(names) > 0 {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return sides.Layout(gtx, marksHeading)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: Padding, Right: Padding, Bottom: Padding}.Layout(gtx, markWidget)
				}),
			)
		}
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return sides.Layout(gtx, catalogueHeading)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: Padding, Right: Padding, Bottom: Padding}.Layout(gtx, gridWidget)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// Grid lays the visible icons out in rows of as many fixed-size cells as fit
// the width, scrolled by the subscription-scoped list state. Each cell shows
// the glyph with its exported name captioned underneath.
func Grid(t themed, visible []int, query string, grid *layout.List) layout.Widget {
	p := t.palette
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max

		if len(visible) == 0 {
			notice := fmt.Sprintf("No icons match %q", query)
			textdraw.FillText(gtx, t.typ.Shaper, t.typ.Notice, image.Rectangle{Max: size}, 0.5, 0.5, p.Muted, notice)
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
				textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, captionRect, 0.5, 0.0, p.Text, IconTable[icon].Name)

				cl.Pop()
			}
			return layout.Dimensions{Size: image.Pt(size.X, cellH)}
		})
	}
}
