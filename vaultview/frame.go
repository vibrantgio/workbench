// frame.go is what the vault window hands patterns/shell's two-column
// arrangement, and nothing else: the shell owns the geometry — the sidebar
// pane down the leading edge, the content column beside it with one toolbar
// across its top, the backlinks aside down the trailing edge behind its
// seam, the status bar under the content column, a splitter on each
// boundary and the shadow cast last — and this window states what stands in
// each slot.
//
// What the window hands over: the pane's column (tree.go), the note column
// (note.go), the backlinks aside (aside.go), the status bar (status.go),
// and the toolbar's three groups — the window's navigation and the note's
// place at the leading end, the find in the page at the trailing one. The
// two column widths and the limits they move between are the window's as
// well, kept across launches by the shell's own arrangement memory.
//
// What is left here is the toolbar's own content: the segmented
// back/forward pair, the trail of places the note stands in, and the room
// the trail is elided to.

package main

import (
	"image"
	"path"
	"strings"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/breadcrumb"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/splitter"
	"github.com/vibrantgio/theme/tokens"
)

// Frame layout constants.
const (
	// frameEdgeDp is the leading inset the toolbar and the pane's strip lead
	// from where the window has no control buttons to clear — away from the
	// full-size-content treatment, in headless rendering, and on every
	// platform that keeps its own decorations. The shell has no default for
	// it: the number is the window's.
	frameEdgeDp = 12

	// frameSplitterDp is the air the content area keeps between the note
	// column's edge and the seam the aside stands behind. The splitter's own
	// band is wider than the line and reaches into both columns, so this gap
	// is not the band — it is what stops the note's last pixel standing hard
	// against the seam.
	frameSplitterDp = 6

	frameAsideDp    = 320
	frameMinAsideDp = 160
	frameMaxAsideDp = 640

	// railToggleWidthDp is the width the platform's bordered toolbar control
	// takes around one symbol, which is what every control standing in this
	// window's toolbar is drawn as — the toggle that recalls the pane, each
	// segment of the window's navigation and the search: the 24 dp mark box
	// with the platform's measured 7 dp of clear toolbar on each side.
	// components/button draws it and owns the measurement; the number is
	// named here because the window reasons about where its controls stand
	// and what is beside them.
	railToggleWidthDp = markLargeDp + 2*7

	// railMarginDp is the window's small edge margin: the inset the rail's
	// pane stands off the window's leading, top and bottom edges, the air
	// its own top strip keeps between its toggle and the pane's trailing
	// edge, and the air the trailing column leaves either side of its
	// scrollbar. It is the pane pattern's own margin, named here because the
	// window spends it in places the pattern knows nothing about.
	railMarginDp = pane.MarginDp

	// seamDp is what any chrome boundary in this window paints: a hairline,
	// the width the platform's own splitters take. It is both the pane's
	// rim and the flush column's seam, so that a window whose two vertical
	// boundaries are drawn for different reasons still draws them at one
	// weight. A seam runs the window's whole height, toolbar included, so its
	// width is the width of the scar it leaves across every strip it crosses.
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

	// paneStripDp is the pane's own top strip: deep enough to hold the
	// buttons where the window puts them with the same air below them as
	// above. The buttons' inset is measured from the glass and the strip
	// from the pane's own edge, so the strip owes the margin back at both
	// ends, which lands the buttons' centre line on the strip's own middle —
	// the line the pane's toggle centres on, so the two sit level.
	paneStripDp = pane.StripDp
)

// toolbarShadowReachDp is how far the drop shadow a bordered toolbar control
// casts carries past it in the light appearance. The measurement is
// tokens.PlatformLight.ToolbarControlShadow's own; it is read from there
// because the window's own assertions read the area around a control and have
// to know what the control reaches.
var toolbarShadowReachDp = int(tokens.PlatformLight.ToolbarControlShadow.Reach)

// paneShadowReachDp is how far the shadow the sidebar pane casts carries
// past it. The measurement is tokens.PlatformLight.PaneShadow's own, which
// patterns/pane spreads; it is read from there because the window's own
// assertions read the surface around the pane.
var paneShadowReachDp = int(tokens.PlatformLight.PaneShadow.Reach)

// windowButtons is where this window's three control buttons stand, derived
// from the inset above by the rule the platform's own windows follow. Every
// number in it is the window's: no rail state, no screen and no pane enters
// into any of them.
var windowButtons = pane.Buttons

// frameLimits are the bounds the window states for the two widths the shell
// keeps for it: the rail's own, and the aside's.
//
// Below the rail's minimum the column is narrower than an indented folder
// name, so the tree it holds shows as ellipses; above the maximum the rail
// is no longer a rail beside the note but a second column competing with it.
// What is left of the window is not these bounds' business — a rail wider
// than half the window is cut to half by the pane pattern, whatever they
// allow.
var frameLimits = shell.Limits{
	Pane:      shell.Clamp{Min: railMinWidthDp, Max: railMaxWidthDp},
	Inspector: shell.Clamp{Min: frameMinAsideDp, Max: frameMaxAsideDp},
}

// The rail's bounds, the counterpart of the aside's.
const (
	railMinWidthDp = 160
	railMaxWidthDp = 480
)

// defaultArrangement is what a window opens with when nothing was kept,
// which is what this window laid out before it kept anything.
func defaultArrangement() shell.Arrangement {
	return shell.Arrangement{PaneWidth: treeWidthDp, InspectorWidth: frameAsideDp, PaneShown: true}
}

// bandFind is the find in the page as the toolbar carries it: the state
// the query left it in, the search field instance drawing it, and the note
// column's keyboard target. A frame laid out for measurement carries none of
// them and the toolbar shows no find at all.
type bandFind struct {
	find  *pageFind
	field layout.Widget
	// read is what the toolbar's magnifier capsule reads the keyboard's holder
	// from when it opens the field, so the field records the holder whichever
	// way it was opened. It is nil wherever no note column was built, and a
	// field opened there records nothing.
	read *reader
}

// frameState is the vault frame's per-subscription state: the toolbar's
// clickables, the two splitters' hands and the widths they move. It is
// touched only on the frame goroutine.
type frameState struct {
	// toggleClick is the one gesture source behind the Toggle Sidebar
	// button, which the arrangement draws bare at the pane's trailing corner
	// while the rail stands and bordered at the toolbar's leading end once
	// it is away.
	toggleClick widget.Clickable

	// The window's navigation, which stands in the toolbar as Finder keeps
	// it: the segmented pair at the leading end of the content column's
	// share, with the document's name bare beside it.
	backClick widget.Clickable
	fwdClick  widget.Clickable

	// The control that opens the find in the page where the platform's own
	// toolbar keeps its search: Finder draws a magnifier capsule at the
	// trailing end of its toolbar and expands it into a field, which is what
	// this window's shortcut now has an affordance for.
	findClick widget.Clickable

	// post is where the toolbar's own answers go — what a press on a place in
	// its trail sends: the live frame's is the frame's message op, which is
	// what a nil leaves, and a test's is a recorder, so a toolbar laid out
	// without a window to post to can still be read back.
	post func(gtx layout.Context, msg mvu.Message)

	// trail is the row of places the toolbar draws in its centre: the one the
	// theme stream built, which holds the row's clickables across frames.
	// It is handed over per frame, as the find field is, because it belongs
	// to the stream and not to this state. A frame with none — a frame laid
	// out to be measured — builds its own from its own tokens.
	trail breadcrumb.TrailLayout

	// band is the find in the page as the toolbar carries it: the state
	// the query left it in, and the search field instance that draws it.
	// Both belong to the note column — the query is marked in the note's own
	// document — and the toolbar borrows them, because the toolbar is where
	// the platform keeps a window's search.
	band bandFind

	// The window's two boundaries, each the seam between two regions made
	// operable. The shell draws both; what differs is the edge each stands
	// on — the pane's own hairline leading, the flush column's plain seam
	// trailing — and how much of that edge a hand may take.
	railSplitter  splitter.State
	asideSplitter splitter.State

	railW  unit.Dp
	asideW unit.Dp

	// widths keeps this frame's two column widths across launches. It is
	// nil wherever a frame is laid out for measurement rather than for a
	// reader, so a stored render arranges a window nobody is sitting at
	// without writing over what a real one kept.
	widths *shell.ArrangementMemory

	// leading pins the trailing edge of the window's control buttons instead
	// of measuring it. The measurement is a live window's: off a frame it
	// reports zero, and on one it reports whatever this machine's macOS puts
	// the buttons at. Neither is a number a stored image may depend on, so a
	// still render states one. The live path leaves this nil and measures.
	leading func() unit.Dp

	// geom is the arrangement the last frame laid out, kept so the
	// composition can be measured after the fact: the chrome budget is a
	// property of what was drawn, and recomputing it from the tokens would
	// assert the arithmetic rather than the frame.
	geom shell.PaneLayout

	// still is the still render this frame is part of, and nil in a
	// window. See [stillWindow].
	still *stillWindow
}

// stillWindow is what a frame rendered to a still image rather than shown
// in a window is given in place of what a window answers, and what it
// reports back through. A live window passes nil: it measures its own
// control buttons, and nothing in it reads an arrangement back once the
// frame carrying it has been drawn.
type stillWindow struct {
	// ButtonsEnd is the trailing edge of the window's control buttons. The
	// measurement is a live window's — off a frame it reports zero, and on
	// one it reports whatever this machine's macOS puts the buttons at —
	// and neither is a number a stored image may depend on, so a still
	// render states the number instead.
	ButtonsEnd unit.Dp

	// Arrangement is what the last frame of this render laid out, written
	// on every frame: a composition is measured off the one the image
	// stored, not off a second one built to be measured.
	Arrangement shell.PaneLayout
}

// record keeps what this frame arranged, for the frame itself and for a
// still render reading its composition back.
func (f *frameState) record(g shell.PaneLayout) {
	f.geom = g
	if f.still != nil {
		f.still.Arrangement = g
	}
}

// newFrameState seats a frame on the arrangement it opens with. Both
// column widths are state rather than constants because both boundaries
// belong to the reader: each is a splitter's to move and the memory's to
// keep.
func newFrameState(a shell.Arrangement) *frameState {
	return &frameState{railW: a.PaneWidth, asideW: a.InspectorWidth}
}

// railToggle is the one button that sends the rail away and brings it back.
// It has two places and not two controls, and the arrangement decides which
// one this frame shows.
func railToggle(click *widget.Clickable, press func(gtx layout.Context)) *shell.ToggleSidebar {
	return &shell.ToggleSidebar{
		Click:        click,
		Mark:         icons.Mark(icons.Sidebar),
		PaneTitle:    "Hide the folder rail",
		ToolbarTitle: "Show the folder rail",
		OnPress:      press,
	}
}

// vaultFrame composes the vault screen from its three column streams and
// the model: the shell's two-column arrangement with the rail in the pane,
// the note in the content column and the backlinks aside beside it. All
// three arrive as streams so a theme change re-renders them; the toolbar
// reads the token and model snapshots at frame time.
//
// The two side columns open at the widths the memory hands over — what the
// reader last left, or the defaults — and that same memory is what every
// frame reports its arrangement back to.
//
// still is nil in a window and states what a window would have answered
// where the frame is being rendered to an image instead; see [stillWindow].
func vaultFrame(
	loadModel func() Model,
	loadTok func() themeTokens,
	widths *shell.ArrangementMemory,
	still *stillWindow,
	find *pageFind,
	read *reader,
	sidebar rx.Observable[shell.PaneColumn],
	aside, main, search rx.Observable[layout.Widget],
	trail rx.Observable[breadcrumb.TrailLayout],
) rx.Observable[layout.Widget] {
	columns := rx.CombineLatest5(sidebar, aside, main, search, trail)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := newFrameState(widths.Arrangement())
		st.widths = widths
		if still != nil {
			st.still = still
			st.leading = func() unit.Dp { return still.ButtonsEnd }
		}
		return rx.Map(columns, func(next rx.Tuple5[shell.PaneColumn, layout.Widget, layout.Widget, layout.Widget, breadcrumb.TrailLayout]) layout.Widget {
			col, asW, mainW, fieldW := next.First, next.Second, next.Third, next.Fourth
			return func(gtx layout.Context) layout.Dimensions {
				st.band = bandFind{find: find, field: fieldW, read: read}
				st.trail = next.Fifth
				return st.layout(gtx, loadModel(), loadTok(), col, asW, mainW)
			}
		})
	})
}

// layout hands the shell what this window puts in each of its slots and
// draws the frame it arranges.
func (f *frameState) layout(gtx layout.Context, m Model, tok themeTokens, col shell.PaneColumn, as, main layout.Widget) layout.Dimensions {
	two := shell.TwoColumn{
		PaneWidth:  f.railW,
		PaneHidden: m.SidebarHidden,
		Pane:       col,
		PaneResize: &shell.PaneResize{
			State: &f.railSplitter,
			Min:   frameLimits.Pane.Min, Max: frameLimits.Pane.Max,
			Floor:    noteMinWidthDp,
			OnChange: func(w unit.Dp) { f.railW = w },
		},
		// The content area stands on the note's own surface: the document is
		// what the window is. The two side columns are chrome and wear the
		// platform's chrome material.
		Surface:       tok.col.WindowBackground,
		ContentFill:   tok.col.TextBackground,
		InspectorFill: chromeSurface(tok.col),
		Toolbar:       f.toolbar(m, tok),
		Toggle: railToggle(&f.toggleClick, func(gtx layout.Context) {
			f.send(gtx, ToggleSidebar{})
		}),
		Radius:  tokens.Radius,
		Density: tok.den,
		Main:    main,
		// The note column is laid out BEFORE the toolbar above it and
		// replayed after: the toolbar carries the find, and the find's keys
		// and the count it reports are the column's to settle.
		MainFirst: true,
		Footer: func(gtx layout.Context) layout.Dimensions {
			return layoutStatusBar(gtx, m, tok, f.contentWidth(gtx))
		},
		FooterDepth:    statusBarHeight(tok),
		Inspector:      as,
		InspectorWidth: f.asideW,
		InspectorGap:   frameSplitterDp,
		InspectorResize: &shell.InspectorResize{
			State: &f.asideSplitter,
			Min:   frameLimits.Inspector.Min, Max: frameLimits.Inspector.Max,
			Floor:    noteMinWidthDp,
			OnChange: func(w unit.Dp) { f.asideW = w },
		},
		WindowButtonsEnd: f.leading,
		Memory:           f.widths,
		Report:           f.record,
	}
	dims, _ := two.Layout(gtx, tok.col)
	return dims
}

// contentWidth is the content column's width, read off the band the footer
// was given: that band runs from the pane's trailing edge to the window's,
// so what it holds over the column is the inspector and the gap in front of
// it, cut to the band the way the arrangement cuts them.
func (f *frameState) contentWidth(gtx layout.Context) int {
	band := gtx.Constraints.Max.X
	gap := max(gtx.Dp(unit.Dp(frameSplitterDp)), 1)
	insp := max(min(gtx.Dp(f.asideW), band-gap), 0)
	return max(band-gap-insp, 0)
}

// toolbar is what the content column carries across its top: the window's
// navigation at the leading end, the trail of places the note stands in
// beside it, and the find in the page at the trailing one.
//
// Composed as the stored toolbars compose theirs. Both Finder captures put
// the segmented back/forward pair at the leading end of the content
// column's share with the title's own columns beginning fourteen clear of
// it, and voicememos-window.png keeps "All Recordings" bare in the same
// manner. The search stands last of all, which is the order all five stored
// toolbars read.
//
// The vault's own two actions stand in the rail pane's top strip and not
// here: they act on the collection the pane shows, and a control that acts
// on a pane's collection waits for the pane. Nor does the toolbar stand in
// for them while the pane is away — only the toggle travels, as this
// platform's sidebars do, and the arrangement is what puts it there.
//
// The leading gutter is the note column's own inset, so the toolbar's
// leading control stands directly over the trail below it and the window
// keeps one grid.
func (f *frameState) toolbar(m Model, tok themeTokens) shell.Toolbar {
	t := shell.Toolbar{
		ButtonFallback: frameEdgeDp,
		Lead:           noteInsetDp,
		Leading: []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			return f.layoutNavigation(gtx, m, tok)
		}},
	}
	if len(notePlaces(m)) > 0 {
		t.Center = func(gtx layout.Context) layout.Dimensions {
			return f.layoutBandTrail(gtx, m, tok)
		}
	}
	if w := f.findTrailing(m, tok); w != nil {
		t.Trailing = []layout.Widget{w}
	}
	return t
}

// layoutNavigation draws the window's history as the platform's segmented
// toolbar control: back and forward in one capsule parted by the control's
// own hairline, each half inert at its end of the stack.
//
// The window has a history and the model drives it: every Navigate pushes an
// entry and moves the cursor, GoBack and GoForward move the cursor along it,
// and the two halves are live exactly while there is somewhere to go. It
// stands in the toolbar because it is the WINDOW's navigation and not the
// document's — finder-window-light.png keeps its pair at the leading end of
// the content column's share, and finder-window-untinted-dark.png keeps the
// same pair at the same columns.
func (f *frameState) layoutNavigation(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	back := m.CanGoBack()
	fwd := m.CanGoForward()
	if f.backClick.Clicked(gtx) && back {
		mvu.MessageOp{Message: GoBack{}}.Add(gtx.Ops)
	}
	if f.fwdClick.Clicked(gtx) && fwd {
		mvu.MessageOp{Message: GoForward{}}.Add(gtx.Ops)
	}
	segs := []button.BorderedSegment{
		navSegment(&f.backClick, icons.ChevronLeft, "Back", back),
		navSegment(&f.fwdClick, icons.ChevronRight, "Forward", fwd),
	}
	// The shadow is cast AROUND the control, the way every bordered control
	// in this toolbar casts its own: it falls outside the box the control
	// reports.
	return button.BorderedShadow(gtx, tok.col, button.RenderState{Variant: button.Chrome}, func(gtx layout.Context) layout.Dimensions {
		// The pair is drawn at its own width: the toolbar hands a control room
		// and not a width, so the shared control divides nothing here.
		gtx.Constraints.Min = image.Point{}
		return button.BorderedSegments(gtx, tok.shaper, tok.col, tokens.Radius, tok.typ.LabelLarge, tok.den, segs)
	})
}

// navSegment is one half of that pair: the set's mark for the direction, the
// state the stack leaves it in, and the clickable that takes the press over
// the segment's own box.
func navSegment(click *widget.Clickable, mark icons.Name, title string, enabled bool) button.BorderedSegment {
	return button.BorderedSegment{
		Icon:  icons.Mark(mark),
		State: button.RenderState{Variant: button.Chrome, Hovered: click.Hovered(), Pressed: click.Pressed(), Disabled: !enabled},
		Target: func(gtx layout.Context) layout.Dimensions {
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.ClassOp(semantic.Button).Add(gtx.Ops)
				semantic.LabelOp(title).Add(gtx.Ops)
				semantic.EnabledOp(enabled).Add(gtx.Ops)
				if enabled {
					pointershape.OverSize(gtx.Ops, gtx.Constraints.Min, pointer.CursorPointer)
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		},
	}
}

// layoutBandTrail draws where the note stands, bare in the toolbar beside the
// navigation: the vault, the folders on the note's way and the note itself,
// parted by the set's chevron.
//
// It stands where the window's title stood, which is where the platform
// shows a document's place — "Applications" in finder-window-light.png and
// "All Recordings" in voicememos-window.png stand bare on the toolbar in that
// same column, and Finder's own path bar draws the same row of places. Each
// earlier place answers a press by revealing its folder in the outline; the
// note itself is last and answers nothing, being where the reader already
// is.
//
// The row is elided rather than cut where the toolbar is too narrow for it:
// see [bandTrailSegments].
func (f *frameState) layoutBandTrail(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	segs := bandTrailSegments(gtx, tok, m, f.bandTrailRoom(gtx, m), f.revealPlace)
	if len(segs) == 0 {
		return layout.Dimensions{}
	}
	// The row is held to the room the toolbar has for it, so a trail that
	// cannot be elided any further is cut off rather than walking the
	// controls after it off the window's edge.
	if room := f.bandTrailRoom(gtx, m); room < gtx.Constraints.Max.X {
		gtx.Constraints.Max.X = max(room, 0)
	}
	return f.bandTrail(tok)(gtx, segs)
}

// revealPlace is the press a place in the toolbar's trail answers: the outline
// opens the whole way down to that folder, and the vault at the trail's head
// returns the outline to its root — every fold under the vault closed and
// the vault itself open.
//
// The note at the trail's end carries none of this: it is where the reader
// already is.
func (f *frameState) revealPlace(dir string) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		if dir == treeRootPath {
			f.send(gtx, RootTree{})
			return
		}
		f.send(gtx, RevealFolder{Dir: dir})
	}
}

// send hands one of the frame's answers on to whatever is collecting this
// frame's messages: the frame's own message op, which is what a nil post
// leaves, and a recorder where a frame is laid out without a window to post
// to.
func (f *frameState) send(gtx layout.Context, msg mvu.Message) {
	if f.post != nil {
		f.post(gtx, msg)
		return
	}
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
}

// bandTrail is the row the toolbar draws its places with: the one the theme
// stream handed over, and one of this frame's own where no stream did — a
// frame laid out to be measured. The row owns its clickables and has to
// outlive the frame it draws, so the one built here is kept.
func (f *frameState) bandTrail(tok themeTokens) breadcrumb.TrailLayout {
	if f.trail == nil {
		f.trail = breadcrumb.NewTrail(tok.shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
			tok.col, tok.sp, tok.typ.TitleSmall)
	}
	return f.trail
}

// bandTrailRoom is how much of the row the trail may take: what the flex has
// left it, less what the toolbar still owes the controls standing after it —
// the find's slot, which keeps one width open or shut, and the trailing
// gutter. The trail is laid out before either of them, so the room they need
// is stated here rather than read off a layout that has not happened yet.
func (f *frameState) bandTrailRoom(gtx layout.Context, m Model) int {
	after := shell.PlatformToolbar.Trail
	if m.CurrentNote() != nil {
		after += unit.Dp(findFieldDp)
	}
	return gtx.Constraints.Max.X - gtx.Dp(after)
}

// vaultName is the open vault's folder name. It heads the note's trail,
// where it is the place a reader climbs back to; the toolbar carries the name
// of the document instead.
func vaultName(m Model) string {
	if m.Vault == "" {
		return ""
	}
	return path.Base(strings.TrimRight(m.Vault, "/"))
}
