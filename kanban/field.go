package main

import (
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/card"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// newCard is the new card at the end of a column, as one theme emission
// draws it: its title in the borderless text field and its checkbox. The
// field's text and its focus live in one subscription, so a theme emission
// or a new model keeps the text being typed.
type newCard struct {
	// opened is the model's Opened count that the card was made for.
	opened int
	field  layout.Widget
	box    layout.Widget
	state  *newCardState
}

// newCardState is what one new card keeps across its emissions.
type newCardState struct {
	// tag is the field's focus tag.
	tag event.Tag
	// focused is true once the field asked for the keyboard.
	focused bool
	// typing is what the field holds, and posted is the last
	// [NewCardField] state that the card posted.
	typing, posted Typing
}

// report posts [NewCardField] for new card number opened when the field's
// state changes. A field without the keyboard holds no typing for the
// board's keys.
func (st *newCardState) report(gtx layout.Context, opened int, post func(layout.Context, mvu.Message)) {
	if t := held(gtx, st.tag, st.typing); t != st.posted {
		st.posted = t
		post(gtx, NewCardField{Opened: opened, Typing: t})
	}
}

// held is typing when the field whose focus tag is tag holds the keyboard,
// and no typing otherwise.
func held(gtx layout.Context, tag event.Tag, typing Typing) Typing {
	if !gtx.Focused(tag) {
		return Typing{}
	}
	return typing
}

// leaveUndo gives the board a Cmd-Z or a Shift-Cmd-Z that a field holds no
// typing for.
func leaveUndo(post func(layout.Context, mvu.Message)) func(layout.Context, bool) {
	return func(gtx layout.Context, redo bool) {
		if redo {
			post(gtx, Redo{})
		} else {
			post(gtx, Undo{})
		}
	}
}

// openCard is the new card made for the model's Opened count opened. Return
// gives an [AddCard] with the field's text to post. The field keeps its
// text after Return, and the added card's next Opened count gives the next
// new card an empty field.
func openCard(th rx.Observable[theme.Theme], opened int, post func(layout.Context, mvu.Message)) rx.Observable[newCard] {
	return rx.Defer(func() rx.Observable[newCard] {
		st := &newCardState{}
		fill := func(p tokens.PlatformColors) color.NRGBA { return card.Fill(card.Props{}, p) }
		field := input.BorderlessTextField(th, input.BorderlessTextFieldProps{
			Placeholder: "Card title",
			Description: "Title of the new card",
			Submit:      true,
			OnSubmit:    func(gtx layout.Context, text string) { post(gtx, AddCard{Title: text}) },
			OnTyping: func(gtx layout.Context, undo, redo bool) {
				st.typing = Typing{Undo: undo, Redo: redo}
				st.report(gtx, opened, post)
			},
			LeaveUndo: leaveUndo(post),
			FocusTag:  func(tag event.Tag) { st.tag = tag },
			Surface:   fill,
		})
		box := input.Checkbox(th, input.CheckboxProps{
			Description: "Completed",
			Surface:     fill,
		})
		return rx.Map(rx.CombineLatest2(field, box), func(n rx.Tuple2[layout.Widget, layout.Widget]) newCard {
			return newCard{opened: opened, field: n.First, box: n.Second, state: st}
		})
	})
}

// newCards gives the new card for each model. The card is made again only
// when the model's Opened count changes, so each opening and each added
// card give an empty field, and each other change keeps the field's text.
func newCards(th rx.Observable[theme.Theme], models rx.Observable[Model], post func(layout.Context, mvu.Message)) rx.Observable[newCard] {
	opened := rx.Map(models, func(m Model) int { return m.Opened }).
		DistinctUntilChanged(func(a, b int) bool { return a == b })
	return rx.SwitchMap(opened, func(n int) rx.Observable[newCard] {
		return openCard(th, n, post)
	})
}

// title is the new card's title slot: the field. On its first frame the
// field takes the keyboard, and Escape posts [CloseAdd] while the field
// holds the keyboard. Each frame posts [NewCardField] when the field's
// state changed.
func (c newCard) title(post func(layout.Context, mvu.Message)) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if !c.state.focused {
			gtx.Execute(key.FocusCmd{Tag: c.state.tag})
			c.state.focused = true
		}
		c.state.report(gtx, c.opened, post)
		for {
			e, ok := gtx.Event(key.Filter{Focus: c.state.tag, Name: key.NameEscape})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				post(gtx, CloseAdd{})
			}
		}
		return c.field(gtx)
	}
}

// checkbox is the new card's checkbox. It draws as an ordinary card's box
// and reads no input, so it does not operate until the card is added.
func (c newCard) checkbox(gtx layout.Context) layout.Dimensions {
	return c.box(gtx.Disabled())
}

// openIn reports whether c is the open new card of m in column col.
func (c newCard) openIn(m Model, col int) bool {
	return c.field != nil && m.IsAdding && m.Adding == col && c.opened == m.Opened
}

// editCard is the field that edits a card's title in place, as one theme
// emission draws it. The field's text and its focus live in one
// subscription, so a theme emission or a new model keeps the text being
// typed.
type editCard struct {
	// edit is the model's Edits count that the field was made for, and
	// card is the card it edits.
	edit  int
	card  Selection
	field layout.Widget
	state *editCardState
}

// editCardState is what one edit keeps across its emissions.
type editCardState struct {
	// tag is the field's focus tag.
	tag event.Tag
	// text is the field's text.
	text string
	// focused is true once the field asked for the keyboard, and ended is
	// true once the edit posted its end.
	focused, ended bool
	// back is the focus tag that takes the keyboard after Return and
	// Escape: the list of the card's column.
	back event.Tag
	// typing is what the field holds, and posted is the last [EditField]
	// state that the edit posted.
	typing, posted Typing
}

// report posts [EditField] for edit number edit when the field's state
// changes.
func (st *editCardState) report(gtx layout.Context, edit int, post func(layout.Context, mvu.Message)) {
	if t := held(gtx, st.tag, st.typing); t != st.posted {
		st.posted = t
		post(gtx, EditField{Edit: edit, Typing: t})
	}
}

// openEdit is the field for edit number edit of card s, whose title is
// title. The field starts with the title, on the selected card's fill.
// Return gives a [KeepEdit] with the field's text to post.
func openEdit(th rx.Observable[theme.Theme], edit int, s Selection, title string, post func(layout.Context, mvu.Message)) rx.Observable[editCard] {
	return rx.Defer(func() rx.Observable[editCard] {
		st := &editCardState{text: title}
		field := input.BorderlessTextField(th, input.BorderlessTextFieldProps{
			Description: "Title of the card",
			Seed:        title,
			Submit:      true,
			OnChange:    func(_ layout.Context, text string) { st.text = text },
			OnTyping: func(gtx layout.Context, undo, redo bool) {
				st.typing = Typing{Undo: undo, Redo: redo}
				st.report(gtx, edit, post)
			},
			LeaveUndo: leaveUndo(post),
			OnSubmit: func(gtx layout.Context, text string) {
				if st.ended {
					return
				}
				st.ended = true
				post(gtx, KeepEdit{Edit: edit, Title: text})
				gtx.Execute(key.FocusCmd{Tag: st.back})
			},
			FocusTag: func(tag event.Tag) { st.tag = tag },
			Surface: func(p tokens.PlatformColors) color.NRGBA {
				return card.Fill(card.Props{Selected: true}, p)
			},
		})
		return rx.Map(field, func(w layout.Widget) editCard {
			return editCard{edit: edit, card: s, field: w, state: st}
		})
	})
}

// editKey is what makes a new edit field: the edit's count, its card and
// the card's title when the edit began.
type editKey struct {
	edit  int
	card  Selection
	title string
}

// editCards gives the edit field for each model. The field is made again
// only when the model's Edits count changes, so each edit starts with the
// card's title, and each other change keeps the text being typed. A model
// with no edit begun gives the zero editCard.
func editCards(th rx.Observable[theme.Theme], models rx.Observable[Model], post func(layout.Context, mvu.Message)) rx.Observable[editCard] {
	keys := rx.Map(models, func(m Model) editKey {
		k := editKey{edit: m.Edits, card: m.Editing}
		if m.IsEditing && m.Board.has(m.Editing) {
			k.title = titleAt(m.Board, m.Editing)
		}
		return k
	}).DistinctUntilChanged(func(a, b editKey) bool { return a.edit == b.edit })
	return rx.SwitchMap(keys, func(k editKey) rx.Observable[editCard] {
		if k.edit == 0 {
			return rx.Of(editCard{})
		}
		return openEdit(th, k.edit, k.card, k.title, post)
	})
}

// title is the edited card's title slot: the field. back is the focus tag
// of the card's column. On its first frame the field takes the keyboard,
// with the whole title selected. Escape posts [CancelEdit] while the field
// holds the keyboard. When the keyboard leaves the field otherwise, as a
// press outside the card makes it leave, the edit posts [KeepEdit] with the
// field's text.
func (c editCard) title(back event.Tag, post func(layout.Context, mvu.Message)) layout.Widget {
	st := c.state
	return func(gtx layout.Context) layout.Dimensions {
		st.back = back
		if !st.focused {
			// The edit gives the field the keyboard by a command, so the
			// field selects the whole title.
			gtx.Execute(key.FocusCmd{Tag: st.tag})
			st.focused = true
		} else if !st.ended && !gtx.Focused(st.tag) {
			st.ended = true
			post(gtx, KeepEdit{Edit: c.edit, Title: st.text})
		}
		st.report(gtx, c.edit, post)
		for {
			e, ok := gtx.Event(key.Filter{Focus: st.tag, Name: key.NameEscape})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press && !st.ended {
				st.ended = true
				post(gtx, CancelEdit{Edit: c.edit})
				gtx.Execute(key.FocusCmd{Tag: back})
			}
		}
		return c.field(gtx)
	}
}

// openIn reports whether c is the open edit of m.
func (c editCard) openIn(m Model) bool {
	return c.field != nil && m.IsEditing && c.edit == m.Edits
}
