package main

import (
	"image"
	"image/color"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/components/scrollbar"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// List renders the todos on the window's own plane using the components
// virtual list, with bar drawn at its trailing edge. The list is what this
// window exists to show rather than something standing on it, so the one
// fill it paints is the content list selection under the selected row.
//
// The list selects as the platform's content lists do: a click on a row
// selects it and hands the list the keyboard, and the arrow keys move the
// selection. The selected row's fill spans the row's full height and the
// list's full width with square ends, and the row's text keeps its own
// colour. state keeps the selection, so no row is selected until the user
// selects one.
//
// The list is the region that scrolls. Every row is the density's content
// list row, edge to edge with its neighbours, and the list has no padding at
// its ends. A row's content is inset on its leading side by the page's
// Padding and its own and on its trailing side by [rowTrailingInset], so the
// bar is at the window's trailing edge. The anchor is what
// [complist.AnchorFor] returns for bar: at Always the bar has a lane of its
// own and the rows stop short of it, otherwise it lies over the rows'
// trailing edge.
//
// state is the list's scroll position, kept at subscription scope and passed
// to every layout, so a change to a todo keeps the rows the reader scrolled to.
func List(typ Type, th rx.Observable[theme.Theme], p Palette, den tokens.Density, bar scrollbar.Style, state *complist.State, model Model) layout.Widget {
	rows := make([]rowLayout, len(model.List))
	ids := make([]int, len(model.List))
	selected := -1
	for i, todo := range model.List {
		rows[i] = Row(typ, th, p, todo)
		ids[i] = todo.Id
		if model.HasSelected && todo.Id == model.Selected {
			selected = i
		}
	}
	return scrollingList(rows, ids, selected, unit.Dp(den.ContentListRowHeight), p.RowSelection, bar, state, postMessage)
}

// postMessage adds msg to the frame's messages for the update.
func postMessage(gtx layout.Context, msg mvu.Message) {
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
}

// rowLayout draws one row of the list. sel tells the row whether it is the
// selected one and gives it the actions that change the selection.
type rowLayout func(gtx layout.Context, sel rowSelection) layout.Dimensions

// rowSelection is what the list tells a row about the selection.
type rowSelection struct {
	// selected is true on the list's selected row.
	selected bool
	// choose selects the row and hands the list the keyboard.
	choose func(gtx layout.Context)
}

// scrollingList lays rows out as the list's region that scrolls, each row
// exactly rowHeight tall, with the insets [List] states. The list's bar
// counts every row at rowHeight. The selected row is drawn over fill across
// the row's full size.
//
// A press on a part of a row that holds no control of its own selects the
// row. The row's checkbox and delete mark keep their own presses, and the
// row's text selects the row through its own click.
//
// Model owns the selection, so the update can read it: ids are the rows'
// todo Ids and selectedRow is the index of Model's selected todo, or -1. The
// first frame sets state's selection from selectedRow. A selection that the
// list changes after that, by a press or a key, is posted through post as
// SelectTodo. The Delete key with no modifier posts DeleteSelected while the
// list holds the keyboard, so a text field keeps its own Delete key.
func scrollingList(rows []rowLayout, ids []int, selectedRow int, rowHeight unit.Dp, fill color.NRGBA, bar scrollbar.Style, state *complist.State, post func(gtx layout.Context, msg mvu.Message)) layout.Widget {
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	clicks := make([]gesture.Click, len(rows))
	state.SetHeightOfRow(func(m unit.Metric, _ int) int { return m.Dp(rowHeight) })
	// choose redraws the window, because the row's text selects its row
	// during the frame that has already drawn the row as not selected.
	choose := func(gtx layout.Context, i int) {
		state.Select(i)
		gtx.Execute(key.FocusCmd{Tag: state.Focus()})
		gtx.Execute(op.InvalidateCmd{})
	}
	synced := false
	return func(gtx layout.Context) layout.Dimensions {
		if !synced {
			state.Select(selectedRow)
			synced = true
		}
		for {
			e, ok := gtx.Event(key.Filter{Focus: state.Focus(), Name: deleteKey})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				post(gtx, DeleteSelected{})
			}
		}
		// The presses are read before the rows are laid out, so the frame
		// that receives a press already draws its row selected.
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
		dims := complist.LayoutSelectableScrollbar(gtx, state, bar, complist.AnchorFor(bar), idx, func(gtx layout.Context, i int, selected bool) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(rowHeight))
			gtx.Constraints = layout.Exact(size)
			if selected {
				paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
			}
			// The row's own press area is registered before the row's
			// content, so a control inside the row takes its press first.
			area := clip.Rect{Max: size}.Push(gtx.Ops)
			clicks[i].Add(gtx.Ops)
			area.Pop()
			sel := rowSelection{
				selected: selected,
				choose:   func(gtx layout.Context) { choose(gtx, i) },
			}
			in := layout.Inset{Left: 2 * Padding, Right: rowTrailingInset}
			return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return rows[i](gtx, sel) })
		})
		if i := state.Selected(); i >= 0 && i < len(ids) && i != selectedRow {
			selectedRow = i
			post(gtx, SelectTodo{Id: ids[i]})
		}
		return dims
	}
}

// emptyListLine is the list's empty state: the line drawn where the rows
// would be when there is no todo.
const emptyListLine = "No Todos"

// EmptyList draws the list's empty state over the frame gtx gives: the line
// centred both ways in the empty state line role, in the tertiary label colour
// over the window's own plane.
//
// The list is the window's one column, so the frame is the window's full
// frame, the title-bar strip included, as a column's frame includes the
// toolbar's share. An empty list draws no bar, so the frame takes no lane.
func EmptyList(gtx layout.Context, th themed) layout.Dimensions {
	size := gtx.Constraints.Max
	egtx := gtx
	egtx.Constraints = layout.Exact(shell.EmptyStateFrame(gtx, size, th.listBar()))
	shell.EmptyState(egtx, th.typ.Shaper, th.platform, th.typography, th.palette.Backdrop,
		shell.EmptyStateProps{Line: emptyListLine})
	return layout.Dimensions{Size: size}
}

// The delete mark is a control inside a row, and is drawn as the platform's
// lists draw theirs, MEASURED in reference/macos/controls.md under "What a
// control inside a row measures": an 18 px circled mark, 16 px inside the
// row's trailing edge, in the accent, drawn on the row under the pointer and
// on the selected row and on no row at rest.
const (
	// rowControlBox is the symbol's square. The reference reads a mark's
	// extent as the pixels more than 20 levels off the fill, and the circled
	// symbol in a 20 dp box reads 18 px by that measure, its ring's
	// antialiased edge included.
	rowControlBox unit.Dp = 20
	// rowTrailingInset is the rows' trailing inset: the box's trailing column
	// is the drawn extent's, so the mark's extent ends 16 px inside the row's
	// trailing edge, which is the window's trailing edge.
	rowTrailingInset unit.Dp = 15
)

// Row is one todo line: a components checkbox toggling completion, the todo
// text, and a delete mark drawn while the pointer is over the row or the row
// is selected. A click on the text of a row that is not selected selects the
// row; a click on the text of the selected row opens the edit dialog. The
// text fills the row's height and is centred in it, and the checkbox and the
// mark are centred on the row's middle. Every change to a todo routes through
// mvu.MessageOp, so the reducers are the only writers of the todos.
func Row(typ Type, th rx.Observable[theme.Theme], p Palette, item Todo) rowLayout {
	row := layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween, Alignment: layout.Middle}

	// th is a static snapshot (rx.Of), so First() resolves synchronously.
	cb, _ := input.Checkbox(th, input.CheckboxProps{
		Description: "completed",
		Checked:     item.Completed,
		Message:     ToggleTodo{Id: item.Id},
	}).First()

	// The text is plain and clickable, not a button: a completed todo
	// is drawn at the second strength.
	var editClick widget.Clickable
	textColor := p.Label
	if item.Completed {
		textColor = p.SecondaryLabel
	}
	title := func(gtx layout.Context, sel rowSelection) layout.Dimensions {
		if editClick.Clicked(gtx) {
			if sel.selected {
				mvu.MessageOp{Message: SelectTodo{Id: item.Id}}.Add(gtx.Ops)
				mvu.MessageOp{Message: SetRoute{Route: "edit.todo"}}.Add(gtx.Ops)
			} else {
				sel.choose(gtx)
			}
		}
		dims := editClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			textdraw.FillText(gtx, typ.Shaper, typ.Row, image.Rectangle{Max: size}, 0.0, 0.5, textColor, item.Text)
			return layout.Dimensions{Size: size}
		})
		if editClick.Hovered() {
			pointershape.OverSize(gtx.Ops, dims.Size, pointer.CursorPointer)
		}
		return dims
	}

	deleteMark, err := raster.Widget(icons.ActionHighlightOff, rowControlBox, rowControlBox, raster.WithColors(p.RowControl))
	if err != nil {
		panic(err)
	}
	var deleteClick widget.Clickable
	// hover reads Enter and Leave only, so it takes no press from the
	// checkbox, the text or the mark laid out inside its area. It is
	// registered to pass presses through, so a press on a part of the row
	// that holds no control reaches the list's row press beneath it.
	var hover gesture.Hover

	return func(gtx layout.Context, sel rowSelection) layout.Dimensions {
		hovered := hover.Update(gtx.Source)
		macro := op.Record(gtx.Ops)
		dims := row.Layout(gtx,
			layout.Rigid(cb),
			layout.Rigid(layout.Spacer{Width: Padding}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return title(gtx, sel) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if deleteClick.Clicked(gtx) {
					mvu.MessageOp{Message: DeleteTodo{Id: item.Id}}.Add(gtx.Ops)
				}
				// The slot is laid out in every state, so the text's width
				// does not change when the mark is drawn.
				dims := deleteClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := image.Pt(gtx.Dp(rowControlBox), gtx.Dp(rowControlBox))
					if hovered || sel.selected {
						gtx.Constraints = layout.Exact(size)
						deleteMark(gtx)
					}
					return layout.Dimensions{Size: size}
				})
				if deleteClick.Hovered() {
					pointershape.OverSize(gtx.Ops, dims.Size, pointer.CursorPointer)
				}
				return dims
			}),
		)
		content := macro.Stop()
		area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		hover.Add(gtx.Ops)
		pass.Pop()
		content.Add(gtx.Ops)
		area.Pop()
		return dims
	}
}
