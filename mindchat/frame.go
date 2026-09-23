// frame.go is this window's composition: the conversation panel set in down
// the leading edge, and beside it the content area — one chrome row across
// its top and the transcript with its input bar underneath.
//
// THE PANE IS SET INTO THE WINDOW, NOT A HALF OF IT. It is the vocabulary's
// PANE: an inset rounded panel one margin in from the window's leading, top
// and bottom edges with the window's own plane showing in those margins,
// flush against the transcript on the fourth side, bounded by its own rim
// and the shadow it casts and by no seam. Hidden, it takes no width at all
// and the transcript reflows from the window's own leading edge. None of
// that geometry is drawn here — the inset, the rim, the shadow, the strip
// arithmetic and the hidden-takes-no-width contract are patterns/pane's,
// and what is left to this file is the column that stands in the panel and
// the window that stands around it.
//
// THE CONTROLS OBEY THE RECALL CONVENTION. A control that travels with the
// pane cannot be the one that recalls it. The pane's toggle and the
// new-chat action ride the pane's top strip while the pane stands; once it
// is away the chrome row carries the same two figures, at the same mark
// size, on the same line. They are the two halves of one switch rather
// than duplicates of one control; what differs is what each figure stands
// in, which is a property of the place and not of the switch — bare on the
// panel, in the platform's bordered toolbar control in the band. New chat is this application's
// primary action and can be operated in both states for that reason, and
// Cmd-N operates it in either.
//
// Settings does not stand in that pair. It sits at the foot of the pane and
// on Cmd-comma: it acts on the application rather than on the conversation,
// and it earns no standing place in a window whose pane is away.
//
// THE WINDOW BUTTONS ARE MEASURED FROM THE GLASS. The three control
// buttons stand a fixed inset in from the window's own top and leading
// edges and stay there whatever the application draws beneath them. The
// pane happens to stand under them while it is up, so its strip is cut
// deep enough to hold them; when the pane goes, nothing about them
// changes. That fixed line is what both halves of the sidebar switch stand
// on: [ChromeRowHeight] is twice it, so a row that centres its content
// centres it exactly where the pane's strip does, and no mark changes step
// in either direction.
//
// THE CHROME ROW IS A TITLE ROW. The current conversation's name stands at
// its leading end in both pane states — the window's one orientation cue
// once the pane is away — with the model picker at its trailing end. A
// chat that has not earned a name shows a muted placeholder rather than a
// filename. The row carries no section label: the strip's line is for
// controls and identities, and a category is neither.
//
// Under the full-size-content treatment the native title bar hands over no
// window drag, so the chrome row and the pane's strip claim it back over
// the parts of themselves that hold no control.

package main

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	gotext "gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/typeset"
)

const (
	// chromeInsetDp is where the chrome row's own content begins while the
	// pane stands: the transcript's row inset, so the conversation's title
	// stands over the first glyphs of the messages under it rather than on a
	// grid of its own.
	chromeInsetDp unit.Dp = 12

	// chromeGapDp is the air between two things standing in the chrome row
	// or the pane's strip — including the air the row owes the window's
	// control buttons, whose reported trailing edge is bare glass and
	// carries no breathing room of its own.
	chromeGapDp unit.Dp = 12

	// controlGapDp is the air between the two halves of the sidebar switch's
	// line: the toggle and new chat stand side by side, each in the
	// platform's bordered toolbar control, and this is the room the platform
	// leaves between two such controls standing apart.
	//
	// MEASURED at 1x: notes-toolbar.png leaves 14 px between its compose
	// capsule (x 8–44) and the group beside it (from x=58), and
	// finder-window-light.png 16 between its view pop-up (to x=742) and the
	// group pull-down (from x=759). Fourteen is the closer of the two, which
	// is what a pair belonging together takes.
	controlGapDp unit.Dp = 14

	// paneMarkDp is the box a BARE mark on the sidebar panel is drawn in:
	// the same 24 dp box the bordered toolbar control centres its own mark
	// in, so one figure is one size wherever it stands.
	paneMarkDp unit.Dp = 24

	// titleMaxDp is the widest the conversation's title may run before it
	// is truncated: far enough that a real name fits whole, near enough
	// that no name can crowd the picker at the row's other end.
	titleMaxDp unit.Dp = 360
)

// windowFrame is the window's per-subscription state: the clickables of the
// controls that stand in the chrome row, kept apart from the pane's own
// (a control and its recalling half are two clickables, not one shared one).
type windowFrame struct {
	rowToggle  widget.Clickable
	rowNewChat widget.Clickable
}

// layout composes the window: the pane, then the content area's chrome row
// and the transcript under it.
//
// The order is the reading order and so the focus ring's: the pane's column
// first, then the row above the transcript, then the transcript itself. The
// pane draws its own strip last for the same reason, inside itself.
func (f *windowFrame) layout(gtx layout.Context, m Model, t themed, sidebar, main, menu layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	bounds := pane.Bounds(gtx, size, SidebarWidth, m.SidebarHidden)
	contentX := 0
	if !bounds.Empty() {
		contentX = bounds.Max.X
	}
	// The window's own plane, under everything, and the content area on the
	// transcript's own surface. The rail's panel is set into the plane and
	// the plane is what shows in the margins around it: MEASURED,
	// voicememos-multi-folder-2026-09-18.png and
	// finder-window-untinted-light.png, the eight pixels either side of the
	// panel carry the window background and nothing else.
	FillRect(gtx, image.Rectangle{Max: size}, 0, t.col.WindowBackground)
	FillRect(gtx, image.Rect(contentX, 0, size.X, size.Y), 0, t.palette.Transcript)

	// The inset rounded panel, its rim, the shadow it casts, the chrome fill
	// and the clip that keeps a scrolled row off its edge are all the
	// pattern's. What is left here is which column stands in it, and what
	// stands behind the two corners the panel rounds away from on its flush
	// side — the transcript, not the plane.
	pane.FillTrailingCorners(gtx, t.palette.Transcript, bounds)
	pane.Layout(gtx, t.col, bounds, sidebar)

	contentW := size.X - contentX
	if contentW <= 0 {
		return layout.Dimensions{Size: size}
	}

	rowH := min(gtx.Dp(ChromeRowHeight), size.Y)
	if main != nil && size.Y-rowH > 0 {
		st := op.Offset(image.Pt(contentX, rowH)).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(contentW, size.Y-rowH))
		main(mgtx)
		st.Pop()
	}

	// The row after the transcript, not before it. What a bordered toolbar
	// control casts on its band reaches past the row's own foot — MEASURED,
	// finder-window-light.png, where the shadow under a control is still
	// darkening the document 38 rows down — and a transcript drawn over it
	// cuts it off at the row's edge with a ruled line, which is the one thing
	// a cast shadow may not have. The pane's own strip already stands last in
	// its column for the same reason. Nothing moves in the hit test: the row's
	// pointer areas lie inside the row, and the transcript's inside the
	// transcript.
	if rowH > 0 {
		st := op.Offset(image.Pt(contentX, 0)).Push(gtx.Ops)
		rgtx := gtx
		rgtx.Constraints = layout.Exact(image.Pt(contentW, rowH))
		f.chromeRow(rgtx, m, t)
		st.Pop()
	}

	// Last, into the cap the row reserved: the picker, whose surface hangs
	// over the transcript when it is open.
	layoutPicker(gtx, menu, contentX, contentW, rowH)

	// The shadow the panel casts, after every column has painted its own
	// surface: the transcript paints its own rows after the pane has laid
	// out — the pane comes first because it comes first in the reading order
	// — and would cover the ramp the panel cast on it. The pattern cuts the
	// panel's own box out of the drawing, so painting it here lands what
	// painting it under the panel landed.
	pane.PaintShadow(gtx, t.col, bounds)

	return layout.Dimensions{Size: size}
}

// chromeRow lays out the content area's title row: the conversation's name
// leading and the model picker's reserved cap trailing, with the two halves
// of the sidebar switch standing before the name while the pane is away.
//
// The leading inset is a measurement in exactly one state. With the pane
// standing, the window's buttons are inside the pane and the row owes them
// nothing, so it starts at the transcript's own inset; with the pane away
// the whole top strip is the row's and it starts past the buttons' reported
// trailing edge, plus the air that edge does not carry.
//
// Every child stands on the row's own middle, which is the window buttons'
// centre line by [ChromeRowHeight]'s arithmetic — so what the row shows
// while the pane is away is level with what the pane's strip showed before
// it went.
func (f *windowFrame) chromeRow(gtx layout.Context, m Model, t themed) layout.Dimensions {
	lead := chromeLead(m.SidebarHidden, windowButtonsEnd())
	children := make([]layout.FlexChild, 0, 8)
	children = append(children, layout.Rigid(dragSpacer(lead)))
	if m.SidebarHidden {
		children = append(children,
			layout.Rigid(f.toggleControl(t)),
			layout.Rigid(dragSpacer(controlGapDp)),
			layout.Rigid(f.newChatControl(t)),
			layout.Rigid(dragSpacer(chromeGapDp)))
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return chatTitle(gtx, m, t)
		}),
		layout.Flexed(1, dragFill),
		// The picker's cap is RESERVED here and drawn afterwards, over the
		// transcript — a popover's surface hangs below its anchor and has
		// to win the paint and the hit test against everything it hangs
		// over, so the one thing in this row that opens is the one thing
		// laid out after the document. What the row spends on it is the
		// space, which is why the reservation stands in the flex: the drag
		// the row claims across its middle must stop before the control, since
		// a move action swallows the press before any control beneath it
		// sees one.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Dp(ToolbarWidth), gtx.Constraints.Max.Y)}
		}),
		layout.Rigid(dragSpacer(chromeInsetDp)))
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// layoutPicker draws the model picker into the chrome row, and what it hands
// it is the WHOLE row inside the content area's insets rather than a box cut
// to the control. That box is patterns/popover's statement of the room the
// open menu may use: the surface hangs off the anchor and the popover keeps
// it inside what it was given, so a box cut to the control would leave the
// menu running off the window's trailing edge with nothing to clamp against.
// Where the control stands in that room is the popover's trailing alignment
// (modelmenu.go), which is why it lands on the content column's edge and not
// a few pixels inboard of it whatever the model is called.
func layoutPicker(gtx layout.Context, menu layout.Widget, contentX, contentW, rowH int) {
	if menu == nil || rowH <= 0 {
		return
	}
	inset := gtx.Dp(chromeInsetDp)
	room := contentW - 2*inset
	if room <= 0 {
		return
	}
	defer op.Offset(image.Pt(contentX+inset, 0)).Push(gtx.Ops).Pop()
	mgtx := gtx
	mgtx.Constraints = layout.Constraints{Max: image.Pt(room, rowH)}
	menu(mgtx)
}

// chromeLead is where the chrome row's own content begins, in dp in from
// the leading edge of the content area.
//
// It is a measurement in exactly one state. With the pane standing, the
// window's control buttons are inside the pane and the row owes them
// nothing: it starts at the transcript's own inset, so the conversation's
// title stands over the first glyphs of the messages under it. With the pane
// away the whole top strip is the row's, and the row starts past the
// buttons' reported trailing edge plus the air that bare edge does not
// carry. Where the platform draws no such buttons the measurement is zero
// and the row falls back to the same inset it uses beside the pane.
func chromeLead(hidden bool, buttonsEnd unit.Dp) unit.Dp {
	if !hidden {
		return chromeInsetDp
	}
	return stripLead(buttonsEnd)
}

// stripLead is where a band standing over the window's control buttons may
// begin: their reported trailing edge plus the air the platform leaves after
// them, and the row's own inset where the window has no such buttons.
//
// Both of this window's bands lead from it — the pane's strip while the pane
// stands, the chrome row once it is away — so the two halves of each switch
// stand in one window column whichever way the pane goes.
func stripLead(buttonsEnd unit.Dp) unit.Dp {
	return desktop.BandLeadFrom(buttonsEnd, pane.ButtonGapDp, chromeInsetDp)
}

// chatTitle draws the conversation the window is showing, which is the
// window's one orientation cue once the pane is away. A chat that has not
// earned a name yet shows a muted placeholder rather than the filename it
// is stored under: the row says what is open, and "new.jsonl" is not that.
func chatTitle(gtx layout.Context, m Model, t themed) layout.Dimensions {
	text, verdict := chatTitleText(m.CurrentChat.Name)
	// The chrome row stands over the transcript, so its title is the
	// platform's label over that content. The sidebar's selected-row
	// foreground is white, and on the light content it paints nothing.
	colour := t.palette.TurnText
	if verdict == titleMuted {
		colour = t.palette.Note
	}
	semantic.LabelOp(text).Add(gtx.Ops)
	st := t.typ.TitleSmall
	label := roleLabel(st, 1)
	label.Alignment, label.Truncator = gotext.Start, "…"
	// The title takes its own width and no more, so the drag between it and
	// the picker is as long as the name is short — capped, so that a long
	// name runs out of room before it runs into the chip.
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(titleMaxDp))
	if gtx.Constraints.Max.X <= 0 {
		return layout.Dimensions{}
	}
	return typeset.Layout(gtx, t.shaper, label, roleFont(st), unit.Sp(st.Size), text, Material(gtx.Ops, colour))
}

// titleVerdict distinguishes a chat that has a name from one that does not, so
// the placeholder reads as an absence rather than as a title.
type titleVerdict int

const (
	titleNamed titleVerdict = iota
	titleMuted
)

// chatTitleText is the chrome row's label for a chat file name, and the
// verdict on whether it is a name at all. The untitled chats are the ones
// the application itself named — new.jsonl and its numbered siblings — and
// they show the placeholder until the conversation earns something better.
func chatTitleText(name string) (string, titleVerdict) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if base == "" || base == "new" || strings.HasPrefix(base, "new-") {
		return "Untitled chat", titleMuted
	}
	return strings.ToUpper(base[:1]) + base[1:], titleNamed
}

// toggleControl is the chrome row's half of the sidebar switch: the control
// that brings the pane back, standing only while it is away. It wears the
// figure the pane's own control wears, at the same size and on the same
// line, because the two are one switch.
func (f *windowFrame) toggleControl(t themed) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		// The pane is away wherever this half of the switch stands, which is
		// the state the control draws: off.
		return sidebarToggle(gtx, t, &f.rowToggle, "Show the conversations", false)
	}
}

// newChatControl is the chrome row's half of the new-chat action: this
// application's primary action, which is why it survives the pane going
// away rather than travelling with it.
func (f *windowFrame) newChatControl(t themed) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return newChatMark(gtx, t, &f.rowNewChat)
	}
}

// sidebarToggle draws one half of the sidebar switch: the design system's own
// sidebar mark in the platform's bordered toolbar control, centred on the line
// of the row it stands in.
//
// The mark comes from components/icons under the name of the control it
// belongs to, so a Mac user sees the pane and list lines that platform draws
// and every other platform sees the neutral pane, from one name here. Both
// halves of the switch come through this function, so the name is asked for
// once and the two stand as one figure.
//
// The figure never morphs. What the control is about to do is in the label
// it carries, which the screen reader speaks; a mark that changed with the
// state would leave a reader guessing whether it shows the present state or
// the next one, and the platform does not change this one either.
//
// What DOES say which way the switch stands is the control around the
// figure: standing is the pane's own state, and a control that records a
// state draws it as the platform draws the chosen segment of a segmented
// control — a lighter patch inside the control's own box. So the reader sees
// that the pane is on without the figure having to change into a second
// drawing.
func sidebarToggle(gtx layout.Context, t themed, click *widget.Clickable, label string, standing bool) layout.Dimensions {
	for click.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	return controlBox(gtx, t, click, label, icons.Mark(icons.Sidebar), standing)
}

// newChatMark draws one half of the new-chat action: the same plus figure
// in the same square, wherever it stands.
func newChatMark(gtx layout.Context, t themed, click *widget.Clickable) layout.Dimensions {
	for click.Clicked(gtx) {
		mvu.MessageOp{Message: NewChat{}}.Add(gtx.Ops)
	}
	// New chat adds something; it records nothing, so it is never drawn on.
	return controlBox(gtx, t, click, "New chat", icons.Mark(icons.Plus), false)
}

// paneMark stands one chrome mark BARE — the figure alone, at the same box
// [controlBox] centres its figure in, in the platform's control text over
// the fill it stands on. It is what the sidebar panel's own controls are
// drawn as: the panel is not a band, and MEASURED,
// voicememos-multi-folder-2026-09-18.png, the two marks in a sidebar
// panel's top trailing corner carry no capsule, no fill and no rim while
// every mark in the band beside them does.
//
// The foreground is the same name [controlBox] reads its mark in, so the two
// halves of one switch are one figure in one colour whichever side of the
// window they stand on: what a toolbar draws its own glyphs in. The panel's
// half records no state: it stands only while the panel does, and a mark with
// nothing around it has nowhere to draw the chosen patch.
func paneMark(gtx layout.Context, t themed, click *widget.Clickable, name icons.Name, label string, msg any) layout.Dimensions {
	for click.Clicked(gtx) {
		mvu.MessageOp{Message: msg}.Add(gtx.Ops)
	}
	box := gtx.Dp(paneMarkDp)
	fg := t.col.ToolbarLabel
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.ClassOp(semantic.Button).Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointershape.OverSize(gtx.Ops, image.Pt(box, box), pointer.CursorPointer)
		icons.Mark(name)(gtx, box, fg)
		return layout.Dimensions{Size: image.Pt(box, box)}
	})
}

// paneToggle and paneNewChat are the panel's two halves, drawn bare through
// [paneMark]. They are named here rather than called through with an icon
// name because the sidebar's own file draws the panel and reaches for a
// different icons package.
func paneToggle(gtx layout.Context, t themed, click *widget.Clickable, label string) layout.Dimensions {
	return paneMark(gtx, t, click, icons.Sidebar, label, ToggleSidebar{})
}

func paneNewChat(gtx layout.Context, t themed, click *widget.Clickable) layout.Dimensions {
	return paneMark(gtx, t, click, icons.Plus, "New chat", NewChat{})
}

// controlBox stands one chrome mark in the platform's BORDERED TOOLBAR
// CONTROL: a capsule at the toolbar control's measured height with the mark
// centred in it, its fill, its rim where the platform draws one, and the drop
// shadow it casts on the band it stands on. components/button's chrome
// variant is the control, drawn through the same internal seam the picker's
// chrome trigger is drawn through, so every symbol standing in this window's
// chrome is one control and not a bare figure on a band.
//
// It is the CHROME ROW's drawing. The panel's own marks stand bare through
// [paneMark]: which of the two a mark takes is a property of what it stands
// on, not of the switch it belongs to.
func controlBox(gtx layout.Context, t themed, click *widget.Clickable, label string, mark func(gtx layout.Context, sizePx int, col color.NRGBA), on bool) layout.Dimensions {
	state := button.RenderState{
		Hovered: click.Hovered(),
		Pressed: click.Pressed(),
		Focused: gtx.Focused(click),
		Checked: on,
	}
	face := button.ChromeFace(mark, t.col, t.den, state)
	// The shadow is cast AROUND the clickable rather than inside it: it falls
	// outside the control's own box, and a clickable clips what it wraps to
	// the box its layout.Widget reports.
	return button.ChromeShadow(gtx, t.col, state, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.ClassOp(semantic.Button).Add(gtx.Ops)
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			return face(gtx)
		})
	})
}

// dragSpacer is a fixed-width gap that moves the window when it is dragged.
// A row standing in the strip the native title bar would otherwise own says
// where the window may be picked up, and it says so over its empty space
// alone — a move action swallows the press before any control beneath it
// sees one.
func dragSpacer(w unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return desktop.DragRun(gtx, gtx.Dp(w))
	}
}

// dragFill is a row's flexible middle: everything between what stands at
// its two ends, draggable end to end.
func dragFill(gtx layout.Context) layout.Dimensions {
	return desktop.DragRun(gtx, gtx.Constraints.Min.X)
}
