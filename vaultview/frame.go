// frame.go is the vault window's chrome and its column composition: the
// sidebar pane down the leading edge, and beside it the content area —
// one chrome row across its top, the note column and the backlinks aside
// under it parted by a splitter, and one status bar across its
// foot. What the foot band carries is in status.go.
//
// The composition is app-local rather than the vocabulary's three-column
// shell because that shell pins its top slot to a full navbar band
// (ControlHeight plus twice the vertical control padding, 52 dp at the
// comfortable density) and this window's chrome is a single tight row.
// Everything else here — a splitter on each of the window's two
// boundaries, the op order that makes Tab follow the reading order — is
// the shell's arrangement.
//
// The sidebar is the window's leading chrome column, set into the window as
// a PANE: an inset rounded panel one margin in from the window's leading,
// top and bottom edges with the window's own plane showing in those
// margins, flush against the note on the fourth side, bounded by its own rim
// and the shadow it casts and by no seam anywhere. The inset, the rim, the
// shadow, the strip's arithmetic, the hidden-takes-no-width contract and the
// recall convention are patterns/pane's; what is left here is the column
// that stands in it. No band crosses above the panel. Its toggle stands bare
// at its top trailing corner with the strip's empty middle moving the
// window.
// The vault's own actions stand in the toolbar band. Hidden, the rail takes
// no width at all and the note column reflows from the window's leading
// edge, so the chrome row carries the toggle that brings the rail back — a
// control that travels with the rail cannot be the one that recalls it.
//
// The window control buttons are measured from the window's own top and
// leading glass and from nothing drawn under them: they stay put whatever
// the application puts beneath, so the pane arriving or leaving may not
// move them. The pane's top strip is therefore cut deep enough to hold them
// with air to spare, and its toggle centres on their line. That line is
// what everything else at the top of the window stands on: the pane's
// toggle, the vault's name, and the toggle the chrome row shows once the
// pane is away, so the two halves of the sidebar switch hold one height
// between them. The row is cut to the same depth as that strip, so a control
// centred in it stands on the buttons' line without being told to.
//
// What the row carries at its trailing end is what acts on the document: the
// vault's two actions and the find, each the platform's bordered toolbar
// control, in the order and at the spacing the stored bands read.
//
// The content area's fill is the same surface the note column lies on, so
// the note draws no edge of its own and the chrome row sits on the document
// rather than on a band above it. Both of the window's side columns are
// chrome drawn at the same level, a measured step under the content in
// either scheme, and there they part: the leading one is a PANE, set into
// the window's plane and bounded by its rim, because a button slides it out
// of the window; the trailing one is FLUSH, running to the window's own
// edges and parted from the document by one seam, because nothing dismisses
// it.
//
// Both boundaries are the reader's to move, and both are the same
// pattern: the seam between two regions made operable, thickening and
// taking a firmer colour while a hand is in the band it is taken by,
// which is the one thing a resting edge cannot say. Leading, the line is
// the panel's own rim, so the splitter draws over it in the rim's own
// colour rather than beside it in a second — a resting window is the same
// window whether or not that edge can be taken hold of — and it therefore
// runs exactly where that rim runs straight: from where the panel's top
// corner lets go to where its bottom corner begins, the toolbar band's rows
// included, since the panel is one object from its top edge to its foot. The
// trailing boundary keeps the whole height it has until what a content-side
// boundary is painted with is ruled.
//
// The two boundaries answer to the note column between them: neither is
// dragged past the point where the note would be left with less than a
// column of prose.
//
// Under the full-size-content treatment the content extends behind the
// native title bar, so the top strip is the application's to lay out in.
// The chrome row takes the content area's share of it and never crosses
// above the sidebar. The strip is not the system's to click in under this
// treatment, so the row's controls are pressable where they stand; what the
// strip does not hand over is the window drag, which the row and the pane's
// strip claim back by declaring a move action over the parts of themselves
// that hold no control. Away from the treatment the measurements report
// zero, the buttons stay where their platform puts them, and the row lays
// out from its own edge inset.

package main

import (
	"image"
	"image/color"
	"path"
	"strings"

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

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/input"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// Frame layout constants. The aside bounds follow the three-column
// shell's, so the two compositions resize alike.
const (
	frameEdgeDp = 12
	frameGapDp  = 16

	// frameGapDp's counterpart at the trailing boundary: the air the
	// content area keeps between the note column's edge and the seam the
	// aside stands behind. The splitter's own band is wider than the line
	// and reaches into both columns, so this gap is not the band — it is
	// what stops the note's last pixel standing hard against the seam.
	frameSplitterDp = 6

	frameAsideDp    = 320
	frameMinAsideDp = 160
	frameMaxAsideDp = 640

	// railToggleWidthDp is the width the platform's bordered toolbar control
	// takes around one symbol, which is what every control standing in this
	// window's band is drawn as — both halves of the sidebar switch, each
	// segment of the window's navigation, the vault's two actions and the
	// search: the 24 dp mark box with the platform's measured 7 dp of clear
	// band on each side. components/button draws it and owns the measurement;
	// the number is named here because the window reasons about where its
	// controls stand and what is beside them.
	railToggleWidthDp = markLargeDp + 2*7

	// bandGapDp is the room the band leaves between two bordered controls
	// standing apart. MEASURED at 1x: finder-window-light.png leaves 16 px
	// between its view pop-up (ending x=742) and the group pull-down beside
	// it (from x=759), and the same window's dark capture leaves 16 between
	// its view control and the pull-down after it. The platform's own spread
	// across the stored bands runs 8 (mail-window.png, inside one cluster of
	// three), 14 (notes-toolbar.png, compose to the group beside it), 16 and
	// 18 (finder-window-light.png: 16 between its two pull-downs, 18 from the
	// second to the trio and from the trio to the search) and 28
	// (mail-window.png, between clusters). Finder's 16 is the reading the
	// ruling names and the one already recorded in
	// reference/macos/controls.md.
	bandGapDp = 16

	// bandNameGapDp is the room the band leaves between the navigation and
	// the document's name beside it. MEASURED at 1x: in
	// finder-window-light.png the back/forward pair's fill ends at x=398 and
	// the title's first painted column stands at x=413, and in
	// finder-window-untinted-dark.png the same pair ends at x=398 with its
	// title's first painted column at x=412 — fourteen clear columns, read twice,
	// and the same fourteen notes-toolbar.png leaves between two bordered
	// controls.
	bandNameGapDp = 14

	// bandTrailingDp is what the band leaves between its last control and
	// the window's own trailing edge. MEASURED at 1x, all four stored
	// toolbar windows agree on eight: finder-window-light.png's search
	// capsule ends x=991 in a window 1000 wide, finder-window-untinted-dark's
	// search field ends 1322 in one 1331 wide, mail-window.png's search
	// recess ends 1191 in one 1200 wide and voicememos-window.png's ends 967
	// in one 976 wide.
	bandTrailingDp = 8

	// toolbarShadowReachDp is how far the drop shadow a bordered toolbar
	// control casts carries past it in the light appearance.
	// components/button draws it and owns the measurement; the number is
	// named here because the window's own assertions read the band around a
	// control and have to know what the control reaches.
	toolbarShadowReachDp = 23

	// railMarginDp is the window's small edge margin: the inset the rail's
	// panel stands off the window's leading, top and bottom edges, the air
	// its own top strip keeps between its toggle and the panel's trailing
	// edge, and the air the trailing column leaves either side of its
	// scrollbar. It is the pane pattern's own margin, named here because the
	// window spends it in places the pattern knows nothing about.
	railMarginDp = pane.MarginDp

	// seamDp is what any chrome boundary in this window paints: a hairline,
	// the width the platform's own splitters take. It is both the panel's
	// rim and the flush column's seam, so that a window whose two vertical
	// boundaries are drawn for different reasons still draws them at one
	// weight. A seam runs the window's whole height, band included, so its
	// width is the width of the scar it leaves across every band it crosses.
	seamDp = splitter.SeamDp

	// buttonInsetDp is how far the window control buttons sit in from the
	// window's own top and leading edges — the drawn circles' own edges,
	// equal on both axes, measured from the glass and from nothing else.
	// The number is the platform's, read off its sidebar apps on this
	// display: Finder, Mail, Notes and Voice Memos all draw the circles
	// nineteen pixels in from both edges, which at one pixel per dp is
	// nineteen. The toolkit left alone lands them at nine, the inset the
	// platform's compact windows use, so the placement is stated rather than
	// defaulted. The centre line and the diameter follow from this one
	// number by the platform's own rule, which the pattern applies.
	buttonInsetDp = pane.ButtonInsetDp

	// paneStripDp is the panel's own top strip: deep enough to hold the
	// buttons where the window puts them with the same air below them as
	// above. The buttons' inset is measured from the glass and the strip
	// from the panel's own edge, so the strip owes the margin back at both
	// ends, which lands the buttons' centre line on the strip's own middle —
	// the line the panel's toggle centres on, so the two sit level.
	paneStripDp = pane.StripDp

	// bandDp is the toolbar band the content column carries beside the
	// panel: the deeper number, since the panel is set one margin into it.
	// The buttons stand on its middle line too, so a control centred in the
	// band and one centred in the panel's strip sit level.
	bandDp = pane.BandDp
)

// windowButtons is where this window's three control buttons stand, derived
// from the inset above by the rule the platform's own windows follow. Every
// number in it is the window's: no rail state, no screen and no pane enters
// into any of them.
var windowButtons = pane.Buttons

// toolbarHeight is the chrome row's depth: the platform's toolbar band, 52
// dp — a 36 dp control with 8 above it and 8 below — which is the depth every
// stored toolbar capture measures and the band the panel beside it is set one
// margin into. The row is what this window puts IN that band, so the band
// settles its depth and the text standing in it does not.
//
// It takes no tokens: the band is the window's and the same in every density,
// as the pane's strip beside it is.
func toolbarHeight() unit.Dp {
	return unit.Dp(bandDp)
}

// bandSurface is the fill a control standing at the TRAILING end of this
// window's band stands on: the trailing column's, which runs the window's
// full height and rises into the band. The band carries no fill of its own —
// it is the fill of whatever region lies under it, continued upward
// (reference/macos/controls.md, "a toolbar band carries no fill of its own")
// — and the trailing end of this window's band lies over the inspector.
func bandSurface(c tokens.PlatformColors) color.NRGBA { return chromeSurface(c) }

// bandFind is the find in the page as the chrome row carries it: the state
// the query left it in, and the search field instance drawing it. A frame
// laid out for measurement carries neither and the band shows no find at all.
type bandFind struct {
	find  *pageFind
	field layout.Widget
}

// frameState is the vault frame's per-subscription state: the toolbar's
// clickables, the two splitters' hands and the widths they move. It is
// touched only on the frame goroutine.
type frameState struct {
	toggleClick widget.Clickable

	// The window's navigation, which stands in the band as Finder keeps it:
	// the segmented pair at the leading end of the content column's share,
	// with the document's name bare beside it.
	backClick widget.Clickable
	fwdClick  widget.Clickable

	// The band's document actions, and the control that opens the find in
	// the page where the platform's own band keeps its search: Finder draws
	// a magnifier capsule at the trailing end of its band and expands it
	// into a field, which is what this window's shortcut now has an
	// affordance for.
	rescanClick widget.Clickable
	switchClick widget.Clickable
	findClick   widget.Clickable

	// band is the find in the page as the chrome row carries it: the state
	// the query left it in, and the search field instance that draws it.
	// Both belong to the note column — the query is marked in the note's own
	// document — and the band borrows them, because the toolbar is where the
	// platform keeps a window's search.
	band bandFind

	// The window's two boundaries, each the seam between two regions made
	// operable. Both are the same pattern; what differs is the edge each
	// stands on — the pane's own hairline leading, the flush column's
	// plain seam trailing — and how much of that edge a hand may take.
	railSplitter  splitter.State
	asideSplitter splitter.State

	railW  unit.Dp
	asideW unit.Dp

	// widths keeps this frame's two column widths across launches. It is
	// nil wherever a frame is laid out for measurement rather than for a
	// reader, so a stored render arranges a window nobody is sitting at
	// without writing over what a real one kept.
	widths *columnMemory

	// leading pins the row's leading inset instead of measuring it. The
	// measurement is a live window's: off a frame it reports zero, and on
	// one it reports whatever this machine's macOS puts the control buttons
	// at. Neither is a number a stored image may depend on, so the static
	// render path states one. The live path leaves this nil and measures.
	leading func() unit.Dp

	// geom is the arrangement the last frame laid out, kept so the
	// composition can be measured after the fact: the chrome budget is a
	// property of what was drawn, and recomputing it from the tokens would
	// assert the arithmetic rather than the frame.
	geom frameGeom
}

// newFrameState seats a frame on the arrangement it opens with. Both
// column widths are state rather than constants because both boundaries
// belong to the reader: each is a splitter's to move and the memory's to
// keep.
func newFrameState(w columnWidths) *frameState {
	return &frameState{railW: w.Rail, asideW: w.Aside}
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
// layout.Widget streams so a theme change re-renders them; the toolbar reads
// the token and model snapshots at frame time.
//
// The two side columns open at the widths the memory hands over — what the
// reader last left, or the defaults — and that same memory is what every
// frame reports its arrangement back to.
func vaultFrame(
	loadModel func() Model,
	loadTok func() themeTokens,
	widths *columnMemory,
	find *pageFind,
	sidebar, aside, main, search rx.Observable[layout.Widget],
) rx.Observable[layout.Widget] {
	columns := rx.CombineLatest4(sidebar, aside, main, search)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := newFrameState(widths.widths())
		st.widths = widths
		return rx.Map(columns, func(next rx.Tuple4[layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			sbW, asW, mainW, fieldW := next.First, next.Second, next.Third, next.Fourth
			return func(gtx layout.Context) layout.Dimensions {
				st.band = bandFind{find: find, field: fieldW}
				return st.layout(gtx, loadModel(), loadTok(), sbW, asW, mainW)
			}
		})
	})
}

// renderWindow is the static counterpart of the whole vault window, used
// by the window golden: the chrome row over the rail pane, the note
// column and the backlinks aside, all with fresh widget.Clickable state, laid
// out once from pre-resolved tokens and processing no events. It is the only
// renderer in this package that composes rather than filling one slot.
//
// It opens at the default column widths and keeps no memory of its own: a
// stored image may not depend on an arrangement whoever runs it happens to
// have left behind, and may not write over one either.
//
// The leading inset is a parameter and not a measurement here, for the
// reason [frameState.leading] gives: the value the live row lays out
// from belongs to a window this render does not have.
//
// The frame is returned beside the layout.Widget so that what a render
// arranged can be read back once it has run — the chrome budget is measured
// off the same composition the golden stores, not off a second one built to
// be measured.
func renderWindow(
	shaper *text.Shaper,
	m Model,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	den tokens.Density,
	leading unit.Dp,
) (layout.Widget, *frameState) {
	return renderWindowFinding(shaper, m, colors, sp, rad, typo, den, leading, pageFind{})
}

// renderWindowFinding is renderWindow with the page's find in the state a
// query has left it, which is the only way a still image can be taken of a
// note being searched.
func renderWindowFinding(
	shaper *text.Shaper,
	m Model,
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	den tokens.Density,
	leading unit.Dp,
	find pageFind,
) (layout.Widget, *frameState) {
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	st := newFrameState(defaultWidths())
	st.leading = func() unit.Dp { return leading }
	cur := &docCursor{}
	sb := renderTree(shaper, m, colors, sp, rad, typo, den, leading)
	// The band and the page share one find: the field stands in the band and
	// the matches are marked in the page, and a still image of a note being
	// searched has to show the one query in both places.
	pf := &find
	// The band's find field is the static search field in the state the query
	// leaves it: no editor, no events, the query drawn where the reader typed
	// it and the field focused, which is where a find in the page is worked
	// from. It is the toolbar's recess and not the sidebar's field.
	if find.open {
		// Built per frame rather than once, for the count alone: what the
		// query has found is the document's answer, and the document lays
		// out before the band does. A field built here and reused would
		// carry the count the page had before it was searched.
		st.band.field = func(gtx layout.Context) layout.Dimensions {
			return input.RenderSearch(shaper, "Search", colors, sp, rad, typo.BodyLarge, den,
				input.RenderState{
					Text: pf.query, Focused: true,
					Count:   pf.label(),
					Variant: input.Chrome, Region: input.Toolbar,
					Surface: bandSurface(colors),
				})(gtx)
		}
	}
	st.band.find = pf
	main := renderNotePageInto(cur, shaper, m, colors, sp, typo, den, pf)
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
// Only the content area has a chrome row and a status bar. The rail runs to
// the window's top and bottom edges, so there is nothing above it and
// nothing below it: no band at either end, and nothing else to measure.
type frameGeom struct {
	pane     image.Rectangle
	contentX int
	rowTop   int
	rowH     int
	footTop  int
}

// frameGeometry measures the pane and the content area. It is separate
// from the drawing so the arrangement can be asserted without a frame:
// that the rail's panel stands one margin inside the window's leading, top
// and bottom edges, that the content reflows to the window's own edge when
// the rail goes, and that the content area's columns run between its two
// bands and no further.
//
// The bands are taken in the order they matter when there is no room for
// either: the chrome row first, because it carries the control that brings
// a dismissed pane back, and the status bar out of what is left, because a
// window with no height for a document has nothing to report about one.
func frameGeometry(gtx layout.Context, size image.Point, railW unit.Dp, barH, footH int, hidden bool) frameGeom {
	h := max(size.Y, 0)
	barH = min(max(barH, 0), h)
	footH = min(max(footH, 0), h-barH)
	g := frameGeom{rowTop: barH, rowH: h - barH - footH, footTop: h - footH}
	// The run is the pattern's: one margin inside the window's leading, top
	// and bottom edges, never more than half the window wide, and empty in
	// every state where there is no rail to draw — hidden above all, where
	// the emptiness IS the contract and the content beside it reflows to the
	// window's own edge.
	g.pane = pane.Bounds(gtx, size, railW, hidden)
	if !g.pane.Empty() {
		g.contentX = g.pane.Max.X
	}
	return g
}

// layout draws the sidebar pane, then the content area's chrome row and
// the columns below it, in the order they read: rail, chrome row, note,
// splitter, aside — and last the status bar under all of them. That order
// is the focus ring's too, and the bar is last in it by holding nothing
// the ring can stop on.
func (f *frameState) layout(gtx layout.Context, m Model, tok themeTokens, sb, as, main layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	barH := gtx.Dp(toolbarHeight())
	if barH > size.Y {
		barH = size.Y
	}

	// Both boundaries are taken hold of before anything is laid out, so
	// that the whole frame is drawn where this frame's drag left them
	// rather than where the last one did. The rail's goes first: the
	// aside's bounds are measured from where the pane ends, and a rail
	// that moved in this same frame has already moved that.
	footH := gtx.Dp(statusBarHeight(tok))
	g := frameGeometry(gtx, size, f.railW, barH, footH, m.SidebarHidden)
	if !g.pane.Empty() {
		f.railSplitter.Update(gtx, f.railProps(gtx, tok, size, g))
		g = frameGeometry(gtx, size, f.railW, barH, footH, m.SidebarHidden)
	}
	f.asideSplitter.Update(gtx, f.asideProps(gtx, tok, size, g))
	f.geom = g

	// The window's own plane, under everything: the rail's panel is set into
	// it and the plane is what shows in the margins around it. MEASURED,
	// voicememos-multi-folder-2026-09-18.png and
	// finder-window-untinted-light.png: the eight pixels either side of the
	// panel carry the window background and nothing else.
	paint.FillShape(gtx.Ops, tok.col.WindowBackground, clip.Rect(image.Rectangle{Max: size}).Op())

	// The content area stands on the note's own surface: the document is what
	// the window is. It starts where the rail's panel stops — flush against
	// it, which is the one side the panel is not set in from — and with the
	// rail gone it starts at the window's own leading edge.
	paint.FillShape(gtx.Ops, tok.col.TextBackground,
		clip.Rect(image.Rect(g.contentX, 0, size.X, size.Y)).Op())

	// The rail is the vocabulary's PANE and nothing here draws it: the inset
	// rounded panel, its rim, the shadow it casts, the chrome fill and the
	// clip that keeps a scrolled row off its edge are all the pattern's.
	// What is left to this window is which column stands in it, and what
	// stands behind the two corners the panel rounds away from on its flush
	// side — the document, not the plane.
	if !g.pane.Empty() {
		pane.FillTrailingCorners(gtx, tok.col.TextBackground, g.pane)
		pane.Layout(gtx, tok.col, g.pane, sb)
	}

	c := contentColumns(gtx, size, g.contentX, f.asideW)
	asidePx, mainW, asideX := c.aside, c.note, c.asideX

	// The trailing column's own surface, painted before the chrome row and
	// running the window's full height: the outline and the backlinks are
	// chrome, so the column wears the platform's chrome material. Full
	// height, so the surface does not read as a block hanging off the chrome
	// row and the two columns are the same shape, one down each edge, with
	// the document between them.
	if asidePx > 0 {
		paint.FillShape(gtx.Ops, chromeSurface(tok.col),
			clip.Rect(image.Rect(asideX, 0, size.X, size.Y)).Op())
	}

	// The note column is laid out BEFORE the chrome row above it and
	// replayed after, so the row's ops still stand ahead of the column's in
	// the reading order while the column's code has already run. The band
	// carries the find, and the find's keys and the count it reports are the
	// column's to settle: drawn the other way round the band would show the
	// query's previous count and open a frame after the shortcut was pressed.
	var (
		mainCall op.CallOp
		hasMain  bool
	)
	if main != nil && g.rowH > 0 {
		macro := op.Record(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(mainW, g.rowH))
		main(mgtx)
		mainCall = macro.Stop()
		hasMain = true
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

	if hasMain {
		st := op.Offset(image.Pt(g.contentX, g.rowTop)).Push(gtx.Ops)
		mainCall.Add(gtx.Ops)
		st.Pop()
	}

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

	// The shadow the rail's panel casts, after every column has painted its
	// own surface: the note column paints its own rows after the pane has
	// laid out — the pane comes first because it comes first in the reading
	// order — and would cover the ramp the panel cast on it. The pattern
	// cuts the panel's own box out of the drawing, so painting it here lands
	// what painting it under the panel landed.
	if !g.pane.Empty() {
		pane.PaintShadow(gtx, tok.col, g.pane)
	}

	// The window's two boundaries, last of everything, so that neither the
	// columns' rows nor the two bands crossing over them can cover a line
	// and so that a press this close to a boundary reaches the boundary.
	if !g.pane.Empty() {
		f.layoutRailSplitter(gtx, tok, size, g)
	}
	if asidePx > 0 {
		f.layoutAsideSplitter(gtx, tok, size, g)
	}

	// What this frame arranged, offered to the memory that keeps it —
	// last, so a boundary the hand moved earlier in this same frame is
	// reported at where it ended up rather than at where it was found.
	// Every frame reports and the memory drops what says nothing new,
	// since a boundary can move in any frame and no frame says whether
	// one did.
	f.widths.record(columnWidths{Rail: f.railW, Aside: f.asideW})

	return layout.Dimensions{Size: size}
}

// columns is the content area's horizontal arrangement for one frame:
// what the trailing column takes, what is left for the note, and where
// the boundary between them stands.
type columns struct {
	aside  int
	note   int
	asideX int
}

// contentColumns measures the content area beside the pane: the aside at
// the width it is kept at, cut to what the window has for it; the note
// column taking the rest; and the boundary one gap trailing of the note.
//
// The aside keeps an absolute width, so a window resize leaves it alone
// and the note column absorbs the change.
func contentColumns(gtx layout.Context, size image.Point, contentX int, asideW unit.Dp) columns {
	gap := max(gtx.Dp(unit.Dp(frameSplitterDp)), 1)
	aside := gtx.Dp(asideW)
	if avail := size.X - contentX - gap; aside > avail {
		aside = avail
	}
	aside = max(aside, 0)
	note := max(size.X-contentX-gap-aside, 0)
	return columns{aside: aside, note: note, asideX: contentX + note + gap}
}

// noteFloor is the room the note column is not squeezed out of: the
// narrowest measure it may lay out at and the insets either side of it,
// or what it already has where the window is too narrow for that much.
//
// A boundary may always be dragged the way that gives the note room and
// never the way that takes more of it away. That is why the floor gives
// way to what the column already has: at a window narrower than all three
// columns' minimums together the note is under its measure before anyone
// touches a boundary, and a bound that cut in front of where a boundary
// already stands would jump it out from under the hand taking hold of it.
func noteFloor(gtx layout.Context, note int) int {
	return min(gtx.Dp(unit.Dp(noteMinWidthDp)), note)
}

// railProps states the splitter on the rail panel's trailing edge, where the
// note stands flush against it and the panel's own RIM is the boundary
// between the two.
//
// So the line this splitter draws at rest is that rim: the same pixel, in
// the panel's own rim colour, drawn over it rather than beside it. A resting
// window is unchanged by the splitter being there; what the reader gains is
// a band to take the edge by and a thickening under the hand that takes it.
// A second line three dp off the first would read as a stray edge, and the
// seam colour here would lay a hairline of a different colour down the one
// boundary the panel already draws.
//
// The boundary is the rim's own leading edge and not the panel's trailing
// one, which is the pixel after it: the splitter draws from the boundary
// outward, so the panel's edge as the boundary would put the line in the
// note column instead of on the panel.
//
// The bounds are the rail's own, and the note's floor over them: the rail
// stops where widening it further would take the note under the narrowest
// column of prose it may be. The panel stands one margin inside the window's
// leading edge, so its width and its trailing edge differ by that margin,
// which is what the boundary carries and the report takes back off.
func (f *frameState) railProps(gtx layout.Context, tok themeTokens, size image.Point, g frameGeom) splitter.Props {
	seamPx := splitter.SeamWidth(gtx)
	marginPx := gtx.Dp(unit.Dp(railMarginDp))
	gap := max(gtx.Dp(unit.Dp(frameSplitterDp)), 1)
	c := contentColumns(gtx, size, g.contentX, f.asideW)

	lo := gtx.Dp(railMinWidthDp)
	hi := max(min(gtx.Dp(railMaxWidthDp), size.X-marginPx-gap-c.aside-noteFloor(gtx, c.note)), lo)

	scale := pxPerDp(gtx)
	return splitter.Props{
		Axis:     layout.Horizontal,
		Boundary: float32(g.pane.Max.X - seamPx),
		Min:      float32(marginPx + lo - seamPx),
		Max:      float32(marginPx + hi - seamPx),
		Colors:   tok.col,
		// At rest the line IS the panel's rim, so it draws the rim's own
		// colour; under a hand it firms as every other boundary in this
		// window does, which is the seam over the rail's own fill.
		Rest:    pane.RimColor(tok.col),
		Surface: chromeSurface(tok.col),
		OnChange: func(at float32) {
			f.railW = clampRail(unit.Dp((at + float32(seamPx-marginPx)) / scale))
		},
	}
}

// layoutRailSplitter draws that hand-hold over the rail panel's trailing
// edge, along the rows that edge runs straight: the line this splitter draws
// IS the panel's rim, so it runs exactly where the rim runs and stops where
// the rim curves away, which pane.EdgeSpan is the span of. A line carried
// through a rounded corner would leave a pixel of itself out on the window's
// plane. The hand-hold runs the same rows as the line — a boundary a reader
// may take hold of along part of the length it is DRAWN at reads as two
// boundaries, one live and one not.
func (f *frameState) layoutRailSplitter(gtx layout.Context, tok themeTokens, size image.Point, g frameGeom) {
	top, bottom := pane.EdgeSpan(gtx, g.pane)
	h := bottom - top
	if g.pane.Dy() <= 0 || h <= 0 {
		return
	}
	st := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	sgtx := gtx
	sgtx.Constraints = layout.Exact(image.Pt(size.X, h))
	f.railSplitter.Layout(sgtx, f.railProps(gtx, tok, size, g))
	st.Pop()
}

// layoutAsideSplitter draws the trailing boundary from the toolbar band's
// lower edge to the window's foot. The band is one across the window's
// columns and no line crosses it — pane.SeamTop is the row it stops on — so
// the trailing boundary stops there as the rail's does, and the band reads as
// one across all three columns.
func (f *frameState) layoutAsideSplitter(gtx layout.Context, tok themeTokens, size image.Point, g frameGeom) {
	top := pane.SeamTop(gtx, image.Rectangle{Max: size})
	h := size.Y - top
	if h <= 0 {
		return
	}
	props := f.asideProps(gtx, tok, size, g)
	// The hand-hold is stated in window rows and this splitter is laid out
	// from the band's foot, so the span moves with the origin.
	props.HitSpan = splitter.Span{Min: g.rowTop - top, Max: g.rowTop + g.rowH - top}
	st := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	sgtx := gtx
	sgtx.Constraints = layout.Exact(image.Pt(size.X, h))
	f.asideSplitter.Layout(sgtx, props)
	st.Pop()
}

// asideProps states the splitter on the trailing column's leading edge.
//
// That column is INTEGRAL CHROME — fixed, flush, with no toggle and no
// way to leave — so it is not outlined the way the rail is. What it takes
// instead is the plain seam the platform gives its own flush side:
// Voice Memos carries no outline there at all and parts its panes with
// a one-pixel seam running from the window's top edge to its bottom,
// band included, and Notes does the same between its list and its note.
// So the line here is the Seam token's, not the pane's edge colour:
// an object's edge circles a pane at the platform's measured whisper,
// and a region's seam is the line between two surfaces. One weight, two
// colours.
//
// A line is drawn here at all because the step it divides is small, and in
// the light appearance the chrome material IS the content's fill, so there
// is no step at all. The platform's answer at a whisper is a line:
// Voice Memos' two panes are the SAME fill and the seam is the whole of
// what parts them.
//
// The line runs from the toolbar band's lower edge to the window's foot, and
// the hand-hold is shorter still. The bands above and below the columns are
// the window's own — one carries the window's drag, the other reports on the
// document — and neither is resized by this boundary, so the hold is the
// document row alone.
//
// The bounds are the aside's own, and the note's floor over them.
func (f *frameState) asideProps(gtx layout.Context, tok themeTokens, size image.Point, g frameGeom) splitter.Props {
	gap := max(gtx.Dp(unit.Dp(frameSplitterDp)), 1)
	c := contentColumns(gtx, size, g.contentX, f.asideW)

	lo := gtx.Dp(frameMinAsideDp)
	hi := max(min(gtx.Dp(frameMaxAsideDp), size.X-g.contentX-gap-noteFloor(gtx, c.note)), lo)

	scale := pxPerDp(gtx)
	return splitter.Props{
		Axis:     layout.Horizontal,
		Boundary: float32(c.asideX),
		Min:      float32(size.X - hi),
		Max:      float32(size.X - lo),
		Colors:   tok.col,
		HitSpan:  splitter.Span{Min: g.rowTop, Max: g.rowTop + g.rowH},
		OnChange: func(at float32) {
			f.asideW = clampAside(unit.Dp((float32(size.X) - at) / scale))
		},
	}
}

// pxPerDp is the metric a boundary reported in pixels is carried back to
// dp by, never zero: a width kept in dp is the same physical measure on a
// screen of a different pixel density, which is the whole reason the two
// widths are kept in dp rather than in what a drag reports.
func pxPerDp(gtx layout.Context) float32 {
	if gtx.Metric.PxPerDp <= 0 {
		return 1
	}
	return gtx.Metric.PxPerDp
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

// layoutToolbar draws the content area's chrome row: the platform's toolbar
// band, the strip along the window's top holding the controls that act on the
// document.
//
// Composed as the stored bands compose theirs. The sidebar toggle stands at
// the leading end, and only while the rail is away — the pane's own toggle
// went with the pane and something has to recall it. Then the window's
// navigation and, bare beside it, the name of the document the window is
// showing: both Finder captures compose their content column's share that way
// — the segmented back/forward pair at x 326-398 with the title's own columns
// beginning fourteen clear of it — and voicememos-window.png keeps "All
// Recordings" bare in the same manner. The document actions cluster at the
// TRAILING end of the band, and the search stands last of all, which is the
// order all four stored windows read: Finder's view pop-up, group pull-down
// and share/tag/more trio run x 694-936 with its search capsule at 955-991 in
// a window 1000 wide; Mail's compose, reply trio, mailbox trio, folder
// pull-down and flag pair run x 404-838 with its search recess at 867-1191 in
// one 1200 wide.
//
// The band runs across the window's columns and the fill change alone says
// where a column's edge is, so a control at the band's trailing end stands
// over the trailing column's fill. The inspector carries no control of its
// own, so what stands there is the find.
//
// The leading space is a measurement, not a constant, and only the hidden
// state spends it: the window controls report where they end, and the row
// adds its own gap after that, because the reported edge is the bare
// glass. With the pane standing the buttons are inside it, the row starts
// where the pane ends, and lead is zero. Where the window has no such
// controls the measurement falls back to the ordinary edge inset.
func (f *frameState) layoutToolbar(gtx layout.Context, m Model, tok themeTokens, lead unit.Dp) layout.Dimensions {
	children := make([]layout.FlexChild, 0, 12)
	if lead > 0 {
		// The window controls' own space is left alone, and so is the air
		// the platform leaves after them, which the lead already carries: a
		// move action declared over the buttons would fight them for the
		// press.
		children = append(children, layout.Rigid(complayout.HSpacer(float32(lead))))
	} else {
		// The row's own edge inset is the note column's, not a smaller one
		// of its own, so the band's leading control stands directly over the
		// trail below it and the window keeps one grid. Finder puts its own
		// pair eight clear of the content column's first pixel, which is the
		// platform's number and not this window's grid.
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
		// The window's navigation and the name of what it is showing, in
		// Finder's own order: the segmented pair at the leading end of the
		// content column's share, the name bare beside it.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutNavigation(gtx, m, tok)
		}),
		layout.Rigid(dragSpacer(bandNameGapDp)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutNoteName(gtx, m, tok)
		}),
		layout.Flexed(1, dragFill),
		// The two vault actions, in the order the rail's foot held them.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutRescan(gtx, tok)
		}),
		layout.Rigid(dragSpacer(bandGapDp)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutSwitchVault(gtx, tok)
		}),
	)
	// The find carries the gap that stands it off the action before it, so a
	// window with no note open — where there is nothing to search — ends the
	// band at the vault switch and not at a gap after it. The slot itself is
	// one width whether the find is open or shut, which is what keeps every
	// control before it standing still when it opens.
	if find := f.findChildren(m, tok); len(find) > 0 {
		children = append(children, layout.Rigid(dragSpacer(bandGapDp)))
		children = append(children, find...)
	}
	// The trailing inset is the measured band's, not the trailing column's:
	// what stands here is a toolbar control and the platform stands its last
	// one eight from the window's edge in every stored window.
	children = append(children, layout.Rigid(dragSpacer(bandTrailingDp)))
	// The row centres what it holds. Its depth IS the band's, and the band
	// puts the window buttons on its own middle, so a control centred in the
	// row stands on the buttons' line without being told to: the 8 dp the
	// platform leaves above a toolbar control and the 8 below it fall out of
	// the arithmetic. What still reaches past the row's foot is the drop
	// shadow its controls cast, which the platform does not cut off at the
	// band's lower edge either.
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

// layoutRescan draws the control that re-walks the vault, and layoutSwitchVault
// the one that leaves it for another. Both act on what the window is showing,
// so both stand in the band; both are the platform's bordered toolbar control
// carrying one symbol, because every document action in every stored band is
// one — Finder's view, group, share, tag and more, Mail's compose, reply,
// archive and mailbox, Notes' format, table and share — and not one of them
// carries a word. components/button draws the chrome variant on the symbol
// path alone for that same reason.
func (f *frameState) layoutRescan(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if f.rescanClick.Clicked(gtx) {
		mvu.MessageOp{Message: Rescan{}}.Add(gtx.Ops)
	}
	return chromeControl(gtx, tok, &f.rescanClick, icons.Refresh, "Rescan")
}

func (f *frameState) layoutSwitchVault(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if f.switchClick.Clicked(gtx) {
		mvu.MessageOp{Message: SwitchVault{}}.Add(gtx.Ops)
	}
	// Title case, which is what the platform's own controls use.
	return chromeControl(gtx, tok, &f.switchClick, icons.OpenFolder, "Switch Vault")
}

// toolbarLeading is where the band's own content may start: the trailing
// edge of the window's control buttons plus the air the platform leaves
// after them, and the ordinary edge inset where the window has no such
// buttons.
//
// Both halves of the window's sidebar switch lead from it — the pane's strip
// while the rail stands, the chrome row once it is away — so the control
// lands on one window column whichever way the rail goes. A control that
// moved when the rail went would be a control that moves under the pointer.
func toolbarLeading() unit.Dp {
	return desktop.BandLead(pane.ButtonGapDp, frameEdgeDp)
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

// dragFill is the row's flexible middle: the whole gap between the
// document's name and the trailing actions, draggable end to end.
func dragFill(gtx layout.Context) layout.Dimensions {
	return desktop.DragRun(gtx, gtx.Constraints.Min.X)
}

// layoutNavigation draws the window's history as the platform's segmented
// toolbar control: back and forward in one capsule parted by the control's
// own hairline, each half inert at its end of the stack.
//
// The window has a history and the model drives it: every Navigate pushes an
// entry and moves the cursor, GoBack and GoForward move the cursor along it,
// and the two halves are live exactly while there is somewhere to go. It
// stands in the band because it is the WINDOW's navigation and not the
// document's — finder-window-light.png keeps its pair at the leading end of
// the content column's share, and finder-window-untinted-dark.png keeps the
// same pair at the same columns.
func (f *frameState) layoutNavigation(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	back := m.Cursor > 0
	fwd := m.Cursor+1 < len(m.History)
	if f.backClick.Clicked(gtx) && back {
		mvu.MessageOp{Message: GoBack{}}.Add(gtx.Ops)
	}
	if f.fwdClick.Clicked(gtx) && fwd {
		mvu.MessageOp{Message: GoForward{}}.Add(gtx.Ops)
	}
	segs := []button.ChromeSegment{
		navSegment(&f.backClick, icons.HistoryBack, "Back", back),
		navSegment(&f.fwdClick, icons.HistoryForward, "Forward", fwd),
	}
	// The shadow is cast AROUND the control, the way every bordered control
	// in this band casts its own: it falls outside the box the control
	// reports.
	return button.ChromeShadow(gtx, tok.col, button.RenderState{}, func(gtx layout.Context) layout.Dimensions {
		return button.ChromeSegments(gtx, tok.col, tok.den, segs)
	})
}

// navSegment is one half of that pair: the set's mark for the direction, the
// state the stack leaves it in, and the clickable that takes the press over
// the segment's own box.
func navSegment(click *widget.Clickable, mark icons.Name, label string, enabled bool) button.ChromeSegment {
	return button.ChromeSegment{
		Icon:  icons.Mark(mark),
		State: button.RenderState{Hovered: click.Hovered(), Pressed: click.Pressed(), Disabled: !enabled},
		Target: func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.ClassOp(semantic.Button).Add(gtx.Ops)
				semantic.LabelOp(label).Add(gtx.Ops)
				semantic.EnabledOp(enabled).Add(gtx.Ops)
				if enabled {
					pointer.CursorPointer.Add(gtx.Ops)
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		},
	}
}

// layoutNoteName draws the name of the document the window is showing, bare
// in the band beside the navigation, in the row's own weight.
//
// Bare and in no control at all, which is how both stored windows that carry
// a title keep it: "Applications" in finder-window-light.png and "All
// Recordings" in voicememos-window.png stand on the band itself. It is a
// label and not an affordance — there is nothing for a press on the window's
// own name to do — and the place a reader climbs from is the trail under it.
func (f *frameState) layoutNoteName(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	name := noteName(m)
	if name == "" {
		return layout.Dimensions{}
	}
	// The label is drawn into an area of its own so that what a reader who
	// cannot see it is told is the window's name and not the band around it:
	// a semantic op with no area under it lands on whatever area is in force.
	macro := op.Record(gtx.Ops)
	dims := drawLabel(gtx, tok.shaper, name, tok.typ.TitleSmall, tok.col.Text)
	call := macro.Stop()
	area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	semantic.LabelOp(name).Add(gtx.Ops)
	call.Add(gtx.Ops)
	area.Pop()
	return dims
}

// noteName is the open note's title — what the window is showing — empty
// while no note is open.
func noteName(m Model) string {
	if n := m.CurrentNote(); n != nil {
		return n.Title
	}
	return ""
}

// vaultName is the open vault's folder name. It heads the note's trail,
// where it is the place a reader climbs back to; the band carries the name
// of the document instead.
func vaultName(m Model) string {
	if m.Vault == "" {
		return ""
	}
	return path.Base(strings.TrimRight(m.Vault, "/"))
}

// layoutRailToggle draws the chrome row's show control, which stands only
// while the rail is away: the same control the pane wears.
func (f *frameState) layoutRailToggle(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	if f.toggleClick.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	label := "Hide the folder rail"
	if m.SidebarHidden {
		label = "Show the folder rail"
	}
	return railToggleControl(gtx, tok, &f.toggleClick, label)
}

// railToggleMark draws the sidebar control's figure, taken from the
// design system's own set: a pane in outline, divided, with the faint
// list lines the host platform puts in the leading column. The set
// resolves that per platform, so a Mac user sees the figure they know
// and everyone else sees the neutral one, from the same name here.
//
// It is the platform's secondary label and not its text colour: the mark
// says what the control is, not what the reader wrote, and patterns/sidebar
// draws its own collapse mark in that same name. Drawn in the text colour it
// is the darkest thing in the rail — darker than every label under it — and
// the eye lands on the toggle before it lands on the open note.
//
// It is one drawing that never morphs, as on the platform: a mark that
// changes leaves the reader guessing whether it shows the present state or
// the next one, and a hollowed variant reads as an unchecked box. What the
// control is about to do is in the label it carries, which the screen
// reader speaks and the tooltip shows.
//
// Both of the window's sidebar controls take it — the one in the panel's
// top trailing corner and the one the chrome row shows once the panel is
// gone — so that the two halves of the same switch are one figure and not
// two. What differs is what the figure stands in, and that is a property of
// where it stands rather than of the switch: a mark in the BAND is the
// platform's bordered toolbar control, and a mark on the sidebar PANEL is
// bare. MEASURED, voicememos-multi-folder-2026-09-18.png: the panel's two
// marks carry no capsule, no fill and no rim, while every mark in the band
// beside them does.
func railToggleControl(gtx layout.Context, tok themeTokens, click *widget.Clickable, label string) layout.Dimensions {
	return chromeControl(gtx, tok, click, icons.Sidebar, label)
}

// paneMark draws one BARE chrome mark — the figure alone, at the mark box
// every chrome mark in this window is drawn at, in the platform's control
// text over the fill it stands on. It is what the sidebar panel's own
// controls are drawn as: the panel is not a band, so nothing standing on it
// wears the band's capsule.
//
// The foreground is the same name the bordered control's mark reads in, so
// the two halves of one switch are one figure in one colour whichever side
// of the window they stand on: what a toolbar draws its own glyphs in.
// MEASURED, voicememos-multi-folder-2026-09-18.png: the panel's own bare
// marks reach #4b4b4b at their darkest, a floor a 1 px stroke at 1x cannot
// pass, against the #4d4d4d the band's own glyphs plateau at.
func paneMark(gtx layout.Context, tok themeTokens, click *widget.Clickable, name icons.Name, label string) layout.Dimensions {
	box := gtx.Dp(unit.Dp(markLargeDp))
	fg := tok.col.ToolbarLabel
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.ClassOp(semantic.Button).Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		icons.Mark(name)(gtx, box, fg)
		return layout.Dimensions{Size: image.Pt(box, box)}
	})
}

// chromeControl draws one bordered toolbar control: the set's mark for name
// centred in the platform's capsule, the capsule's own fill and rim, and the
// drop shadow it casts on the band. Every control standing in this window's
// band is drawn through here, so the band holds one control drawn one way.
func chromeControl(gtx layout.Context, tok themeTokens, click *widget.Clickable, name icons.Name, label string) layout.Dimensions {
	state := button.RenderState{
		Hovered: click.Hovered(),
		Pressed: click.Pressed(),
		Focused: gtx.Focused(click),
	}
	face := button.ChromeFace(icons.Mark(name), tok.col, tok.den, state)
	// The shadow is cast AROUND the clickable: it falls outside the control's
	// own box, and a clickable clips what it wraps to the box it reports.
	return button.ChromeShadow(gtx, tok.col, state, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.ClassOp(semantic.Button).Add(gtx.Ops)
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			return face(gtx)
		})
	})
}
