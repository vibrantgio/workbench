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
			FocusTag:    func(tag event.Tag) { st.tag = tag },
			Surface:     fill,
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
// holds the keyboard.
func (c newCard) title(post func(layout.Context, mvu.Message)) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if !c.state.focused {
			gtx.Execute(key.FocusCmd{Tag: c.state.tag})
			c.state.focused = true
		}
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
