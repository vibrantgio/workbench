package main

import (
	"image"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
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
)

// List renders the todos on the window's own plane using the components
// virtual list, with bar drawn at its trailing edge. It paints no fill of its
// own: the list is what this window exists to show rather than something
// standing on it.
//
// The list is the region that scrolls, and the page's padding is inside it:
// each row is inset on its leading side by the page's Padding and its own, on
// its trailing side by [rowTrailingInset], and above and below by the page's
// Padding; the first row adds the page's Padding above, and the last row adds
// the page's Padding below, so the bar is at the window's trailing edge and
// never inset by the padding. The anchor is what
// [complist.AnchorFor] returns for bar: at Always the bar has a lane of its
// own and the rows stop short of it, otherwise it lies over the rows'
// trailing edge.
//
// state is the list's scroll position, kept at subscription scope and passed
// to every layout, so a change to a todo keeps the rows the reader scrolled to.
func List(typ Type, th rx.Observable[theme.Theme], p Palette, bar scrollbar.Style, state *complist.State, model Model) layout.Widget {
	rows := make([]layout.Widget, len(model.List))
	for i := range model.List {
		rows[i] = Row(typ, th, p, model.List[i])
	}
	return scrollingList(rows, bar, state)
}

// scrollingList lays rows out as the list's region that scrolls, with the
// insets [List] states.
func scrollingList(rows []layout.Widget, bar scrollbar.Style, state *complist.State) layout.Widget {
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	return func(gtx layout.Context) layout.Dimensions {
		return complist.LayoutScrollbar(gtx, state, bar, complist.AnchorFor(bar), idx, func(gtx layout.Context, i int) layout.Dimensions {
			in := layout.Inset{Top: Padding, Bottom: Padding, Left: 2 * Padding, Right: rowTrailingInset}
			if i == 0 {
				in.Top += Padding
			}
			if i == len(rows)-1 {
				in.Bottom += Padding
			}
			return in.Layout(gtx, rows[i])
		})
	}
}

// emptyListLine is the list's empty state: the line drawn where the rows
// would be when there is no todo.
const emptyListLine = "No Todos"

// EmptyList draws the list's empty state over the frame gtx gives: the line
// centred both ways in the Headline Medium role, in the tertiary label colour
// over the window's own plane.
//
// The list is the window's one column, so the frame is the window's full
// frame, the title-bar strip included, as a column's frame includes the
// toolbar's share. It is passed through [shell.EmptyStateFrame] with the bar
// the list draws, so a standing bar's lane is outside it.
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
// on the selected row and on no row at rest. The list selects no row, so the
// mark is drawn on the row under the pointer only.
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

// Row is one todo line: a components checkbox toggling completion, the todo text
// (clickable — opens the edit dialog), and a delete mark drawn while the
// pointer is over the row. Every event routes through mvu.MessageOp, so the
// reducers are the only state writers.
func Row(typ Type, th rx.Observable[theme.Theme], p Palette, item Todo) layout.Widget {
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
	title := func(gtx layout.Context) layout.Dimensions {
		if editClick.Clicked(gtx) {
			mvu.MessageOp{Message: SelectTodo{Id: item.Id}}.Add(gtx.Ops)
			mvu.MessageOp{Message: SetRoute{Route: "edit.todo"}}.Add(gtx.Ops)
		}
		dims := editClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			h := textdraw.MeasureText(gtx, typ.Shaper, typ.Title, "W").Y
			size := image.Pt(gtx.Constraints.Max.X, h+gtx.Dp(Padding))
			textdraw.FillText(gtx, typ.Shaper, typ.Title, image.Rectangle{Max: size}, 0.0, 0.5, textColor, item.Text)
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
	// checkbox, the text or the mark laid out inside its area.
	var hover gesture.Hover

	return func(gtx layout.Context) layout.Dimensions {
		hovered := hover.Update(gtx.Source)
		macro := op.Record(gtx.Ops)
		dims := row.Layout(gtx,
			layout.Rigid(cb),
			layout.Rigid(layout.Spacer{Width: Padding}.Layout),
			layout.Flexed(1, title),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if deleteClick.Clicked(gtx) {
					mvu.MessageOp{Message: DeleteTodo{Id: item.Id}}.Add(gtx.Ops)
				}
				// The slot is laid out in every state, so the text's width
				// does not change when the mark is drawn.
				dims := deleteClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := image.Pt(gtx.Dp(rowControlBox), gtx.Dp(rowControlBox))
					if hovered {
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
		hover.Add(gtx.Ops)
		content.Add(gtx.Ops)
		area.Pop()
		return dims
	}
}
