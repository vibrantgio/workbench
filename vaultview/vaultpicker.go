// vaultpicker.go is the vault-switch dialog: the folder browser raised as a
// patterns/modal decision over the vault on screen, which is what Switch
// Vault opens once a vault is open. It is the library's own modal, for the
// platforms that offer no open panel of their own.
//
// Its body is picker.go's browser — the same breadcrumb, the same rows and
// the same annotations the full-screen picker shows — starting at the vault
// already open, so Open with nothing changed reopens it. Cancel and Escape
// leave the vault and every piece of its state exactly as they were; Open
// switches the whole window to the chosen directory.

package main

import (
	"image"
	"sync/atomic"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/breadcrumb"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// The dialog's own measurements, read off
// `.github/reference/macos/save-dialog-{light,dark}.png`, whose footer is
// the one this footer is drawn after. Both appearances agree to the pixel.
const (
	// dialogButtonWDp is the width of a push button in that footer: Cancel
	// spans x 359–432 and Save x 441–514, 74 px apiece. It is the platform's
	// minimum for a dialog button, which is what both of those labels are
	// under, so the two stand equal.
	dialogButtonWDp unit.Dp = 74

	// vaultPickerRows is how many folder rows the dialog opens showing. No
	// stored capture holds the platform's own open panel, so its width and
	// height have no reading; the dialog states this instead and takes its
	// height from it. Eight rows is what the list needs to read as a list
	// rather than as a preview, and the panel's own numbers replace it when
	// a capture holds one.
	vaultPickerRows = 8
)

// The footer's gap and inset are the modal pattern's own and are already the
// save dialog's: `SpacingScale.S2` is 8 dp against the 8 clear px between
// that footer's two buttons (x 433–440), and the surface inset `S5` is 20 dp
// against the 20 px from the trailing button's edge to the sheet's inner
// edge (x 514 to x 533) and the 20 from its last row to the sheet's foot
// (y 524 to y 544). Nothing here restates them.

// vaultPickerLayer builds the vault-switch dialog's stream. The open state,
// the directory on show and its rows all live in the model; the dialog is
// pure view over them. The body reads the model and token snapshots at frame
// time, as the chooser's does.
func vaultPickerLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	loadModel func() Model,
	loadTok func() themeTokens,
	arb *modal.Arbiter,
) rx.Observable[layout.Widget] {
	isOpen := rx.Map(modelObs, func(m Model) bool { return m.PickerOpen }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))

	// The trail comes off the theme stream for the reason picker.go's does:
	// its interaction state lives in the stream, so a click survives the
	// palette changing under the pointer. It stands on the dialog's own
	// plane, which is the trail's zero Surface.
	trailObs := breadcrumb.Trail(th, breadcrumb.TrailProps{Chevron: trailChevronDp})

	return rx.Defer(func() rx.Observable[layout.Widget] {
		v := &pickerView{list: list.NewState()}
		var cancelClick, openClick widget.Clickable

		cancel, openVault := vaultPickerAnswers(loadModel, postMessage)

		// One filled action per surface: Open is what this dialog is for, so
		// it keeps the Filled emphasis and Cancel stands beside it as the
		// ordinary push button the platform draws there. MEASURED,
		// save-dialog-{light,dark}.png: that sheet's "Cancel" at x 359-432
		// carries the push button's fill, #ececec light and #333a3f dark,
		// under controlText — the Tonal emphasis — against the accent-filled
		// default beside it. Both stand on the window's own plane, which is
		// the fill a floating surface takes on this platform and the zero
		// value of the button's Surface, so neither is told anything.
		cancelObs := button.Button(th, button.Props{
			Label:     "Cancel",
			Emphasis:  button.Tonal,
			Clickable: &cancelClick,
			OnClick:   cancel,
		})
		openBtnObs := button.Button(th, button.Props{
			Label:     "Open",
			Clickable: &openClick,
			OnClick:   openVault,
		})

		// The body and the two actions are static slots; the live streams
		// reach them through cells, the observable-over-static-slot hand-off
		// the settings dialog in mindchat uses.
		var trailCell, cancelCell, openCell atomic.Value
		body := func(gtx layout.Context) layout.Dimensions {
			trail, _ := trailCell.Load().(breadcrumb.TrailLayout)
			if trail == nil {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}
			return vaultPickerBody(gtx, v, loadModel(), loadTok(), trail)
		}

		modalObs := modal.Modal(th, modal.Props{
			Open:    isOpen,
			Title:   "Open Vault",
			Body:    body,
			Arbiter: arb,
			Actions: []layout.Widget{
				dialogAction(&cancelCell),
				dialogAction(&openCell),
			},
			// The list leads the Tab cycle and takes the dialog's opening
			// focus, so the arrows walk the folders the moment it is up.
			DynamicFocusTags: func() []event.Tag { return []event.Tag{v.list.Focus()} },
			ActionFocusTags:  []event.Tag{&cancelClick, &openClick},
			Decision:         vaultPickerDecision(cancel, openVault),
		})

		return rx.Map(rx.CombineLatest4(modalObs, trailObs, cancelObs, openBtnObs),
			func(next rx.Tuple4[layout.Widget, breadcrumb.TrailLayout, layout.Widget, layout.Widget]) layout.Widget {
				trailCell.Store(next.Second)
				cancelCell.Store(next.Third)
				openCell.Store(next.Fourth)
				return next.First
			})
	})
}

// vaultPickerAnswers builds the two answers this dialog accepts. Each is
// both a footer button's OnClick and one half of the decision below,
// because the footer and the keyboard must answer identically — Escape is
// Cancel, Return is Open.
//
// post is where an answer goes: the live dialog's is the message op, and a
// test's is a recorder, which is how the two bindings are read back without
// a window to press keys into.
func vaultPickerAnswers(
	loadModel func() Model,
	post func(gtx layout.Context, msg mvu.Message),
) (cancel, openVault func(gtx layout.Context)) {
	cancel = func(gtx layout.Context) { post(gtx, CancelSwitch{}) }
	// Open takes the directory the browser is IN, not a row's selection:
	// entering a folder is what makes it the answer, and the dialog opens
	// already inside the vault on screen, so Open with nothing changed
	// reopens that vault.
	openVault = func(gtx layout.Context) { post(gtx, OpenVault{Path: loadModel().PickerDir}) }
	return cancel, openVault
}

// postMessage is the live sink: an answer reaches the loop as this frame's
// message op.
func postMessage(gtx layout.Context, msg mvu.Message) {
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
}

// vaultPickerDecision makes this dialog a DECISION rather than a panel: the
// two footer buttons are the only two answers, which removes the close X,
// makes the backdrop inert and binds Escape to Cancel and Return to Open.
//
// Open is not destructive — it opens a vault and destroys nothing the
// reader cannot get back by switching again — so Return reaches it.
func vaultPickerDecision(cancel, openVault func(gtx layout.Context)) *modal.Decision {
	return &modal.Decision{Confirm: openVault, Cancel: cancel}
}

// vaultPickerBody lays the folder browser out as the dialog's body: the
// trail over exactly vaultPickerRows rows, so the dialog hugs a stated
// height. The list scrolls inside that height — a directory with more
// children than the rows shown is scrolled, never cut off.
func vaultPickerBody(
	gtx layout.Context,
	v *pickerView,
	m Model,
	tok themeTokens,
	trail breadcrumb.TrailLayout,
) layout.Dimensions {
	rowsPx := vaultPickerRows * gtx.Dp(list.RowHeight(tok.den))
	// No inset of the row's own: the surface has already spent 20 dp on
	// every side, so a row's symbol stands in the sidebar row's measured
	// column from the same edge the header and the breadcrumb lead on, and
	// its annotation ends on the column the Open button ends on.
	return v.browser(gtx, m, tok, trail, tok.col.WindowBackground, 0, rowsPx)
}

// dialogAction pins one footer button to the save dialog's measured push
// button width and hands it the cell its live layout.Widget arrives in. The
// height is the button's own: the density's control height is the same 24 px
// that footer's buttons measure.
func dialogAction(cell *atomic.Value) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = gtx.Dp(dialogButtonWDp)
		if w, ok := cell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(tokens.Comfortable.ControlHeight)))}
	}
}

// renderVaultPicker draws the dialog with pre-resolved tokens and no event
// handling, which is how a stored image is taken of it. The composition is
// the live one: the same modal, the same body, the same two actions.
func renderVaultPicker(
	shaper *text.Shaper,
	m Model,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	den tokens.Density,
) layout.Widget {
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	v := &pickerView{list: list.NewState()}
	trail := breadcrumb.NewTrail(shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
		colors, sp, typo.TitleSmall)
	body := func(gtx layout.Context) layout.Dimensions {
		return vaultPickerBody(gtx, v, m, tok, trail)
	}
	action := func(label string, emphasis button.Emphasis) layout.Widget {
		w := button.Render(shaper, label, colors, sp, rad, typo.LabelLarge, den,
			button.RenderState{Emphasis: emphasis})
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			gtx.Constraints.Max.X = gtx.Dp(dialogButtonWDp)
			return w(gtx)
		}
	}
	return modal.Render(shaper, modal.Props{
		Title: "Open Vault",
		Body:  body,
		Actions: []layout.Widget{
			action("Cancel", button.Tonal),
			action("Open", button.Filled),
		},
		Decision: &modal.Decision{},
	}, true, colors, sp, rad, typo.TitleMedium, den)
}
