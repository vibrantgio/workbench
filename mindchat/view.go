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

	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/shell"
	"github.com/vibrantgio/ivg"
	"github.com/vibrantgio/ivg/encode"
	"github.com/vibrantgio/ivg/generate"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/a11y"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/list"
	"github.com/vibrantgio/prism/scrollbar"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/pulse/depth"
	"github.com/vibrantgio/style"
	"github.com/vibrantgio/textdraw"

	"slices"
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
	// reduceMotion mirrors the OS accessibility preference; the streaming
	// dot renders static when it is set (llms.txt rule 5).
	reduceMotion bool
}

// Chroma styles for the two appearance modes; built once, shared by every
// message. FromTokens leaves Highlight nil, so assigning these is the app's
// opt-in to syntax highlighting (the sitedocs recipe).
var (
	mdHighlightLight = highlight.New("github")
	mdHighlightDark  = highlight.New("github-dark")
)

// ContentLayer renders the page: the chat pane with the prompt field, and
// the conversation sidebar. The stateful widgets live at subscription scope,
// OUTSIDE the per-emission Map (llm.txt rule 2): the two scroll positions,
// the sidebar clickables, and the prompt TextField, whose editor state is
// Defer-scoped inside the component and subscribed exactly once by the
// CombineLatest3 below. Constructing any of them per emission would reset
// scroll or typing on every completion-stream delta.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	// The Roboto faces lead (the shaper's default), followed by the Go
	// collection so markdown code spans resolve their "Go Mono" typeface.
	shaper := text.NewShaper(text.WithCollection(append(style.FontFaces(), gofont.Collection()...)))

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
		Shaper:        shaper,
	})

	colorThemes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Type), func(ct rx.Tuple2[tokens.ColorTokens, tokens.TypeScale]) themed {
			c, ts := ct.First, ct.Second
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
			md := markdown.FromTokens(c, ts)
			if isDarkColor(c.Background) {
				md.Highlight = mdHighlightDark
			} else {
				md.Highlight = mdHighlightLight
			}
			md.Text.OnLinkClick = func(_ layout.Context, url string) { openURL(url) }
			return themed{palette: p, bar: scrollbar.FromTokens(c), avatar: avatar, remove: remove, edit: edit, add: add, gear: gear, md: md}
		})
	})
	themes := rx.Map(rx.CombineLatest2(colorThemes, a11y.Live(time.Second)),
		func(next rx.Tuple2[themed, a11y.A11yPrefs]) themed {
			t := next.First
			t.reduceMotion = next.Second.ReduceMotion
			return t
		})

	var newChatClick, settingsClick, toggleClick, undoClick widget.Clickable

	// The shell's Main and Navbar Brand are STATIC slots while the model and
	// theme are live streams, so the latest widgets are bridged through
	// atomic cells read at frame time (the observable-over-static-slot
	// hand-off from watchlist/app.go). Folding main and undo onto the
	// sidebar stream means every model change re-emits the sidebar, which
	// re-emits the Shell — a same-frame repaint.
	var mainCell, undoCell atomic.Value

	type parts struct {
		sidebar, main, undo layout.Widget
	}
	// The model-menu popover widget (the header chip + its surface) reaches
	// ChatPane's header slot through a cell; its stream joins the final
	// combine below so menu updates repaint.
	var menuCell atomic.Value
	menuSlot := func(gtx layout.Context) layout.Dimensions {
		if w, ok := menuCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	combined := rx.Map(rx.CombineLatest3(themes, prompt, modelObs),
		func(next rx.Tuple3[themed, layout.Widget, Model]) parts {
			t, promptW, model := next.First, next.Second, next.Third
			streaming := make(map[string]bool, len(model.Streams))
			for _, s := range model.Streams {
				streaming[s.Chat] = true
			}
			// While the current chat's exchange runs a server-side tool,
			// its status shows as a transient row under the history.
			history := model.CurrentChat.History
			if id, ok := model.StreamFor(model.CurrentChat.Name); ok {
				if status := model.Streams[id].Status; status != "" {
					history = append(slices.Clone(history), Message{Role: RoleStatus, Content: status})
				}
			}
			return parts{
				sidebar: Sidebar(shaper, t, model.ChatList, model.CurrentChat.Name, streaming, chatList, rowClicks, deleteClicks, renameClicks, &newChatClick, &toggleClick, &settingsClick),
				main:    ChatPane(shaper, t, msgDocs.Rows(history), histList, promptW, menuSlot),
				undo:    UndoBar(shaper, t, model.Pending, &undoClick),
			}
		})

	var sidebarCell atomic.Value
	partsObs := rx.Map(combined, func(p parts) int {
		sidebarCell.Store(p.sidebar)
		mainCell.Store(p.main)
		undoCell.Store(p.undo)
		return 0
	})

	slot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	// The split position follows the model: the [|] toggle and divider
	// drags reduce into SidebarRatio/SidebarCollapsed, and the pane's
	// minimum ratio doubles as the collapsed icon rail.
	ratioObs := rx.Map(modelObs, func(m Model) float32 { return m.EffectiveRatio() }).
		Pipe(rx.DistinctUntilChanged(func(a, b float32) bool { return a == b }))

	shellObs := shell.Shell(th, shell.Props{
		Layout:     shell.SplitPane,
		Left:       slot(&sidebarCell),
		Right:      slot(&mainCell),
		SplitRatio: ratioObs,
		OnSplitChange: func(gtx layout.Context, ratio float32) {
			mvu.MessageOp{Message: SetSidebarRatio{Ratio: ratio}}.Add(gtx.Ops)
		},
	})

	renameObs := RenameModal(th, shaper, modelObs)
	settingsObs := SettingsModal(th, shaper, modelObs)
	menuObs := ModelMenu(th, shaper, modelObs)

	// Global Cmd/Ctrl-Z undoes a pending chat delete (the reducer ignores
	// it when nothing is pending). A focused text editor claims the chord
	// first for its own text undo — correct layering, not a conflict.
	undoShortcut := OnShortcutKey("Z", func(gtx layout.Context) {
		mvu.MessageOp{Message: UndoDelete{}}.Add(gtx.Ops)
	})

	// Overlays: the undo bar and the modals draw over the shell (the
	// settings modal last — its scrim covers everything). partsObs joins
	// the combine so every model emission re-emits the top widget — the
	// same-frame repaint the sidebar stream used to provide; menuObs joins
	// so the header picker's chip and surface stay current.
	return rx.Map(rx.CombineLatest5(shellObs, renameObs, settingsObs, menuObs, partsObs),
		func(next rx.Tuple5[layout.Widget, layout.Widget, layout.Widget, layout.Widget, int]) layout.Widget {
			shellW, renameW, settingsW := next.First, next.Second, next.Third
			menuCell.Store(next.Fourth)
			return func(gtx layout.Context) layout.Dimensions {
				// Key area first, at the BOTTOM of the hit stack (the
				// todos convention) — it must never sit over the content.
				undoShortcut(gtx)
				dims := shellW(gtx)
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
	// fieldTagCell carries the current field instance's focus tag (new per
	// epoch) into the modal's Tab cycle.
	var nameCell, fieldTagCell atomic.Value
	nameCell.Store("")

	fieldObs := rx.SwitchMap(editObs, func(e renameTarget) rx.Observable[layout.Widget] {
		nameCell.Store(e.seed)
		return input.TextField(th, input.TextFieldProps{
			Placeholder:   "Chat name",
			Description:   "chat name",
			Seed:          e.seed,
			FocusTag:      func(tag event.Tag) { fieldTagCell.Store(tag) },
			Shaper:        shaper,
			Submit:        true,
			SubmitMessage: func(text string) any { return RenameChat{To: text} },
			OnChange:      func(_ layout.Context, text string) { nameCell.Store(text) },
		})
	})

	// Footer actions: an explicit Cancel (which is why the modal's close
	// button is hidden) and Rename. Their clickables join the Tab cycle via
	// ActionFocusTags.
	var cancelClick, submitClick widget.Clickable
	cancelObs := button.Button(th, button.Props{
		Label:     "Cancel",
		Clickable: &cancelClick,
		Shaper:    shaper,
		OnClick: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseRename{}}.Add(gtx.Ops)
		},
	})
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
	// prism text buttons fill their available width, so each footer action
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
		HideClose:       true,
		Shaper:          shaper,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseRename{}}.Add(gtx.Ops)
		},
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

// ChatPane stacks the header (with the model-picker chip), the scrolling
// message history, and the prompt field. The menu widget (the popover:
// chip anchor + model list surface) is drawn LAST, over the history, so
// the open surface wins the paint and hit-test order against the rows
// below the header.
func ChatPane(shaper *text.Shaper, t themed, chat []msgRow, hist *list.State, prompt, menu layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = ClampWidth(gtx, 0, ChatPaneWidth)
		size := gtx.Constraints.Max
		headerH := gtx.Dp(HeaderRowHeight)

		layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// The chip itself is overlaid below; the header row only
				// reserves the space and draws its separator.
				sep := image.Rectangle{
					Min: image.Pt(gtx.Dp(12), headerH-gtx.Dp(1)),
					Max: image.Pt(gtx.Constraints.Max.X-gtx.Dp(12), headerH),
				}
				FillRect(gtx, sep, 0, t.palette.Separator)
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, headerH)}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return list.LayoutScrollbar(gtx, hist, t.bar, list.Occupy, chat,
					func(gtx layout.Context, row msgRow) layout.Dimensions {
						return MessageRow(gtx, shaper, t, row)
					})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(8).Layout(gtx, prompt)
			}),
		)

		// The popover gets an Exact chip-sized box in the header's right
		// corner; it centres its anchor (the chip) in that box and hangs
		// the surface below it.
		chipW, chipH := gtx.Dp(ChipWidth), gtx.Dp(ChipHeight)
		defer op.Offset(image.Pt(size.X-chipW-gtx.Dp(24), (headerH-chipH)/2)).Push(gtx.Ops).Pop()
		mg := gtx
		mg.Constraints = layout.Exact(image.Pt(chipW, chipH))
		menu(mg)

		return layout.Dimensions{Size: size}
	}
}

// MessageRow renders one history entry: a full-width bubble with the body
// indented past the avatar column, and the assistant avatar on its (and
// error notices') rows. User and assistant bodies lay out their markdown
// Document — inline styles and code fences, links live — in the bubble's
// text colours; error rows read as plain labels in the error colour,
// transient status rows ("Searching the web…") in the heading colour. An
// answer's citations arrive inside the Document (messageSource).
func MessageRow(gtx layout.Context, shaper *text.Shaper, t themed, row msgRow) layout.Dimensions {
	msg := row.Msg
	p := t.palette
	st := style.BodyText1

	isUser := msg.Role == RoleUser
	fill, textColor := p.BotBubble, p.BotText
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
		if row.Doc != nil {
			md := t.md
			md.Text.Color = textColor
			if isUser {
				// The token link colour (Primary) would vanish on the
				// Primary user bubble; the underline still marks links.
				md.Text.LinkColor = textColor
			}
			gtx.Constraints.Min = image.Point{}
			dims = row.Doc.LayoutColumn(gtx, shaper, md)
			// The bubble spans the full row width regardless of the
			// column's natural content width.
			dims.Size.X = gtx.Constraints.Max.X
		} else {
			textMaterial := Material(gtx.Ops, textColor)
			label := widget.Label{Alignment: text.Start, MaxLines: st.MaxLines, Truncator: st.Truncator}
			dims = label.Layout(gtx, shaper, st.Font, st.Size, msg.Content, textMaterial)
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

// Sidebar renders the shell's left pane: the brand row with the collapse
// toggle, the conversation list, and the settings row anchored at the
// bottom. Below RailThreshold width it renders as an icon rail (toggle,
// new chat, settings) — the collapsed state the [|] toggle drives.
func Sidebar(shaper *text.Shaper, t themed, chats ChatList, current string, streaming map[string]bool, rows *list.State, rowClicks, deleteClicks, renameClicks map[string]*widget.Clickable, newChat, toggle, settings *widget.Clickable) layout.Widget {
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
		FillRect(gtx, image.Rectangle{Max: size}, 0, t.palette.Sidebar)

		if size.X < gtx.Dp(RailThresholdWidth) {
			SidebarRail(gtx, t, toggle, newChat, settings)
			return layout.Dimensions{Size: size}
		}

		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SidebarBrand(gtx, shaper, t, toggle)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SidebarHeader(gtx, shaper, t, newChat)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return list.LayoutScrollbar(gtx, rows, t.bar, list.Overlay, chats,
					func(gtx layout.Context, name string) layout.Dimensions {
						return ChatRow(gtx, shaper, t, name, name == current, streaming[name], rowClicks[name], renameClicks[name], deleteClicks[name])
					})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SidebarFooter(gtx, shaper, t, settings)
			}),
		)
		return layout.Dimensions{Size: size}
	}
}

// SidebarBrand is the sidebar's top row: the app title and the [|]
// collapse toggle sharing one vertical centre on the 16dp gutter.
func SidebarBrand(gtx layout.Context, shaper *text.Shaper, t themed, toggle *widget.Clickable) layout.Dimensions {
	for toggle.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	p := t.palette
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(BrandRowHeight))
	left, right := gtx.Dp(16), gtx.Dp(12)
	iconSz := gtx.Dp(ToggleIconSize)

	titleRect := image.Rect(left, 0, size.X-right-iconSz-gtx.Dp(8), size.Y)
	textdraw.FillText(gtx, shaper, style.Subtitle1, titleRect, 0, 0.5, p.RowActive, "MindChat")

	defer op.Offset(image.Pt(size.X-right-iconSz, (size.Y-iconSz)/2)).Push(gtx.Ops).Pop()
	icon := gtx
	icon.Constraints = layout.Exact(image.Pt(iconSz, iconSz))
	IconButton(icon, toggle, ToggleIconSize, func(gtx layout.Context, sz int) {
		PanelGlyph(gtx, sz, p.Heading)
	})
	return layout.Dimensions{Size: size}
}

// SidebarFooter anchors the settings affordance bottom-left: a hairline
// separator over a full-width hoverable row, gear and label sharing one
// vertical centre on the 16dp gutter.
func SidebarFooter(gtx layout.Context, shaper *text.Shaper, t themed, settings *widget.Clickable) layout.Dimensions {
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
		textdraw.FillText(gtx, shaper, style.Subtitle2, labelRect, 0, 0.5, textColor, "Settings")
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	return layout.Dimensions{Size: image.Pt(width, rowH+sep)}
}

// SidebarRail is the collapsed sidebar: the toggle on top, new chat below
// it, settings pinned at the bottom — icons only, centred in the rail.
func SidebarRail(gtx layout.Context, t themed, toggle, newChat, settings *widget.Clickable) layout.Dimensions {
	for toggle.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	for newChat.Clicked(gtx) {
		mvu.MessageOp{Message: NewChat{}}.Add(gtx.Ops)
	}
	for settings.Clicked(gtx) {
		mvu.MessageOp{Message: OpenSettings{}}.Add(gtx.Ops)
	}
	p := t.palette
	size := gtx.Constraints.Max
	rail := func(gtx layout.Context, click *widget.Clickable, draw func(layout.Context, int)) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return IconButton(gtx, click, ToggleIconSize, draw)
				})
			})
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return rail(gtx, toggle, func(gtx layout.Context, sz int) { PanelGlyph(gtx, sz, p.Heading) })
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return rail(gtx, newChat, func(gtx layout.Context, sz int) {
				icon := gtx
				icon.Constraints = layout.Exact(image.Pt(sz, sz))
				t.add(icon)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return rail(gtx, settings, func(gtx layout.Context, sz int) {
				icon := gtx
				icon.Constraints = layout.Exact(image.Pt(sz, sz))
				t.gear(icon)
			})
		}),
	)
	return layout.Dimensions{Size: size}
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
// cadence convention for chrome glyphs): a rounded outline with a divider
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
	hint := "(Ctrl+Z)"
	if runtime.GOOS == "darwin" {
		hint = "(⌘Z)"
	}

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
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return label.Layout(gtx, shaper, style.Caption.Font, style.Caption.Size, hint, Material(gtx.Ops, p.Row))
				}),
			)
		})
		content := m.Stop()

		pos := image.Pt((max.X-dims.Size.X)/2, max.Y-dims.Size.Y-gtx.Dp(UndoBarMargin))
		defer op.Offset(pos).Push(gtx.Ops).Pop()
		// The cadence toast treatment: a cast shadow under an accent-tinted
		// fill ringed in the accent, so the bar separates from the chat
		// surfaces it floats over (RowSelected alone sat at ~1.2:1 against
		// them, and ~1:1 against bot bubbles in dark mode).
		bounds := image.Rectangle{Max: dims.Size}
		depth.Shadow(gtx, bounds, tokens.Level3)
		radius := gtx.Dp(UndoBarRadius)
		FillRect(gtx, bounds, radius, Blend(p.RowSelected, p.Accent, 0x33))
		ring := clip.RRect{Rect: bounds, SE: radius, SW: radius, NE: radius, NW: radius}
		paint.FillShape(gtx.Ops, p.Accent, clip.Stroke{Path: ring.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
		content.Add(gtx.Ops)
		return layout.Dimensions{}
	}
}

// ChatRow renders a single chat entry in the sidebar with hover and
// selection states, and rename/delete icons revealed while the row is
// active.
func ChatRow(gtx layout.Context, shaper *text.Shaper, t themed, name string, selected, streaming bool, row, ren, del *widget.Clickable) layout.Dimensions {
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

// StreamDot draws the in-flight-completion indicator: an accent dot,
// centred in its slot, gently pulsing. Animation follows llms.txt rule 5 —
// it self-schedules the next frame only while visible, and renders static
// when the OS asks for reduced motion.
func StreamDot(gtx layout.Context, t themed, slot image.Point) {
	c := t.palette.Accent
	if !t.reduceMotion {
		const period = 1200
		phase := float64(gtx.Now.UnixMilli()%period) / period
		pulse := 0.45 + 0.55*(0.5+0.5*math.Sin(2*math.Pi*phase))
		c.A = uint8(float64(c.A) * pulse)
		gtx.Execute(op.InvalidateCmd{})
	}
	d := gtx.Dp(StreamDotSize)
	defer op.Offset(image.Pt((slot.X-d)/2, (slot.Y-d)/2)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
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
