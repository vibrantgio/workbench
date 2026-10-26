// frame.go is what this window hands patterns/shell's two-column
// arrangement, and nothing else: the shell owns the geometry — the
// conversation pane down the leading edge with the splitter on its trailing
// edge, the content column beside it with one toolbar across its top, the
// shadow cast last — and this window states what stands in each slot.
//
// What the window hands over: the pane's column (view.go), the transcript
// with its input bar (view.go), and the toolbar's two groups — the chat's
// title in the centre, the model picker's reserved cap at the trailing end.
// The pane's own width and the bounds it moves between are the window's as
// well, kept across launches by the shell's own arrangement memory.
//
// What is left here is the title the toolbar carries, the room the picker's
// surface hangs in, and the bounds the pane's splitter moves between.
//
// There is no inspector and no footer. Settings acts on the application
// rather than on the conversation, so it stands in the application menu on
// Cmd-comma, which is where this platform keeps it, and the pane carries no
// footer for it.

package main

import (
	"image"
	"path/filepath"
	"strings"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/attachment"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

const (
	// transcriptInsetDp is the content column's own row inset, and so the
	// toolbar's leading gutter: the conversation's title stands over the
	// first glyphs of the messages under it rather than on a grid of its
	// own. The shell has no default for it — the number is the column's,
	// not the toolbar's.
	//
	// It is also the leading inset the toolbar and the pane's strip lead
	// from where the window has no control buttons to clear: away from the
	// full-size-content treatment, in headless rendering, and on every
	// platform that keeps its own decorations.
	transcriptInsetDp unit.Dp = 12

	// titleMaxDp is the widest the conversation's title may run before it
	// is truncated: far enough that a real name fits whole, near enough
	// that no name can crowd the picker at the row's other end.
	titleMaxDp unit.Dp = 360

	// paneMinWidthDp and paneMaxWidthDp are the bounds the pane's splitter
	// moves between. Below the minimum a conversation's name is nothing but
	// an ellipsis; above the maximum the pane is no longer a column beside
	// the transcript but a second one competing with it. The stored
	// platform reference carries no reading for how narrow a sidebar may be
	// dragged, so these are the window's own.
	paneMinWidthDp = 160
	paneMaxWidthDp = 480

	// transcriptMinWidthDp is the narrowest the content column may be
	// squeezed to by a drag of that splitter: sixty characters of this
	// system's body text and the air a turn keeps either side of itself.
	// The sixty is [UserCardMeasure]'s own — the bottom of the band
	// [TurnMeasure] sits at the top of — and narrower than it the
	// transcript is no longer a column of prose.
	//
	// It bounds the splitter and nothing else, so a window is free to be
	// narrower than it: what it stops is a boundary taking the last of the
	// conversation's room, not a small window from opening.
	transcriptMinWidthDp = 432 + 2*TurnInset
)

// frameLimits are the bounds the window states for the one width the shell
// keeps for it. There is no inspector in this window, so the inspector's
// clamp stays empty.
var frameLimits = shell.Limits{
	Pane: shell.Clamp{Min: paneMinWidthDp, Max: paneMaxWidthDp},
}

// defaultArrangement is what the window opens with when nothing was kept:
// the pane at the sidebar's own column, standing.
func defaultArrangement() shell.Arrangement {
	return shell.Arrangement{PaneWidth: SidebarWidth, PaneShown: true}
}

// windowFrame is the window's per-subscription state: the one gesture source
// behind the Toggle Sidebar button, the pane splitter's hand, the width it
// moves, and the memory that keeps that width across launches.
type windowFrame struct {
	// toggleClick is the ONE gesture source behind the Toggle Sidebar
	// button, which the arrangement draws bare at the pane's trailing
	// corner while the pane stands and bordered at the toolbar's leading
	// end once it is away. The two places are one control, so they are one
	// clickable and one message.
	toggleClick widget.Clickable

	// paneSplitter is the hand on the pane's trailing edge across frames,
	// and paneW the width it moves.
	paneSplitter splitter.State
	paneW        unit.Dp

	// widths keeps that width and the pane's shown state across launches.
	// It is nil wherever a frame is laid out for measurement rather than
	// for a reader, so a stored render arranges a window nobody is sitting
	// at without writing over what a real one kept.
	widths *shell.ArrangementMemory

	// geom is the arrangement the last frame laid out, kept so the
	// composition can be measured after the fact rather than recomputed
	// from the tokens that placed it.
	geom shell.PaneLayout

	// post is where the frame's own answers go: the frame's message op,
	// which is what a nil leaves, and a recorder where a frame is laid out
	// without a window to post to.
	post func(gtx layout.Context, msg mvu.Message)
}

// newWindowFrame seats a frame on the arrangement it opens with.
func newWindowFrame(widths *shell.ArrangementMemory) *windowFrame {
	a := defaultArrangement()
	if widths != nil {
		a = widths.Arrangement()
	}
	return &windowFrame{paneW: a.PaneWidth, widths: widths}
}

// send hands one of the frame's answers on to whatever is collecting this
// frame's messages.
func (f *windowFrame) send(gtx layout.Context, msg mvu.Message) {
	if f.post != nil {
		f.post(gtx, msg)
		return
	}
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
}

// layout hands the shell what this window puts in each of its slots and
// draws the frame it arranges.
func (f *windowFrame) layout(gtx layout.Context, m Model, t themed, col shell.PaneColumn, main, menu layout.Widget) layout.Dimensions {
	two := shell.TwoColumn{
		PaneWidth:  f.paneW,
		PaneHidden: m.SidebarHidden,
		Pane:       col,
		PaneResize: &shell.PaneResize{
			State: &f.paneSplitter,
			Min:   frameLimits.Pane.Min, Max: frameLimits.Pane.Max,
			Floor:    transcriptMinWidthDp,
			OnChange: func(w unit.Dp) { f.paneW = w },
		},
		// The content column stands on the transcript's own surface: the
		// conversation is what the window is. The pane is chrome and is drawn in
		// the platform's chrome material, which patterns/pane resolves.
		Surface:     t.col.WindowBackground,
		ContentFill: t.palette.Transcript,
		Toolbar:     f.toolbar(m, t),
		Toggle: &shell.ToggleSidebar{
			Click:        &f.toggleClick,
			Mark:         icons.Mark(icons.SidebarLeft),
			PaneTitle:    "Hide the conversations",
			ToolbarTitle: "Show the conversations",
			OnPress:      func(gtx layout.Context) { f.send(gtx, ToggleSidebar{}) },
		},
		Radius:           tokens.Radius,
		Density:          t.den,
		Main:             main,
		Over:             f.picker(menu),
		WindowButtonsEnd: windowButtonsEnd,
		Memory:           f.widths,
		Report:           func(g shell.PaneLayout) { f.geom = g },
	}
	dims, _ := two.Layout(gtx, t.col)
	return dims
}

// toolbar is what the content column carries across its top: the
// conversation's name in the centre, which is the window's one orientation
// cue once the pane is away, and the model picker's reserved cap at the
// trailing end.
//
// The leading group is empty. A pane's own marks go with the pane, so what
// the toolbar gains when the pane leaves is the Toggle Sidebar button and
// nothing else — and the arrangement is what puts it there. New chat adds to
// the list the pane shows and waits for it.
//
// The row carries no section label: the strip's line is for controls and
// identities, and a category is neither.
func (f *windowFrame) toolbar(m Model, t themed) shell.Toolbar {
	return shell.Toolbar{
		ButtonFallback: transcriptInsetDp,
		Lead:           transcriptInsetDp,
		Center: func(gtx layout.Context) layout.Dimensions {
			return chatTitle(gtx, m, t)
		},
		// The picker's cap is RESERVED here and drawn afterwards, over the
		// transcript: a popover's surface hangs below its anchor and has to
		// win the paint and the hit test against everything it hangs over,
		// so the one thing in this toolbar that opens is the one thing laid
		// out after the document. What the toolbar spends on it is the
		// room, which is why the reservation stands in the group — the drag
		// the toolbar claims across its middle must stop before the
		// control, since a move action swallows the press before any
		// control beneath it sees one.
		Trailing: []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Dp(ToolbarWidth), gtx.Constraints.Max.Y)}
		}},
	}
}

// picker is the model picker as the arrangement's own overlay slot, handed
// the whole toolbar row inside the content column's gutters rather than a box
// cut to the control. The control stands at the popover's trailing alignment
// (modelmenu.go), so it lands on the toolbar's own trailing gutter whatever
// the model is called. The menu's surface is placed in the window's
// available room, which is moved here by the gutter the row is offset by.
func (f *windowFrame) picker(menu layout.Widget) layout.Widget {
	if menu == nil {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		lead, trail := gtx.Dp(transcriptInsetDp), gtx.Dp(shell.PlatformToolbar.Trail)
		depth := min(gtx.Dp(shell.ToolbarDepth), gtx.Constraints.Max.Y)
		room := gtx.Constraints.Max.X - lead - trail
		if room <= 0 || depth <= 0 {
			return layout.Dimensions{}
		}
		defer op.Offset(image.Pt(lead, 0)).Push(gtx.Ops).Pop()
		mgtx := attachment.AvailableRoomAt(gtx, image.Pt(lead, 0))
		mgtx.Constraints = layout.Constraints{Max: image.Pt(room, depth)}
		return menu(mgtx)
	}
}

// chatTitle draws the conversation the window is showing, which is the
// window's one orientation cue once the pane is away. A chat that has not
// earned a name yet shows a muted placeholder rather than the filename it
// is stored under: the toolbar says what is open, and "new.jsonl" is not
// that.
func chatTitle(gtx layout.Context, m Model, t themed) layout.Dimensions {
	text, verdict := chatTitleText(m.CurrentChat.Name)
	// The toolbar stands over the transcript, so its title is the
	// platform's label over that content. The sidebar's selected-row
	// foreground is white, and on the light content it paints nothing.
	colour := t.palette.TurnText
	if verdict == titleMuted {
		colour = t.palette.Note
	}
	semantic.LabelOp(text).Add(gtx.Ops)
	// The title takes its own width and no more, so the drag between it and
	// the picker is as long as the name is short — capped, so that a long
	// name runs out of room before it runs into the chip.
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(titleMaxDp))
	if gtx.Constraints.Max.X <= 0 {
		return layout.Dimensions{}
	}
	return typeset.Text(gtx, t.shaper, text, t.typ.Headline, colour, 1)
}

// titleVerdict distinguishes a chat that has a name from one that does not, so
// the placeholder reads as an absence rather than as a title.
type titleVerdict int

const (
	titleNamed titleVerdict = iota
	titleMuted
)

// chatTitleText is the toolbar's title for a chat file name, and the
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

// paneNewChat is the pane's own mark, drawn bare in the pane's own strip —
// before the Toggle Sidebar button, which the arrangement puts last. It is
// the application's primary action and Cmd-N operates it in either pane
// state; the mark itself goes with the pane, because new chat adds to the
// list the pane shows and waits for it.
func paneNewChat(gtx layout.Context, t themed, click *widget.Clickable, post func(layout.Context, mvu.Message)) layout.Dimensions {
	for click.Clicked(gtx) {
		post(gtx, NewChat{})
	}
	return shell.PaneMark(gtx, t.col, click, icons.Mark(icons.Plus), "New chat")
}
