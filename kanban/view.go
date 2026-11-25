package main

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/dragdrop"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/input"
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
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// scrollState is the board's scroll positions, kept at subscription scope so
// that a change to the board or to the theme keeps what the reader scrolled
// to: the board's sideways offset and each column's list.
type scrollState struct {
	board   *scrollarea.State
	columns []*complist.State
	ends    []columnEnd
	// presses is each column's press targets, one for each card. They are
	// kept across models, so the press after a press that selects a card
	// counts as its second click.
	presses [][]gesture.Click
	// tag is the board's press target, which ends an edit.
	tag struct{}
	// drag is the board's drag area. Each column is one of its sequences,
	// numbered as the board numbers its columns.
	drag dragdrop.State
	// follows is the model's Follows count that the board last read.
	follows int
}

// columnEnd is what a column's list last scrolled to its end for: the
// model's Opened count and the column's card count.
type columnEnd struct {
	opened, cards int
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

// clicks is the press targets of the n cards of column i. It keeps the
// targets that it gave before.
func (s *scrollState) clicks(i, n int) []gesture.Click {
	for len(s.presses) <= i {
		s.presses = append(s.presses, nil)
	}
	if len(s.presses[i]) < n {
		s.presses[i] = append(s.presses[i], make([]gesture.Click, n-len(s.presses[i]))...)
	}
	return s.presses[i][:n]
}

// toEnd reports whether column i's list scrolls to its end for a field
// opened at count opened, over a column of cards cards: when the field
// opens and after each card the column gains.
func (s *scrollState) toEnd(i, opened, cards int) bool {
	for len(s.ends) <= i {
		s.ends = append(s.ends, columnEnd{})
	}
	e := &s.ends[i]
	scroll := e.opened != opened || cards > e.cards
	e.opened, e.cards = opened, cards
	return scroll
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

// ContentLayer combines the latest theme snapshot, the latest Model, the
// new card and the edit field into the window's page. The new card and the
// edit field are subscribed to once here, so the text typed in them stays
// through each new model and each theme.
func ContentLayer(th rx.Observable[theme.Theme], models rx.Observable[Model], bars desktop.ScrollBarSetting) rx.Observable[layout.Widget] {
	return contentLayer(th, models, bars, postMessage)
}

// contentLayer is [ContentLayer] with post, which gives a message to the
// update.
func contentLayer(th rx.Observable[theme.Theme], models rx.Observable[Model], bars desktop.ScrollBarSetting, post func(layout.Context, mvu.Message)) rx.Observable[layout.Widget] {
	state := newScrollState()
	fresh := newCards(th, models, post)
	edits := editCards(th, models, post)
	return rx.Map(rx.CombineLatest4(themes(th, bars), models, fresh, edits), func(n rx.Tuple4[themed, Model, newCard, editCard]) layout.Widget {
		return view(n.First, n.Second, n.Third, n.Fourth, state, func() unit.Dp { return shell.ChromeTop(shell.ToolbarDepth) }, post)
	})
}

// view is the page for one theme and one model: the toolbar across the
// band at the window's top, and the board below it. fresh is the new card,
// and edit is the edit field.
// band is the band's depth, which a test states because it has no window to
// measure. post gives a message to the update.
func view(th themed, m Model, fresh newCard, edit editCard, state *scrollState, band func() unit.Dp, post func(layout.Context, mvu.Message)) layout.Widget {
	toolbar := Toolbar(th, m.Name, band)
	page := desktop.InsetTop(band, boardView(th, m, fresh, edit, state, post))
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
// gets its events. The column whose new card is open ends with fresh, and
// every other column ends with its "Add Card" button. The open column ends
// with nothing while fresh is the card of an older Opened count, so no
// frame draws the text of a card that was added. The card whose title is
// edited has edit in its title's place while edit is the field of the
// model's edit. While an edit is open, a press on the board takes the
// keyboard from the field, and so ends the edit, unless a press target
// inside the board takes the keyboard again. post gives a message to the
// update.
func boardView(th themed, m Model, fresh newCard, edit editCard, state *scrollState, post func(layout.Context, mvu.Message)) layout.Widget {
	bar := th.bar()
	b := m.Board
	columns := make([]layout.Widget, len(b.Columns))
	for i, c := range b.Columns {
		selected := -1
		if m.HasSelected && m.Selected.Column == i {
			selected = m.Selected.Card
		}
		end, toEnd := addButton(th, i, post), false
		switch {
		case fresh.openIn(m, i):
			end = newCardView(th, fresh, post)
			toEnd = state.toEnd(i, m.Opened, len(c.Cards()))
		case m.IsAdding && m.Adding == i:
			end = func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
		}
		ed := cardEdit{card: -1}
		if edit.openIn(m) && m.Editing.Column == i {
			list := state.column(i)
			ed = cardEdit{card: m.Editing.Card, title: edit.title(list.Focus(), post), tag: edit.state.tag}
		}
		columns[i] = columnView(th, i, c, selected, state.column(i), state.clicks(i, len(c.Cards())), ed, end, toEnd, &state.drag, post)
	}
	editing := edit.openIn(m)
	undo := boardUndo(m, post)
	dragStyle := dragdrop.FromTokens(th.platform)
	img := dragImage(th, b)
	return func(gtx layout.Context) layout.Dimensions {
		gtx = input.WithWindowUndo(gtx, undo)
		// The board's press target is under every press target inside the
		// board, so it reads each press on the board. Its focus command runs
		// before the commands of the targets inside the board.
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		event.Op(gtx.Ops, &state.tag)
		for {
			e, ok := gtx.Event(pointer.Filter{Target: &state.tag, Kinds: pointer.Press})
			if !ok {
				break
			}
			if pe, ok := e.(pointer.Event); ok && pe.Kind == pointer.Press && editing {
				gtx.Execute(key.FocusCmd{Tag: nil})
				gtx.Execute(op.InvalidateCmd{})
			}
		}
		// After an undo, a redo or a move by the Card menu or its keys, the
		// column that holds the selected card takes the keyboard from
		// another column. A field that holds the keyboard keeps it.
		if m.Follows != state.follows {
			state.follows = m.Follows
			if m.HasSelected {
				to := state.column(m.Selected.Column).Focus()
				for i := range columns {
					if i != m.Selected.Column && gtx.Source.Focused(state.column(i).Focus()) {
						gtx.Execute(key.FocusCmd{Tag: to})
						gtx.Execute(op.InvalidateCmd{})
						break
					}
				}
			}
		}
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
		// The drag area holds the whole board, so no column clips the
		// image of a card that moves to another column.
		dims := state.drag.Layout(gtx, dragStyle, img, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			return scrollarea.FromTokens(th.platform).LayoutScrollbar(gtx, state.board, bar, layoutColumns)
		})
		// A card that lands is selected at its new place, and the column it
		// lands in takes the keyboard, as a press on the card gives it.
		for _, mv := range state.drag.Update() {
			to := Selection{Column: mv.To.Sequence, Card: mv.Index()}
			post(gtx, MoveCard{From: Selection{Column: mv.From.Sequence, Card: mv.From.Index}, To: to})
			list := state.column(to.Column)
			list.Select(to.Card)
			gtx.Execute(key.FocusCmd{Tag: list.Focus()})
			gtx.Execute(op.InvalidateCmd{})
		}
		// Cmd-Z undoes and Shift-Cmd-Z redoes. The first reader of a key
		// event in a frame takes it, so the board reads both keys after the
		// columns: a field that holds the keyboard takes them first, and
		// gives the board's Undo each key that it holds no typing for.
		for {
			e, ok := gtx.Event(key.Filter{Name: "Z", Required: key.ModShortcut, Optional: key.ModShift})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				undo(gtx, ke.Modifiers.Contain(key.ModShift))
			}
		}
		// The Card menu's Up and Down take Ctrl-Cmd-Up and Ctrl-Cmd-Down
		// while they are enabled. The board reads the same keys for a
		// platform with no menu bar, and for a disabled item, which gives
		// its key to the window. The update moves no card when the move is
		// not available, so a key acts exactly when its item is enabled.
		// The columns' arrow keys take no modifier, so these keys reach
		// the board.
		for {
			e, ok := gtx.Event(
				key.Filter{Name: key.NameUpArrow, Required: moveModifiers},
				key.Filter{Name: key.NameDownArrow, Required: moveModifiers},
			)
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				if ke.Name == key.NameUpArrow {
					post(gtx, MoveUp{})
				} else {
					post(gtx, MoveDown{})
				}
			}
		}
		return dims
	}
}

// moveModifiers is the modifiers of the Card menu's Up and Down keys:
// Control and Command on macOS. Away from macOS the shortcut modifier is
// Control, so the keys are Ctrl-Up and Ctrl-Down there.
const moveModifiers = key.ModShortcut | key.ModCtrl

// boardUndo is the board's Undo and Redo for model m: the window's Undo of
// the board's fields, and the action of the board's own keys. It posts
// [Undo] or [Redo] when the stack holds a change for it, and reports
// whether it posted. It counts each change that it posted, so two presses
// before the next model act on two changes.
func boardUndo(m Model, post func(layout.Context, mvu.Message)) input.WindowUndo {
	undos, redos := len(m.Undo), len(m.Redo)
	return func(gtx layout.Context, redo bool) bool {
		switch {
		case redo && redos > 0:
			redos, undos = redos-1, undos+1
			post(gtx, Redo{})
		case !redo && undos > 0:
			undos, redos = undos-1, redos+1
			post(gtx, Undo{})
		default:
			return false
		}
		return true
	}
}

// addButton is the "Add Card" button at the end of column col: a borderless
// button with the plus before its title, as wide as both, which posts
// [OpenAdd].
func addButton(th themed, col int, post func(layout.Context, mvu.Message)) layout.Widget {
	// The theme is a static snapshot, so First gives the button at once.
	btn, _ := button.Button(rx.Of(th.components), button.Props{
		Title:    "Add Card",
		Symbol:   icons.Plus,
		Emphasis: button.Ghost,
		Surface:  th.platform.ControlBackground,
		Shaper:   th.shaper,
		OnClick:  func(gtx layout.Context) { post(gtx, OpenAdd{Column: col}) },
	}).First()
	return func(gtx layout.Context) layout.Dimensions {
		// A button never draws narrower than its title and its padding,
		// so no width gives the button its title's width.
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = 0, 0
		return btn(gtx)
	}
}

// checkboxBox is the side in dp of the box the library's checkbox draws at
// every density. The box's row is never smaller than the box.
const checkboxBox float32 = 16

// deleteKeys delete the selected card: Delete and Backspace with no
// modifier, as the platform's lists take them. The column reads them only
// while its list holds the keyboard, so a text field keeps its own keys.
var deleteKeys = []key.Name{key.NameDeleteBackward, key.NameDeleteForward}

// cardEdit is the edit of a card's title in a column: the card's index, or
// -1 when no card of the column is edited, the field that is in the card's
// title slot, and the field's focus tag.
type cardEdit struct {
	card  int
	title layout.Widget
	tag   event.Tag
}

// columnView is column c at index col, drawn across the constraints it is
// given: the library's group, titled with the column's name and its card
// count, that holds the column's cards in a list, and end after the list.
// The group runs the
// column's full height, so an empty column shows its extent too. The list
// is the group's content that scrolls, so its bar stands at the group's
// trailing edge, beside the cards only. The column paints no fill: it is a
// column of the board, and cards do not nest.
//
// The column's list selects as a content list does. A press on a card
// selects it and gives the list the keyboard, Up and Down move the
// selection, and Delete or Backspace posts [DeleteCard] for the selected
// card. Return and a double-click on a card post [EditCard]. clicks is the
// cards' press targets. The card that ed names has ed's field in its
// title's place, and a press on that card gives the field the keyboard. A
// selection that the list makes is posted as [SelectCard]. selected
// is the model's selected card in this column, or -1, and the list takes it
// on the first frame. The focus ring goes around the whole group and
// straddles its hairline, with the group's corner radius, while the list
// holds the keyboard.
//
// end stands outside the list, directly after the last card, so Down and
// End never select it. The list is as tall as its cards, and no taller than
// the group leaves it beside end. toEnd scrolls the list to its end on the
// first frame.
func columnView(th themed, col int, c Column, selected int, list *complist.State, clicks []gesture.Click, ed cardEdit, end layout.Widget, toEnd bool, drag *dragdrop.State, post func(layout.Context, mvu.Message)) layout.Widget {
	cards := c.Cards()
	bar := th.bar()
	boxes := make([]layout.Widget, len(cards))
	// onSelected[i] is true while card i is drawn selected, so its box
	// stands on the selected fill.
	onSelected := make([]bool, len(cards))
	for i, cd := range cards {
		toggle := ToggleCard{Selection{Column: col, Card: i}}
		// The theme is a static snapshot, so First gives the checkbox at once.
		boxes[i], _ = input.Checkbox(rx.Of(th.components), input.CheckboxProps{
			Description: "Completed",
			Checked:     cd.Done(),
			OnChange:    func(gtx layout.Context, _ bool) { post(gtx, toggle) },
			Surface: func(p tokens.PlatformColors) color.NRGBA {
				return card.Fill(card.Props{Selected: onSelected[i]}, p)
			},
		}).First()
	}
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
		inner := max(gtx.Constraints.Max.X-left-right, 0)

		// end is measured first, so the list knows the height it leaves.
		egtx := gtx
		egtx.Constraints = layout.Constraints{Max: image.Pt(inner, gtx.Constraints.Max.Y)}
		rec := op.Record(gtx.Ops)
		endDims := end(egtx)
		endCall := rec.Stop()
		// A column without a card has the top padding above end.
		lead := 0
		if len(cards) == 0 {
			lead = top
		}

		// The top padding and the gap under each card are inside the
		// list, so they scroll with the cards.
		lgtx := gtx
		lgtx.Constraints.Min = image.Point{}
		lgtx.Constraints.Max.Y = max(gtx.Constraints.Max.Y-lead-endDims.Size.Y-bottom, 0)
		list.ScrollPixels(drag.Scroll(gtx, col))
		listDims := complist.LayoutSelectableScrollbar(lgtx, list, bar, complist.AnchorFor(bar), idx, func(gtx layout.Context, i int, sel bool) layout.Dimensions {
			above := 0
			if i == 0 {
				above = top
			}
			cgtx := gtx
			cgtx.Constraints = layout.Constraints{Max: image.Pt(inner, gtx.Constraints.Max.Y)}
			onSelected[i] = sel
			off := op.Offset(image.Pt(left, above)).Push(gtx.Ops)
			title := cardTitle(th, cards[i], sel)
			if i == ed.card {
				title = ed.title
			}
			var dims layout.Dimensions
			if i == ed.card {
				// The edited card is no item: a drag in its field selects
				// text.
				dims = cardView(cgtx, th, sel, &clicks[i], boxes[i], title)
			} else {
				dims = drag.Item(cgtx, dragdrop.Place{Sequence: col, Index: i}, gapPx, func(gtx layout.Context) layout.Dimensions {
					return cardView(gtx, th, sel, &clicks[i], boxes[i], title)
				})
			}
			off.Pop()
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, above+dims.Size.Y+gapPx)}
		})

		y := lead + listDims.Size.Y
		// The edge bands are over the list's viewport, after its cards.
		off := op.Offset(image.Pt(0, lead)).Push(gtx.Ops)
		drag.Edges(gtx, col, listDims.Size)
		off.Pop()
		// The place after the last card begins at the last card's bottom
		// edge, where the gap under it begins, and runs to the column's
		// bottom. In a column without a card it is the whole column.
		endTop := y - gapPx
		if len(cards) == 0 {
			endTop = 0
		}
		off = op.Offset(image.Pt(left, endTop)).Push(gtx.Ops)
		dgtx := gtx
		dgtx.Constraints = layout.Exact(image.Pt(inner, max(gtx.Constraints.Max.Y-endTop, 0)))
		drag.End(dgtx, col, len(cards), gapPx)
		off.Pop()
		off = op.Offset(image.Pt(left, y)).Push(gtx.Ops)
		endCall.Add(gtx.Ops)
		off.Pop()
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y+endDims.Size.Y+bottom)}
	}
	props := group.Props{
		Title:   c.Title,
		Count:   strconv.Itoa(len(cards)),
		Scroll:  scroll,
		Surface: th.platform.ControlBackground,
	}
	grp := group.Render(th.shaper, props, th.platform, th.spacing, th.radius, group.TitleStyle(th.typ))
	filters := make([]event.Filter, 0, len(deleteKeys)+1)
	for _, name := range append([]key.Name{key.NameReturn}, deleteKeys...) {
		filters = append(filters, key.Filter{Focus: list.Focus(), Name: name})
	}
	synced := false
	return func(gtx layout.Context) layout.Dimensions {
		if !synced {
			list.Select(selected)
			if toEnd {
				list.ScrollToEnd(len(cards))
			}
			synced = true
		}
		for {
			e, ok := gtx.Event(filters...)
			if !ok {
				break
			}
			ke, ok := e.(key.Event)
			if !ok || ke.State != key.Press {
				continue
			}
			if i := list.Selected(); i >= 0 && i < len(cards) {
				if ke.Name == key.NameReturn {
					post(gtx, EditCard{Selection{Column: col, Card: i}})
				} else {
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
				switch {
				case e.Kind != gesture.KindPress:
				case i == ed.card:
					gtx.Execute(key.FocusCmd{Tag: ed.tag})
				case e.NumClicks == 2:
					choose(gtx, i)
					post(gtx, EditCard{Selection{Column: col, Card: i}})
				default:
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

// dragImage is the image of a dragged card of board b: the card as it is
// drawn when the press begins, selected, because the press selects it.
// Its checkbox is one of its own and reads no input. The image keeps the
// checkbox that it made for the card it last drew.
func dragImage(th themed, b Board) dragdrop.Image {
	var box layout.Widget
	var boxFor dragdrop.Place
	return func(gtx layout.Context, from dragdrop.Place) layout.Dimensions {
		cd := b.Columns[from.Sequence].Cards()[from.Index]
		if box == nil || boxFor != from {
			box, _ = input.Checkbox(rx.Of(th.components), input.CheckboxProps{
				Description: "Completed",
				Checked:     cd.Done(),
				Surface: func(p tokens.PlatformColors) color.NRGBA {
					return card.Fill(card.Props{Selected: true}, p)
				},
			}).First()
			boxFor = from
		}
		disabled := func(gtx layout.Context) layout.Dimensions { return box(gtx.Disabled()) }
		return cardView(gtx, th, true, nil, disabled, cardTitle(th, cd, true))
	}
}

// newCardView is the new card fresh as the end of a column draws it: the
// card that the column's cards draw, with the borderless field in the
// title's place and a checkbox that does not operate.
func newCardView(th themed, fresh newCard, post func(layout.Context, mvu.Message)) layout.Widget {
	title := fresh.title(post)
	return func(gtx layout.Context) layout.Dimensions {
		return cardView(gtx, th, false, nil, fresh.checkbox, title)
	}
}

// cardTitle is the title of card cd in the Body role. A completed task's
// title stays in the label colour, and a selected card's title keeps its
// own colour over the selected fill.
func cardTitle(th themed, cd Card, selected bool) layout.Widget {
	fg := vgcolor.Flatten(th.platform.Label, card.Fill(card.Props{Selected: selected}, th.platform))
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return typeset.Text(gtx, th.shaper, cd.Title, th.typ.Body, fg, 0)
	}
}

// cardView draws one card across the width it is given, as tall as its
// title needs: the library's card, whose body is box, the checkbox that
// marks the task completed, at the card's leading edge, and title after
// it. A selected card has the card pattern's selected fill. click is the
// card's press target, which covers the card. A nil click gives the card
// no press target.
//
// The title is laid out once in each frame and its operations are replayed
// in the card, so a field in the title's place reads its events once.
func cardView(gtx layout.Context, th themed, selected bool, click *gesture.Click, box, title layout.Widget) layout.Dimensions {
	p := th.platform
	props := card.Props{Selected: selected}
	fill := card.Fill(props, p)
	// The box's row is the density's checkbox row, square, and never
	// smaller than the box. Its centre is on the centre of the title's
	// first line, so the row can reach into the card's inset above the
	// title.
	row := gtx.Dp(unit.Dp(max(th.density.CheckboxRowHeight, checkboxBox)))
	boxSize := image.Pt(row, row)
	gap := gtx.Dp(unit.Dp(th.spacing.S2))
	lead := row + gap
	line := gtx.Sp(unit.Sp(th.typ.Body.LineHeight))
	width := gtx.Constraints.Max.X
	pad := gtx.Dp(unit.Dp(th.spacing.S4))

	// The card fills the constraints it is given, so the title is laid out
	// first at the width inside the card's inset and after the box, and the
	// card is given that height.
	tgtx := gtx
	tgtx.Constraints = layout.Constraints{Max: image.Pt(max(width-2*pad-lead, 0), 1e6)}
	tgtx = typeset.WithFill(tgtx, fill)
	rec := op.Record(gtx.Ops)
	text := title(tgtx)
	titleCall := rec.Stop()

	props.Body = func(gtx layout.Context) layout.Dimensions {
		off := op.Offset(image.Pt(0, (line-row)/2)).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Constraints{Max: boxSize}
		box(bgtx)
		off.Pop()
		off = op.Offset(image.Pt(lead, 0)).Push(gtx.Ops)
		titleCall.Add(gtx.Ops)
		off.Pop()
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, text.Size.Y)}
	}

	size := image.Pt(width, text.Size.Y+2*pad)
	gtx.Constraints = layout.Exact(size)
	// The press area is registered before the card's content, so a control
	// inside the card takes its press first.
	if click != nil {
		area := clip.Rect{Max: size}.Push(gtx.Ops)
		click.Add(gtx.Ops)
		area.Pop()
	}
	card.Render(props, p, th.spacing, th.radius)(gtx)
	return layout.Dimensions{Size: size}
}
