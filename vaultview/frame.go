// frame.go is the vault window's chrome and its column composition: the
// sidebar pane down the leading edge, and beside it the content area —
// one chrome row across its top, the note column and the backlinks aside
// under it parted by a draggable divider, and one status bar across its
// foot. The two bands are the same span and the same treatment at either
// end of the same column; what the foot one carries is in status.go.
//
// The composition is app-local rather than the vocabulary's three-column
// shell for one reason: that shell pins its top slot to a full navbar
// band (ControlHeight plus twice the vertical control padding, 52 dp at
// the comfortable density), and this window's whole point is that its
// chrome is a single tight row. Everything else here — a divider that
// tracks an absolute aside width, the op order that makes Tab follow the
// reading order — is the shell's arrangement, kept deliberately
// recognisable.
//
// The sidebar is the leading column and it owns the top of the window the
// way the platform's own sidebars do: not by being the window's edge but
// by floating just inside it — inset from the window's leading, top and
// bottom edges by one margin, rounded on all four corners, with the
// window's ground showing around it on every side. No band crosses above
// it, because on this platform none does; the only thing over the pane is
// the margin of ground it floats off. The pane's toggle sits at its
// top-right corner, where the pane ends, with the strip's empty middle
// moving the window. The slivers of ground the margin reveals claim no
// drag of their own: a hand aims for the strip, not for an eight-dp gap,
// and a move action there would promise a handle too thin to hit. The
// pane is also where the vault's own actions live, at its foot — the pane
// stands for the vault, so what acts on the whole vault belongs to it.
// Hidden, the pane takes no width at all, the note column reflows from
// the window's leading edge, and the chrome row carries the toggle that
// brings the pane back, since a control that travels with the pane cannot
// be the one that recalls it.
//
// The window control buttons are the window's own and are measured from
// its edges, not from anything drawn under them: they stand a fixed inset
// in from the window's top and leading glass, the inset the platform's
// own sidebar apps use, and they stay there whatever the application puts
// beneath. The pane happens to be under them while it stands, so its top
// strip is cut deep enough to hold them with air to spare and its toggle
// centres on their line; when the pane goes, nothing about them changes.
// A control that belongs to the window cannot shift because a pane the
// reader dismissed used to be behind it.
//
// That line is what everything else at the top of the window stands on
// too: the pane's toggle, the vault's name, and the toggle the chrome row
// shows once the pane is away. The buttons are the fixed thing up there
// and the tallest, so they are what the rest lines up with — and the two
// halves of the sidebar switch then hold one height between them, rather
// than the mark jumping as the pane comes and goes. The chrome row is
// shallower than that line is deep, so its content hangs below the row's
// own height; the row still spends the height it always did, and what
// hangs falls in the margin the note column keeps above its first line.
//
// The window's ground is the same paper the note column lies on, so the
// note has no edge of its own to draw and the chrome row sits on the
// document rather than on a band above it. What is furniture says so by
// standing off that ground; what is document simply is the ground. Both
// of the window's edges are furniture and both stand on the same floor,
// a measured step under the paper in either scheme — but they are two
// different KINDS of furniture, and this window is where the difference
// is drawn. Leading, the sidebar is a FLOATING PANE: a button slides it
// out of the window, so it is an object, and it carries its own hairline
// just inside its rounded edge to say so. Trailing, the column of the
// note's outline and the notes citing it is INTEGRAL FURNITURE: fixed,
// flush, with nothing to dismiss it, so it takes no outline at all and
// its leading edge is a plain seam. Neither is the document, so neither
// lies on its paper, and the document is a column of paper between two
// panes rather than a shape cut out of one.
//
// Both boundaries paint one hairline, and both run the window's whole
// height: the platform does not exempt its top band from a split seam,
// and a seam that stopped at a band would say the window is divided in
// one place and joined in another. What is left to the pointer is the
// one thing a resting edge cannot say — that this particular seam moves.
// The grab area stays as wide as a hand needs, and the seam inside it
// thickens and takes a firmer ink while a hand is on it, rather than a
// second bar appearing beside a line that is already drawn.
//
// Where the chrome row sits is a platform fact, not a taste. Under the
// full-size-content treatment the content extends behind the native
// title bar, so the top strip is the application's to lay out in. The row
// takes the content area's share of it and stops there: it begins where
// the sidebar ends and never crosses above it. The strip is not the
// system's to click in under this treatment, so the row's controls are
// pressable where they stand; what the strip does not hand over is the
// window drag, which the row and the pane's strip claim back by declaring
// a move action over the parts of themselves that hold no control. Away
// from the treatment the measurements report zero, the buttons stay where
// their platform puts them, and the row lays out from its own edge inset.

package main

import (
	"image"
	"path"
	"strings"

	"gioui.org/io/event"
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

	"github.com/vibrantgio/components/icons"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// Frame layout constants. The aside bounds and divider width follow the
// three-column shell's, so the two compositions resize alike.
const (
	frameEdgeDp     = 12
	frameGapDp      = 16
	frameDividerDp  = 6
	frameAsideDp    = 320
	frameMinAsideDp = 160
	frameMaxAsideDp = 640

	// railToggleMarkDp is the square the sidebar control's mark takes.
	// Both of the window's sidebar controls measure by it, so the two
	// halves of one switch are the same size as well as the same figure.
	railToggleMarkDp = markLargeDp

	// railMarginDp is the frame's small edge margin: the inset the sidebar
	// pane floats off the window's leading, top and bottom edges, what the
	// divider's hairline holds clear of the chrome row and the window's
	// bottom edge, what the sidebar's own top strip keeps around its
	// toggle, and the air the trailing column leaves either side of a
	// pane's scrollbar — which is what stands that bar off the window's
	// edge by what the note's stands off this column's.
	railMarginDp = 8

	// seamDp is what any chrome boundary in this window paints: a hairline,
	// the width the platform's own split dividers take and the width the
	// vocabulary's shell settled on for the seam between its panes. It is
	// the pane's internal outline and it is the flush column's seam, so
	// that a window whose two vertical boundaries are drawn for different
	// reasons still draws them at one weight. Wider is worse in a way that
	// is easy to miss: a seam runs the window's whole height, band
	// included, so its width is the width of the scar it leaves across
	// every band it crosses.
	seamDp = 1

	// seamGrabbedDp is what the movable seam paints while a hand is on it:
	// the same line, thick enough to be seen as a change of state rather
	// than as a second edge beside the first.
	seamGrabbedDp = 2

	// railRadiusDp rounds the sidebar pane's four corners. The pane floats
	// inside the window rather than being its edge, so its corners are its
	// own to round — the window's, which the platform rounds, are a margin
	// away.
	railRadiusDp = 10

	// buttonInsetDp is how far the window control buttons sit in from the
	// window's own top and leading edges — the drawn circles' own edges,
	// equal on both axes, measured from the glass and from nothing else.
	// The number is the platform's, read off its sidebar apps on this
	// display: Finder, Mail, Notes and Voice Memos all draw the circles
	// nineteen pixels in from both edges, which at one pixel per dp is
	// nineteen. It is not what this window's toolkit would do left alone —
	// unasked, the buttons land at nine, the inset the platform's compact
	// windows use — so the placement is stated rather than defaulted. The
	// rest of the run — the centre line the placement call wants, the
	// diameter that converts one into the other — follows from this one
	// number by the platform's own rule, which desktop.ButtonRunAt applies.
	buttonInsetDp = 19

	// paneStripDp is the pane's own top strip: deep enough to hold the
	// buttons where the window puts them with the same air below them as
	// above. The buttons' inset is measured from the glass and the strip
	// from the pane's own edge, so the strip owes the difference back —
	// which lands the buttons' centre line on the strip's own middle, the
	// line the pane's toggle centres on too, so the two sit level.
	paneStripDp = 2*(buttonInsetDp-railMarginDp) + desktop.WindowButtonDiameter
)

// windowButtons is where this window's three control buttons stand, derived
// from the inset above by the rule the platform's own windows follow. It is
// the whole placement, and every number in it is the window's: no rail
// state, no screen and no pane enters into any of them.
var windowButtons = desktop.ButtonRunAt(buttonInsetDp)

// toolbarHeight is the chrome row's height: one LabelLarge line box with
// the smallest spacing step above and below. It deliberately does not
// take the density's control padding — this is a title row, not a row of
// controls, and every dp it spends is a dp the document does not get.
func toolbarHeight(tok themeTokens) unit.Dp {
	return unit.Dp(tok.typ.LabelLarge.LineHeight + 2*tok.sp.S1)
}

// frameState is the vault frame's per-subscription state: the toolbar's
// clickables and the aside divider's drag. It is touched only on the
// frame goroutine.
type frameState struct {
	toggleClick widget.Clickable
	vaultClick  widget.Clickable

	dividerTag struct{}
	asideW     unit.Dp
	pressX     float32
	startW     unit.Dp
	dragging   bool
	hovering   bool

	// leading pins the row's leading inset instead of measuring it. The
	// measurement is the window's, and the window is the one thing a
	// headless render does not have: off a real frame it reports zero, and
	// on a real frame it reports whatever this machine's macOS puts the
	// control buttons at. Neither is a number a stored image may depend
	// on, so the static render path states one. The live path leaves this
	// nil and measures.
	leading func() unit.Dp

	// geom is the arrangement the last frame laid out. It is kept so the
	// composition can be measured after the fact — the chrome budget is a
	// property of what was drawn, and a test that recomputed it from the
	// tokens would be asserting its own arithmetic rather than the frame's.
	geom frameGeom
}

// toolbarLeading answers where this frame's row may start: the pinned
// value where one was given, and the platform measurement otherwise.
func (f *frameState) toolbarLeading() unit.Dp {
	if f.leading != nil {
		return f.leading()
	}
	return toolbarLeading()
}

// vaultFrame composes the vault screen from its three column streams and
// the model: a toolbar row over the columns. All three columns arrive as
// widget streams so a theme change re-renders them; the toolbar reads the
// token and model snapshots at frame time.
func vaultFrame(
	loadModel func() Model,
	loadTok func() themeTokens,
	sidebar, aside, main rx.Observable[layout.Widget],
) rx.Observable[layout.Widget] {
	columns := rx.CombineLatest3(sidebar, aside, main)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := &frameState{asideW: frameAsideDp}
		return rx.Map(columns, func(next rx.Tuple3[layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			sbW, asW, mainW := next.First, next.Second, next.Third
			return func(gtx layout.Context) layout.Dimensions {
				return st.layout(gtx, loadModel(), loadTok(), sbW, asW, mainW)
			}
		})
	})
}

// renderWindow is the static counterpart of the whole vault window, used
// by the window golden: the chrome row over the rail pane, the note
// column and the backlinks aside, all with fresh widget state, laid out
// once from pre-resolved tokens and processing no events. It is the only
// renderer in this package that composes rather than filling one slot.
//
// The leading inset is a parameter and not a measurement here, for the
// reason [frameState.leading] gives: the value the live row lays out
// from belongs to a window this render does not have.
//
// The frame is returned beside the widget so that what a render arranged
// can be read back from it once the widget has run — the chrome budget is
// measured off the same composition the golden stores, not off a second
// one built to be measured.
func renderWindow(
	shaper *text.Shaper,
	m Model,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	den tokens.Density,
	leading unit.Dp,
) (layout.Widget, *frameState) {
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	st := &frameState{asideW: frameAsideDp, leading: func() unit.Dp { return leading }}
	cur := &docCursor{}
	sb := renderTree(shaper, m, colors, sp, rad, typo, den, leading)
	main := renderNotePageInto(cur, shaper, m, colors, sp, typo, den)
	av := newAsideView(cur)
	as := func(gtx layout.Context) layout.Dimensions { return av.layout(gtx, m, tok) }
	return func(gtx layout.Context) layout.Dimensions {
		return st.layout(gtx, m, tok, sb, as, main)
	}, st
}

// frameGeom is the arrangement one frame laid out: the sidebar pane's
// rectangle in frame coordinates, empty when the rail is hidden or the
// window has no room for it; the leading edge the content area starts
// from — past the pane when the rail stands, the window's own edge when
// it does not — the top and height of the content area's first document
// row, which the chrome row stands above, and the line that row stops on,
// which is where the status bar begins.
//
// Only the content area has a chrome row and a status bar. The pane floats
// one margin inside the window's leading, top and bottom edges, so the
// only thing above it is that margin of ground and the only thing below it
// the same: no band at either end, and nothing else to measure.
type frameGeom struct {
	pane     image.Rectangle
	contentX int
	rowTop   int
	rowH     int
	footTop  int
}

// frameGeometry measures the pane and the content area. It is separate
// from the drawing so the arrangement can be asserted without a frame:
// that the pane floats one margin inside the window's leading, top and
// bottom edges, that the content reflows to the window's own edge when the
// rail goes, and that the content area's columns run between its two
// bands and no further.
//
// The bands are taken in the order they matter when there is no room for
// either: the chrome row first, because it carries the control that brings
// a dismissed pane back, and the status bar out of what is left, because a
// window with no height for a document has nothing to report about one.
func frameGeometry(gtx layout.Context, size image.Point, barH, footH int, hidden bool) frameGeom {
	h := max(size.Y, 0)
	barH = min(max(barH, 0), h)
	footH = min(max(footH, 0), h-barH)
	g := frameGeom{rowTop: barH, rowH: h - barH - footH, footTop: h - footH}
	if hidden || size.X <= 0 || size.Y <= 0 {
		return g
	}
	margin := gtx.Dp(unit.Dp(railMarginDp))
	railW := gtx.Dp(unit.Dp(treeWidthDp))
	// The pane and its margin may never take more than half the window: a
	// narrow window owes the note a readable column before it owes the
	// rail its width.
	if maxW := size.X/2 - margin; railW > maxW {
		railW = maxW
	}
	if railW <= 0 || size.Y <= 2*margin {
		return g
	}
	g.pane = image.Rect(margin, margin, margin+railW, size.Y-margin)
	g.contentX = g.pane.Max.X
	return g
}

// layout draws the sidebar pane, then the content area's chrome row and
// the columns below it, in the order they read: rail, chrome row, note,
// divider, aside — and last the status bar under all of them. That order
// is the focus ring's too, and the bar is last in it by holding nothing
// the ring can stop on.
func (f *frameState) layout(gtx layout.Context, m Model, tok themeTokens, sb, as, main layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	// The window's ground is the note's own paper, not a chrome fill: the
	// document is what the window is, and only the sidebar rises off it.
	paint.FillShape(gtx.Ops, tok.col.Background, clip.Rect{Max: size}.Op())

	barH := gtx.Dp(toolbarHeight(tok))
	if barH > size.Y {
		barH = size.Y
	}

	g := frameGeometry(gtx, size, barH, gtx.Dp(statusBarHeight(tok)), m.SidebarHidden)
	f.geom = g

	if sb != nil && !g.pane.Empty() {
		f.layoutRailPane(gtx, tok, g.pane, sb)
	}

	f.processDividerDrag(gtx)

	dividerW := gtx.Dp(unit.Dp(frameDividerDp))
	if dividerW < 1 {
		dividerW = 1
	}
	asidePx := gtx.Dp(f.asideW)
	if avail := size.X - g.contentX - dividerW; asidePx > avail {
		asidePx = avail
	}
	if asidePx < 0 {
		asidePx = 0
	}
	mainW := size.X - g.contentX - dividerW - asidePx
	if mainW < 0 {
		mainW = 0
	}
	asideX := g.contentX + mainW + dividerW

	// The trailing column's ground, painted before the chrome row and
	// running the window's full height: the outline and the backlinks are
	// furniture, and this window's furniture is the FLOOR the paper lies
	// on — a surface step UNDER the document, toward the scheme's dark
	// extreme, in both schemes. Full height because a surface that began
	// under the chrome row would read as a block hanging off it — the same
	// arrangement the leading pane was taken out of — and because the two
	// panes are then the same shape, one down each edge, with the document
	// between them.
	if asidePx > 0 {
		paint.FillShape(gtx.Ops, chromeSurface(tok.col),
			clip.Rect(image.Rect(asideX, 0, size.X, size.Y)).Op())
	}

	// The chrome row belongs to the content area alone. With the pane
	// standing, the window buttons are inside it and the row owes them no
	// leading space; with the pane away, the row starts after their
	// measured trailing edge, as the whole top strip is then its own.
	if rowW := size.X - g.contentX; rowW > 0 && barH > 0 {
		lead := unit.Dp(0)
		if g.pane.Empty() {
			lead = f.toolbarLeading()
		}
		bst := op.Offset(image.Pt(g.contentX, 0)).Push(gtx.Ops)
		bgtx := gtx
		bgtx.Constraints = layout.Exact(image.Pt(rowW, barH))
		f.layoutToolbar(bgtx, m, tok, lead)
		bst.Pop()
	}

	if main != nil && g.rowH > 0 {
		st := op.Offset(image.Pt(g.contentX, g.rowTop)).Push(gtx.Ops)
		// What the chrome row's content hangs below the row is clipped out
		// of the note, which is drawn after the row and would otherwise
		// repaint the hanging part away with its own ground. Nothing is
		// lost by the clip: that ground is the window's own paper, already
		// painted under everything, and the note's first ink is a full
		// margin below the row — the band the clip takes is bare either
		// way.
		over := min(buttonLineDrop(gtx, barH), g.rowH)
		clipped := clip.Rect(image.Rect(0, over, mainW, g.rowH)).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(mainW, g.rowH))
		main(mgtx)
		clipped.Pop()
		st.Pop()
	}

	// The divider's hit area is registered in frame-local coordinates —
	// no offset transform pushed — so drag deltas measure against a
	// stable origin even as the divider itself moves.
	dividerRect := image.Rect(g.contentX+mainW, g.rowTop, g.contentX+mainW+dividerW, g.rowTop+g.rowH)
	area := clip.Rect(dividerRect).Push(gtx.Ops)
	event.Op(gtx.Ops, &f.dividerTag)
	pointer.CursorColResize.Add(gtx.Ops)
	area.Pop()

	if as != nil && g.rowH > 0 {
		st := op.Offset(image.Pt(asideX, g.rowTop)).Push(gtx.Ops)
		agtx := gtx
		agtx.Constraints = layout.Exact(image.Pt(asidePx, g.rowH))
		as(agtx)
		st.Pop()
	}

	// The status bar goes last: it is the content area's foot, nothing
	// stands on it, and it holds nothing the keyboard can reach — so the
	// reading order the ops above set down ends with the document's own
	// columns rather than with a line about them.
	if rowW := size.X - g.contentX; rowW > 0 && g.footTop < size.Y {
		st := op.Offset(image.Pt(g.contentX, g.footTop)).Push(gtx.Ops)
		fgtx := gtx
		fgtx.Constraints = layout.Exact(image.Pt(rowW, size.Y-g.footTop))
		layoutStatusBar(fgtx, m, tok)
		st.Pop()
	}

	// The trailing column's own boundary, and the last thing painted so
	// that neither the column's rows nor the two bands crossing over it can
	// cover it. It runs the window's full height, band and status bar
	// included, because that is where the platform runs a split seam and
	// because a seam that stopped at a band would divide the window in one
	// place and not another.
	if asidePx > 0 {
		f.paintAsideSeam(gtx, tok, asideX, size.Y)
	}

	return layout.Dimensions{Size: size}
}

// layoutRailPane draws the sidebar pane — its own edge, then its fill —
// and lays the rail inside it, clipped to the fill rather than to the
// boundary so a scrolled row can neither cross an edge, poke through a
// corner, nor paint over the edge itself.
//
// WHAT SAYS THE PANE FLOATS IS ITS EDGE, NOT ITS TONE. The pane is chrome
// furniture and chrome furniture is the window's floor, in both schemes
// (ADR-022 V2): it is darker than the document beside it, and it stays
// darker for sliding out of the window. Depth here is semantic and not
// geometric — a pane does not climb the ladder by leaving the wall — so
// what marks it as an object is the hairline it carries just inside its
// own boundary, at the platform's measured seam strength (paneSeam), and
// the inset and the corner radius that already say the same thing. The
// platform's worked example draws exactly this: a floating panel painted
// darker than the content beside it, outlined internally at 1.51:1, with
// the flush side of the same window carrying no outline at all.
//
// The edge is internal — drawn inside the pane's own rounded rectangle,
// never on the ground outside it — for the reason the platform's is:
// half a line lying on the window's ground would blur the one boundary a
// reader uses to tell where the pane stops.
//
// THE SHADOW IS RETIRED, AND THE LADDER RETIRED IT. The pane used to cast
// one at Level2, from the reading that anything floating and dismissible
// earns a shadow. What that missed is which storey the pane occupies: its
// storey is the floor, and tokens.Elevation gives the floor zero dp — "the
// window's desk is behind everything and has nothing to cast onto". So a
// shadow resolved against the pane's own storey and a shadow retired in
// favour of the edge are the same answer, arrived at from opposite ends
// (ADR-022's amendment leaves the choice open and this is why it does not
// matter). It also settles the paradox the ledger was holding: a fixed
// black fringe cast by the darkest region of the window read as lighter
// than the thing casting it, which is not what a shadow is. Nine paint
// operations per frame go with it.
func (f *frameState) layoutRailPane(gtx layout.Context, tok themeTokens, pane image.Rectangle, sb layout.Widget) {
	r := gtx.Dp(unit.Dp(railRadiusDp))
	w := max(gtx.Dp(unit.Dp(seamDp)), 1)
	// Two concentric fills rather than a stroke: a stroke is centred on the
	// path it follows and antialiases both of its sides, so a one-pixel one
	// arrives as two rows of half-strength ink and the line the palette
	// asked for is never actually painted. Filling the pane in the seam's
	// ink and filling the inset pane back in over it leaves the difference
	// — exactly one pixel of the seam's own colour down every straight run,
	// with the corners' arcs antialiased against each other the way a
	// fence's rim is drawn.
	rr := clip.RRect{Rect: pane, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, paneSeam(tok.col), rr.Op(gtx.Ops))
	inner := clip.RRect{Rect: pane.Inset(w), NE: max(r-w, 0), NW: max(r-w, 0), SE: max(r-w, 0), SW: max(r-w, 0)}
	paint.FillShape(gtx.Ops, chromeSurface(tok.col), inner.Op(gtx.Ops))
	// The rail is clipped to the pane's inside, not to its boundary, so a
	// selected row that runs the pane's full width cannot paint over the
	// edge that says the pane is an object.
	defer inner.Push(gtx.Ops).Pop()
	defer op.Offset(pane.Min).Push(gtx.Ops).Pop()
	sgtx := gtx
	sgtx.Constraints = layout.Exact(pane.Size())
	sb(sgtx)
}

// paintAsideSeam draws the boundary of the trailing column: a plain
// hairline down its leading edge, running the window's full height.
//
// The column is INTEGRAL FURNITURE — fixed, flush, with no toggle and no
// way to leave — so it is not outlined the way the rail is (ADR-022's
// amendment, and the owner's earlier ruling that the pane treatment
// belongs to what can be moved away). What it takes instead is the plain
// seam the platform gives its own flush side: Voice Memos carries no
// outline there at all and parts its panes with a one-pixel divider that
// runs from the window's top edge to its bottom, band included, and Notes
// does the same between its list and its note. That is R6's split hairline
// and the weight the vocabulary's shell settled on.
//
// The ink is Divider — the token whose whole job is the line between two
// regions — and not the pane's own seam ink, deliberately. The two
// boundaries in this window are two different things and the window is
// meant to say so: an object's edge circles a pane at the platform's
// measured whisper, and a region's seam is a divider between grounds. One
// weight, two inks, and the difference is the doctrine.
//
// A line here is new, and what changed is the size of the step it draws
// over. The worked example this frame used to follow — where two sides
// stand on different grounds the edge between them IS the seam and there
// is nothing to draw — was written when furniture and paper stood a full
// band step apart in both schemes. The floor's dark step is a measured
// 1.47 L\* now, a whisper the eye can lose, and the platform's answer at a
// whisper is a line: Voice Memos' two panes are the SAME fill and the
// divider is the whole of what parts them.
//
// UNDER THE HAND THE SEAM ITSELF THICKENS. This boundary is also the one
// the reader can move, and a resting edge cannot say so — a fresh reviewer
// read the whole split as fixed. What says it is this same line growing
// and taking a firmer ink while the pointer is in the grab band, with the
// resize cursor beside it. The mark used to be a separate bar floating in
// the middle of the grab band, which was the only thing available when the
// boundary was unmarked; beside a drawn seam it would read as a stray
// second edge three dp off the real one. One line, two states.
func (f *frameState) paintAsideSeam(gtx layout.Context, tok themeTokens, x, height int) {
	w := max(gtx.Dp(unit.Dp(seamDp)), 1)
	ink := tok.col.Divider
	if f.hovering || f.dragging {
		w = max(gtx.Dp(unit.Dp(seamGrabbedDp)), w)
		ink = tok.col.Ramps.Neutral.Step(500)
	}
	seam := image.Rect(x-w/2, 0, x-w/2+w, height)
	if seam.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, ink, clip.Rect(seam).Op())
}

// processDividerDrag tracks the aside divider. The aside keeps an
// absolute width, so a window resize leaves it alone and the note column
// absorbs the change.
func (f *frameState) processDividerDrag(gtx layout.Context) {
	scale := gtx.Metric.PxPerDp
	if scale <= 0 {
		scale = 1
	}
	for {
		e, ok := gtx.Event(pointer.Filter{
			Target: &f.dividerTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			f.pressX = pe.Position.X
			f.startW = f.asideW
			f.dragging = true
		case pointer.Drag:
			if f.dragging {
				// The aside sits trailing of the divider, so dragging
				// right shrinks it.
				f.asideW = clampAside(f.startW - unit.Dp((pe.Position.X-f.pressX)/scale))
			}
		case pointer.Release, pointer.Cancel:
			f.dragging = false
		case pointer.Enter:
			f.hovering = true
		case pointer.Leave:
			f.hovering = false
		}
	}
}

func clampAside(w unit.Dp) unit.Dp {
	if w < frameMinAsideDp {
		return frameMinAsideDp
	}
	if w > frameMaxAsideDp {
		return frameMaxAsideDp
	}
	return w
}

// layoutToolbar draws the content area's chrome row on one baseline: the
// vault's name as what this window is showing, and nothing else but the
// window's own drag. With the sidebar away the row also leads with the
// toggle that brings it back — the pane's own toggle went with the pane,
// and something has to recall it.
//
// The two vault actions that used to end this row now stand at the foot
// of the sidebar pane. They are the vault's, not the document's, and the
// row above a document was both the wrong place to say so and width the
// row did not need to spend.
//
// The leading space is a measurement, not a constant, and only the hidden
// state spends it: the window controls report where they end, and the row
// adds its own gap after that, because the reported edge is the bare
// glass. With the pane standing the buttons are inside it, the row starts
// where the pane ends, and lead is zero. Where the window has no such
// controls the measurement falls back to the ordinary edge inset.
//
// The vault's name is the row's own affordance rather than a breadcrumb
// segment: it is window state, and as a crumb it promised a parent to
// climb to that a vault does not have. Pressing it still returns the
// folder tree to its root, which is what the crumb did.
func (f *frameState) layoutToolbar(gtx layout.Context, m Model, tok themeTokens, lead unit.Dp) layout.Dimensions {
	children := make([]layout.FlexChild, 0, 10)
	if lead > 0 {
		// The window controls' own space is left alone: a move action
		// declared over the buttons would fight them for the press.
		children = append(children,
			layout.Rigid(complayout.HSpacer(float32(lead))),
			layout.Rigid(dragSpacer(frameGapDp)))
	} else {
		// The row's own edge inset is the note column's, not a smaller one
		// of its own: the vault's name stands directly over the breadcrumb
		// below it, and a fresh reviewer counted the twelve dp they were
		// apart as two grids where the window has one.
		children = append(children, layout.Rigid(dragSpacer(noteInsetDp)))
	}
	if m.SidebarHidden {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return onButtonLine(gtx, func(gtx layout.Context) layout.Dimensions {
					return f.layoutRailToggle(gtx, m, tok)
				})
			}),
			layout.Rigid(dragSpacer(unit.Dp(tok.sp.S3))))
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return onButtonLine(gtx, func(gtx layout.Context) layout.Dimensions {
				return f.layoutVaultName(gtx, m, tok)
			})
		}),
		layout.Flexed(1, dragFill),
		// The trailing inset is the backlinks column's own: the row ends
		// where the column under it ends. With the actions gone it is the
		// tail of one long drag rather than the gap after a control.
		layout.Rigid(dragSpacer(asideInsetDp)))
	// Each child places itself down the row rather than the row placing
	// them all: the drag spans take the row's own height, and what the
	// reader can see stands on the window buttons' line, which is lower.
	// A row that centred its children on one another would drag whichever
	// is shorter off that line.
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
}

// onButtonLine stands w in a row-deep box whose middle is the window
// buttons' centre line, rather than the chrome row's own middle.
//
// Everything in the row that carries ink takes it. The row is one line
// box deep and the buttons centre below that, so a row that centred its
// content on itself stood the vault's name a dozen dp above the buttons
// beside it — and moved the sidebar mark by those same dozen every time
// the pane came and went, the pane's own toggle being on the buttons'
// line already.
//
// The box keeps the row's depth, so what stands in it is as pressable as
// it was; all that changes is where that depth sits. It ends below the
// row's foot, which is the point: the row's height is what the content
// area puts above its first document row, and moving what stands in the
// row is not allowed to spend more of it.
func onButtonLine(gtx layout.Context, w layout.Widget) layout.Dimensions {
	h := gtx.Constraints.Max.Y
	cgtx := gtx
	cgtx.Constraints.Min.Y, cgtx.Constraints.Max.Y = h, h
	mac := op.Record(gtx.Ops)
	dims := w(cgtx)
	call := mac.Stop()
	top := buttonLineDrop(gtx, h)
	defer op.Offset(image.Pt(0, top)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(dims.Size.X, top+max(dims.Size.Y, h))}
}

// buttonLineDrop is how far the chrome row's content stands below where
// the row's own middle would have put it: from that middle down to the
// window buttons' centre line. The box the content stands in is as deep
// as the row, so the same number is how far the content hangs past the
// row's foot.
func buttonLineDrop(gtx layout.Context, rowH int) int {
	return max(gtx.Dp(windowButtons.Center)-rowH/2, 0)
}

// toolbarLeading is where the row's own content may start: the trailing
// edge of the window's control buttons where the platform puts them in
// the content area, and the ordinary edge inset where it does not. The
// row asks for no air past the buttons — it holds its things at the edge
// inset it holds everything at, and the buttons are one of its things.
func toolbarLeading() unit.Dp {
	return desktop.BandLead(0, frameEdgeDp)
}

// dragSpacer is a fixed-width gap in the chrome row that moves the window
// when it is dragged. The row stands in the strip the native title bar
// would otherwise own, so the window's top edge is only a handle where
// the row says it is — and it says so over its empty space alone, since a
// move action swallows the press before any control beneath it sees one.
func dragSpacer(w unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return desktop.DragRun(gtx, gtx.Dp(w))
	}
}

// dragFill is the row's flexible middle: the whole gap between the vault
// name and the trailing actions, draggable end to end.
func dragFill(gtx layout.Context) layout.Dimensions {
	return desktop.DragRun(gtx, gtx.Constraints.Min.X)
}

// layoutVaultName draws the open vault's folder name, pressable, in the
// row's own weight.
func (f *frameState) layoutVaultName(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	name := vaultName(m)
	if name == "" {
		return layout.Dimensions{}
	}
	if f.vaultClick.Clicked(gtx) {
		mvu.MessageOp{Message: RootTree{}}.Add(gtx.Ops)
	}
	return f.vaultClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(name).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		return drawLabel(gtx, tok.shaper, name, tok.typ.TitleSmall, tok.col.Text)
	})
}

// vaultName is the open vault's folder name — what the window is
// showing — empty before a vault is open.
func vaultName(m Model) string {
	if m.Vault == "" {
		return ""
	}
	return path.Base(strings.TrimRight(m.Vault, "/"))
}

// layoutRailToggle draws the chrome row's show control, which stands only
// while the rail is away: the same mark the pane wears.
func (f *frameState) layoutRailToggle(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	if f.toggleClick.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	label := "Hide the folder rail"
	if m.SidebarHidden {
		label = "Show the folder rail"
	}
	return f.toggleClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return railToggleMark(gtx, tok, label)
	})
}

// railToggleMark draws the sidebar control's figure, taken from the
// design system's own set: a pane in outline, divided, with the faint
// list lines the host platform puts in the leading column. The set
// resolves that per platform, so a Mac user sees the figure they know
// and everyone else sees the neutral one, from the same name here.
//
// It is one drawing that never morphs. The mark used to hollow out with
// the rail away, and a fresh reviewer read the hollow one as an
// unchecked box: the platform does not change this figure to advertise
// the action either, because a mark that changes leaves the reader
// guessing whether it shows the present state or the next one. What the
// control is about to do is in the label it carries, which the screen
// reader speaks and the tooltip shows.
//
// Both of the window's sidebar controls take it — the one in the pane's
// top strip and the one the chrome row shows once the pane is gone — so
// that the two halves of the same switch are one figure and not two.
func railToggleMark(gtx layout.Context, tok themeTokens, label string) layout.Dimensions {
	semantic.LabelOp(label).Add(gtx.Ops)
	semantic.EnabledOp(true).Add(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	w := gtx.Dp(unit.Dp(railToggleMarkDp))
	// The mark is centred in a row-tall box, so the whole height of the
	// row it stands in is pressable rather than the ink alone.
	boxH := max(gtx.Constraints.Max.Y, w)
	st := op.Offset(image.Pt(0, (boxH-w)/2)).Push(gtx.Ops)
	drawMark(gtx, icons.Sidebar, railToggleMarkDp, tok.col.Text)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(w, boxH)}
}
