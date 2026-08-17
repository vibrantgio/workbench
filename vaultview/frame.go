// frame.go is the vault window's chrome and its column composition: one
// toolbar row across the top — the rail toggle, the vault's name, and the
// two vault actions — over the folder rail, the note column and the
// backlinks aside, the last two parted by a draggable divider.
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
// The folder rail is not one of the columns. It is a pane: an inset
// rounded rectangle a surface step above the window's own ground, with
// the ground visible on all four sides of it. A full-height column would
// have to begin somewhere, and wherever it began — under the toolbar row,
// as it once did — the eye read the start as a seam rather than as a
// decision. Floating the pane makes every one of its edges deliberate,
// and it is the idiom the platform's own document windows use. Hidden,
// the pane takes no width at all and the note column reflows from the
// window's leading edge.
//
// The window's ground is the same paper the note column lies on, so the
// note has no edge of its own to draw and the toolbar row sits on the
// document rather than on a band above it. What is furniture — the rail
// pane — says so by standing a step off that ground; what is document
// simply is the ground. The divider between the note and the backlinks
// follows from this: with no column edges left to butt against, it is a
// hairline drawn down the middle of a wide-enough grab area, not a bar.
//
// Where the row sits is a platform fact, not a taste. Under the
// full-size-content treatment the content extends behind the native
// title bar, and the window control buttons are the only thing standing
// in it — so the row takes that strip for itself: the window is asked to
// centre its buttons on the row's own middle line, and the row starts
// after their measured trailing edge rather than at the window's edge.
// The strip is not the system's to click in under this treatment, so the
// row's controls are pressable where they stand; what the strip does not
// hand over is the window drag, which the row claims back by declaring a
// move action over the parts of itself that hold no control. Away from
// the treatment both measurements report zero, the buttons stay where
// their platform puts them, and the row lays out from the left edge.

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

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
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

	// The rail toggle's mark: a pane in outline, with a leading column
	// the width of a sidebar filled in when the rail stands.
	toggleMarkWDp   = 16
	toggleMarkHDp   = 12
	toggleMarkColDp = 5

	// The rail pane's margin from the window's edges and from the toolbar
	// row above it, and its corner radius.
	railMarginDp = 8
	railRadiusDp = 10
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
	rescanClick widget.Clickable
	switchClick widget.Clickable

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
	sb := renderTree(shaper, m, colors, sp, rad, typo, den)
	main := renderNotePage(shaper, m, colors, sp, typo, den)
	av := &asideView{list: list.NewState()}
	as := func(gtx layout.Context) layout.Dimensions { return av.layout(gtx, m, tok) }
	return func(gtx layout.Context) layout.Dimensions {
		return st.layout(gtx, m, tok, sb, as, main)
	}, st
}

// frameGeom is where the frame puts things below the toolbar row: the
// rail pane's rectangle in frame coordinates, empty when the rail is
// hidden or the window has no room for it, and the leading edge the note
// column starts from — past the pane and its margin when the rail
// stands, the window's own edge when it does not.
type frameGeom struct {
	pane     image.Rectangle
	contentX int
	rowTop   int
	rowH     int
}

// frameGeometry measures the pane and the content edge. It is separate
// from the drawing so the arrangement can be asserted without a frame:
// the four margins around the pane, and the reflow when the rail goes.
func frameGeometry(gtx layout.Context, size image.Point, barH int, hidden bool) frameGeom {
	g := frameGeom{rowTop: barH, rowH: max(size.Y-barH, 0)}
	if hidden || g.rowH <= 0 {
		return g
	}
	margin := gtx.Dp(unit.Dp(railMarginDp))
	railW := gtx.Dp(unit.Dp(treeWidthDp))
	// A pane may never take more than half the window: a narrow window
	// owes the note a readable column before it owes the rail its width.
	if maxW := size.X/2 - 2*margin; railW > maxW {
		railW = maxW
	}
	paneH := g.rowH - 2*margin
	if railW <= 0 || paneH <= 0 {
		return g
	}
	g.pane = image.Rect(margin, barH+margin, margin+railW, barH+margin+paneH)
	g.contentX = g.pane.Max.X + margin
	return g
}

// layout draws the toolbar row, then the rail pane and the columns below
// it, in the order they read: rail, note, divider, aside.
func (f *frameState) layout(gtx layout.Context, m Model, tok themeTokens, sb, as, main layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	// The window's ground is the note's own paper, not a chrome fill: the
	// document is what the window is, and only the rail pane rises off it.
	paint.FillShape(gtx.Ops, tok.col.Background, clip.Rect{Max: size}.Op())

	barH := gtx.Dp(toolbarHeight(tok))
	if barH > size.Y {
		barH = size.Y
	}
	bgtx := gtx
	bgtx.Constraints = layout.Exact(image.Pt(size.X, barH))
	f.layoutToolbar(bgtx, m, tok)

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
		st := op.Offset(image.Pt(g.contentX+mainW+dividerW, g.rowTop)).Push(gtx.Ops)
		agtx := gtx
		agtx.Constraints = layout.Exact(image.Pt(asidePx, g.rowH))
		as(agtx)
		st.Pop()
	}

	return layout.Dimensions{Size: size}
}

// layoutRailPane fills the pane and lays the rail inside it, clipped to
// the same rounded rectangle so a scrolled row cannot cross a corner.
func (f *frameState) layoutRailPane(gtx layout.Context, tok themeTokens, pane image.Rectangle, sb layout.Widget) {
	r := gtx.Dp(unit.Dp(railRadiusDp))
	rr := func() clip.RRect {
		return clip.RRect{Rect: pane, NE: r, NW: r, SE: r, SW: r}
	}
	paint.FillShape(gtx.Ops, tok.col.Surface, rr().Op(gtx.Ops))
	defer rr().Push(gtx.Ops).Pop()
	defer op.Offset(pane.Min).Push(gtx.Ops).Pop()
	sgtx := gtx
	sgtx.Constraints = layout.Exact(pane.Size())
	sb(sgtx)
}

// paintDividerLine draws the note/aside divider as a hairline down the
// middle of its grab area, held clear of the toolbar row and the window's
// bottom edge by the pane's own margin. The area stays wide enough to
// catch a pointer; only the ink is thin, because with the note and the
// backlinks sharing one ground there are no column edges for a bar to
// separate — just a seam the reader may move.
//
// A hairline that never changes is a hairline nobody knows they may take
// hold of — a fresh reviewer read the whole split as fixed — so under the
// pointer, and for as long as it is being dragged, the line thickens and
// darkens. That is the affordance; the cursor already says which way.
func (f *frameState) paintDividerLine(gtx layout.Context, tok themeTokens, area image.Rectangle) {
	w := max(gtx.Dp(unit.Dp(1)), 1)
	ink := tok.col.Divider
	if f.hovering || f.dragging {
		w = max(gtx.Dp(unit.Dp(2)), 2)
		ink = tok.col.Ramps.Neutral.Step(500)
	}
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

// layoutToolbar draws the chrome row on one baseline: the window's own
// controls lead, the rail toggle follows them, the vault's name stands
// beside it as what this window is showing, and the two vault actions
// trail as a group at the far edge.
//
// The leading space is a measurement, not a constant: the window controls
// report where they end, and the row adds its own gap after that, because
// the reported edge is the bare glass. Where the window has no such
// controls the measurement is zero and the row falls back to its ordinary
// edge inset.
//
// The vault's name is the row's own affordance rather than a breadcrumb
// segment: it is window state, and as a crumb it promised a parent to
// climb to that a vault does not have. Pressing it still returns the
// folder tree to its root, which is what the crumb did.
func (f *frameState) layoutToolbar(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		// The window controls' own space is left alone: a move action
		// declared over the buttons would fight them for the press.
		layout.Rigid(complayout.HSpacer(float32(f.toolbarLeading()))),
		layout.Rigid(dragSpacer(frameGapDp)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutRailToggle(gtx, m, tok)
		}),
		layout.Rigid(dragSpacer(unit.Dp(tok.sp.S3))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutVaultName(gtx, m, tok)
		}),
		layout.Flexed(1, dragFill),
		layout.Rigid(toolbarAction(&f.rescanClick, "Rescan", tok, Rescan{})),
		layout.Rigid(dragSpacer(frameGapDp)),
		// Title case, which is what the platform's own controls use.
		layout.Rigid(toolbarAction(&f.switchClick, "Switch Vault", tok, SwitchVault{})),
		layout.Rigid(dragSpacer(frameEdgeDp)),
	)
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

// layoutRailToggle draws the rail's show/hide control: a pane outline
// with its leading column filled while the rail stands and hollow while
// it is away, so the mark is a picture of the window's own state rather
// than a menu's three bars — which is what a fresh reviewer read the
// three bars as. It is drawn rather than typeset, so the mark does not
// depend on a face carrying it.
func (f *frameState) layoutRailToggle(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	if f.toggleClick.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	label := "Hide the folder rail"
	if m.SidebarHidden {
		label = "Show the folder rail"
	}
	return f.toggleClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		fg := tok.col.Text
		if m.SidebarHidden {
			fg = tok.col.Ramps.Neutral.Step(600)
		}
		w := gtx.Dp(unit.Dp(toggleMarkWDp))
		h := gtx.Dp(unit.Dp(toggleMarkHDp))
		stroke := max(gtx.Dp(unit.Dp(1)), 1)
		rad := max(gtx.Dp(unit.Dp(2)), 1)
		// The mark is centred in a row-tall box, so the whole height of
		// the row is pressable rather than the ink alone.
		boxH := max(gtx.Constraints.Max.Y, h)
		top := (boxH - h) / 2
		box := image.Rect(0, top, w, top+h)
		rr := clip.RRect{Rect: box, NE: rad, NW: rad, SE: rad, SW: rad}
		paint.FillShape(gtx.Ops, fg, clip.Stroke{
			Path:  rr.Path(gtx.Ops),
			Width: float32(stroke),
		}.Op())
		if !m.SidebarHidden {
			cw := max(gtx.Dp(unit.Dp(toggleMarkColDp)), 1)
			defer clip.RRect{Rect: box, NE: rad, NW: rad, SE: rad, SW: rad}.Push(gtx.Ops).Pop()
			paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(box.Min.X, box.Min.Y, box.Min.X+cw, box.Max.Y)).Op())
		}
		return layout.Dimensions{Size: image.Pt(w, boxH)}
	})
}

// toolbarAction renders one chrome-row affordance: a pressable label that
// emits its message on the frame the press lands.
func toolbarAction(click *widget.Clickable, label string, tok themeTokens, msg mvu.Message) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if click.Clicked(gtx) {
			mvu.MessageOp{Message: msg}.Add(gtx.Ops)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			// Full text contrast, not the low-contrast neutral step: a
			// bare label at that step reads as a disabled control rather
			// than a live one, which is what a fresh reviewer called it.
			return drawLabel(gtx, tok.shaper, label, tok.typ.LabelLarge, tok.col.Text)
		})
	}
}
