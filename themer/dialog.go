// The dialog a click on the theme colour's swatch opens. It says that an
// image dropped on the window gives the window its colour.

package main

import (
	"sync/atomic"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/paragraph"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/dialog"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// DialogTitle is the dialog's title, the name of the group whose swatch
// opens it.
const DialogTitle = "Theme colour"

// DialogText is the dialog's body text.
const DialogText = "Drop an image on this window to take the theme colour from it."

// closeDialog is the dialog's close, Escape and a click outside it: each one
// sends [CloseDialog].
func closeDialog(gtx layout.Context) {
	mvu.MessageOp{Message: CloseDialog{}}.Add(gtx.Ops)
}

// DialogLayer is the dialog over the page. It is a dismissible dialog: a
// close in its header, no footer, and Escape and a click outside it close it
// too. Each of the three calls onClose.
//
// It is drawn through [WindowTheme], so it follows the scheme control with
// the rest of the window. The drop zone is the page's, recorded under the
// dialog, so a drop on the window still reaches the window while the dialog
// is open. keys gives the keyboard back when the dialog closes; nil keys
// leaves the keyboard where the dialog put it.
func DialogLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], keys *keyboardReturn, onClose func(gtx layout.Context)) rx.Observable[layout.Widget] {
	win := WindowTheme(th, modelObs)
	open := rx.Map(modelObs, func(m Model) bool { return m.DialogOpen })
	st := paragraph.NewState()

	// The dialog's body is a fixed slot and the text's colours change with
	// the theme, so the body reads them from this cell when it is laid out.
	var bodyCell atomic.Value
	body := func(gtx layout.Context) layout.Dimensions {
		if w, ok := bodyCell.Load().(layout.Widget); ok {
			return w(gtx)
		}
		return layout.Dimensions{}
	}
	bodies := rx.SwitchMap(win, func(t theme.Theme) rx.Observable[layout.Widget] {
		return rx.Map(rx.CombineLatest2(t.Platform, t.Typography),
			func(n rx.Tuple2[tokens.PlatformColors, tokens.Typography]) layout.Widget {
				return DialogBody(n.First, n.Second.SystemFont, n.Second.Shaper(), st)
			})
	})
	box := dialog.Dialog(win, dialog.Props{
		Open:    open,
		Title:   DialogTitle,
		Body:    body,
		OnClose: onClose,
	})
	return rx.Map(rx.CombineLatest3(box, bodies, open),
		func(n rx.Tuple3[layout.Widget, layout.Widget, bool]) layout.Widget {
			bodyCell.Store(n.Second)
			draw, isOpen := n.First, n.Third
			return func(gtx layout.Context) layout.Dimensions {
				keys.frame(gtx, isOpen)
				return draw(gtx)
			}
		})
}

// DialogBody is the dialog's body: [DialogText] in the role the window's
// rows use, wrapped to the width the dialog gives it. It is drawn in the
// platform's Label on the dialog's fill, the colour its title uses: Text is full black
// or white, and a body in it is more prominent than its title.
func DialogBody(c tokens.PlatformColors, role tokens.TextStyle, shaper *text.Shaper, st *paragraph.State) layout.Widget {
	style := paragraph.FromTokens(c, role, c.WindowBackground)
	style.Color = vgcolor.Flatten(c.Label, c.WindowBackground)
	span := paragraph.SpanStyle{Typeface: font.Typeface(role.Typeface), Content: DialogText}
	if role.Weight != 0 {
		span.Weight = tokens.FontWeight(role.Weight)
	}
	spans := []paragraph.SpanStyle{span}
	return func(gtx layout.Context) layout.Dimensions {
		return paragraph.Layout(gtx, st, shaper, style, spans)
	}
}

// keyboardReturn gives the keyboard back to the control that held it before
// the dialog opened. Gio answers whether one tag holds the keyboard and does
// not name the tag that does, so keyboardReturn asks about each tag it knows.
// The colour field's editor is the one tag it knows.
type keyboardReturn struct {
	field atomic.Value // event.Tag of the colour field's editor
	// held and open are read and written only on the frame goroutine.
	held event.Tag
	open bool
}

// watch takes the colour field's focus tag.
func (k *keyboardReturn) watch(tag event.Tag) {
	if k != nil && tag != nil {
		k.field.Store(tag)
	}
}

// frame runs once per frame, before the dialog is laid out. While the dialog
// is closed, it notes whether a known tag holds the keyboard. On the first
// frame after the dialog closes, it gives the keyboard to that tag, or to no
// tag when none held it.
func (k *keyboardReturn) frame(gtx layout.Context, open bool) {
	if k == nil {
		return
	}
	switch {
	case open:
		k.open = true
	case k.open:
		k.open = false
		gtx.Execute(key.FocusCmd{Tag: k.held})
	default:
		k.held = nil
		if tag, ok := k.field.Load().(event.Tag); ok && gtx.Focused(tag) {
			k.held = tag
		}
	}
}
