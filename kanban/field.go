package main

import (
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// addField is the text field for a new card, as one theme emission draws
// it. The field's text and its focus live in one subscription, so a theme
// emission or a new model keeps the text being typed.
type addField struct {
	// opened is the model's Opened count that the field was made for.
	opened int
	widget layout.Widget
	state  *addFieldState
}

// addFieldState is what one field keeps across its emissions.
type addFieldState struct {
	// tag is the field's focus tag.
	tag event.Tag
	// focused is true once the field asked for the keyboard.
	focused bool
}

// newAddField is the text field for a new card, made for the model's
// Opened count opened. Return gives an [AddCard] with the field's text to
// post, and the field clears its text and stays open.
func newAddField(th rx.Observable[theme.Theme], opened int, post func(layout.Context, mvu.Message)) rx.Observable[addField] {
	return rx.Defer(func() rx.Observable[addField] {
		st := &addFieldState{}
		field := input.TextField(th, input.TextFieldProps{
			Placeholder: "Card title",
			Description: "Title of the new card",
			Submit:      true,
			OnSubmit:    func(gtx layout.Context, text string) { post(gtx, AddCard{Title: text}) },
			FocusTag:    func(tag event.Tag) { st.tag = tag },
			Surface:     func(p tokens.PlatformColors) color.NRGBA { return p.ControlBackground },
		})
		return rx.Map(field, func(w layout.Widget) addField {
			return addField{opened: opened, widget: w, state: st}
		})
	})
}

// addFields gives the text field for a new card for each model. The field
// is made again only when the model's Opened count changes, so each opening
// gives an empty field and each other change keeps the field's text.
func addFields(th rx.Observable[theme.Theme], models rx.Observable[Model], post func(layout.Context, mvu.Message)) rx.Observable[addField] {
	opened := rx.Map(models, func(m Model) int { return m.Opened }).
		DistinctUntilChanged(func(a, b int) bool { return a == b })
	return rx.SwitchMap(opened, func(n int) rx.Observable[addField] {
		return newAddField(th, n, post)
	})
}

// row is the field as the end of a column draws it. On its first frame the
// field takes the keyboard, and Escape posts [CloseAdd] while the field
// holds the keyboard.
func (f addField) row(post func(layout.Context, mvu.Message)) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if !f.state.focused {
			gtx.Execute(key.FocusCmd{Tag: f.state.tag})
			f.state.focused = true
		}
		for {
			e, ok := gtx.Event(key.Filter{Focus: f.state.tag, Name: key.NameEscape})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				post(gtx, CloseAdd{})
			}
		}
		return f.widget(gtx)
	}
}

// openIn reports whether f is the open field of m in column col.
func (f addField) openIn(m Model, col int) bool {
	return f.widget != nil && m.IsAdding && m.Adding == col && f.opened == m.Opened
}
