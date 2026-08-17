// frame.go is the vault window's chrome and its column composition: the
// sidebar pane down the leading edge, and beside it the content area —
// one chrome row across its top, and under that the note column and the
// backlinks aside, parted by a draggable divider.
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
// The window's ground is the same paper the note column lies on, so the
// note has no edge of its own to draw and the chrome row sits on the
// document rather than on a band above it. What is furniture says so by
// standing off that ground; what is document simply is the ground. Both
// of the window's edges are furniture: leading, the sidebar — raised by
// tint first and shadow second, its surface step doing the separating
// and a cast shadow under it saying it floats — and trailing, the column
// that carries the note's outline and the notes citing it, a full-height
// surface behind the chrome row. Neither is the document, so neither
// lies on its paper, and the document is a column of paper between two
// panes rather than a shape cut out of one.
//
// The divider between the note and that column follows from this. Its two
// sides now stand on different grounds, so the edge between them is the
// seam and there is no line to draw at rest — the sidebar's edge on the
// other side is drawn no other way. The grab area stays as wide as a hand
// needs, and inks while a hand is on it, which is the only thing the
// resting edge cannot say for itself.
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
	"gioui.org/io/system"
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
	"github.com/vibrantgio/effects/depth"
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
	// bottom edge, and what the sidebar's own top strip keeps around its
	// toggle.
	railMarginDp = 8

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
	// windows use — so the placement is stated rather than defaulted.
	buttonInsetDp = 19

	// buttonDiameterDp is the drawn diameter of one control button on this
	// machine's macOS, measured from a live capture rather than assumed —
	// the buttons are the platform's, and so is their size. It converts an
	// edge inset into the centre line the placement call wants.
	buttonDiameterDp = 14

	// buttonCenterDp is the line the buttons' centres sit on, below the
	// window's top edge. It and buttonInsetDp are the whole placement, and
	// both are the window's measurements: no rail state, no screen and no
	// pane enters into either.
	buttonCenterDp = buttonInsetDp + buttonDiameterDp/2

	// paneStripDp is the pane's own top strip: deep enough to hold the
	// buttons where the window puts them with the same air below them as
	// above. The buttons' inset is measured from the glass and the strip
	// from the pane's own edge, so the strip owes the difference back —
	// which lands the buttons' centre line on the strip's own middle, the
	// line the pane's toggle centres on too, so the two sit level.
	paneStripDp = 2*(buttonInsetDp-railMarginDp) + buttonDiameterDp
)

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
// the model: a toolbar row over the columns. The columns arrive as widget
// streams so a theme change re-renders them; the toolbar reads the token
// and model snapshots at frame time.
func vaultFrame(
	loadModel func() Model,
	loadTok func() themeTokens,
	sidebar, aside rx.Observable[layout.Widget],
	main layout.Widget,
) rx.Observable[layout.Widget] {
	columns := rx.CombineLatest2(sidebar, aside)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := &frameState{asideW: frameAsideDp}
		return rx.Map(columns, func(next rx.Tuple2[layout.Widget, layout.Widget]) layout.Widget {
			sbW, asW := next.First, next.Second
			return func(gtx layout.Context) layout.Dimensions {
				return st.layout(gtx, loadModel(), loadTok(), sbW, asW, main)
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
// it does not — and the top and height of the content area's first
// document row, which the chrome row stands above.
//
// Only the content area has a chrome row. The pane floats one margin
// inside the window's leading, top and bottom edges, so the only thing
// above it is that margin of ground: no band, and nothing else to
// measure.
type frameGeom struct {
	pane     image.Rectangle
	contentX int
	rowTop   int
	rowH     int
}

// frameGeometry measures the pane and the content area. It is separate
// from the drawing so the arrangement can be asserted without a frame:
// that the pane floats one margin inside the window's leading, top and
// bottom edges, and that the content reflows to the window's own edge
// when the rail goes.
func frameGeometry(gtx layout.Context, size image.Point, barH int, hidden bool) frameGeom {
	g := frameGeom{rowTop: min(barH, max(size.Y, 0)), rowH: max(size.Y-barH, 0)}
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
// divider, aside. That order is the focus ring's too.
func (f *frameState) layout(gtx layout.Context, m Model, tok themeTokens, sb, as, main layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	// The window's ground is the note's own paper, not a chrome fill: the
	// document is what the window is, and only the sidebar rises off it.
	paint.FillShape(gtx.Ops, tok.col.Background, clip.Rect{Max: size}.Op())

	barH := gtx.Dp(toolbarHeight(tok))
	if barH > size.Y {
		barH = size.Y
	}

	g := frameGeometry(gtx, size, barH, m.SidebarHidden)
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
	// furniture, and this window's furniture stands a surface step off the
	// paper the document lies on. Full height because a surface that began
	// under the chrome row would read as a block hanging off it — the same
	// arrangement the leading pane was taken out of — and because the two
	// panes are then the same shape, one down each edge, with the document
	// between them.
	if asidePx > 0 {
		paint.FillShape(gtx.Ops, tok.col.Surface,
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
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(mainW, g.rowH))
		main(mgtx)
		st.Pop()
	}

	// The divider's hit area is registered in frame-local coordinates —
	// no offset transform pushed — so drag deltas measure against a
	// stable origin even as the divider itself moves.
	dividerRect := image.Rect(g.contentX+mainW, g.rowTop, g.contentX+mainW+dividerW, g.rowTop+g.rowH)
	f.paintDividerLine(gtx, tok, dividerRect)
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

	return layout.Dimensions{Size: size}
}

// layoutRailPane raises the sidebar pane off the window's ground and lays
// the rail inside it, clipped to the pane's own rounded rectangle so a
// scrolled row cannot cross an edge or poke through a corner.
//
// The pane is raised by tint first and shadow second. The tint is the
// surface step it has always worn — one fill, the primary cue — and under
// it a cast shadow from effects/depth, which that package reserves for
// what floats and can leave. The sidebar qualifies under that criterion
// as written: it floats above the window's ground, inset with the ground
// showing around it on every side, and it can leave, dismissed from its
// own toggle — and the platform's own sidebar visibly casts one. The
// shadow's geometry takes the middle rung of the elevation ladder: above
// the one-dp fringe of a card raised in place, below the toasts that
// float over everything including this pane. What it costs is
// effects/depth's documented price — nine paint operations per frame —
// paid only while the pane stands.
func (f *frameState) layoutRailPane(gtx layout.Context, tok themeTokens, pane image.Rectangle, sb layout.Widget) {
	r := gtx.Dp(unit.Dp(railRadiusDp))
	depth.Shadow(gtx, pane, tokens.Level2, r, 1)
	rr := clip.RRect{Rect: pane, NE: r, NW: r, SE: r, SW: r}
	paint.FillShape(gtx.Ops, tok.col.Surface, rr.Op(gtx.Ops))
	defer rr.Push(gtx.Ops).Pop()
	defer op.Offset(pane.Min).Push(gtx.Ops).Pop()
	sgtx := gtx
	sgtx.Constraints = layout.Exact(pane.Size())
	sb(sgtx)
}

// paintDividerLine inks the note/aside divider under the pointer, and for
// as long as it is being dragged. At rest it draws nothing: the trailing
// column stands on its own surface now, so the seam between the document
// and the panel is a change of ground — the same edge the sidebar has on
// the other side, and that one has never needed a line down it either.
// A hairline three dp off a hard edge is not a seam, it is a stray mark.
//
// What the resting state cannot say is that the seam moves, and a fresh
// reviewer did read the whole split as fixed. So the line appears where
// the pointer is: a grab area wide enough to catch a hand, ink only while
// a hand is on it, and the cursor already saying which way it goes.
func (f *frameState) paintDividerLine(gtx layout.Context, tok themeTokens, area image.Rectangle) {
	if !f.hovering && !f.dragging {
		return
	}
	w := max(gtx.Dp(unit.Dp(2)), 2)
	ink := tok.col.Ramps.Neutral.Step(500)
	inset := gtx.Dp(unit.Dp(railMarginDp))
	x := area.Min.X + (area.Dx()-w)/2
	line := image.Rect(x, area.Min.Y+inset, x+w, area.Max.Y-inset)
	if line.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, ink, clip.Rect(line).Op())
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
				return f.layoutRailToggle(gtx, m, tok)
			}),
			layout.Rigid(dragSpacer(unit.Dp(tok.sp.S3))))
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutVaultName(gtx, m, tok)
		}),
		layout.Flexed(1, dragFill),
		// The trailing inset is the backlinks column's own: the row ends
		// where the column under it ends. With the actions gone it is the
		// tail of one long drag rather than the gap after a control.
		layout.Rigid(dragSpacer(asideInsetDp)))
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// toolbarLeading is where the row's own content may start: the trailing
// edge of the window's control buttons where the platform puts them in
// the content area, and the ordinary edge inset where it does not.
func toolbarLeading() unit.Dp {
	if lead := desktop.LeadingInset(); lead > 0 {
		return lead
	}
	return frameEdgeDp
}

// dragSpacer is a fixed-width gap in the chrome row that moves the window
// when it is dragged. The row stands in the strip the native title bar
// would otherwise own, so the window's top edge is only a handle where
// the row says it is — and it says so over its empty space alone, since a
// move action swallows the press before any control beneath it sees one.
func dragSpacer(w unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return windowDragArea(gtx, gtx.Dp(w))
	}
}

// dragFill is the row's flexible middle: the whole gap between the vault
// name and the trailing actions, draggable end to end.
func dragFill(gtx layout.Context) layout.Dimensions {
	return windowDragArea(gtx, gtx.Constraints.Min.X)
}

// windowDragArea declares a w-wide, row-tall move handle at the current
// offset and takes the row's full height as its size, so that the handle
// reaches the top and bottom of the row rather than a band around the
// line the labels sit on.
func windowDragArea(gtx layout.Context, w int) layout.Dimensions {
	h := gtx.Constraints.Max.Y
	if w <= 0 || h <= 0 {
		return layout.Dimensions{Size: image.Pt(max(w, 0), 0)}
	}
	defer clip.Rect{Max: image.Pt(w, h)}.Push(gtx.Ops).Pop()
	system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(w, h)}
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
