package main

import (
	"image"
	"strings"
	"sync/atomic"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/alert"
	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/composite"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/components/toast"
	"github.com/vibrantgio/components/tooltip"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/dialog"
	"github.com/vibrantgio/patterns/group"
	"github.com/vibrantgio/patterns/notifications"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/table"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// themeTokens is the colour/typography snapshot the app's own drawing code
// reads at frame time. The shaper is the theme's cached Typography shaper: the
// app builds none of its own, so the typeface — Roboto, plus the Roboto Mono
// face the Raw tab's Code style names — comes from the theme.
type themeTokens struct {
	col    tokens.PlatformColors
	typ    tokens.Typography
	den    tokens.Density
	shaper *text.Shaper
}

// postMessage is the live sink for an answer a control gives: the message
// reaches the loop as this frame's message op. It is a parameter wherever a
// control takes it so a test can drive the very control the window lays out
// and read back what it posted.
func postMessage(gtx layout.Context, msg mvu.Message) {
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
}

// mirrorTokens subscribes the theme's Platform and Typography streams into an
// atomic cell and returns a frame-time loader. It is the layer-boundary
// adapter for closures that run outside any rx scope (static component
// slots, table cell closures, toolbar titles) — the same hand-off pattern
// the layout.Widget cells in feedsShellLayer use.
func mirrorTokens(th rx.Observable[theme.Theme]) func() themeTokens {
	var cell atomic.Value
	cell.Store(themeTokens{
		col:    tokens.PlatformLight,
		typ:    tokens.DefaultTypography,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.Shaper(),
	})
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	denObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Density] { return t.Density })
	_ = rx.CombineLatest3(colorObs, typObs, denObs).Subscribe(rx.GoroutineContext(), func(t rx.Tuple3[tokens.PlatformColors, tokens.Typography, tokens.Density], _ error, done bool) {
		if !done {
			typ := t.Second
			cell.Store(themeTokens{col: t.First, typ: typ, den: t.Third, shaper: typ.Shaper()})
		}
	})
	return func() themeTokens { return cell.Load().(themeTokens) }
}

// buildLayers returns the theme/window build function. The model
// observable drives selection, paging, sort, and which rail sections are
// open; the theme observable flows in independently per window. widths is
// the memory the shell keeps the window's two column widths in, nil for a
// frame laid out for measurement rather than for a reader.
func buildLayers(modelObs rx.Observable[Model], widths *shell.ArrangementMemory, bars desktop.ScrollBarSetting) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			feedsShellLayer(th, modelObs, widths, bars),
		}
	}
}

// scrollBarStyle is every bar in this window: the fading overlay bar the
// colours alone give, standing without fading where the reader asked for
// scroll bars always. The setting is read once where the window is
// composed, so no frame asks the platform and no still render depends on
// the machine it runs on.
func scrollBarStyle(s scrollbar.Style, bars desktop.ScrollBarSetting) scrollbar.Style {
	if bars == desktop.ScrollBarsAlways {
		return s.Always()
	}
	return s
}

// backdropLayer paints the window's own plane under everything else:
// WindowBackground, the fill the platform gives a window. The regions above
// it — the content, the chrome rail — paint their own. The fill goes through
// the shared `backdrop.Widget` rather than a hand-rolled paint.FillShape, so
// the name is read in one place.
func backdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	return rx.Map(
		rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
			return t.Platform
		}),
		func(c tokens.PlatformColors) layout.Widget {
			return backdrop.Widget(c.WindowBackground)
		},
	)
}

// feedsShellLayer composes what this window stands in patterns/shell's
// three-column arrangement: the rail's pane down the leading edge, the
// articles table as the list column beside it, and the article with its
// tabs as the content column. Selection, paging, sort, which rail sections
// are open, the detail tab, the Share popover and whether the pane stands
// at all are derived from modelObs; theme tokens flow independently through
// th. The geometry is the shell's (frame.go); what is composed here is the
// slots that stand in it and the streams behind them.
//
// The arrangement takes its slots as static layout.Widget values. So every
// live layout.Widget stream is folded onto the rail-driving observable, and
// the latest one of each is published into an atomic cell — a
// layer-boundary adapter read by the corresponding static slot at frame
// time. Any model change therefore re-emits the rail's stream, which makes
// the frame re-emit and the window repaint on the same frame; state held
// outside the layer chain would leave a click unpainted until the next
// unrelated input event.
func feedsShellLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	widths *shell.ArrangementMemory,
	bars desktop.ScrollBarSetting,
) rx.Observable[layout.Widget] {
	// This window's arbiters. They are plain values with no
	// synchronisation, so the scope they are created at is the scope they are
	// safe at: theme/window calls the build function once per window and this
	// layer is composed exactly once inside it, which makes this function body
	// the window. Every popover, tooltip and dialog below is handed one of
	// these — a second arbitrable LAYER would have to take them as parameters
	// instead, because it would be composed beside this one rather than within
	// it.
	popArb := popover.NewArbiter()
	tipArb := tooltip.NewArbiter()
	dialogArb := dialog.NewArbiter()

	// The cold derivations of modelObs. Each is subscribed by the consumers
	// below — several of them more than once — and every subscription reads
	// the model in force the moment it attaches.
	openSectionsObs := rx.Map(modelObs, func(m Model) map[int]bool { return m.openSections })
	feedsObs := rx.Map(modelObs, func(m Model) []feedGroup { return m.feeds })
	selectedFeedObs := rx.Map(modelObs, func(m Model) FeedID { return m.selectedFeed })
	currentPageObs := rx.Map(modelObs, func(m Model) int { return m.currentPage })
	sortObs := rx.Map(modelObs, func(m Model) table.Sort { return m.sort })
	selectedArticleObs := rx.Map(modelObs, func(m Model) ArticleID { return m.selectedArticle })
	selectedTabObs := rx.Map(modelObs, func(m Model) int { return m.selectedTab })
	shareOpenObs := rx.Map(modelObs, func(m Model) bool { return m.shareOpen })
	confirmDeleteObs := rx.Map(modelObs, func(m Model) FeedID { return m.confirmDelete })
	addFeedOpenObs := rx.Map(modelObs, func(m Model) bool { return m.addFeedOpen })
	addFeedErrorObs := rx.Map(modelObs, func(m Model) bool { return m.addFeedError })
	prefsOpenObs := rx.Map(modelObs, func(m Model) bool { return m.prefsOpen })
	rowsPerPageObs := rx.Map(modelObs, func(m Model) int { return m.rowsPerPage })
	unreadOnlyObs := rx.Map(modelObs, func(m Model) bool { return m.unreadOnly })
	notesObs := rx.Map(modelObs, func(m Model) []notifications.Notification { return m.notes.Items() })
	queryObs := rx.Map(modelObs, func(m Model) string { return m.query })

	articlesObs := articlesMain(th, selectedFeedObs, selectedArticleObs, currentPageObs, sortObs, rowsPerPageObs, unreadOnlyObs, queryObs, tipArb)
	searchFieldObs := articlesSearch(th, postMessage)
	detailObs := detailPane(th, selectedArticleObs, selectedTabObs, bars)
	shareObs := sharePopover(th, shareOpenObs, popArb, postMessage)
	dialogObs := addFeedDialog(th, addFeedOpenObs, addFeedErrorObs, dialogArb)
	prefsObs := preferencesPane(th, prefsOpenObs, rowsPerPageObs, unreadOnlyObs, dialogArb)
	notesColumnObs := notifications.Column(th, notifications.Props{Position: notifications.TopRight, Notifications: notesObs})

	// The window's accelerators, one key area per chord of menu.go's table —
	// ⌘N and ⌘, on macOS, Ctrl-N and Ctrl-, elsewhere. They are app chrome,
	// laid out under everything else: the dialog owns dismissal, the app owns
	// arrival, and the two halves never meet in one component. Where the menu
	// bar carries the same chord the bar answers it first and these never
	// fire; away from macOS the bar is inert and these are the only route.
	chords := ChordAreas(postMessage)

	// The window's own frame: the arrangement the shell draws, seated on
	// the two widths this reader last left.
	frame := newWindowFrame(widths)

	// The platform set the frame is drawn from, read through the same
	// layer-boundary cell the toolbar's own titles use.
	loadFrameTok := mirrorTokens(th)

	sidebarObs := feedsSidebar(th, openSectionsObs, feedsObs, selectedFeedObs, confirmDeleteObs, popArb, postMessage, bars)

	// The slots the arrangement is handed, folded onto one stream so a
	// model change re-emits the whole composition — which is the same-frame
	// repaint.
	slotsObs := rx.Map(
		rx.CombineLatest5(sidebarObs, articlesObs, detailObs, searchFieldObs, shareObs),
		func(n rx.Tuple5[layout.Widget, articleSlots, layout.Widget, layout.Widget, layout.Widget]) frameSlots {
			return frameSlots{
				Rail:   n.First,
				Rows:   n.Second.Rows,
				Pager:  n.Second.Pager,
				Main:   n.Third,
				Search: n.Fourth,
				Share:  n.Fifth,
			}
		},
	)

	shellObs := rx.Map(
		rx.CombineLatest2(slotsObs, modelObs),
		func(n rx.Tuple2[frameSlots, Model]) layout.Widget {
			slots, model := n.First, n.Second
			return func(gtx layout.Context) layout.Dimensions {
				return frame.layout(gtx, model, loadFrameTok(), slots)
			}
		},
	)

	// Overlay composition: the Add-feed dialog, the Preferences pane and the
	// toast stack draw OVER the whole window. They are folded onto the shell
	// stream and drawn after the shell inside the returned layout.Widget, which
	// reports the shell's dims, rather than becoming a third buildLayers
	// layer. Every model change still re-emits this stream — driving the
	// same-frame repaint.
	//
	// The accelerators' key areas go FIRST, at the bottom of the hit stack:
	// a key area occludes pointer events, so it must never sit over the
	// content, and an open dialog's own scrim should shadow it the way it
	// shadows the rest of the app.
	return rx.Map(
		rx.CombineLatest4(shellObs, dialogObs, prefsObs, notesColumnObs),
		func(n rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			shellW, dialogW, prefsW, toastW := n.First, n.Second, n.Third, n.Fourth
			return func(gtx layout.Context) layout.Dimensions {
				// The window's content area is the available room every
				// tooltip below is placed in; each layer between here and
				// a tooltip moves it by the offset it places its child at.
				gtx = shell.WithAvailableRoom(gtx, image.Rectangle{Max: gtx.Constraints.Max})
				// The window's own plane, declared before anything is
				// offset, so a dialog's scrim composites over the frame
				// beneath it rather than blending with it.
				composite.Frame(gtx)
				for _, area := range chords {
					area(gtx)
				}
				// The window's own drag is the arrangement's: the pane's
				// strip and the toolbar's two shares claim it over every run
				// of themselves that holds no control, which is what a
				// toolbar under the full-size-content treatment has to do
				// once the native title bar is gone.
				dims := shellW(gtx)
				if dialogW != nil {
					dialogW(gtx)
				}
				if prefsW != nil {
					prefsW(gtx)
				}
				if toastW != nil {
					toastW(gtx)
				}
				return dims
			}
		},
	)
}

// shareDestinations are the three no-op share targets the popover lists.
var shareDestinations = []string{"Copy link", "Email", "Mastodon"}

const (
	shareRowHDp  = 28
	shareListWDp = 120
)

// sharePopover composes the toolbar's Share control (the popover anchor) and its
// destination-list surface. Open state is model-derived; the anchor click
// lands ToggleShare, each destination click and OnDismiss land CloseShare —
// all mvu.MessageOps, so the popover opens/closes on the same frame. post is
// where those messages go, and it is a parameter so a test can drive the very
// button the window lays out and read back what it posted.
//
// The anchor is the toolbar's own bordered button, the one the pane toggle
// and every other control in a toolbar band is drawn as: the capsule, its
// fill and rim, the shadow it casts, and the share symbol as its title in the
// toolbar's label colour. A toolbar action is a button, never a word in the
// accent and never a link.
//
// The destination list overrides its incoming constraints: patterns/popover
// measures Content against half the space it is given, and that space here is
// the button-sized Exact wrapper from feedsNavbarProps — half a button could
// not fit one title. The content sizes itself and returns its own dims, which
// popover then pads into the surface rect.
func sharePopover(
	th rx.Observable[theme.Theme],
	shareOpenObs rx.Observable[bool],
	popArb *popover.Arbiter,
	post func(gtx layout.Context, msg mvu.Message),
) rx.Observable[layout.Widget] {
	loadTok := mirrorTokens(th)

	var anchorClick widget.Clickable
	anchor := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		if anchorClick.Clicked(gtx) {
			post(gtx, ToggleShare{})
		}
		return shell.ToolbarControl(gtx, s.col, tokens.Radius, s.den,
			&anchorClick, icons.Mark(icons.SquareAndArrowUp), "Share", false)
	}

	destClicks := make([]widget.Clickable, len(shareDestinations))
	content := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		rowH := gtx.Dp(unit.Dp(shareRowHDp))
		listW := gtx.Dp(unit.Dp(shareListWDp))
		for i, dest := range shareDestinations {
			if destClicks[i].Clicked(gtx) {
				// No-op destination: sharing is out of scope; the click
				// just dismisses the popover.
				post(gtx, CloseShare{})
			}
			off := op.Offset(image.Pt(0, i*rowH)).Push(gtx.Ops)
			rowGtx := gtx
			rowGtx.Constraints = layout.Exact(image.Pt(listW, rowH))
			dest := dest
			destClicks[i].Layout(rowGtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp(dest).Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointershape.OverSize(gtx.Ops, image.Pt(listW, rowH), pointer.CursorPointer)
				return typeset.Text(gtx, s.shaper, dest, s.typ.BodyMedium,
					vgcolor.Flatten(s.col.Label, s.col.WindowBackground), 1)
			})
			off.Pop()
		}
		return layout.Dimensions{Size: image.Pt(listW, rowH*len(shareDestinations))}
	}

	return popover.Popover(th, popover.Props{
		Open:      shareOpenObs,
		Arbiter:   popArb,
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		// The anchor stands at the trailing end of the room the toolbar
		// leaves it, which is where this window's acts stand and where the
		// share's own reservation in the toolbar is.
		Align:         popover.AlignTrailing,
		AvailableRoom: attachment.AvailableRoom,
		OnDismiss: func(gtx layout.Context) {
			post(gtx, CloseShare{})
		},
	})
}

// addFeedDimsDp sizes the group's interior rows. The textfield is
// height-constrained to a single row; the alert (shown only on empty submit)
// gets its own band above the field.
const (
	addFeedFieldHDp = 48
	addFeedBtnHDp   = 48
	addFeedAlertHDp = 56
	addFeedGapDp    = 12
)

// addFeedDialog composes the "Add feed" dialog: a patterns/dialog whose Body is a
// patterns/group holding (optionally) a components/alert, a components/input/textfield
// for the URL, and a components/button submit. addFeedOpenObs drives the dialog's
// Open; addFeedErrorObs drives whether the empty-URL alert shows.
//
// A group and not a card: the dialog's body is one section of a form, so the
// hairline parts the dialog and nothing inside it is being singled out. The
// hairline is the platform's separator flattened onto the fill the group
// stands on, which here is the dialog's own plane.
//
// Component-prop shapes:
//   - dialog.Props.Body and group.Props.* are STATIC layout.Widget slots, but
//     textfield/button/alert/group are rx.Observable[layout.Widget]. Each is
//     bridged through an atomic layer-boundary cell read by the static slot at
//     frame time — the same pattern the toolbar's Share popover uses.
//   - components/input/textfield is uncontrolled: its widget.Editor lives in the
//     component's rx.Defer scope and is never exposed, and the submit is a
//     components/button (not Enter), so the button has no handle to clear the
//     editor. The latest text is mirrored into urlCell via OnChange and read by
//     the button's OnClick; the reducer cannot clear the visible field.
func addFeedDialog(
	th rx.Observable[theme.Theme],
	addFeedOpenObs rx.Observable[bool],
	addFeedErrorObs rx.Observable[bool],
	dialogArb *dialog.Arbiter,
) rx.Observable[layout.Widget] {
	// urlCell holds the latest editor text (textfield is uncontrolled).
	var urlCell atomic.Value
	urlCell.Store("")

	// fieldTag holds the field's own focus tag, which the pane declares to
	// the dialog: it is the body's first focusable, so it is what the dialog
	// opens with the keyboard on.
	var fieldTag atomic.Value

	field := input.TextField(th, input.TextFieldProps{
		Placeholder: "https://example.com/feed.xml",
		Description: "Feed URL",
		// Surface is left unsaid: a dialog's plane is WindowBackground, which
		// is what the field falls back to, so the edge it flattens its
		// hairline onto is already the fill it stands on.
		FocusTag: func(tag event.Tag) { fieldTag.Store(tag) },
		OnChange: func(_ layout.Context, txt string) {
			urlCell.Store(txt)
		},
	})

	var submitClick widget.Clickable
	submit := button.Button(th, button.Props{
		Title:     "Add",
		Clickable: &submitClick,
		OnClick: func(gtx layout.Context) {
			url, _ := urlCell.Load().(string)
			if strings.TrimSpace(url) != "" {
				// The toast is a second message off the same click, landed on
				// the same ops queue as SubmitFeed below: the reducer queues
				// it and owns the append/close. Empty submit raises no toast —
				// the reducer raises the dialog alert instead.
				notifications.Notify(gtx, toast.Success, "Feed added")
			}
			mvu.MessageOp{Message: SubmitFeed{URL: url}}.Add(gtx.Ops)
		},
	})

	alertObs := alert.Alert(th, alert.Props{
		Status: alert.Error,
		Title:  "Feed URL required",
	})

	// Layer-boundary cells for the static dialog/group slots.
	var fieldCell, submitCell, alertCell atomic.Value
	cellSlot := func(c *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := c.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	// errorCell mirrors addFeedError so the static group body decides whether
	// to draw the alert band at frame time.
	var errorCell atomic.Bool
	_ = addFeedErrorObs.Subscribe(rx.GoroutineContext(), func(v bool, _ error, done bool) {
		if !done {
			errorCell.Store(v)
		}
	})

	// What the group holds: optional alert band, then the URL field, then the
	// submit button. A static layout.Widget assembled from the bridged cells.
	groupBody := func(gtx layout.Context) layout.Dimensions {
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

	groupObs := group.Group(th, group.Props{
		Content: []layout.Widget{groupBody},
	})
	var groupCell atomic.Value
	dialogBody := func(gtx layout.Context) layout.Dimensions {
		if w, ok := groupCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	dialogObs := dialog.Dialog(th, dialog.Props{
		Open:    addFeedOpenObs,
		Title:   "Add feed",
		Body:    dialogBody,
		Arbiter: dialogArb,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseAddFeed{}}.Add(gtx.Ops)
		},
		// The body's own focusables, in the order it lays them out: the URL
		// field first, then the submit button under it. The field being first
		// is what the pane opens with the keyboard on, so a reader can type
		// a feed's address the moment the dialog is up.
		DynamicFocusTags: func() []event.Tag {
			var tags []event.Tag
			if tag, ok := fieldTag.Load().(event.Tag); ok && tag != nil {
				tags = append(tags, tag)
			}
			return append(tags, &submitClick)
		},
	})

	// Fold every live component stream into the dialog stream so the dialog
	// re-emits when any of them re-emits, and store the latest into its cell.
	// Positions: dialogObs, groupObs, field, submit, alertObs.
	return rx.Map(
		rx.CombineLatest5(dialogObs, groupObs, field, submit, alertObs),
		func(n rx.Tuple5[layout.Widget, layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			groupCell.Store(n.Second)
			fieldCell.Store(n.Third)
			submitCell.Store(n.Fourth)
			alertCell.Store(n.Fifth)
			return n.First
		},
	)
}
