package main

import (
	"image"
	"strconv"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollarea"
	"github.com/vibrantgio/mvu"
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
	page := desktop.InsetTop(band, boardView(th, m, state, postMessage))
	return func(gtx layout.Context) layout.Dimensions {
		page(gtx)
		toolbar(gtx)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// postMessage adds msg to the frame's messages for the update.
func postMessage(gtx layout.Context, msg mvu.Message) {
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
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
// [columnGap] between them and [boardInset] around them. Above the columns
// it keeps [complist.RingWidth] clear, so the focus ring around a column is
// not cut at the scroll area's top edge. The board scrolls sideways when the
// columns are wider than the window, with its bar at the window's bottom
// edge.
//
// Each column's layout.Widget is built here, once for each model, not in the
// frame: a click target made in the frame is new on each frame and never
// gets its events. post gives a message to the update.
func boardView(th themed, m Model, state *scrollState, post func(layout.Context, mvu.Message)) layout.Widget {
	bar := th.bar()
	b := m.Board
	columns := make([]layout.Widget, len(b.Columns))
	for i, c := range b.Columns {
		selected := -1
		if m.HasSelected && m.Selected.Column == i {
			selected = m.Selected.Card
		}
		columns[i] = columnView(th, i, c, selected, state.column(i), post)
	}
	return func(gtx layout.Context) layout.Dimensions {
		n := len(columns)
		inset, gap, width := gtx.Dp(boardInset), gtx.Dp(columnGap), gtx.Dp(columnWidth)
		ring := gtx.Dp(complist.RingWidth)
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
		layoutColumns := func(gtx layout.Context) layout.Dimensions {
			colH := max(height-inset-ring, 0)
			for i, column := range columns {
				x := inset + i*(width+gap)
				off := op.Offset(image.Pt(x, ring)).Push(gtx.Ops)
				cgtx := gtx
				cgtx.Constraints = layout.Exact(image.Pt(width, colH))
				column(cgtx)
				off.Pop()
			}
			return layout.Dimensions{Size: image.Pt(content, height)}
		}
		gtx.Constraints.Min = image.Point{}
		return scrollarea.FromTokens(th.platform).LayoutScrollbar(gtx, state.board, bar, layoutColumns)
	}
}

// deleteKeys delete the selected card: Delete and Backspace with no
// modifier, as the platform's lists take them. The column reads them only
// while its list holds the keyboard, so a text field keeps its own keys.
var deleteKeys = []key.Name{key.NameDeleteBackward, key.NameDeleteForward}

// columnView is column c at index col, drawn across the constraints it is
// given: the library's group, titled with the column's name and its card
// count, that holds the column's cards in a list. The group runs the
// column's full height, so an empty column shows its extent too. The list
// is the group's content that scrolls, so its bar stands at the group's
// trailing edge, beside the cards only. The column paints no fill: it is a
// column of the board, and cards do not nest.
//
// The column's list selects as a content list does. A press on a card
// selects it and gives the list the keyboard, Up and Down move the
// selection, and Delete or Backspace posts [DeleteCard] for the selected
// card. A selection that the list makes is posted as [SelectCard]. selected
// is the model's selected card in this column, or -1, and the list takes it
// on the first frame. The focus ring goes around the whole group,
// outside its hairline, with the group's corner radius, while the list
// holds the keyboard.
func columnView(th themed, col int, c Column, selected int, list *complist.State, post func(layout.Context, mvu.Message)) layout.Widget {
	cards := c.Cards()
	bar := th.bar()
	clicks := make([]gesture.Click, len(cards))
	idx := make([]int, len(cards))
	for i := range idx {
		idx[i] = i
	}
	// choose redraws the window, so the frame after a press draws the card
	// selected also when the press came after the card was drawn.
	choose := func(gtx layout.Context, i int) {
		list.Select(i)
		gtx.Execute(key.FocusCmd{Tag: list.Focus()})
		gtx.Execute(op.InvalidateCmd{})
	}
	scroll := func(gtx layout.Context, pad layout.Inset) layout.Dimensions {
		top, bottom := gtx.Dp(pad.Top), gtx.Dp(pad.Bottom)
		left, right := gtx.Dp(pad.Left), gtx.Dp(pad.Right)
		gapPx := gtx.Dp(cardGap)
		last := len(cards) - 1
		// The padding is inside the list: the first card has the top
		// padding above it and the last card the bottom padding under it,
		// so the padding scrolls with the cards.
		return complist.LayoutSelectableScrollbar(gtx, list, bar, complist.AnchorFor(bar), idx, func(gtx layout.Context, i int, sel bool) layout.Dimensions {
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
			dims := cardView(cgtx, th, cards[i], sel, &clicks[i])
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
	grp := group.Render(th.shaper, props, th.platform, th.spacing, th.radius, group.TitleStyle(th.typ))
	filters := make([]event.Filter, len(deleteKeys))
	for i, name := range deleteKeys {
		filters[i] = key.Filter{Focus: list.Focus(), Name: name}
	}
	synced := false
	return func(gtx layout.Context) layout.Dimensions {
		if !synced {
			list.Select(selected)
			synced = true
		}
		for {
			e, ok := gtx.Event(filters...)
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				if i := list.Selected(); i >= 0 && i < len(cards) {
					post(gtx, DeleteCard{Selection{Column: col, Card: i}})
				}
			}
		}
		// The presses are read before the cards are laid out, so the frame
		// that receives a press already draws its card selected.
		for i := range clicks {
			for {
				e, ok := clicks[i].Update(gtx.Source)
				if !ok {
					break
				}
				if e.Kind == gesture.KindPress {
					choose(gtx, i)
				}
			}
		}
		dims := complist.FocusRingRadius(gtx, list, th.platform, th.platform.ControlBackground, unit.Dp(th.radius.Lg), grp)
		if i := list.Selected(); i >= 0 && i != selected {
			selected = i
			post(gtx, SelectCard{Selection{Column: col, Card: i}})
		}
		return dims
	}
}

// cardView draws one card across the width it is given, as tall as its
// title needs: the library's card, whose body is the task's title in the
// Body role. A completed task's title is in the secondary label colour. A
// selected card has the card pattern's selected fill, and its title keeps
// its own colour over that fill. click is the card's press target, which
// covers the card.
func cardView(gtx layout.Context, th themed, cd Card, selected bool, click *gesture.Click) layout.Dimensions {
	p := th.platform
	props := card.Props{Selected: selected}
	fill := card.Fill(props, p)
	fg := vgcolor.Flatten(p.Label, fill)
	if cd.Done() {
		fg = vgcolor.Flatten(p.SecondaryLabel, fill)
	}
	title := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return typeset.Text(gtx, th.shaper, cd.Title, th.typ.Body, fg, 0)
	}
	props.Body = title
	width := gtx.Constraints.Max.X
	pad := gtx.Dp(unit.Dp(th.spacing.S4))

	// The card fills the constraints it is given, so the title is measured
	// first at the width inside the card's inset and the card is given that
	// height.
	mgtx := gtx
	mgtx.Constraints = layout.Constraints{Max: image.Pt(max(width-2*pad, 0), 1e6)}
	mgtx = typeset.WithFill(mgtx, fill)
	rec := op.Record(gtx.Ops)
	text := title(mgtx)
	rec.Stop()

	size := image.Pt(width, text.Size.Y+2*pad)
	gtx.Constraints = layout.Exact(size)
	// The press area is registered before the card's content, so a control
	// inside the card takes its press first.
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	card.Render(props, p, th.spacing, th.radius)(gtx)
	return layout.Dimensions{Size: size}
}
