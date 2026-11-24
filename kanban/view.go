package main

import (
	"image"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollarea"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/card"
	"github.com/vibrantgio/patterns/group"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/typeset"
)

// scrollState is the board's scroll positions, kept at subscription scope so
// that a change to the board or to the theme keeps what the reader scrolled
// to: the board's sideways offset and each column's list.
type scrollState struct {
	board   *scrollarea.State
	columns []*complist.State
}

func newScrollState() *scrollState {
	return &scrollState{board: scrollarea.NewState()}
}

// column is the list state of column i. It makes states for new columns.
func (s *scrollState) column(i int) *complist.State {
	for len(s.columns) <= i {
		s.columns = append(s.columns, complist.NewState())
	}
	return s.columns[i]
}

// buildLayers returns the layer-builder the theme window renders: the
// content fill full-bleed, and the toolbar and the board over it.
func buildLayers(models rx.Observable[Model], bars desktop.ScrollBarSetting) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, models, bars),
		}
	}
}

// ContentLayer combines the latest theme snapshot with the latest Model into
// the window's page.
func ContentLayer(th rx.Observable[theme.Theme], models rx.Observable[Model], bars desktop.ScrollBarSetting) rx.Observable[layout.Widget] {
	state := newScrollState()
	return rx.Map(rx.CombineLatest2(themes(th, bars), models), func(n rx.Tuple2[themed, Model]) layout.Widget {
		return view(n.First, n.Second, state, func() unit.Dp { return shell.ChromeTop(shell.ToolbarDepth) })
	})
}

// view is the page for one theme and one model: the toolbar across the
// band at the window's top, and the board below it. band is the band's
// depth, which a test states because it has no window to measure.
func view(th themed, m Model, state *scrollState, band func() unit.Dp) layout.Widget {
	toolbar := Toolbar(th, m.Name, band)
	page := desktop.InsetTop(band, boardView(th, m.Board, state))
	return func(gtx layout.Context) layout.Dimensions {
		page(gtx)
		toolbar(gtx)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// Toolbar draws the window's toolbar across a band of the given depth: the
// file's name in the Headline role after the window's control buttons. The
// band paints no fill, because the content fill is under it, and the band
// moves the window everywhere but over the control buttons.
func Toolbar(th themed, name string, depth func() unit.Dp) layout.Widget {
	fg := vgcolor.Flatten(th.platform.Label, th.platform.ControlBackground)
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(depth()))
		gtx.Constraints = layout.Exact(size)
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		buttons := gtx.Dp(desktop.LeadingInset())
		layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(buttons, size.Y)}
			}),
			layout.Rigid(pane.DragSpacer(boardInset)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				gtx.Constraints.Max.X -= gtx.Dp(shell.PlatformToolbar.Trail)
				gtx = typeset.WithFill(gtx, th.platform.ControlBackground)
				return typeset.Text(gtx, th.shaper, name, th.typ.Headline, fg, 1)
			}),
			layout.Flexed(1, pane.DragFill),
		)
		return layout.Dimensions{Size: size}
	}
}

// boardView lays the board's columns out side by side at [columnWidth], with
// [columnGap] between them and [boardInset] around them. The board scrolls
// sideways when the columns are wider than the window, with its bar at the
// window's bottom edge.
func boardView(th themed, b Board, state *scrollState) layout.Widget {
	bar := th.bar()
	return func(gtx layout.Context) layout.Dimensions {
		n := len(b.Columns)
		inset, gap, width := gtx.Dp(boardInset), gtx.Dp(columnGap), gtx.Dp(columnWidth)
		content := 2 * inset
		if n > 0 {
			content += n*width + (n-1)*gap
		}
		// A standing bar takes a lane under the columns, which the area adds
		// to its child's height, so the child is that much shorter.
		height := gtx.Constraints.Max.Y
		if content > gtx.Constraints.Max.X && complist.AnchorFor(bar) == complist.Occupy {
			height -= gtx.Dp(bar.Width())
		}
		columns := func(gtx layout.Context) layout.Dimensions {
			colH := max(height-inset, 0)
			for i, c := range b.Columns {
				x := inset + i*(width+gap)
				off := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
				cgtx := gtx
				cgtx.Constraints = layout.Exact(image.Pt(width, colH))
				columnView(cgtx, th, c, state.column(i))
				off.Pop()
			}
			return layout.Dimensions{Size: image.Pt(content, height)}
		}
		gtx.Constraints.Min = image.Point{}
		return scrollarea.FromTokens(th.platform).LayoutScrollbar(gtx, state.board, bar, columns)
	}
}

// columnView draws one column across the constraints it is given: the
// library's group, titled with the column's name and its card count, that
// holds the column's cards in a list. The group runs the column's full
// height, so an empty column shows its extent too. The list is the group's
// content that scrolls, so its bar stands at the group's trailing edge,
// beside the cards only. The column paints no fill: it is a column of the
// board, and cards do not nest.
func columnView(gtx layout.Context, th themed, c Column, list *complist.State) layout.Dimensions {
	cards := c.Cards()
	bar := th.bar()
	scroll := func(gtx layout.Context, pad layout.Inset) layout.Dimensions {
		top, bottom := gtx.Dp(pad.Top), gtx.Dp(pad.Bottom)
		left, right := gtx.Dp(pad.Left), gtx.Dp(pad.Right)
		gapPx := gtx.Dp(cardGap)
		last := len(cards) - 1
		idx := make([]int, len(cards))
		for i := range idx {
			idx[i] = i
		}
		// The padding is inside the list: the first card has the top
		// padding above it and the last card the bottom padding under it,
		// so the padding scrolls with the cards.
		return complist.LayoutScrollbar(gtx, list, bar, complist.AnchorFor(bar), idx, func(gtx layout.Context, i int) layout.Dimensions {
			above, below := 0, gapPx
			if i == 0 {
				above = top
			}
			if i == last {
				below = bottom
			}
			cgtx := gtx
			cgtx.Constraints = layout.Constraints{Max: image.Pt(max(gtx.Constraints.Max.X-left-right, 0), gtx.Constraints.Max.Y)}
			off := op.Offset(image.Pt(left, above)).Push(gtx.Ops)
			dims := cardView(cgtx, th, cards[i])
			off.Pop()
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, above+dims.Size.Y+below)}
		})
	}
	props := group.Props{
		Title:   c.Title,
		Count:   strconv.Itoa(len(cards)),
		Scroll:  scroll,
		Surface: th.platform.ControlBackground,
	}
	return group.Render(th.shaper, props, th.platform, th.spacing, th.radius, group.TitleStyle(th.typ))(gtx)
}

// cardView draws one card across the width it is given, as tall as its
// title needs: the library's card, whose body is the task's title in the
// Body role. A completed task's title is in the secondary label colour.
func cardView(gtx layout.Context, th themed, cd Card) layout.Dimensions {
	p := th.platform
	fg := vgcolor.Flatten(p.Label, p.CardFill)
	if cd.Done() {
		fg = vgcolor.Flatten(p.SecondaryLabel, p.CardFill)
	}
	title := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return typeset.Text(gtx, th.shaper, cd.Title, th.typ.Body, fg, 0)
	}
	width := gtx.Constraints.Max.X
	pad := gtx.Dp(unit.Dp(th.spacing.S4))

	// The card fills the constraints it is given, so the title is measured
	// first at the width inside the card's inset and the card is given that
	// height.
	mgtx := gtx
	mgtx.Constraints = layout.Constraints{Max: image.Pt(max(width-2*pad, 0), 1e6)}
	mgtx = typeset.WithFill(mgtx, p.CardFill)
	rec := op.Record(gtx.Ops)
	text := title(mgtx)
	rec.Stop()

	size := image.Pt(width, text.Size.Y+2*pad)
	gtx.Constraints = layout.Exact(size)
	card.Render(card.Props{Body: title}, p, th.spacing, th.radius)(gtx)
	return layout.Dimensions{Size: size}
}
