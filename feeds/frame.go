// frame.go is what this window hands patterns/shell's three-column
// arrangement, and nothing else: the shell owns the geometry — the rail's
// pane down the leading edge with a splitter on its trailing edge, the
// article list beside it at an absolute width behind a second splitter, the
// article itself absorbing the rest, one toolbar across the two of them with
// a share each, and the shadow cast last — and this window states what
// stands in each slot.
//
// What the window hands over: the rail's column (sidebar.go) with Add feed
// as the pane's own mark, the table and its pager (articles.go), the article
// with its tabs (detail.go), and the toolbar's two shares — the open feed's
// name and the search field over the list, Share over the article. The
// pane's and the list column's widths are the window's as well, kept across
// launches by the shell's own arrangement memory.
//
// There is no footer under the content column: this window reports nothing
// about the article it is showing, so it has no status bar.

package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/patterns/splitter"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

const (
	// listInsetDp is the list column's own gutter, and so the leading
	// gutter of the toolbar's share over it: the table's own cell padding,
	// so the open feed's name up in the share stands over the first glyph
	// of the titles under it.
	listInsetDp = unit.Dp(cellPadDp)

	// articleInsetDp is the content column's own row inset, and so the
	// leading gutter of the toolbar's share over it.
	articleInsetDp = unit.Dp(detailPadDp)

	// paneMinWidthDp and paneMaxWidthDp are the bounds the pane's splitter
	// moves between. Below the minimum a feed's name is nothing but an
	// ellipsis; above the maximum the pane is no longer a column beside the
	// list but a second one competing with it. The stored platform
	// reference carries no reading for how narrow a sidebar may be dragged,
	// so these are the window's own, and they are the pair mindchat states.
	paneMinWidthDp = 160
	paneMaxWidthDp = 480

	// searchFieldWDp is the room the search field takes in the toolbar's
	// share over the list: wide enough for a query of a few words between
	// the looking glass and the clear mark, and narrow enough that the open
	// feed's name keeps the leading half of the share.
	searchFieldWDp = 180

	// shareCapDp is the room RESERVED in the toolbar's share over the
	// content for the Share anchor, which is drawn afterwards over the
	// article. "Share" is 37 wide at 1x in the role it is drawn in.
	shareCapDp = 40

	// titleMaxDp is the widest the open feed's name may run in the list's
	// share before it is truncated, so a long name runs out of room before
	// it runs into the search field.
	titleMaxDp unit.Dp = 220
)

// frameLimits are the bounds the window states for the two widths the shell
// keeps for it. There is no inspector in this window, so the inspector's
// clamp stays empty, and the list column states no MAXIMUM: the platform
// states none either — the column grows until the article beside it reaches
// the content floor, and that floor is the whole of the ceiling.
var frameLimits = shell.Limits{
	Pane: shell.Clamp{Min: paneMinWidthDp, Max: paneMaxWidthDp},
	List: shell.Clamp{Min: listColMinWDp},
}

// defaultArrangement is what the window opens with when nothing was kept:
// the pane at the sidebar's own column, the list column at the platform's
// own width, and the pane standing.
func defaultArrangement() shell.Arrangement {
	return shell.Arrangement{
		PaneWidth: patsidebar.ExpandedWidth,
		ListWidth: shell.PlatformList.Width,
		PaneShown: true,
	}
}

// windowButtonsEnd reports the trailing edge of the window's three control
// buttons, and is the one place this app asks the window about them. A
// render with no window behind it is told zero — which is every render in
// the test suite, and every platform that keeps its own decorations, where
// there are no such buttons to clear. It is a variable so that a test can
// state a measurement it has no window to take.
var windowButtonsEnd = desktop.LeadingInset

// windowFrame is the window's per-subscription state: the one gesture source
// behind the Toggle Sidebar button, the two splitters' hands, the widths
// they move, and the memory that keeps those widths across launches.
type windowFrame struct {
	// toggleClick is the ONE gesture source behind the Toggle Sidebar
	// button, which the arrangement draws bare at the pane's trailing
	// corner while the pane stands and bordered at the leading end of the
	// list's share of the toolbar once it is away.
	toggleClick widget.Clickable

	// addClick is the gesture source behind Add feed, which stands bare in
	// the pane's own strip before the Toggle Sidebar button.
	addClick widget.Clickable

	// paneSplitter and listSplitter are the hands on the two boundaries
	// across frames, and paneW and listW the widths they move.
	paneSplitter splitter.State
	listSplitter splitter.State
	paneW        unit.Dp
	listW        unit.Dp

	// widths keeps those two widths and the pane's shown state across
	// launches. It is nil wherever a frame is laid out for measurement
	// rather than for a reader, so a stored render arranges a window nobody
	// is sitting at without writing over what a real one kept.
	widths *shell.ArrangementMemory

	// geom is the arrangement the last frame laid out, kept so the
	// composition can be measured after the fact rather than recomputed
	// from the tokens that placed it.
	geom shell.ThreeColumnLayout

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
	return &windowFrame{paneW: a.PaneWidth, listW: a.ListWidth, widths: widths}
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

// openedOn seats the window's first model on the arrangement that was kept:
// the pane stands where the reader left it, which is what the kept shown
// state is for — a reader who sent the pane away and found it back on the
// next launch would be reading a defect.
func openedOn(init func() (Model, mvu.Command), a shell.Arrangement) func() (Model, mvu.Command) {
	return func() (Model, mvu.Command) {
		m, cmd := init()
		m.sidebarHidden = !a.PaneShown
		return m, cmd
	}
}

// frameSlots are what the window puts in the arrangement's slots on one
// frame: everything composed out of a live stream, read at frame time.
type frameSlots struct {
	Rail   layout.Widget
	Rows   layout.Widget
	Pager  layout.Widget
	Search layout.Widget
	Main   layout.Widget
	Share  layout.Widget
}

// layout hands the shell what this window puts in each of its slots and
// draws the frame it arranges.
func (f *windowFrame) layout(gtx layout.Context, m Model, tok themeTokens, s frameSlots) layout.Dimensions {
	three := shell.ThreeColumn{
		PaneWidth:  f.paneW,
		PaneHidden: m.sidebarHidden,
		// Add feed is the pane's own mark: it adds to the collection the
		// pane lists, so it waits for the pane and goes away with it.
		Pane: shell.PaneColumn{
			Marks: []layout.Widget{
				func(gtx layout.Context) layout.Dimensions {
					return f.paneAddFeed(gtx, tok)
				},
			},
			Rows: s.Rail,
		},
		PaneResize: &shell.PaneResize{
			State: &f.paneSplitter,
			Min:   frameLimits.Pane.Min, Max: frameLimits.Pane.Max,
			OnChange: func(w unit.Dp) { f.paneW = w },
		},
		ListWidth: f.listW,
		List: shell.ListColumn{
			Rows:   s.Rows,
			Footer: s.Pager,
		},
		ListResize: &shell.ListResize{
			State:    &f.listSplitter,
			Min:      frameLimits.List.Min,
			OnChange: func(w unit.Dp) { f.listW = w },
		},
		// The window's own surface is the backdrop layer's and is not named
		// here, which is why the arrangement paints none. Both document
		// columns stand on the content's own fill: the list and the article
		// are one reading surface parted by the seam, which is Mail's
		// reading and what this window already drew.
		ContentFill: tok.col.ControlBackground,
		ListToolbar: f.listShare(m, tok, s.Search),
		Toolbar:     f.contentShare(),
		Toggle: &shell.ToggleSidebar{
			Click:        &f.toggleClick,
			Mark:         icons.Mark(icons.Sidebar),
			PaneTitle:    "Hide the feeds",
			ToolbarTitle: "Show the feeds",
			OnPress:      func(gtx layout.Context) { f.send(gtx, ToggleSidebar{}) },
		},
		Radius:  tokens.Radius,
		Density: tok.den,
		Main:    s.Main,
		// The content column's empty state centres in the column's FULL
		// frame, so it is laid out in a box reaching a toolbar's share
		// above the slot. Recording the slot and replaying it under the
		// toolbar's ops leaves what stands in the toolbar painting over
		// that strip.
		MainFirst:        true,
		Over:             f.overlay(s.Share),
		WindowButtonsEnd: windowButtonsEnd,
		Memory:           f.widths,
		Report:           func(g shell.ThreeColumnLayout) { f.geom = g },
	}
	dims, _ := three.Layout(gtx, tok.col)
	return dims
}

// listShare is the toolbar's share over the list column: the open feed's
// name at the leading end and the search field at the trailing one, which is
// what Mail puts over its own list — the mailbox's name and the control that
// narrows it.
//
// The name stands in Center rather than in the leading group: it is what the
// share SAYS the column is showing, not a control, and with no leading group
// in front of it the two lay out in the same window column.
func (f *windowFrame) listShare(m Model, tok themeTokens, search layout.Widget) shell.Toolbar {
	return shell.Toolbar{
		ButtonFallback: listInsetDp,
		Lead:           listInsetDp,
		Center: func(gtx layout.Context) layout.Dimensions {
			return feedTitle(gtx, m, tok)
		},
		Trailing: []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			return searchRoom(gtx, search)
		}},
	}
}

// searchRoom lays the search field out at the width the share gives it,
// letting the field report its own depth so the row centres it.
func searchRoom(gtx layout.Context, w layout.Widget) layout.Dimensions {
	box := gtx.Dp(unit.Dp(searchFieldWDp))
	if w == nil {
		return layout.Dimensions{Size: image.Pt(box, 0)}
	}
	fgtx := gtx
	fgtx.Constraints.Min = image.Pt(box, 0)
	fgtx.Constraints.Max.X = box
	dims := w(fgtx)
	return layout.Dimensions{Size: image.Pt(box, dims.Size.Y)}
}

// contentShare is the toolbar's share over the content column: the Share
// anchor at its trailing end, which is where the stored toolbars stand the
// acts on a document. Add feed is not among them — it acts on the feeds the
// pane lists rather than on the article, so it stands on the pane.
//
// It carries no title. The article's own name is the first thing the
// document draws, and a toolbar that repeated it would say the same thing
// twice across one window.
//
// The Share anchor's room is RESERVED here and the control drawn afterwards,
// over the article: a popover's surface hangs below its anchor and has to
// win the paint and the hit test against everything it hangs over, so the
// one thing in this toolbar that opens is the one thing laid out after the
// document. The drag the toolbar claims across its middle must stop before
// it, since a move action swallows the press before any control beneath it
// sees one.
func (f *windowFrame) contentShare() shell.Toolbar {
	return shell.Toolbar{
		ButtonFallback: articleInsetDp,
		Lead:           articleInsetDp,
		Trailing: []layout.Widget{
			func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Dp(unit.Dp(shareCapDp)), 0)}
			},
		},
	}
}

// paneAddFeed is the pane's own mark, drawn bare in the pane's own strip
// before the Toggle Sidebar button, which the arrangement puts last. The
// mark goes with the pane, because Add feed adds to the feed list the pane
// shows and waits for it.
func (f *windowFrame) paneAddFeed(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if f.addClick.Clicked(gtx) {
		f.send(gtx, OpenAddFeed{})
	}
	return shell.PaneMark(gtx, tok.col, &f.addClick, icons.Mark(icons.Plus), "Add feed")
}

// overlay is the Share popover as the arrangement's own overlay slot. What
// it is handed is the whole toolbar row less its trailing gutter rather than
// a box cut to the control: that box is patterns/popover's statement of the
// room the open surface may use, and a box cut to the anchor would leave the
// list running off the window's trailing edge with nothing to clamp against.
// Where the anchor stands in that room is the popover's trailing alignment,
// which lands it on the toolbar's own trailing gutter.
func (f *windowFrame) overlay(share layout.Widget) layout.Widget {
	if share == nil {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		trail := gtx.Dp(shell.PlatformToolbar.Trail)
		depth := min(gtx.Dp(shell.ToolbarDepth), gtx.Constraints.Max.Y)
		room := gtx.Constraints.Max.X - trail
		if room <= 0 || depth <= 0 {
			return layout.Dimensions{}
		}
		sgtx := gtx
		sgtx.Constraints = layout.Exact(image.Pt(room, depth))
		return share(sgtx)
	}
}

// feedTitle draws the feed the list column is showing, which is the window's
// one orientation cue for that column once the pane is away. A window
// showing no feed at all names the collection instead.
func feedTitle(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	name := feedName(m.feeds, m.selectedFeed)
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(titleMaxDp))
	if gtx.Constraints.Max.X <= 0 {
		return layout.Dimensions{}
	}
	return typeset.Text(gtx, tok.shaper, name, tok.typ.TitleSmall,
		vgcolor.Flatten(tok.col.Label, tok.col.ControlBackground), 1)
}

// feedName is the open feed's own title, or the collection's name where no
// feed is open.
func feedName(groups []feedGroup, id FeedID) string {
	for _, g := range groups {
		for _, e := range g.Entries {
			if e.ID == id {
				return e.Title
			}
		}
	}
	return "Articles"
}
