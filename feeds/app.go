package main

import (
	"image"
	"image/color"
	"strings"
	"sync/atomic"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/alert"
	"github.com/vibrantgio/cadence/card"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/navbar"
	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/cadence/shell"
	"github.com/vibrantgio/cadence/table"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

// modelObsConsumers is the EXACT number of cold subscriptions that reach
// modelObs when feedsShellLayer is subscribed once (as spectrum/window does).
// It is LOAD-BEARING and must be measured, not hand-counted: mvuWin.Messages()
// drains a channel and rx.Publish() multicasts WITHOUT replay, so
// Publish().AutoConnect(modelObsConsumers) in run() connects the loop's
// upstream scan — and lets the seed emitted by mvu.Loop flow — only once the
// count-th subscription attaches. Too low and the consumers that attach after Connect miss the seed
// (the launch table/pagination render blank until the first real message); too
// high and Connect never fires (the app is frozen).
//
// The count is NOT the rx.Map(modelObs, …) variable count: each derived
// stream is cold and fans out to multiple downstream subscribers, so the real
// total is larger. The derivations and their fan-out:
//  1. openSectionsObs    → accordion Open                                 (1)
//  2. selectedFeedObs    → filtered, subscribed by paged + pageCountObs   (2)
//  3. currentPageObs     → paged + the pagination CombineLatest           (2)
//  4. sortObs            → sortCell mirror + table Sort prop + filtered×2 (4)
//  5. selectedArticleObs → the detail-pane CombineLatest                  (1)
//  6. selectedTabObs     → tabs Selected prop                             (1)
//  7. shareOpenObs       → popover Open prop                              (1)
//  8. splitRatioObs      → shell SplitPane SplitRatio prop                (1)
//  9. feedsObs           → sidebar CombineLatest (G5.2d)                  (1)
// 10. addFeedOpenObs     → modal Open prop (G5.2d)                        (1)
// 11. addFeedErrorObs    → modal errorCell mirror (G5.2d)                 (1)
// Total = 16, confirmed empirically by TestModelObsConsumerCountMatchesConst,
// which fails if a future edit changes the topology without updating this.
const modelObsConsumers = 16

// buildLayers returns the spectrum/window build function. The model
// observable drives selection, paging, sort, and accordion open state; the
// theme observable flows in independently per window.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			feedsShellLayer(th, shaper, modelObs),
		}
	}
}

func backdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	return rx.Map(
		rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
			return t.Color
		}),
		func(c tokens.ColorTokens) layout.Widget {
			fill := c.Surface
			return func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
				return layout.Dimensions{Size: size}
			}
		},
	)
}

// feedsShellLayer composes the feeds sidebar, navbar (with the Share
// popover), and a nested SplitPane — articles table on the left, article
// detail on the right — into a SidebarHeaderMain shell. Selection, paging,
// sort, accordion open state, the detail tab, the Share popover, and the
// split position are all derived from modelObs; theme tokens flow
// independently through th.
//
// cadence/shell exposes Sidebar as an rx.Observable[layout.Widget] but Main
// (and SplitPane's Left/Right, and navbar Actions) as static layout.Widget
// slots, and Shell re-emits (driving spectrum/window's Invalidate) only when
// its Sidebar or Navbar stream emits. So every live widget stream is folded
// onto the sidebar-driving observable, and the latest widget of each is
// published into an atomic cell — a layer-boundary adapter read by the
// corresponding static slot at frame time — the same hand-off mvu/window.go
// uses for its layer snapshot. Any model change therefore re-emits the
// sidebar stream, which makes Shell re-emit and the window repaint on the
// same frame. (The previous per-stream controller + mirrorWidget pattern
// severed state from the layer chain — the FEEDBACK-G5.1/G5.2 "click does
// nothing until the mouse moves" defect.)
func feedsShellLayer(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	modelObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	// The cold derivations of modelObs. Their count is mirrored by
	// modelObsConsumers above — keep them in sync.
	openSectionsObs := rx.Map(modelObs, func(m Model) map[int]bool { return m.openSections })
	feedsObs := rx.Map(modelObs, func(m Model) []feedGroup { return m.feeds })
	selectedFeedObs := rx.Map(modelObs, func(m Model) FeedID { return m.selectedFeed })
	currentPageObs := rx.Map(modelObs, func(m Model) int { return m.currentPage })
	sortObs := rx.Map(modelObs, func(m Model) table.Sort { return m.sort })
	selectedArticleObs := rx.Map(modelObs, func(m Model) ArticleID { return m.selectedArticle })
	selectedTabObs := rx.Map(modelObs, func(m Model) int { return m.selectedTab })
	shareOpenObs := rx.Map(modelObs, func(m Model) bool { return m.shareOpen })
	splitRatioObs := rx.Map(modelObs, func(m Model) float32 { return m.splitRatio })
	addFeedOpenObs := rx.Map(modelObs, func(m Model) bool { return m.addFeedOpen })
	addFeedErrorObs := rx.Map(modelObs, func(m Model) bool { return m.addFeedError })

	articlesObs := articlesMain(th, shaper, selectedFeedObs, currentPageObs, sortObs)
	detailObs := detailPane(th, shaper, selectedArticleObs, selectedTabObs)
	shareObs := sharePopover(th, shaper, shareOpenObs)
	modalObs := addFeedModal(th, shaper, addFeedOpenObs, addFeedErrorObs)
	toastObs := toast.Stack(th, toast.Props{Position: toast.TopRight, Shaper: shaper})

	// Layer-boundary cells (see the function comment). articlesCell and
	// detailCell feed the SplitPane's static Left/Right slots; splitCell
	// feeds the outer shell's static Main slot; shareCell feeds the navbar
	// Share action widget.
	var articlesCell, detailCell, splitCell, shareCell atomic.Value
	slot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	// The articles/detail split. Divider drags land SetSplitRatio messages
	// through OnSplitChange, and the committed position flows back in via
	// SplitRatio — the divider survives theme re-emissions and is
	// replayable like every other piece of model state. (A feeds-local
	// split.go replaced this component while its dragState raced under
	// model-driven emissions — see FEEDBACK-G5.2.md; cadence v0.1.1 moved
	// all drag state onto the frame goroutine, so the workaround is gone.)
	splitObs := shell.Shell(th, shell.Props{
		Layout:     shell.SplitPane,
		Left:       slot(&articlesCell),
		Right:      slot(&detailCell),
		SplitRatio: splitRatioObs,
		OnSplitChange: func(gtx layout.Context, ratio float32) {
			mvu.MessageOp{Message: SetSplitRatio{Ratio: ratio}}.Add(gtx.Ops)
		},
	})

	sidebarObs := feedsSidebar(th, shaper, openSectionsObs, feedsObs)
	sidebarDriven := rx.Map(
		rx.CombineLatest5(sidebarObs, articlesObs, detailObs, splitObs, shareObs),
		func(n rx.Tuple5[layout.Widget, layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			articlesCell.Store(n.Second)
			detailCell.Store(n.Third)
			splitCell.Store(n.Fourth)
			shareCell.Store(n.Fifth)
			return n.First
		},
	)

	shellObs := shell.Shell(th, shell.Props{
		Layout:  shell.SidebarHeaderMain,
		Sidebar: sidebarDriven,
		Navbar:  feedsNavbarProps(shaper, slot(&shareCell)),
		Main:    slot(&splitCell),
	})

	// Overlay composition: the Add-feed modal scrim and the toast stack draw
	// OVER the whole window. Rather than adding a third buildLayers layer
	// (which would change TestBuildLayersConstructsWithoutPanic's asserted
	// count), they are folded onto the shell stream and drawn after the shell
	// inside the returned widget, which then reports the shell's dims. Every
	// model change still re-emits this stream — driving the same-frame repaint.
	return rx.Map(
		rx.CombineLatest3(shellObs, modalObs, toastObs),
		func(n rx.Tuple3[layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			shellW, modalW, toastW := n.First, n.Second, n.Third
			return func(gtx layout.Context) layout.Dimensions {
				dims := shellW(gtx)
				if modalW != nil {
					modalW(gtx)
				}
				if toastW != nil {
					toastW(gtx)
				}
				return dims
			}
		},
	)
}

// shareCanvasDp sizes the Exact canvas handed to the Share popover widget in
// the navbar action slot. cadence/popover centres its anchor in the canvas
// and sizes its outside-press absorber to it, so the canvas must be (a)
// small enough to sit in the navbar's action row and (b) wide enough that
// the Bottom-placed surface (centred under the anchor) stays on-screen when
// the action row hugs the window's trailing edge. 160 dp leaves ~55 dp of
// margin either side of the anchor — enough for the ~130 dp surface.
// (Friction logged in FEEDBACK-G5.2.md.)
const (
	shareCanvasWDp = 160
	shareCanvasHDp = 28
)

// feedsNavbarProps builds the navbar with the brand, the (decorative) Add
// feed action, and the Share popover slot. shareSlot reads the latest
// popover widget from its layer-boundary cell; the wrapper pins the
// popover's canvas to an Exact size so the anchor centres where the button
// should sit and the returned dims do not blow up the navbar's Flex row
// (popover returns Dimensions{Size: canvas}).
func feedsNavbarProps(shaper *text.Shaper, shareSlot layout.Widget) navbar.Props {
	brand := func(gtx layout.Context) layout.Dimensions {
		return drawLabel(gtx, shaper, "Feeds", unit.Sp(18), color.NRGBA{A: 0xff})
	}
	var addClick widget.Clickable
	addFeed := func(gtx layout.Context) layout.Dimensions {
		if addClick.Clicked(gtx) {
			mvu.MessageOp{Message: OpenAddFeed{}}.Add(gtx.Ops)
		}
		return addClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Add feed").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, shaper, "Add feed", unit.Sp(14), color.NRGBA{R: 0x60, G: 0x80, B: 0xff, A: 0xff})
		})
	}
	share := func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(unit.Dp(shareCanvasWDp)), gtx.Dp(unit.Dp(shareCanvasHDp)))
		gtx.Constraints = layout.Exact(size)
		return shareSlot(gtx)
	}
	return navbar.Props{
		Brand:   brand,
		Shaper:  shaper,
		Actions: []layout.Widget{addFeed, share},
	}
}

// shareDestinations are the three no-op share targets the popover lists.
var shareDestinations = []string{"Copy link", "Email", "Mastodon"}

const (
	shareRowHDp  = 28
	shareListWDp = 120
)

// sharePopover composes the navbar Share button (the popover anchor) and its
// destination-list surface. Open state is model-derived; the anchor click
// lands ToggleShare, each destination click and OnDismiss land CloseShare —
// all mvu.MessageOps, so the popover opens/closes on the same frame.
//
// The destination list deliberately overrides its incoming constraints:
// cadence/popover measures Content against canvas/2, and the canvas here is
// the button-sized Exact wrapper from feedsNavbarProps — half a button could
// not fit one label. The content sizes itself and returns its own dims,
// which popover then pads into the surface rect. (Friction logged in
// FEEDBACK-G5.2.md.)
func sharePopover(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	shareOpenObs rx.Observable[bool],
) rx.Observable[layout.Widget] {
	type tokenState struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	var tokenCell atomic.Value
	tokenCell.Store(tokenState{col: tokens.DefaultLight, typ: tokens.DefaultTypeScale})
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typeObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	_ = rx.CombineLatest2(colorObs, typeObs).Subscribe(func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale], _ error, done bool) {
		if !done {
			tokenCell.Store(tokenState{col: t.First, typ: t.Second})
		}
	}, rx.Goroutine)

	var anchorClick widget.Clickable
	anchor := func(gtx layout.Context) layout.Dimensions {
		s := tokenCell.Load().(tokenState)
		if anchorClick.Clicked(gtx) {
			mvu.MessageOp{Message: ToggleShare{}}.Add(gtx.Ops)
		}
		return anchorClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Share").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, shaper, "Share", unit.Sp(s.typ.LabelLarge), color.NRGBA{R: 0x60, G: 0x80, B: 0xff, A: 0xff})
		})
	}

	destClicks := make([]widget.Clickable, len(shareDestinations))
	content := func(gtx layout.Context) layout.Dimensions {
		s := tokenCell.Load().(tokenState)
		rowH := gtx.Dp(unit.Dp(shareRowHDp))
		listW := gtx.Dp(unit.Dp(shareListWDp))
		for i, dest := range shareDestinations {
			if destClicks[i].Clicked(gtx) {
				// No-op destination: sharing is out of scope; the click
				// just dismisses the popover.
				mvu.MessageOp{Message: CloseShare{}}.Add(gtx.Ops)
			}
			off := op.Offset(image.Pt(0, i*rowH)).Push(gtx.Ops)
			rowGtx := gtx
			rowGtx.Constraints = layout.Exact(image.Pt(listW, rowH))
			dest := dest
			destClicks[i].Layout(rowGtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp(dest).Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				return drawLabel(gtx, shaper, dest, unit.Sp(s.typ.BodyMedium), s.col.OnSurface)
			})
			off.Pop()
		}
		return layout.Dimensions{Size: image.Pt(listW, rowH*len(shareDestinations))}
	}

	return popover.Popover(th, popover.Props{
		Open:      shareOpenObs,
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		OnDismiss: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseShare{}}.Add(gtx.Ops)
		},
	})
}

// addFeedDimsDp sizes the body card's interior rows. The card's textfield is
// height-constrained to a single row; the alert (shown only on empty submit)
// gets its own band above the field.
const (
	addFeedFieldHDp = 48
	addFeedBtnHDp   = 48
	addFeedAlertHDp = 56
	addFeedGapDp    = 12
)

// addFeedModal composes the "Add feed" modal: a cadence/modal whose Body is a
// cadence/card wrapping (optionally) a cadence/alert, a prism/input/textfield
// for the URL, and a prism/button submit. addFeedOpenObs drives the modal's
// Open; addFeedErrorObs drives whether the empty-URL alert shows.
//
// Component-prop shapes (logged in FEEDBACK-G5.2.md):
//   - modal.Props.Body and card.Props.* are STATIC layout.Widget slots, but
//     textfield/button/alert/card are rx.Observable[layout.Widget]. Each is
//     bridged through an atomic layer-boundary cell read by the static slot at
//     frame time — the same pattern the navbar Share popover uses.
//   - prism/input/textfield is uncontrolled: its widget.Editor lives in the
//     component's rx.Defer scope and is never exposed, and the submit is a
//     prism/button (not Enter), so the button has no handle to clear the
//     editor. The latest text is mirrored into urlCell via OnChange and read by
//     the button's OnClick; the reducer cannot clear the visible field.
func addFeedModal(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	addFeedOpenObs rx.Observable[bool],
	addFeedErrorObs rx.Observable[bool],
) rx.Observable[layout.Widget] {
	// urlCell holds the latest editor text (textfield is uncontrolled).
	var urlCell atomic.Value
	urlCell.Store("")

	field := input.TextField(th, input.TextFieldProps{
		Placeholder: "https://example.com/feed.xml",
		Description: "Feed URL",
		Shaper:      shaper,
		OnChange: func(_ layout.Context, txt string) {
			urlCell.Store(txt)
		},
	})

	var submitClick widget.Clickable
	submit := button.Button(th, button.Props{
		Label:     "Add",
		Clickable: &submitClick,
		Shaper:    shaper,
		OnClick: func(gtx layout.Context) {
			url, _ := urlCell.Load().(string)
			if strings.TrimSpace(url) != "" {
				// Success toast fires from the callback (the reducer is pure);
				// the reducer owns the append/close. Empty submit fires no
				// toast — the reducer raises the modal alert instead.
				toast.Notify(toast.Success, "Feed added")
			}
			mvu.MessageOp{Message: SubmitFeed{URL: url}}.Add(gtx.Ops)
		},
	})

	alertObs := alert.Alert(th, alert.Props{
		Variant: alert.Error,
		Title:   "Feed URL required",
		Shaper:  shaper,
	})

	// Layer-boundary cells for the static modal/card slots.
	var fieldCell, submitCell, alertCell atomic.Value
	cellSlot := func(c *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := c.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	// errorCell mirrors addFeedError so the static card body decides whether
	// to draw the alert band at frame time.
	var errorCell atomic.Bool
	_ = addFeedErrorObs.Subscribe(func(v bool, _ error, done bool) {
		if !done {
			errorCell.Store(v)
		}
	}, rx.Goroutine)

	// The card body: optional alert band, then the URL field, then the submit
	// button. Static widget assembled from the bridged cells.
	cardBody := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		gap := gtx.Dp(unit.Dp(addFeedGapDp))
		fieldH := gtx.Dp(unit.Dp(addFeedFieldHDp))
		btnH := gtx.Dp(unit.Dp(addFeedBtnHDp))
		alertH := gtx.Dp(unit.Dp(addFeedAlertHDp))
		y := 0
		if errorCell.Load() {
			stk := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			ag := gtx
			ag.Constraints = layout.Exact(image.Pt(w, alertH))
			cellSlot(&alertCell)(ag)
			stk.Pop()
			y += alertH + gap
		}
		fStk := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		fg := gtx
		fg.Constraints = layout.Exact(image.Pt(w, fieldH))
		cellSlot(&fieldCell)(fg)
		fStk.Pop()
		y += fieldH + gap

		bStk := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		bg := gtx
		bg.Constraints = layout.Exact(image.Pt(w, btnH))
		cellSlot(&submitCell)(bg)
		bStk.Pop()
		y += btnH
		return layout.Dimensions{Size: image.Pt(w, y)}
	}

	cardObs := card.Card(th, card.Props{Body: cardBody})
	var cardCell atomic.Value
	modalBody := func(gtx layout.Context) layout.Dimensions {
		if w, ok := cardCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	modalObs := modal.Modal(th, modal.Props{
		Open:   addFeedOpenObs,
		Title:  "Add feed",
		Body:   modalBody,
		Shaper: shaper,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseAddFeed{}}.Add(gtx.Ops)
		},
	})

	// Fold every live component widget into the modal stream so the modal
	// re-emits when any of them re-emits, and store the latest into its cell.
	// Positions: modalObs, cardObs, field, submit, alertObs.
	return rx.Map(
		rx.CombineLatest5(modalObs, cardObs, field, submit, alertObs),
		func(n rx.Tuple5[layout.Widget, layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			cardCell.Store(n.Second)
			fieldCell.Store(n.Third)
			submitCell.Store(n.Fourth)
			alertCell.Store(n.Fifth)
			return n.First
		},
	)
}

func drawLabel(
	gtx layout.Context,
	shaper *text.Shaper,
	msg string,
	size unit.Sp,
	c color.NRGBA,
) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	wl := widget.Label{MaxLines: 1}
	return wl.Layout(gtx, shaper, font.Font{}, size, msg, material)
}
