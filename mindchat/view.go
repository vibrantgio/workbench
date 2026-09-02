package main

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/effects/depth"
	"github.com/vibrantgio/ivg"
	"github.com/vibrantgio/ivg/encode"
	"github.com/vibrantgio/ivg/generate"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"

	"slices"
)

// buildLayers returns the layer-builder the theme window renders: a
// backdrop layer and a content layer, both reacting to the live theme.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// themed pairs one theme emission's palette with the icon widgets prebuilt
// in that theme's glyph colours (rebuilding raster widgets per frame would
// discard their rasterisation cache).
type themed struct {
	palette Palette
	bar     scrollbar.Style
	avatar  layout.Widget
	remove  layout.Widget
	edit    layout.Widget
	add     layout.Widget
	gear    layout.Widget
	// md is the message-body markdown style: token defaults plus the app's
	// opt-ins — chroma highlighting matched to the appearance, and links
	// opening in the system browser. MessageRow adapts its text colours per
	// bubble role.
	md markdown.Style
	// col is the emission's whole ColorTokens, kept beside the derived
	// Palette because patterns/pane resolves its own fill and its own edge
	// ink from the palette rather than taking them as colours: the pane is
	// the vocabulary's object and what it is painted with is the pattern's
	// business, not this app's.
	col tokens.ColorTokens
	// typ and shaper carry the theme's Typography and its cached shaper —
	// the app builds no shaper of its own, so the typefaces (Roboto, and
	// Roboto Mono for code) come from the theme.
	typ    tokens.Typography
	shaper *text.Shaper
	// motion is the theme's duration scale, and it is the app's ONLY
	// reduce-motion signal. The theme already composes the OS preference:
	// while Reduce Motion is on, LiveTheme emits tokens.Motion.Reduced(),
	// whose every stop is zero — so a zero stop means "do not animate" and
	// the waiting indicator and the streaming dot both render static. Reading
	// theme/a11y here as well would be a second path to the same preference,
	// and a second poller.
	motion tokens.MotionScale
}

// Chroma styles for the two appearance modes; built once, shared by every
// message. FromTokens leaves Highlight nil, so assigning these is the app's
// opt-in to syntax highlighting (the sitedocs recipe).
var (
	mdHighlightLight = highlight.New("github")
	mdHighlightDark  = highlight.New("github-dark")
)

// messageMarkdownStyle derives the chat-body markdown style for the current
// colour and typography tokens: the token-themed defaults plus the app's
// opt-ins — chroma highlighting matched to the appearance, links opening
// in the system browser, and the bundled image provider. Mono and CodeSize
// are re-resolved from the theme's Code role, which keeps code spans and
// fences in chat bodies rendering in the theme's mono face at its size.
//
// The insets a reply can grow — a fenced block, an inline code chip — keep
// FromTokens' grounds, and that is this app's choice: a message body is read
// on the transcript's paper, FromTokens puts Paper at the Background pin and
// the code grounds one neutral step off it, and one step off the local paper
// is exactly the rung a raised inset takes. It reads as raised in both
// schemes the same way — LIGHTER than the page on paper and on slate alike, a
// whisper in the light scheme with the derived rim carrying the edge.
//
// Wearing a chroma base (highlight.Wear) would hand the fence the base
// author's own background instead, and a plate fitted to white paper puts a
// ground LIGHTER than this light scheme's page under the block — a step in
// the wrong direction. So the chroma style is taken for its inks only
// (highlight.New), matched to the appearance the ground reports.
func messageMarkdownStyle(c tokens.ColorTokens, typ tokens.Typography) markdown.Style {
	md := markdown.FromTokens(c, typ)
	md.Mono = font.Typeface(typ.Code.Typeface)
	md.CodeSize = unit.Sp(typ.Code.Size)
	// The appearance is read off the Background pin, which is the ground the
	// transcript — and so every fence in it — actually rests on.
	if isDarkColor(c.Background) {
		md.Highlight = mdHighlightDark
	} else {
		md.Highlight = mdHighlightLight
	}
	md.Text.OnLinkClick = func(_ layout.Context, url string) { openURL(url) }
	md.Images = mdImages
	return md
}

// ContentLayer renders the page: the window's frame — the floating
// conversation pane, the chrome row and the transcript beside them — with
// the modals and the undo bar over it. The composition itself is frame.go's;
// this is the wiring. The stateful widgets live at subscription scope,
// OUTSIDE the per-emission Map: the two scroll positions,
// the sidebar clickables, and the prompt TextField, whose editor state is
// Defer-scoped inside the component and subscribed exactly once by the
// CombineLatest3 below. Constructing any of them per emission would reset
// scroll or typing on every completion-stream delta.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	// This window's arbitration registers. They are plain values
	// with no synchronisation, so the scope they are created at is the scope
	// they are safe at: theme/window calls the build function once per
	// window and this layer is composed exactly once inside it, which makes
	// this function body the window. Every popover and modal below is handed
	// one of these — a second arbitrable LAYER would have to take them as
	// parameters instead, because it would be composed beside this one rather
	// than within it.
	popArb := popover.NewArbiter()
	modalArb := modal.NewArbiter()

	histList := list.NewState()
	chatList := list.NewState()
	msgDocs := newDocCache()
	rowClicks := map[string]*widget.Clickable{}
	deleteClicks := map[string]*widget.Clickable{}
	renameClicks := map[string]*widget.Clickable{}

	prompt := input.TextField(th, input.TextFieldProps{
		Placeholder:   "Send a message",
		Description:   "chat prompt",
		Submit:        true,
		SubmitMessage: func(text string) any { return Prompt{Content: text} },
	})

	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest3(t.Color, t.Typography, t.Motion), func(ct rx.Tuple3[tokens.ColorTokens, tokens.Typography, tokens.MotionScale]) themed {
			c, typ, motion := ct.First, ct.Second, ct.Third
			p := PaletteFrom(c)
			avatar, err := raster.Widget(ChatGPT, AvatarSize, AvatarSize, raster.WithColors(p.Icon))
			if err != nil {
				panic(err)
			}
			remove, err := raster.Widget(icons.ContentClear, DeleteIconSize, DeleteIconSize, raster.WithColors(p.Row))
			if err != nil {
				panic(err)
			}
			edit, err := raster.Widget(icons.EditorModeEdit, DeleteIconSize, DeleteIconSize, raster.WithColors(p.Row))
			if err != nil {
				panic(err)
			}
			add, err := raster.Widget(icons.ContentAdd, AddIconSize, AddIconSize, raster.WithColors(p.Heading))
			if err != nil {
				panic(err)
			}
			gear, err := raster.Widget(icons.ActionSettings, SettingsIconSize, SettingsIconSize, raster.WithColors(p.Heading))
			if err != nil {
				panic(err)
			}
			md := messageMarkdownStyle(c, typ)
			return themed{palette: p, col: c, bar: scrollbar.FromTokens(c), avatar: avatar, remove: remove, edit: edit, add: add, gear: gear, md: md, typ: typ, shaper: typ.Shaper(), motion: motion}
		})
	})

	// The window's own frame: the floating pane, the chrome row and the
	// content area beside them.
	frame := &windowFrame{}

	var undoClick widget.Clickable
	// The pane's own controls and the chrome row's are separate widgets: the
	// toggle that rides the pane and the one that recalls it are the two
	// halves of one switch, never the same widget standing in two places, and
	// only one of the two is laid out in any frame.
	var paneToggle, paneNewChat, settingsClick widget.Clickable

	// The frame and the undo bar are composed per emission and read at
	// frame time through atomic cells — the observable-over-static-slot
	// hand-off. Folding them onto one stream means every model change
	// re-emits the whole composition, which is the same-frame repaint.
	var frameCell, undoCell atomic.Value

	// The model-menu popover widget (the picker chip + its surface) reaches
	// the chrome row through a cell; its stream joins the final combine
	// below so menu updates repaint.
	var menuCell atomic.Value
	menuSlot := func(gtx layout.Context) layout.Dimensions {
		if w, ok := menuCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	partsObs := rx.Map(rx.CombineLatest3(themes, prompt, modelObs),
		func(next rx.Tuple3[themed, layout.Widget, Model]) int {
			t, promptW, model := next.First, next.Second, next.Third
			streaming := make(map[string]bool, len(model.Streams))
			for _, s := range model.Streams {
				streaming[s.Chat] = true
			}
			sidebar := SidebarPane(t, model.ChatList, model.CurrentChat.Name, streaming, chatList, rowClicks, deleteClicks, renameClicks, &paneNewChat, &paneToggle, &settingsClick)
			main := ChatPane(t, msgDocs.Rows(visibleHistory(model)), histList, promptW)
			frameCell.Store(layout.Widget(func(gtx layout.Context) layout.Dimensions {
				return frame.layout(gtx, model, t, sidebar, main, menuSlot)
			}))
			undoCell.Store(UndoBar(t, model.Pending, &undoClick))
			return 0
		})

	renameObs := RenameModal(th, modelObs, modalArb)
	settingsObs := SettingsModal(th, modelObs, modalArb)
	menuObs := ModelMenu(th, modelObs, popArb)

	// The window's chords, each the one its platform already spends on that
	// action, laid out from the one table the application menu is built from
	// too. A focused text editor claims a chord it wants first, for
	// its own editing; that is correct layering rather than a conflict, which
	// is why these can be global at all.
	//
	// Where the menu carries the same chord, the menu answers it first and
	// the key area below never sees it — the two post the same message, so
	// which one answers is invisible. The area is laid out all the same: away
	// from macOS the menu declaration is inert, and then this is the action's
	// only route.
	shortcuts := ChordAreas(func(gtx layout.Context, msg mvu.Message) {
		mvu.MessageOp{Message: msg}.Add(gtx.Ops)
	})

	// Overlays: the undo bar and the modals draw over the frame (the
	// settings modal last — its scrim covers everything). partsObs joins
	// the combine so every model emission re-emits the top widget; menuObs
	// joins so the picker's chip and surface stay current.
	return rx.Map(rx.CombineLatest4(renameObs, settingsObs, menuObs, partsObs),
		func(next rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, int]) layout.Widget {
			renameW, settingsW := next.First, next.Second
			menuCell.Store(next.Third)
			return func(gtx layout.Context) layout.Dimensions {
				// Key areas first, at the BOTTOM of the hit stack — they must
				// never sit over the content.
				for _, s := range shortcuts {
					s(gtx)
				}
				dims := layout.Dimensions{Size: gtx.Constraints.Max}
				if w, ok := frameCell.Load().(layout.Widget); ok && w != nil {
					dims = w(gtx)
				}
				if w, ok := undoCell.Load().(layout.Widget); ok && w != nil {
					w(gtx)
				}
				if renameW != nil {
					renameW(gtx)
				}
				if settingsW != nil {
					settingsW(gtx)
				}
				return dims
			}
		})
}

// visibleHistory is what the chat pane draws: the current chat's own history
// plus the two transient rows its in-flight exchange adds under it. Neither
// is persisted, and neither exists for a chat whose stream is not the current
// one — model.StreamFor is the whole test.
//
// The status row reports a server-side tool ("Searching the web…") while one
// runs. The pending row covers the gap between the request going out and the
// first token coming back: it appears as soon as the stream is registered and
// stands down the instant the first AssistantDelta opens the assistant row —
// which is the one condition below, since a delta is the only thing that puts
// an assistant row last. A reasoning model can spend four seconds before its
// first token, and an inert pane reads as a hung application.
func visibleHistory(model Model) []Message {
	id, streaming := model.StreamFor(model.CurrentChat.Name)
	if !streaming {
		return model.CurrentChat.History
	}
	own := model.CurrentChat.History
	history := slices.Clone(own)
	if status := model.Streams[id].Status; status != "" {
		history = append(history, Message{Role: RoleStatus, Content: status})
	}
	// Tested against the chat's OWN last row, not the appended status row:
	// a tool running after the answer has started must not resurrect the
	// waiting indicator.
	if n := len(own); n == 0 || own[n-1].Role != RoleAssistant {
		history = append(history, Message{Role: RolePending})
	}
	return history
}

// renameTarget keys the rebuild of the rename modal's uncontrolled text
// field: a new epoch means a fresh field seeded with the target's name.
type renameTarget struct {
	epoch int
	seed  string // current name without extension
}

// RenameModal builds the rename-chat modal stream: a patterns/modal DECISION
// whose body is an epoch-rebuilt components TextField and whose two answers are
// Cancel and Rename. Validation is the reducer's job — an invalid RenameChat
// is rejected and the modal stays open; a valid one, or Escape, closes it.
// Both model derivations are DistinctUntilChanged so completion-stream deltas
// cannot rebuild the field mid-typing.
func RenameModal(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], modalArb *modal.Arbiter) rx.Observable[layout.Widget] {
	openObs := rx.Map(modelObs, func(m Model) bool { return m.Rename.Target != "" }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))
	editObs := rx.Map(modelObs, func(m Model) renameTarget {
		return renameTarget{
			epoch: m.Rename.Epoch,
			seed:  strings.TrimSuffix(m.Rename.Target, filepath.Ext(m.Rename.Target)),
		}
	}).Pipe(rx.DistinctUntilChanged(func(a, b renameTarget) bool { return a == b }))

	// nameCell mirrors the field text (the field is uncontrolled), reseeded
	// on each open so an untouched field submits the unchanged name.
	// fieldTagCell carries the current field instance's focus tag (new per
	// epoch) into the modal's Tab cycle.
	var nameCell, fieldTagCell atomic.Value
	nameCell.Store("")

	fieldObs := rx.SwitchMap(editObs, func(e renameTarget) rx.Observable[layout.Widget] {
		nameCell.Store(e.seed)
		// No editor Submit: the modal is a decision and claims Return for
		// its default action before the field is laid out, so an editor
		// submit binding here could never fire. The decision's Confirm
		// renames with the same text, from nameCell.
		return input.TextField(th, input.TextFieldProps{
			Placeholder: "Chat name",
			Description: "chat name",
			Seed:        e.seed,
			// The rename field stands on the decision modal's surface — a
			// level-2 plane, not the window's own surface.
			Level:    tokens.Level2,
			FocusTag: func(tag event.Tag) { fieldTagCell.Store(tag) },
			OnChange: func(_ layout.Context, text string) { nameCell.Store(text) },
		})
	})

	// The two answers this dialog accepts, each wired to both a footer
	// button and one half of modal.Decision so the keyboard and the footer
	// answer identically. Their clickables join the Tab cycle via
	// ActionFocusTags.
	var cancelClick, submitClick widget.Clickable
	cancel := func(gtx layout.Context) {
		mvu.MessageOp{Message: CloseRename{}}.Add(gtx.Ops)
	}
	rename := func(gtx layout.Context) {
		if name, ok := nameCell.Load().(string); ok {
			mvu.MessageOp{Message: RenameChat{To: name}}.Add(gtx.Ops)
		}
	}
	// The rename dialog's footer stands on its level-2 fill, like the field
	// above it. Filled buttons ring against their own fill, so nothing
	// moves; the declaration keeps the level with the widget.
	cancelObs := button.Button(th, button.Props{
		Label:     "Cancel",
		Level:     tokens.Level2,
		Clickable: &cancelClick,
		OnClick:   cancel,
	})
	submitObs := button.Button(th, button.Props{
		Label:     "Rename",
		Level:     tokens.Level2,
		Clickable: &submitClick,
		OnClick:   rename,
	})

	// The modal body and actions are static slots; the live field/button
	// widgets reach them through cells (the observable-over-static-slot
	// hand-off).
	var fieldCell, cancelCell, submitCell atomic.Value
	slot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}
	body := func(gtx layout.Context) layout.Dimensions {
		cg := gtx
		cg.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(RenameFieldHeight)))
		slot(&fieldCell)(cg)
		return layout.Dimensions{Size: cg.Constraints.Max}
	}
	// components text buttons fill their available width, so each footer action
	// gets a fixed-size box.
	action := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(RenameButtonWidth), gtx.Dp(RenameButtonHeight)))
			return slot(cell)(gtx)
		}
	}

	modalObs := modal.Modal(th, modal.Props{
		Open:    openObs,
		Title:   "Rename chat",
		Body:    body,
		Arbiter: modalArb,
		Actions: []layout.Widget{action(&cancelCell), action(&submitCell)},
		// The field leads the Tab cycle — and, being first, receives focus
		// when the modal opens, so typing starts immediately. Its tag is
		// dynamic: each open rebuilds the field (new editor, new tag).
		DynamicFocusTags: func() []event.Tag {
			if tag, ok := fieldTagCell.Load().(event.Tag); ok && tag != nil {
				return []event.Tag{tag}
			}
			return nil
		},
		ActionFocusTags: []event.Tag{&cancelClick, &submitClick},
		// A DECISION, not a panel: "what shall this chat be called?" has
		// two answers and the footer is both of them. Declaring it drops the
		// close X, makes the backdrop inert so a stray click cannot throw
		// away a typed name, and binds Escape to Cancel and Return to
		// Rename.
		//
		// Rename is not Destructive: it moves a history file to a new name,
		// keeps its contents, and is undone by renaming back. Return
		// therefore stays on the primary.
		Decision: &modal.Decision{Confirm: rename, Cancel: cancel},
	})

	// Fold the live field/button streams onto the modal stream so their
	// emissions repaint it.
	return rx.Map(rx.CombineLatest4(modalObs, fieldObs, cancelObs, submitObs),
		func(next rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			fieldCell.Store(next.Second)
			cancelCell.Store(next.Third)
			submitCell.Store(next.Fourth)
			return next.First
		})
}

// ChatPane stacks the scrolling message history and the prompt field. The
// model picker stands in the window's chrome row, beside the conversation's
// title, so the pane begins at the transcript.
//
// The input bar is the last thing in the pane and so owns the window's
// bottom edge on this side, with nothing standing under it and nothing
// beside it: a bottom rhythm invented on one side of a window and answered
// on neither is what a reader reads as two applications.
//
// The pane paints its own ground before any of that, and it has to: the
// shell fills the whole split — both halves — with the window's FLOOR, so
// anything the pane does not paint over shows furniture where the
// transcript should be. The message rows paint their own ground, so the
// bleed only appears where the transcript is SHORTER than the window, which
// no whole-window golden in this package is ever in: the demo conversation
// the headless frame renders fills the viewport. The transcript's ground is
// the header band, the turns AND the space around them, and one fill states
// all three.
//
// It is painted at the SLOT's width, before the pane clamps itself to its
// reading measure: on a window wider than ChatPaneWidth plus the sidebar,
// the strip the pane declines to occupy is still the pane's half of the
// split, and a band of furniture down the trailing edge of the content
// region is the same defect at a few pixels wide.
func ChatPane(t themed, chat []msgRow, hist *list.State, prompt layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		FillRect(gtx, image.Rectangle{Max: gtx.Constraints.Max}, 0, t.palette.Ground)
		// The transcript is a reading measure, and a reading measure that is
		// narrower than the room it is given is CENTRED in it — left alone it
		// hugs the leading edge, which with the pane away leaves a column of
		// prose pinned to one side of an empty half.
		avail := gtx.Constraints.Max
		gtx.Constraints = ClampWidth(gtx, 0, ChatPaneWidth)
		size := gtx.Constraints.Max
		if slack := avail.X - size.X; slack > 0 {
			defer op.Offset(image.Pt(slack/2, 0)).Push(gtx.Ops).Pop()
		}

		layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return list.LayoutScrollbar(gtx, hist, t.bar, list.Occupy, chat,
					func(gtx layout.Context, row msgRow) layout.Dimensions {
						return MessageRow(gtx, t, row)
					})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// The seam between the transcript and the composer: the
				// composer stands off the paper the messages lie on, and
				// what parts two things on one ground is a hairline. It is
				// the transcript's only rule — one region, one edge, drawn
				// where the content actually changes.
				seam := gtx.Dp(1)
				FillRect(gtx, image.Rectangle{
					Min: image.Pt(gtx.Dp(12), 0),
					Max: image.Pt(gtx.Constraints.Max.X-gtx.Dp(12), seam),
				}, 0, t.palette.Separator)
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, seam)}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(PaneMargin).Layout(gtx, prompt)
			}),
		)

		return layout.Dimensions{Size: size}
	}
}

// MessageRow renders one history entry: a full-width row with the body
// indented past the avatar column, and the assistant avatar on its (and
// error notices') rows. User and assistant bodies lay out their markdown
// Document — inline styles and code fences, links live — in the row's text
// colours; error rows read as plain labels in the error colour, transient
// status rows ("Searching the web…") in the heading colour. An answer's
// citations arrive inside the Document (messageSource).
//
// Only the user's own turn carries a fill. Everything else rests on the
// transcript's ground — the Background pin — because the transcript is what
// this window exists to show and a resting expanse of it may not be filled
// at a rung the ladder keeps for things that appear and leave. The row
// paints that ground itself rather than letting the backdrop show through,
// so a raised inset inside a reply (a code fence) has a stated paper to step
// up from wherever the row is composed.
func MessageRow(gtx layout.Context, t themed, row msgRow) layout.Dimensions {
	msg := row.Msg
	p := t.palette
	st := t.typ.BodyLarge

	isUser := msg.Role == RoleUser
	fill, textColor := p.Ground, p.BotText
	switch msg.Role {
	case RoleUser:
		fill, textColor = p.UserBubble, p.UserText
	case RoleError:
		textColor = p.Error
	case RoleStatus:
		textColor = p.Heading
	}

	m := op.Record(gtx.Ops)
	dims := layout.UniformInset(12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		margin := gtx.Dp(50)
		defer op.Offset(image.Pt(margin, 0)).Push(gtx.Ops).Pop()
		gtx.Constraints.Max.X -= margin
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		var dims layout.Dimensions
		if msg.Role == RolePending {
			dims = WaitingDots(gtx, t)
			dims.Size.X = gtx.Constraints.Max.X
		} else if row.Doc != nil {
			md := t.md
			md.Text.Color = textColor
			if isUser {
				// The token link colour (Primary) would vanish on the
				// Primary user bubble; the underline still marks links.
				md.Text.LinkColor = textColor
			}
			gtx.Constraints.Min = image.Point{}
			dims = row.Doc.LayoutColumn(gtx, t.shaper, md)
			// The row spans the full pane width regardless of the
			// column's natural content width.
			dims.Size.X = gtx.Constraints.Max.X
		} else {
			textMaterial := Material(gtx.Ops, textColor)
			label := roleLabel(st, 0)
			label.Alignment, label.Truncator = text.Start, "…"
			dims = typeset.Layout(gtx, t.shaper, label, roleFont(st), unit.Sp(st.Size), msg.Content, textMaterial)
		}
		dims.Size.X += margin
		return dims
	})
	foreground := m.Stop()

	FillRect(gtx, image.Rectangle{Max: dims.Size}, 0, fill)

	if !isUser && msg.Role != RoleStatus {
		constraints := gtx.Constraints
		iconSize := gtx.Dp(AvatarSize)
		gtx.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
		t.avatar(gtx)
		gtx.Constraints = constraints
	}

	foreground.Add(gtx.Ops)
	return dims
}

// SidebarPane renders the column that stands inside the floating pane: the
// top strip the window's control buttons pass through, the conversation
// list, and the settings row at the foot.
//
// There is no wordmark and no section heading: the menu bar and the Dock
// carry this application's identity, and a list with exactly one section does
// not announce itself.
//
// The strip is reserved by the flex and DRAWN AFTERWARDS, which is a
// statement about the keyboard rather than about paint. Focus follows the
// order the ops are written in, and the reading order of this pane is the
// conversations, then the settings that act on all of them, then the pane's
// own controls — a reader who tabs into the pane means to reach a
// conversation, not to put the pane away.
func SidebarPane(t themed, chats ChatList, current string, streaming map[string]bool, rows *list.State, rowClicks, deleteClicks, renameClicks map[string]*widget.Clickable, newChat, toggle, settings *widget.Clickable) layout.Widget {
	// Ensure every chat has persistent Clickables for hover/click state.
	for _, name := range chats {
		if _, ok := rowClicks[name]; !ok {
			rowClicks[name] = new(widget.Clickable)
		}
		if _, ok := deleteClicks[name]; !ok {
			deleteClicks[name] = new(widget.Clickable)
		}
		if _, ok := renameClicks[name]; !ok {
			renameClicks[name] = new(widget.Clickable)
		}
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		gtx.Constraints = layout.Exact(size)

		// The pane's own strip: cut deep enough to hold the window's
		// control buttons where the window keeps them, with the same air
		// below them as above, by the pattern's arithmetic. The pane fills
		// itself, so nothing here paints a ground.
		stripH := min(gtx.Dp(unit.Dp(pane.StripDp)), size.Y)

		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(size.X, stripH)}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return list.LayoutScrollbar(gtx, rows, t.bar, list.Overlay, chats,
					func(gtx layout.Context, name string) layout.Dimensions {
						return ChatRow(gtx, t, name, name == current, streaming[name], rowClicks[name], renameClicks[name], deleteClicks[name])
					})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SidebarFooter(gtx, t, settings)
			}),
		)

		sgtx := gtx
		sgtx.Constraints = layout.Exact(image.Pt(size.X, stripH))
		SidebarStrip(sgtx, t, toggle, newChat)
		return layout.Dimensions{Size: size}
	}
}

// SidebarStrip is the band across the top of the pane: the window control
// buttons' span skipped at the leading end, a stretch that moves the window
// across the middle, and the pane's two controls at the trailing corner.
//
// The toggle rides here because it puts the pane away and a dismiss
// control belongs to the thing it dismisses; new chat
// rides here because it is the application's primary action and the list
// under it is what it adds to. Both stand again in the chrome row once the
// pane is gone, at the same size and on the same line — they are two halves
// of one switch each, not two controls that happen to look alike.
func SidebarStrip(gtx layout.Context, t themed, toggle, newChat *widget.Clickable) layout.Dimensions {
	return pane.Strip(gtx, windowButtonsEnd(),
		func(gtx layout.Context) layout.Dimensions {
			return sidebarToggle(gtx, t, toggle, "Hide the conversations")
		},
		func(gtx layout.Context) layout.Dimensions {
			return desktop.DragRun(gtx, gtx.Dp(controlGapDp))
		},
		func(gtx layout.Context) layout.Dimensions {
			return newChatMark(gtx, t, newChat)
		})
}

// windowButtonsEnd reports the trailing edge of the window's three control
// buttons, and is the one place this app asks the window about them. The edge
// is read from the buttons themselves rather than worked out from the
// placement that put them there: their size and spacing are the platform's,
// and a number written down here would drift with the next release of it.
//
// A render with no window behind it is told zero — which is every render in
// the test suite, and every platform that keeps its own decorations, where
// there are no such buttons to clear. It is a variable so that a test can
// state a measurement it has no window to take.
var windowButtonsEnd = desktop.LeadingInset

// SidebarFooter is the pane's foot: a hairline off the rows and, under it,
// the settings affordance — gear and label sharing one vertical centre on
// the rows' own gutter.
//
// Settings stands here and nowhere else in the window's chrome. It acts on
// the application rather than on the conversation, so it earns no standing
// place in a window whose pane is away; Cmd-comma is how it is reached then,
// which is where this platform keeps it anyway.
//
// The hairline is the pane's OWN, drawn inside its outline and running only
// the pane's width — it says the scrolling stops here, which is a fact
// about this column and not a band across the window.
func SidebarFooter(gtx layout.Context, t themed, settings *widget.Clickable) layout.Dimensions {
	for settings.Clicked(gtx) {
		mvu.MessageOp{Message: OpenSettings{}}.Add(gtx.Ops)
	}
	p := t.palette
	width := gtx.Constraints.Max.X
	sep := gtx.Dp(1)
	rowH := gtx.Dp(FooterRowHeight)

	FillRect(gtx, image.Rectangle{Min: image.Pt(gtx.Dp(12), 0), Max: image.Pt(width-gtx.Dp(12), sep)}, 0, p.Separator)

	defer op.Offset(image.Pt(0, sep)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(width, rowH))
	settings.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		textColor := p.Row
		if settings.Hovered() {
			textColor = p.RowActive
			FillRect(gtx, image.Rectangle{Max: gtx.Constraints.Max}, 0, p.RowHovered)
		}
		left := gtx.Dp(16)
		iconSz := gtx.Dp(FooterIconSize)
		off := op.Offset(image.Pt(left, (rowH-iconSz)/2)).Push(gtx.Ops)
		icon := gtx
		icon.Constraints = layout.Exact(image.Pt(iconSz, iconSz))
		t.gear(icon)
		off.Pop()

		labelRect := image.Rect(left+iconSz+gtx.Dp(10), 0, gtx.Constraints.Max.X-gtx.Dp(12), rowH)
		textdraw.FillText(gtx, t.shaper, roleText(t.typ.BodyMedium), labelRect, 0, 0.5, textColor, "Settings")
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	return layout.Dimensions{Size: image.Pt(width, rowH+sep)}
}

// IconButton lays a square icon inside a clickable with a pointer cursor.
func IconButton(gtx layout.Context, click *widget.Clickable, size unit.Dp, draw func(gtx layout.Context, sizePx int)) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		sz := gtx.Dp(size)
		gtx.Constraints = layout.Exact(image.Pt(sz, sz))
		draw(gtx, sz)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

// PanelGlyph draws the [|] sidebar-toggle icon with clip paths (the
// patterns convention for chrome glyphs): a rounded outline with a divider
// line a third of the way in.
func PanelGlyph(gtx layout.Context, sizePx int, col color.NRGBA) {
	stroke := float32(gtx.Dp(unit.Dp(1.5)))
	inset := gtx.Dp(unit.Dp(1))
	r := image.Rect(inset, inset+sizePx/8, sizePx-inset, sizePx-inset-sizePx/8)
	rr := clip.RRect{Rect: r, NW: sizePx / 6, NE: sizePx / 6, SW: sizePx / 6, SE: sizePx / 6}
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: rr.Path(gtx.Ops), Width: stroke}.Op())
	x := r.Min.X + r.Dx()/3
	bar := image.Rect(x, r.Min.Y, x+int(stroke), r.Max.Y)
	paint.FillShape(gtx.Ops, col, clip.Rect(bar).Op())
}

// UndoBar renders the transient bottom-centre undo affordance while a
// delete's undo window is open. It is fully model-driven: the bar exists
// exactly while model.Pending names a chat, and the reducer closes the
// window (UndoDelete or the ConfirmDelete timer).
func UndoBar(t themed, pending PendingDelete, undo *widget.Clickable) layout.Widget {
	if pending.Name == "" {
		return func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
	p := t.palette
	display := strings.TrimSuffix(pending.Name, filepath.Ext(pending.Name))
	if len(display) > 0 {
		display = strings.ToUpper(display[:1]) + display[1:]
	}
	msg := "Deleted “" + display + "”"
	hint := "(Ctrl+Z)"
	if runtime.GOOS == "darwin" {
		hint = "(⌘Z)"
	}

	return func(gtx layout.Context) layout.Dimensions {
		for undo.Clicked(gtx) {
			mvu.MessageOp{Message: UndoDelete{}}.Add(gtx.Ops)
		}
		max := gtx.Constraints.Max

		inner := gtx
		inner.Constraints = layout.Constraints{Max: max}
		m := op.Record(gtx.Ops)
		dims := layout.UniformInset(12).Layout(inner, func(gtx layout.Context) layout.Dimensions {
			body, action, caption := t.typ.BodyMedium, t.typ.LabelLarge, t.typ.BodySmall
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return typeset.Layout(gtx, t.shaper, roleLabel(body, 1), roleFont(body), unit.Sp(body.Size), msg, Material(gtx.Ops, p.BotText))
				}),
				layout.Rigid(layout.Spacer{Width: 16}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return undo.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return typeset.Layout(gtx, t.shaper, roleLabel(action, 1), roleFont(action), unit.Sp(action.Size), "Undo", Material(gtx.Ops, p.Accent))
					})
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return typeset.Layout(gtx, t.shaper, roleLabel(caption, 1), roleFont(caption), unit.Sp(caption.Size), hint, Material(gtx.Ops, p.Row))
				}),
			)
		})
		content := m.Stop()

		pos := image.Pt((max.X-dims.Size.X)/2, max.Y-dims.Size.Y-gtx.Dp(UndoBarMargin))
		defer op.Offset(pos).Push(gtx.Ops).Pop()
		// The patterns toast treatment: a cast shadow under an accent-tinted
		// fill ringed in the accent, so the bar separates from the chat
		// surfaces it floats over (a level-2 fill alone sat at ~1.2:1 against
		// them, and ~1:1 against the assistant's rows in dark mode).
		//
		// The base is the toast rung rather than the selected-row fill: that
		// fill is a Primary tint, and tinting it again with the accent would
		// leave the bar a purple wash with nothing neutral under the ring.
		bounds := image.Rectangle{Max: dims.Size}
		radius := gtx.Dp(UndoBarRadius)
		depth.Shadow(gtx, bounds, tokens.Level3, radius, 1)
		FillRect(gtx, bounds, radius, Blend(p.Toast, p.Accent, 0x33))
		ring := clip.RRect{Rect: bounds, SE: radius, SW: radius, NE: radius, NW: radius}
		paint.FillShape(gtx.Ops, p.Accent, clip.Stroke{Path: ring.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
		content.Add(gtx.Ops)
		return layout.Dimensions{}
	}
}

// ChatRow renders a single chat entry in the sidebar with hover and
// selection states, and rename/delete icons revealed while the row is
// active.
func ChatRow(gtx layout.Context, t themed, name string, selected, streaming bool, row, ren, del *widget.Clickable) layout.Dimensions {
	p := t.palette

	// Drain pending clicks before Layout — Layout's internal update loop
	// consumes click events and discards them, so Clicked must run first.
	// The icons sit on top of the row, so an icon click suppresses any
	// row-select click registered on the same press.
	iconClicked := false
	for del.Clicked(gtx) {
		iconClicked = true
		mvu.MessageOp{Message: DeleteChat{Name: name}}.Add(gtx.Ops)
	}
	for ren.Clicked(gtx) {
		iconClicked = true
		mvu.MessageOp{Message: OpenRename{Name: name}}.Add(gtx.Ops)
	}
	for row.Clicked(gtx) {
		if !iconClicked {
			mvu.MessageOp{Message: SelectChat{Name: name}}.Add(gtx.Ops)
		}
	}

	displayName := strings.TrimSuffix(name, filepath.Ext(name))
	// Title-case the first letter for a cleaner look.
	if len(displayName) > 0 {
		displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
	}

	// The icons' input areas occlude the row's, so hovering an icon must
	// still count as hovering the row (else the icons would flicker away).
	hovered := row.Hovered() || del.Hovered() || ren.Hovered()
	var bgColor color.NRGBA
	var textColor color.NRGBA
	switch {
	case selected:
		bgColor = p.RowSelected
		textColor = p.RowActive
	case hovered:
		bgColor = p.RowHovered
		textColor = p.RowActive
	default:
		bgColor = p.Sidebar
		textColor = p.Row
	}

	label := roleLabel(t.typ.BodyMedium, 1)
	label.Alignment, label.Truncator = text.Start, "…"

	return row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		textMaterial := Material(gtx.Ops, textColor)

		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(11), Bottom: unit.Dp(11), Left: unit.Dp(20), Right: unit.Dp(12)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						rowStyle := t.typ.BodyMedium
						dims := typeset.Layout(gtx, t.shaper, label, roleFont(rowStyle), unit.Sp(rowStyle.Size), displayName, textMaterial)
						// Claim the full flex share so the icon sits at
						// the row's right edge, not after the text.
						dims.Size.X = gtx.Constraints.Max.X
						return dims
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// The dot slot is always reserved (no layout
						// shift); the dot itself shows only while this
						// chat has an in-flight completion.
						slot := image.Pt(gtx.Dp(StreamDotSlot), gtx.Dp(DeleteIconSize))
						if streaming {
							StreamDot(gtx, t, slot)
						}
						return layout.Dimensions{Size: slot}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// The slots are always reserved so revealing the
						// icons never shifts the layout; the glyphs and
						// their click areas exist only while the row is
						// active.
						iconSize := gtx.Dp(DeleteIconSize)
						gap := gtx.Dp(6)
						size := image.Pt(2*iconSize+gap, iconSize)
						gtx.Constraints = layout.Exact(size)
						if selected || hovered {
							icon := gtx
							icon.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
							ren.Layout(icon, t.edit)
							defer op.Offset(image.Pt(iconSize+gap, 0)).Push(gtx.Ops).Pop()
							del.Layout(icon, t.remove)
						}
						return layout.Dimensions{Size: size}
					}),
				)
			},
		)
		foreground := m.Stop()

		FillRect(gtx, image.Rectangle{Max: dims.Size}, 0, bgColor)
		// Left accent bar for the selected item.
		if selected {
			FillRect(gtx, image.Rectangle{Max: image.Pt(gtx.Dp(3), dims.Size.Y)}, 0, p.Accent)
		}
		foreground.Add(gtx.Ops)
		return dims
	})
}

// motionPhase returns where the frame's instant sits in a cycle of the given
// length, as a fraction in [0,1), with lead shifting this caller ahead of the
// others sharing the cycle.
//
// A cycle of zero reports ok=false, and that is how both indicators honour
// reduce-motion: the theme emits tokens.Motion.Reduced() while the preference
// is on, every stop zero, so a cycle derived from a stop is zero too — no
// phase, nothing to animate, and no frame to schedule.
func motionPhase(now time.Time, cycle, lead time.Duration) (float64, bool) {
	if cycle <= 0 {
		return 0, false
	}
	off := (now.UnixNano() - int64(lead)) % int64(cycle)
	if off < 0 {
		off += int64(cycle)
	}
	return float64(off) / float64(cycle), true
}

// dotPulse is the alpha curve both in-flight indicators pulse on: a sine over
// the phase, floored at 45% of the colour's own alpha so a dot fades but
// never vanishes.
func dotPulse(alpha uint8, phase float64) uint8 {
	return uint8(float64(alpha) * (0.45 + 0.55*(0.5+0.5*math.Sin(2*math.Pi*phase))))
}

// StreamDot draws the sidebar's in-flight-completion indicator: an accent
// dot, centred in its slot, gently pulsing over two of the theme's slowest
// duration stops. It self-schedules the next frame only while visible, and
// renders static (a plain accent dot) when the theme's motion scale is the
// reduced one.
func StreamDot(gtx layout.Context, t themed, slot image.Point) {
	c := t.palette.Accent
	if phase, animate := motionPhase(gtx.Now, 2*t.motion.DurXSlow, 0); animate {
		c.A = dotPulse(c.A, phase)
		gtx.Execute(op.InvalidateCmd{})
	}
	d := gtx.Dp(StreamDotSize)
	defer op.Offset(image.Pt((slot.X-d)/2, (slot.Y-d)/2)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
}

// WaitingDots draws the chat pane's waiting indicator: three accent dots in
// the assistant bubble, pulsing in a wave that travels across them — the
// cycle is one DurXSlow stop per dot and each dot leads the next by one stop.
// It occupies exactly the body line box, so the row does not change height
// when the first delta replaces it with the answer.
//
// Under reduce-motion the theme's stops are zero, so the three dots draw once,
// solid and still, and no further frame is requested. The user still learns
// that the request is in flight; nothing moves.
func WaitingDots(gtx layout.Context, t themed) layout.Dimensions {
	st := t.typ.BodyLarge
	height := gtx.Sp(unit.Sp(st.LineHeight))
	if height <= 0 {
		height = gtx.Sp(unit.Sp(st.Size))
	}
	d, gap := gtx.Dp(StreamDotSize), gtx.Dp(WaitingDotGap)
	cycle := WaitingDotCount * t.motion.DurXSlow
	animate := false
	for i := range WaitingDotCount {
		c := t.palette.Accent
		if phase, ok := motionPhase(gtx.Now, cycle, time.Duration(i)*t.motion.DurXSlow); ok {
			c.A = dotPulse(c.A, phase)
			animate = true
		}
		stack := op.Offset(image.Pt(i*(d+gap), (height-d)/2)).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
		stack.Pop()
	}
	if animate {
		gtx.Execute(op.InvalidateCmd{})
	}
	return layout.Dimensions{Size: image.Pt(WaitingDotCount*d+(WaitingDotCount-1)*gap, height)}
}

var ChatGPT = func() []byte {
	// generate ivg data bytes on the fly for the logo.
	enc := &encode.Encoder{}
	// dlog := &ivg.DestinationLogger{Destination: enc}
	dlog := enc
	gen := &generate.Generator{Destination: dlog}
	// Palette that can be referenced from CReg array, gets overidden with colors from by externally set palette.
	pal := ivg.DefaultPalette
	pal[0] = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff} // white
	pal[1] = color.RGBA{A: 0xff}                            // black
	gen.Reset(ivg.ViewBox{MinX: 0, MinY: 0, MaxX: 2406, MaxY: 2406}, pal)
	gen.SetCReg(0, true, ivg.PaletteIndexColor(0)) // CReg[0] => palette[0] (white) selected via adj 2
	gen.SetCReg(0, true, ivg.PaletteIndexColor(1)) // CReg[1] => palette[1] (black) selected via adj 1
	// CSel will now be set to 2
	adj := byte(2)
	gen.SetPathData("M1107.3 299.1c-198 0-373.9 127.3-435.2 315.3C544.8 640.6 434.9 720.2 370.5 833c-99.3 171.4-76.6 386.9 56.4 533.8-41.1 123.1-27 257.7 38.6 369.2 98.7 172 297.3 260.2 491.6 219.2 86.1 97 209.8 152.3 339.6 151.8 198 0 373.9-127.3 435.3-315.3 127.5-26.3 237.2-105.9 301-218.5 99.9-171.4 77.2-386.9-55.8-533.9v-.6c41.1-123.1 27-257.8-38.6-369.8-98.7-171.4-297.3-259.6-491-218.6-86.6-96.8-210.5-151.8-340.3-151.2zm0 117.5-.6.6c79.7 0 156.3 27.5 217.6 78.4-2.5 1.2-7.4 4.3-11 6.1L952.8 709.3c-18.4 10.4-29.4 30-29.4 51.4V1248l-155.1-89.4V755.8c-.1-187.1 151.6-338.9 339-339.2zm434.2 141.9c121.6-.2 234 64.5 294.7 169.8 39.2 68.6 53.9 148.8 40.4 226.5-2.5-1.8-7.3-4.3-10.4-6.1l-360.4-208.2c-18.4-10.4-41-10.4-59.4 0L1024 984.2V805.4L1372.7 604c51.3-29.7 109.5-45.4 168.8-45.5zM650 743.5v427.9c0 21.4 11 40.4 29.4 51.4l421.7 243-155.7 90L597.2 1355c-162-93.8-217.4-300.9-123.8-462.8C513.1 823.6 575.5 771 650 743.5zm807.9 106 348.8 200.8c162.5 93.7 217.6 300.6 123.8 462.8l.6.6c-39.8 68.6-102.4 121.2-176.5 148.2v-428c0-21.4-11-41-29.4-51.4l-422.3-243.7 155-89.3zM1201.7 997l177.8 102.8v205.1l-177.8 102.8-177.8-102.8v-205.1L1201.7 997zm279.5 161.6 155.1 89.4v402.2c0 187.3-152 339.2-339 339.2v-.6c-79.1 0-156.3-27.6-217-78.4 2.5-1.2 8-4.3 11-6.1l360.4-207.5c18.4-10.4 30-30 29.4-51.4l.1-486.8zM1380 1421.9v178.8l-348.8 200.8c-162.5 93.1-369.6 38-463.4-123.7h.6c-39.8-68-54-148.8-40.5-226.5 2.5 1.8 7.4 4.3 10.4 6.1l360.4 208.2c18.4 10.4 41 10.4 59.4 0l421.9-243.7z", adj)
	icon, err := enc.Bytes()
	if err != nil {
		panic(err)
	}
	return icon
}()

// ClampWidth limits the layout.Context's min and max width to low and high.
// A min above the max is pulled down to it, and the current width is pulled
// into the resulting range.
func ClampWidth(gtx layout.Context, low, high unit.Dp) layout.Constraints {
	if gtx.Constraints.Min.X < gtx.Dp(low) {
		gtx.Constraints.Min.X = gtx.Dp(low)
	}
	if gtx.Constraints.Max.X > gtx.Dp(high) {
		gtx.Constraints.Max.X = gtx.Dp(high)
	}
	if gtx.Constraints.Min.X > gtx.Constraints.Max.X {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
	}
	return gtx.Constraints
}

func FillRect(gtx layout.Context, r image.Rectangle, radius int, c color.NRGBA) {
	if radius == 0 {
		paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
	} else {
		paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, radius).Op(gtx.Ops))
	}
}

func Material(ops *op.Ops, c color.NRGBA) op.CallOp {
	m := op.Record(ops)
	paint.ColorOp{Color: c}.Add(ops)
	return m.Stop()
}
