package main

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"sync/atomic"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/navbar"
	"github.com/vibrantgio/cadence/shell"
	"github.com/vibrantgio/ivg"
	"github.com/vibrantgio/ivg/encode"
	"github.com/vibrantgio/ivg/generate"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"

	openai "github.com/sashabaranov/go-openai"
)

// buildLayers returns the layer-builder the spectrum window renders: a
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
	avatar  layout.Widget
	remove  layout.Widget
	edit    layout.Widget
	add     layout.Widget
	gear    layout.Widget
}

// ContentLayer renders the page: the chat pane with the prompt field, and
// the conversation sidebar. The stateful widgets live at subscription scope,
// OUTSIDE the per-emission Map (llm.txt rule 2): the two scroll positions,
// the sidebar clickables, and the prompt TextField, whose editor state is
// Defer-scoped inside the component and subscribed exactly once by the
// CombineLatest3 below. Constructing any of them per emission would reset
// scroll or typing on every completion-stream delta.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	shaper := text.NewShaper(text.WithCollection(style.FontFaces()))

	histList := &layout.List{Axis: layout.Vertical, ScrollToEnd: true, Alignment: layout.Start}
	chatList := &layout.List{Axis: layout.Vertical, Alignment: layout.Start}
	rowClicks := map[string]*widget.Clickable{}
	deleteClicks := map[string]*widget.Clickable{}
	renameClicks := map[string]*widget.Clickable{}

	prompt := input.TextField(th, input.TextFieldProps{
		Placeholder:   "Send a message",
		Description:   "chat prompt",
		Submit:        true,
		SubmitMessage: func(text string) any { return Prompt{Content: text} },
		Shaper:        shaper,
	})

	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(t.Color, func(c tokens.ColorTokens) themed {
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
			return themed{palette: p, avatar: avatar, remove: remove, edit: edit, add: add, gear: gear}
		})
	})

	var newChatClick, settingsClick, undoClick widget.Clickable

	// The shell's Main and Navbar Brand are STATIC slots while the model and
	// theme are live streams, so the latest widgets are bridged through
	// atomic cells read at frame time (the observable-over-static-slot
	// hand-off from watchlist/app.go). Folding main and undo onto the
	// sidebar stream means every model change re-emits the sidebar, which
	// re-emits the Shell — a same-frame repaint.
	var themedCell, mainCell, undoCell atomic.Value

	type parts struct {
		sidebar, main, undo layout.Widget
	}
	combined := rx.Map(rx.CombineLatest3(themes, prompt, modelObs),
		func(next rx.Tuple3[themed, layout.Widget, Model]) parts {
			t, promptW, model := next.First, next.Second, next.Third
			themedCell.Store(t)
			return parts{
				sidebar: Sidebar(shaper, t, model.ChatList, model.CurrentChat.Name, chatList, rowClicks, deleteClicks, renameClicks, &newChatClick),
				main:    ChatPane(shaper, t, model.CurrentChat.History, histList, promptW),
				undo:    UndoBar(shaper, t, model.Pending, &undoClick),
			}
		})

	sidebarDriven := rx.Map(combined, func(p parts) layout.Widget {
		mainCell.Store(p.main)
		undoCell.Store(p.undo)
		return p.sidebar
	})

	mainSlot := func(gtx layout.Context) layout.Dimensions {
		if w, ok := mainCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	brand := func(gtx layout.Context) layout.Dimensions {
		t, ok := themedCell.Load().(themed)
		if !ok {
			return layout.Dimensions{}
		}
		return Brand(gtx, shaper, t, &settingsClick)
	}

	shellObs := shell.Shell(th, shell.Props{
		Layout:  shell.SidebarHeaderMain,
		Sidebar: sidebarDriven,
		Navbar:  navbar.Props{Brand: brand, Shaper: shaper},
		Main:    mainSlot,
	})

	renameObs := RenameModal(th, shaper, modelObs)

	// Overlays: the undo bar and the rename modal draw over the shell (the
	// modal last — its scrim covers everything, undo bar included).
	return rx.Map(rx.CombineLatest2(shellObs, renameObs),
		func(next rx.Tuple2[layout.Widget, layout.Widget]) layout.Widget {
			shellW, modalW := next.First, next.Second
			return func(gtx layout.Context) layout.Dimensions {
				dims := shellW(gtx)
				if w, ok := undoCell.Load().(layout.Widget); ok && w != nil {
					w(gtx)
				}
				if modalW != nil {
					modalW(gtx)
				}
				return dims
			}
		})
}

// renameTarget keys the rebuild of the rename modal's uncontrolled text
// field: a new epoch means a fresh field seeded with the target's name.
type renameTarget struct {
	epoch int
	seed  string // current name without extension
}

// RenameModal builds the rename-chat modal stream: a cadence/modal whose
// body is an epoch-rebuilt prism TextField plus a Rename button (the
// watchlist rename-modal recipe). Validation is the reducer's job — an
// invalid RenameChat is rejected and the modal stays open; a valid one (or
// an empty submit, or Escape/scrim via OnClose) closes it. Both model
// derivations are DistinctUntilChanged so completion-stream deltas cannot
// rebuild the field mid-typing.
func RenameModal(th rx.Observable[theme.Theme], shaper *text.Shaper, modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
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
	var nameCell atomic.Value
	nameCell.Store("")

	fieldObs := rx.SwitchMap(editObs, func(e renameTarget) rx.Observable[layout.Widget] {
		nameCell.Store(e.seed)
		placeholder := e.seed
		if placeholder == "" {
			placeholder = "Chat name"
		}
		return input.TextField(th, input.TextFieldProps{
			Placeholder:   placeholder,
			Description:   "chat name",
			Shaper:        shaper,
			Submit:        true,
			SubmitMessage: func(text string) any { return RenameChat{To: text} },
			OnChange:      func(_ layout.Context, text string) { nameCell.Store(text) },
		})
	})

	var submitClick widget.Clickable
	submitObs := button.Button(th, button.Props{
		Label:     "Rename",
		Clickable: &submitClick,
		Shaper:    shaper,
		OnClick: func(gtx layout.Context) {
			if name, ok := nameCell.Load().(string); ok {
				mvu.MessageOp{Message: RenameChat{To: name}}.Add(gtx.Ops)
			}
		},
	})

	// The modal body is a static slot; the live field/button widgets reach
	// it through cells (the observable-over-static-slot hand-off).
	var fieldCell, submitCell atomic.Value
	slot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}
	body := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		gap := gtx.Dp(12)
		fieldH := gtx.Dp(RenameFieldHeight)
		btnH := gtx.Dp(RenameButtonHeight)
		place := func(cell *atomic.Value, y, h int) {
			defer op.Offset(image.Pt(0, y)).Push(gtx.Ops).Pop()
			cg := gtx
			cg.Constraints = layout.Exact(image.Pt(w, h))
			slot(cell)(cg)
		}
		place(&fieldCell, 0, fieldH)
		place(&submitCell, fieldH+gap, btnH)
		return layout.Dimensions{Size: image.Pt(w, fieldH+gap+btnH)}
	}

	modalObs := modal.Modal(th, modal.Props{
		Open:   openObs,
		Title:  "Rename chat",
		Body:   body,
		Shaper: shaper,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseRename{}}.Add(gtx.Ops)
		},
	})

	// Fold the live field/button streams onto the modal stream so their
	// emissions repaint it.
	return rx.Map(rx.CombineLatest3(modalObs, fieldObs, submitObs),
		func(next rx.Tuple3[layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			fieldCell.Store(next.Second)
			submitCell.Store(next.Third)
			return next.First
		})
}

// Brand fills the navbar's leading slot: the settings button (the future
// OPENAI_API_KEY configuration surface) and the app title.
func Brand(gtx layout.Context, shaper *text.Shaper, t themed, settings *widget.Clickable) layout.Dimensions {
	for settings.Clicked(gtx) {
		mvu.MessageOp{Message: OpenSettings{}}.Add(gtx.Ops)
	}
	p := t.palette
	label := widget.Label{MaxLines: 1}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			iconSize := gtx.Dp(SettingsIconSize)
			gtx.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
			return settings.Layout(gtx, t.gear)
		}),
		layout.Rigid(layout.Spacer{Width: 12}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return label.Layout(gtx, shaper, style.H6.Font, style.H6.Size, "MindChat", Material(gtx.Ops, p.RowActive))
		}),
	)
}

// ChatPane stacks the scrolling message history above the prompt field.
func ChatPane(shaper *text.Shaper, t themed, chat []openai.ChatCompletionMessage, list *layout.List, prompt layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = ClampWidth(gtx, 0, ChatPaneWidth)

		layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return list.Layout(gtx, len(chat), func(gtx layout.Context, index int) layout.Dimensions {
					return MessageRow(gtx, shaper, t, chat[index])
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(8).Layout(gtx, prompt)
			}),
		)

		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// MessageRow renders one history entry: a full-width bubble with the text
// indented past the avatar column, and the assistant avatar on its rows.
func MessageRow(gtx layout.Context, shaper *text.Shaper, t themed, msg openai.ChatCompletionMessage) layout.Dimensions {
	p := t.palette
	st := style.BodyText1

	isUser := msg.Role == openai.ChatMessageRoleUser
	fill, textColor := p.BotBubble, p.BotText
	if isUser {
		fill, textColor = p.UserBubble, p.UserText
	}

	textMaterial := Material(gtx.Ops, textColor)
	label := widget.Label{Alignment: text.Start, MaxLines: st.MaxLines, Truncator: st.Truncator}

	m := op.Record(gtx.Ops)
	dims := layout.UniformInset(12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		margin := gtx.Dp(50)
		defer op.Offset(image.Pt(margin, 0)).Push(gtx.Ops).Pop()
		gtx.Constraints.Max.X -= margin
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		dims := label.Layout(gtx, shaper, st.Font, st.Size, msg.Content, textMaterial)
		dims.Size.X += margin
		return dims
	})
	foreground := m.Stop()

	FillRect(gtx, image.Rectangle{Max: dims.Size}, 0, fill)

	if !isUser {
		constraints := gtx.Constraints
		iconSize := gtx.Dp(AvatarSize)
		gtx.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
		t.avatar(gtx)
		gtx.Constraints = constraints
	}

	foreground.Add(gtx.Ops)
	return dims
}

// Sidebar renders the conversation list as the shell's full-height leading
// column: surface fill, a header with the new-chat button, and the rows.
func Sidebar(shaper *text.Shaper, t themed, chats ChatList, current string, list *layout.List, rowClicks, deleteClicks, renameClicks map[string]*widget.Clickable, newChat *widget.Clickable) layout.Widget {
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
		size := image.Pt(min(gtx.Dp(SidebarWidth), gtx.Constraints.Max.X), gtx.Constraints.Max.Y)
		gtx.Constraints = layout.Exact(size)
		FillRect(gtx, image.Rectangle{Max: size}, 0, t.palette.Sidebar)
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SidebarHeader(gtx, shaper, t, newChat)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return list.Layout(gtx, len(chats), func(gtx layout.Context, index int) layout.Dimensions {
					name := chats[index]
					return ChatRow(gtx, shaper, t, name, name == current, rowClicks[name], renameClicks[name], deleteClicks[name])
				})
			}),
		)
		return layout.Dimensions{Size: size}
	}
}

// SidebarHeader renders the "CONVERSATIONS" heading with the new-chat
// button at the top of the sidebar.
func SidebarHeader(gtx layout.Context, shaper *text.Shaper, t themed, newChat *widget.Clickable) layout.Dimensions {
	// Drain before Layout, like the rows.
	for newChat.Clicked(gtx) {
		mvu.MessageOp{Message: NewChat{}}.Add(gtx.Ops)
	}

	p := t.palette
	label := widget.Label{Alignment: text.Start, MaxLines: 1, Truncator: "…"}
	textMaterial := Material(gtx.Ops, p.Heading)

	m := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(10), Left: unit.Dp(16), Right: unit.Dp(12)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					dims := label.Layout(gtx, shaper, style.Caption.Font, style.Caption.Size, "CONVERSATIONS", textMaterial)
					dims.Size.X = gtx.Constraints.Max.X
					return dims
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					iconSize := gtx.Dp(AddIconSize)
					gtx.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
					return newChat.Layout(gtx, t.add)
				}),
			)
		},
	)
	foreground := m.Stop()

	// Bottom separator line
	sepRect := image.Rectangle{
		Min: image.Pt(gtx.Dp(12), dims.Size.Y-gtx.Dp(1)),
		Max: image.Pt(dims.Size.X-gtx.Dp(12), dims.Size.Y),
	}
	FillRect(gtx, sepRect, 0, p.Separator)
	foreground.Add(gtx.Ops)
	return dims
}

// UndoBar renders the transient bottom-centre undo affordance while a
// delete's undo window is open. It is fully model-driven: the bar exists
// exactly while model.Pending names a chat, and the reducer closes the
// window (UndoDelete or the ConfirmDelete timer).
func UndoBar(shaper *text.Shaper, t themed, pending PendingDelete, undo *widget.Clickable) layout.Widget {
	if pending.Name == "" {
		return func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	}
	p := t.palette
	display := strings.TrimSuffix(pending.Name, filepath.Ext(pending.Name))
	if len(display) > 0 {
		display = strings.ToUpper(display[:1]) + display[1:]
	}
	msg := "Deleted “" + display + "”"

	return func(gtx layout.Context) layout.Dimensions {
		for undo.Clicked(gtx) {
			mvu.MessageOp{Message: UndoDelete{}}.Add(gtx.Ops)
		}
		max := gtx.Constraints.Max
		label := widget.Label{MaxLines: 1}

		inner := gtx
		inner.Constraints = layout.Constraints{Max: max}
		m := op.Record(gtx.Ops)
		dims := layout.UniformInset(12).Layout(inner, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label.Layout(gtx, shaper, style.Subtitle2.Font, style.Subtitle2.Size, msg, Material(gtx.Ops, p.BotText))
				}),
				layout.Rigid(layout.Spacer{Width: 16}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return undo.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return label.Layout(gtx, shaper, style.Subtitle2.Font, style.Subtitle2.Size, "Undo", Material(gtx.Ops, p.Accent))
					})
				}),
			)
		})
		content := m.Stop()

		pos := image.Pt((max.X-dims.Size.X)/2, max.Y-dims.Size.Y-gtx.Dp(UndoBarMargin))
		defer op.Offset(pos).Push(gtx.Ops).Pop()
		FillRect(gtx, image.Rectangle{Max: dims.Size}, gtx.Dp(UndoBarRadius), p.RowSelected)
		content.Add(gtx.Ops)
		return layout.Dimensions{}
	}
}

// ChatRow renders a single chat entry in the sidebar with hover and
// selection states, and rename/delete icons revealed while the row is
// active.
func ChatRow(gtx layout.Context, shaper *text.Shaper, t themed, name string, selected bool, row, ren, del *widget.Clickable) layout.Dimensions {
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

	label := widget.Label{Alignment: text.Start, MaxLines: 1, Truncator: "…"}

	return row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		textMaterial := Material(gtx.Ops, textColor)

		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(11), Bottom: unit.Dp(11), Left: unit.Dp(20), Right: unit.Dp(12)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						dims := label.Layout(gtx, shaper, style.Subtitle2.Font, style.Subtitle2.Size, displayName, textMaterial)
						// Claim the full flex share so the icon sits at
						// the row's right edge, not after the text.
						dims.Size.X = gtx.Constraints.Max.X
						return dims
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

// ClampWidth will limit the min and max width of the layout.Context to the given
// values low and high. If the min width is greater than the max width, the min
// width will be set to the max width. If the min width is greater than the
// current width, the current width will be set to the min width. If the max
// width is less than the current width, the current width will be set to the
// max width.
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
