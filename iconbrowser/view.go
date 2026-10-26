package main

import (
	"fmt"
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// buildLayers returns the layer-builder the theme window renders: the backdrop
// full-bleed to the window's top edge, the title-bar strip included, and the
// page over it starting below that strip.
func buildLayers(modelObs rx.Observable[Model], bars desktop.ScrollBarSetting) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs, bars),
		}
	}
}

// themed holds one theme emission's palette and typography plus the 961
// icons prebuilt in that theme's text colour. Prebuilding is cheap —
// `raster.Widget` only decodes the viewBox up front and rasterises lazily,
// caching per size — and it means a keystroke narrows the grid to prebuilt
// icons instead of reconstructing them.
type themed struct {
	palette Palette
	typ     Type
	icons   []layout.Widget

	// The platform's own set and the whole type set, carried beside the
	// app's view of them because patterns/shell reads them directly: the
	// grid's empty state is the pattern's, and it resolves its colour and
	// its line's typeface from these.
	platform   tokens.PlatformColors
	typography tokens.Typography

	// scrollBars is the platform's "Show scroll bars" setting, read once
	// when the window is composed and passed here so that no frame asks
	// the platform and no still render depends on the machine it runs on.
	scrollBars desktop.ScrollBarSetting
}

// scrollBarStyle is every bar in this window: the fading overlay bar the
// colours alone give, standing without fading where the reader asked for
// scroll bars always.
func scrollBarStyle(s scrollbar.Style, bars desktop.ScrollBarSetting) scrollbar.Style {
	if bars == desktop.ScrollBarsAlways {
		return s.Always()
	}
	return s
}

// gridBar is the bar the icon grid is drawn with: built from the platform's
// colours over the window's own plane, which is the fill the grid is drawn
// on, against the reader's setting.
func gridBar(t themed) scrollbar.Style {
	return scrollBarStyle(scrollbar.FromTokens(t.platform, t.palette.Backdrop), t.scrollBars)
}

// symbols builds every catalogue symbol in the palette's text colour: an icon
// on the content is drawn in the text colour, and the accent is passed only
// to a selected one, which this window has none of.
func symbols(p Palette) []layout.Widget {
	widgets := make([]layout.Widget, len(IconTable))
	for i, icon := range IconTable {
		w, err := raster.Widget(icon.Data, IconSize, IconSize, raster.WithColors(p.Text))
		if err != nil {
			panic(fmt.Sprintf("icon %s: %v", icon.Name, err))
		}
		widgets[i] = w
	}
	return widgets
}

// ContentLayer renders the page: the search field over the icon grid the
// query narrows, held down past the native title-bar strip the window opens
// the top of itself into.
//
// The two pieces of view state are kept at subscription scope, outside the
// per-emission Map: the grid's scroll position, and the search field, whose
// editor state is Defer-scoped inside the component and subscribed exactly
// once by the CombineLatest3 below. Constructing either per emission would
// reset scroll or typing on every keystroke.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], bars desktop.ScrollBarSetting) rx.Observable[layout.Widget] {
	grid := list.NewState()

	post := func(gtx layout.Context, msg mvu.Message) { mvu.MessageOp{Message: msg}.Add(gtx.Ops) }

	// field is the search field's focus tag. findsSeen is the Find count
	// the page last answered. The frame reads and writes both, and nothing
	// else does.
	var field event.Tag
	findsSeen := 0
	search := searchField(th, input.SearchFieldProps{FocusTag: func(tag event.Tag) { field = tag }},
		func(gtx layout.Context, msg SetQuery) { post(gtx, msg) })
	chords := ChordAreas(post)

	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Platform, t.Typography),
			func(n rx.Tuple2[tokens.PlatformColors, tokens.Typography]) themed {
				p := PaletteFrom(n.First)
				return themed{palette: p, typ: TypeFrom(n.Second), icons: symbols(p),
					platform: n.First, typography: n.Second, scrollBars: bars}
			})
	})

	return underTitleBar(rx.Map(rx.CombineLatest3(themes, search, modelObs),
		func(next rx.Tuple3[themed, layout.Widget, Model]) layout.Widget {
			page := Page(next.First, next.Second, next.Third, grid)
			finds := next.Third.Finds
			return func(gtx layout.Context) layout.Dimensions {
				for _, c := range chords {
					c(gtx)
				}
				if finds != findsSeen && field != nil {
					findsSeen = finds
					gtx.Execute(key.FocusCmd{Tag: field})
				}
				return page(gtx)
			}
		}))
}

// searchField is the page's search field: the platform's search field with
// its looking glass, the prompt "Search" and the clear mark, posting SetQuery
// with the field's text on every change, the clear mark's and Escape's
// emptying included.
//
// The field is drawn as the form variant, the platform's search field on
// content: it stands at the top of the page on the window's own fill, which is
// content and not chrome, in no sidebar and no toolbar band. The form variant
// is the field measured off System Settings' "add an input source" sheet: the
// hairline around the surface it stands on, its ends fully rounded, the
// looking glass 7 px inside the hairline and the prompt 4 px clear of it.
// Surface is left nil because the fill the field is drawn on is the window's
// own.
//
// extra states what a render made outside the window has to: the Seed, the
// Shaper, a FocusTag of its own. The rest of the props are set here, so the
// static render and the live window build one field.
func searchField(th rx.Observable[theme.Theme], extra input.SearchFieldProps, post func(layout.Context, SetQuery)) rx.Observable[layout.Widget] {
	props := extra
	props.Placeholder = "Search"
	props.Description = "search icons by name"
	props.Variant = input.Form
	props.OnChange = func(gtx layout.Context, text string) {
		post(gtx, SetQuery{Text: text})
	}
	return input.SearchField(th, props)
}

// underTitleBar holds the page down past the native title-bar strip the
// full-size-content window opens the top of itself into, and claims that same
// strip for the window's own drag.
//
// The strip carries no fill of its own: the region it caps is the window's own
// plane, which BackdropLayer already fills full-bleed, so the fill reaches the
// top edge without anything being painted twice. Nothing in this window is
// chrome — the catalogue stands on the window's own plane, the section labels
// are text on it, and the search field is a control drawn on it in the page's
// own vertical flow, below the strip rather than in it: the strip is the band
// the window buttons are centred in, and the field is the page's control, not
// the title bar's.
//
// So what has to clear the platform's three control buttons is the field, and
// the inset is what buys that clearance, since the field is the page's topmost
// paint and the page starts below the strip.
//
// desktop.CapTop also claims the strip for the window's drag — without it the
// window could not be moved by its top edge. The claim is recorded before the
// page, so every region the page declares below it keeps its own presses.
// desktop.TopInset is read at frame time, so away from the full-size-content
// treatment the whole cap is an exact no-op.
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
// own marks, then the grid of Material icons matching the query. A section the
// query matches nothing in is removed with its heading and its count. When
// neither section matches, the region under the field shows one empty state.
func Page(t themed, search layout.Widget, model Model, grid *list.State) layout.Widget {
	visible := FilterIcons(model.Query)
	return pageOver(t, search, model, len(visible), Grid(t, visible, model.Query, grid))
}

// pageOver is the page over a given content region: the stacking Page is,
// with the region that stretches handed in. Page hands it the icon grid; a
// render that has to know the frame that region is given hands in a probe
// reporting it.
func pageOver(t themed, search layout.Widget, model Model, count int, gridWidget layout.Widget) layout.Widget {
	names := FilterMarks(model.Query)
	markWidget := MarkGrid(t, names)
	marksHeading := Heading(t, MarksHeading, MarkSizeNote(names))
	catalogueHeading := Heading(t, CatalogueHeading, fmt.Sprintf("%d icons", count))
	sides := layout.Inset{Left: Padding, Right: Padding}
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 5)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(Padding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				// The field keeps its measured width at the page's
				// leading edge and never runs the page's full width.
				w := min(gtx.Dp(SearchFieldWidth), gtx.Constraints.Max.X)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
				return search(gtx)
			})
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
		switch {
		case count > 0:
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return sides.Layout(gtx, catalogueHeading)
				}),
				layout.Flexed(1, gridWidget),
			)
		case len(names) == 0:
			// Neither section matches: the content region under the field
			// is given whole to the grid, which draws the one empty state.
			children = append(children, layout.Flexed(1, gridWidget))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// gridColumns returns how many cells of cellW by cellH fit across one row of
// a column of size, for count cells. The cells fill the width inside pad on
// both sides. A standing bar's lane of lane pixels is taken off that width
// only when the rows overflow the column's height, because a lane exists
// only for a bar that is drawn. The rows are count/cols cells of cellH and a
// last row of pad. Fewer columns make more rows, so rows that overflow at the
// full width overflow short of the lane too.
func gridColumns(size image.Point, pad, lane, cellW, cellH, count int) int {
	cols := max(1, (size.X-2*pad)/cellW)
	rows := (count + cols - 1) / cols
	if lane > 0 && rows*cellH+pad > size.Y {
		cols = max(1, (size.X-2*pad-lane)/cellW)
	}
	return cols
}

// NoMatchLine is the empty state's line when no icon matches the query: a
// few words in title case that state the condition and hold the query, with
// no period. The query is set between typographic quotes as typed, with no
// escapes.
func NoMatchLine(query string) string {
	return "No Icons Match \u201c" + query + "\u201d"
}

// Grid lays the visible icons out in rows of as many fixed-size cells as fit
// the width, scrolled by the subscription-scoped list state, with the
// window's bar along the trailing edge. Each cell shows the symbol with its
// exported name captioned underneath.
//
// The region that scrolls is the page's column, the window's full width
// down to its bottom edge, and the page's Padding is inside it: the cells
// are drawn Padding in from the column's leading and trailing edges, and a
// last row of Padding's height follows them. The bar is drawn at the
// column's trailing edge, which is the window's, never inset by the
// Padding.
func Grid(t themed, visible []int, query string, grid *list.State) layout.Widget {
	p := t.palette
	bar := gridBar(t)
	anchor := list.AnchorFor(bar)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		pad := gtx.Dp(Padding)

		if len(visible) == 0 {
			// The grid is the page's content region, so what is drawn in
			// place of the icons is the content column's empty state: the
			// line centred, at the measured size, in the column's full
			// frame: an empty grid draws no bar, so it takes no lane.
			egtx := gtx
			egtx.Constraints = layout.Exact(shell.EmptyStateFrame(gtx, size, bar))
			shell.EmptyState(egtx, t.typ.Shaper, t.platform, t.typography, p.Backdrop,
				shell.EmptyStateProps{Line: NoMatchLine(query)})
			return layout.Dimensions{Size: size}
		}

		cellW, cellH := gtx.Dp(CellW), gtx.Dp(CellH)
		iconPx := gtx.Dp(IconSize)
		lane := 0
		if anchor == list.Occupy {
			lane = gtx.Dp(bar.Width())
		}
		cols := gridColumns(size, pad, lane, cellW, cellH, len(visible))
		// One row index per row of cells, and padRow for the last row,
		// which is the column's bottom Padding.
		const padRow = -1
		n := (len(visible) + cols - 1) / cols
		rows := make([]int, n+1)
		for i := range n {
			rows[i] = i
		}
		rows[n] = padRow

		return list.LayoutScrollbar(gtx, grid, bar, anchor, rows, func(gtx layout.Context, row int) layout.Dimensions {
			if row == padRow {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, pad)}
			}
			lead := op.Offset(image.Pt(pad, 0)).Push(gtx.Ops)
			defer lead.Pop()
			for col := 0; col < cols; col++ {
				i := row*cols + col
				if i >= len(visible) {
					break
				}
				icon := visible[i]
				cell := image.Rect(col*cellW, 0, (col+1)*cellW, cellH)

				cl := clip.Rect(cell).Push(gtx.Ops)

				// Symbol centred in the cell's upper part.
				off := op.Offset(image.Pt(cell.Min.X+(cellW-iconPx)/2, gtx.Dp(8))).Push(gtx.Ops)
				cgtx := gtx
				cgtx.Constraints = layout.Exact(image.Pt(iconPx, iconPx))
				t.icons[icon](cgtx)
				off.Pop()

				// Name captioned below the symbol.
				captionRect := image.Rect(cell.Min.X, gtx.Dp(8)+iconPx+gtx.Dp(4), cell.Max.X, cellH)
				textdraw.FillText(gtx, t.typ.Shaper, t.typ.Caption, captionRect, 0.5, 0.0, p.Text, IconTable[icon].Name)

				cl.Pop()
			}
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, cellH)}
		})
	}
}
