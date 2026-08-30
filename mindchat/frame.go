// frame.go is this window's composition: the conversation pane floating
// down the leading edge, and beside it the content area — one chrome row
// across its top and the transcript with its input bar underneath.
//
// THE PANE IS AN OBJECT, NOT A HALF OF THE WINDOW. What stood here before
// was a split: two panes of a window sharing its width by a ratio, with a
// draggable seam between them, and a collapsed state that shrank the
// leading half to a narrow rail rather than letting it go. The rail was
// the defect. A region whose only job is to store the controls a collapse
// displaced invents a rhythm of its own — the sidebar toggle dropped to a
// lower rung as the pane closed, so the control just clicked jumped out
// from under the pointer, and a hairline appeared over a gear that nothing
// on the content side answered.
//
// So the sidebar joins the vocabulary's FLOATING PANE: inset from the
// window's leading, top and bottom edges by one margin, rounded on all
// four corners, carrying its own hairline just inside that edge, with the
// window's ground showing around it. Hidden, it takes no width at all and
// the transcript reflows from the window's own leading edge. None of that
// geometry is drawn here — the float, the outline, the strip arithmetic
// and the hidden-takes-no-width contract are patterns/pane's, and what is
// left to this file is the column that stands in the pane and the window
// that stands around it.
//
// THE CONTROLS OBEY THE RECALL CONVENTION. A control that travels with the
// pane cannot be the one that recalls it. The pane's toggle and the
// new-chat action ride the pane's top strip while the pane stands; once it
// is away the chrome row carries the same two figures, at the same mark
// size, on the same line. They are the two halves of one switch rather
// than duplicates of one control. New chat is this application's primary
// action and is reachable in both states for that reason, and Cmd-N
// reaches it in either.
//
// Settings does not stand in that pair. It retreats to the foot of the
// pane and to Cmd-comma: it acts on the application rather than on the
// conversation, and it earns no standing place in a window whose pane is
// away.
//
// THE WINDOW BUTTONS ARE MEASURED FROM THE GLASS. The three control
// buttons stand a fixed inset in from the window's own top and leading
// edges and stay there whatever the application draws beneath them. The
// pane happens to float under them while it stands, so its strip is cut
// deep enough to hold them; when the pane goes, nothing about them
// changes. That fixed line is what both halves of the sidebar switch stand
// on: [ChromeRowHeight] is twice it, so a row that centres its content
// centres it exactly where the pane's strip does, and no mark changes rung
// in either direction.
//
// THE CHROME ROW IS A TITLE ROW. The current conversation's name stands at
// its leading end in both pane states — the window's one orientation cue
// once the pane is away — with the model picker at its trailing end. A
// chat that has not earned a name shows a muted placeholder rather than a
// filename. The row carries no section label: the strip's line is for
// controls and identities, and a category is neither, which is why the
// CONVERSATIONS heading retired with the header it sat in. The wordmark
// went with it — the menu bar and the Dock carry this application's
// identity, and a band that truncates the name it cannot afford is the
// proof the name never belonged there.
//
// Under the full-size-content treatment the native title bar hands over no
// window drag, so the chrome row and the pane's strip claim it back over
// the parts of themselves that hold no control.

package main

import (
	"image"
	"path/filepath"
	"strings"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	gotext "gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/typeset"
)

const (
	// chromeInsetDp is where the chrome row's own content begins while the
	// pane stands: the transcript's row inset, so the conversation's title
	// stands over the first ink of the messages under it rather than on a
	// grid of its own.
	chromeInsetDp unit.Dp = 12

	// chromeGapDp is the air between two things standing in the chrome row
	// or the pane's strip — including the air the row owes the window's
	// control buttons, whose reported trailing edge is bare glass and
	// carries no breathing room of its own.
	chromeGapDp unit.Dp = 12

	// controlGapDp is the closer air between the two halves of the sidebar
	// switch's line: the toggle and new chat are one group of controls and
	// are set tighter than the group is set from anything else.
	controlGapDp unit.Dp = 4

	// controlBoxDp is the square hit area a chrome control takes around its
	// mark. Both the pane's strip and the chrome row use it, so the two
	// halves of one switch are one size as well as one figure.
	controlBoxDp unit.Dp = 28

	// titleMaxDp is the widest the conversation's title may run before it
	// is truncated: far enough that a real name fits whole, near enough
	// that no name can crowd the picker at the row's other end.
	titleMaxDp unit.Dp = 360
)

// windowFrame is the window's per-subscription state: the clickables of the
// controls that stand in the chrome row, kept apart from the pane's own
// (a control and its recalling half are two widgets, not one shared one).
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
	// The window's ground is the transcript's own paper: the document is
	// what this window is, and only the pane rises off it.
	FillRect(gtx, image.Rectangle{Max: size}, 0, t.palette.Ground)

	bounds := pane.Bounds(gtx, size, SidebarWidth, m.SidebarHidden)
	// The float, the rounded outline at the platform's measured whisper,
	// the floor fill and the clip that keeps a scrolled row off the edge
	// are all the pattern's. What is left here is which column stands in it.
	pane.Layout(gtx, t.col, bounds, sidebar)

	contentX := 0
	if !bounds.Empty() {
		contentX = bounds.Max.X
	}
	contentW := size.X - contentX
	if contentW <= 0 {
		return layout.Dimensions{Size: size}
	}

	rowH := min(gtx.Dp(ChromeRowHeight), size.Y)
	if rowH > 0 {
		st := op.Offset(image.Pt(contentX, 0)).Push(gtx.Ops)
		rgtx := gtx
		rgtx.Constraints = layout.Exact(image.Pt(contentW, rowH))
		f.chromeRow(rgtx, m, t)
		st.Pop()
	}

	if main != nil && size.Y-rowH > 0 {
		st := op.Offset(image.Pt(contentX, rowH)).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(contentW, size.Y-rowH))
		main(mgtx)
		st.Pop()
	}

	// Last, into the cap the row reserved: the picker, whose surface hangs
	// over the transcript when it is open.
	layoutPicker(gtx, menu, contentX, contentW, rowH)

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
		// the row claims across its middle must stop before the chip, since
		// a move action swallows the press before any control beneath it
		// sees one.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Dp(ChipWidth), gtx.Constraints.Max.Y)}
		}),
		layout.Rigid(dragSpacer(chromeInsetDp)))
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// layoutPicker draws the model picker into the cap the chrome row reserved
// for it: a control-sized box one inset in from the content area's trailing
// edge. components/picker sizes its anchor to the value and the box is only
// how far that value may run before it is clipped, so the box is a CAP rather
// than a shape — and the anchor is pinned to its trailing edge (modelmenu.go),
// which is why the control lands on the content column's edge and not a few
// pixels inboard of it whatever the model is called.
func layoutPicker(gtx layout.Context, menu layout.Widget, contentX, contentW, rowH int) {
	if menu == nil || rowH <= 0 {
		return
	}
	chipW := gtx.Dp(ChipWidth)
	x := contentX + contentW - gtx.Dp(chromeInsetDp) - chipW
	if x < contentX {
		return
	}
	defer op.Offset(image.Pt(x, 0)).Push(gtx.Ops).Pop()
	mgtx := gtx
	mgtx.Constraints = layout.Constraints{Max: image.Pt(chipW, rowH)}
	menu(mgtx)
}

// chromeLead is where the chrome row's own content begins, in dp in from
// the leading edge of the content area.
//
// It is a measurement in exactly one state. With the pane standing, the
// window's control buttons are inside the pane and the row owes them
// nothing: it starts at the transcript's own inset, so the conversation's
// title stands over the first ink of the messages under it. With the pane
// away the whole top strip is the row's, and the row starts past the
// buttons' reported trailing edge plus the air that bare edge does not
// carry. Where the platform draws no such buttons the measurement is zero
// and the row falls back to the same inset it uses beside the pane.
func chromeLead(hidden bool, buttonsEnd unit.Dp) unit.Dp {
	if !hidden {
		return chromeInsetDp
	}
	return desktop.BandLeadFrom(buttonsEnd, chromeGapDp, chromeInsetDp)
}

// chatTitle draws the conversation the window is showing, which is the
// window's one orientation cue once the pane is away. A chat that has not
// earned a name yet shows a muted placeholder rather than the filename it
// is stored under: the row says what is open, and "new.jsonl" is not that.
func chatTitle(gtx layout.Context, m Model, t themed) layout.Dimensions {
	text, ink := chatTitleText(m.CurrentChat.Name)
	colour := t.palette.RowActive
	if ink == titleMuted {
		colour = t.palette.Heading
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

// titleInk distinguishes a chat that has a name from one that does not, so
// the placeholder reads as an absence rather than as a title.
type titleInk int

const (
	titleNamed titleInk = iota
	titleMuted
)

// chatTitleText is the chrome row's label for a chat file name, and the
// verdict on whether it is a name at all. The untitled chats are the ones
// the application itself named — new.jsonl and its numbered siblings — and
// they show the placeholder until the conversation earns something better.
func chatTitleText(name string) (string, titleInk) {
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
		return sidebarToggle(gtx, t, &f.rowToggle, "Show the conversations")
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

// sidebarToggle draws one half of the sidebar switch: the [|] figure in a
// square hit area, centred on the line of the row it stands in.
//
// The figure never morphs. What the control is about to do is in the label
// it carries, which the screen reader speaks; a mark that changed with the
// state would leave a reader guessing whether it shows the present state or
// the next one, and the platform does not change this one either.
func sidebarToggle(gtx layout.Context, t themed, click *widget.Clickable, label string) layout.Dimensions {
	for click.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	return controlBox(gtx, click, label, func(gtx layout.Context, sz int) {
		PanelGlyph(gtx, sz, t.palette.Heading)
	})
}

// newChatMark draws one half of the new-chat action: the same plus figure
// in the same square, wherever it stands.
func newChatMark(gtx layout.Context, t themed, click *widget.Clickable) layout.Dimensions {
	for click.Clicked(gtx) {
		mvu.MessageOp{Message: NewChat{}}.Add(gtx.Ops)
	}
	return controlBox(gtx, click, "New chat", func(gtx layout.Context, sz int) {
		icon := gtx
		icon.Constraints = layout.Exact(image.Pt(sz, sz))
		t.add(icon)
	})
}

// controlBox stands one chrome mark in a square hit area, named for the
// screen reader and answering the pointer. The box is what makes the two
// halves of a switch the same size: the mark is [ToggleIconSize] and the
// area around it is [controlBoxDp], in the pane's strip and in the chrome
// row alike.
func controlBox(gtx layout.Context, click *widget.Clickable, label string, draw func(layout.Context, int)) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		box := gtx.Dp(controlBoxDp)
		mark := gtx.Dp(ToggleIconSize)
		off := op.Offset(image.Pt((box-mark)/2, (box-mark)/2)).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(mark, mark))
		draw(mgtx, mark)
		off.Pop()
		return layout.Dimensions{Size: image.Pt(box, box)}
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
