// Command sitedocs is the Vibrant Gio documentation desktop app ("Site
// Docs"). Outline disclosure and heading selection live in the canonical
// MVU Model/Update/Messages loop; MessageOp emissions fire within the same
// frame that originated the click.
//
// The window shows one page: the application guide (the workbench root's
// llms.txt) as one markdown document, its ##/### outline tree in a leading
// column. The page is built once and kept subscribed, so its scroll
// position and outline state stay while the window is open.

package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"os"
	"strconv"
	"time"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	specwin "github.com/vibrantgio/theme/window"
)

const (
	windowW = 1200
	windowH = 800
)

func main() {
	go run()
	app.Main()
}

func run() {
	// mvu/desktop's full-size-content treatment: on macOS the content
	// extends behind a transparent title bar with the traffic lights
	// floating over it; on every other platform FullSizeContent returns no
	// options and the window keeps its normal decorations. app.Title stays
	// even though the treatment hides the title text — Mission Control, the
	// Dock and VoiceOver read it all the same. withToolbar lays the toolbar
	// along the window's top and keeps the page (and so the outline tree)
	// below it, clear of the window buttons.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Site Docs"),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)...)
	// Gio re-hides the standard window buttons on every configuration
	// rebuild, so StandWindowButtons registers a re-assertion on the mvu
	// OnConfigure seam. It also centres them on the toolbar's strip.
	// Post-construction options must therefore go through mvuWin.Option —
	// never mvuWin.Window().Option — or the buttons vanish.
	shell.StandWindowButtons(mvuWin, nil)

	// Seam proof hook: SITEDOCS_RETITLE_MS=<n> retitles the window through
	// mvuWin.Option n milliseconds after launch. A runtime title change is
	// the exact sequence that re-hides the traffic lights without the
	// OnConfigure re-assertion, so this keeps the invariant reproducible: run
	// with the variable set and watch the buttons survive.
	if ms := os.Getenv("SITEDOCS_RETITLE_MS"); ms != "" {
		if n, err := strconv.Atoi(ms); err == nil && n > 0 {
			go func() {
				time.Sleep(time.Duration(n) * time.Millisecond)
				mvuWin.Option(app.Title("Site Docs — retitled"))
			}()
		}
	}

	// The kept brand is applied to the theme stream: what a brand file still pins
	// is the type and the theme colour that stands in where the platform
	// uses its accent. The colour set itself is the platform's under either
	// appearance, so nothing kept pins that.
	// The platform's "Show scroll bars" setting, read once here and handed
	// to the composition: a window asks the platform when it is built,
	// never on a frame.
	bars := desktop.ScrollBars()

	w := specwin.New(mvuWin, themeObservable(brand.Kept()))

	// The window's collector registers on each FrameEvent so MessageOp.Add(gtx.Ops)
	// calls made during layout are collected and delivered here on the same
	// frame.
	//
	// mvu.Loop's models observable is a replay-latest multicast: it shares one
	// upstream subscription however many streams derive from it, and hands
	// each of them the model in force the moment it attaches.
	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
	// The menu bar holds the window's actions, from the same table as the
	// window's chords (menu.go). Away from macOS the declaration is inert.
	menu := desktop.NewMenuBar(mvuWin, MenuItems()...)
	models, runner := mvu.Loop(rx.Merge(mvuWin.Messages(), menu.Messages()), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	if err := w.Render(buildLayers(models, bars)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

// themeObservable returns the live system-driven theme stream, with
// the brand this user kept if they kept one. The 5 s poll interval is the
// intended low-CPU default, not a workaround: each darwin Appearance read
// is a `defaults` fork+exec (~5.5 ms), so 5 s polling costs ~0.1% CPU at
// idle while keeping dark-mode response well under a second of a toggle.
func themeObservable(b brand.Brand) rx.Observable[theme.Theme] {
	return specsystem.LiveTheme(5*time.Second, b.Options()...)
}

// themeTokens is the colour/typography snapshot the app's own drawing code
// reads at frame time. The shaper is the theme's cached Typography shaper:
// the app builds none of its own, so the typefaces — Roboto, plus
// the Roboto Mono face the guide's code style names — come from the
// theme.
type themeTokens struct {
	col    tokens.PlatformColors
	typ    tokens.Typography
	shaper *text.Shaper
}

// buildLayers returns a function that theme/window.Render passes the
// per-window theme to. It returns the two rendering layers: a backdrop and
// the window's content, the toolbar over the page. The model observable
// drives the outline state and Edit ▸ Find….
func buildLayers(modelObs rx.Observable[Model], bars desktop.ScrollBarSetting) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		guide, guideFail := loadGuide()
		post := func(gtx layout.Context, msg mvu.Message) { mvu.MessageOp{Message: msg}.Add(gtx.Ops) }
		content, _, _ := docsWindow(th, modelObs, guide, guideFail, bars, post)
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			content,
		}
	}
}

// toolbarSearchWDp is the width of the search field in the toolbar.
// MEASURED at 1x: the toolbar search recess runs x 867–1191 in
// mail-window.png and x 643–967 in voicememos-window.png, 325 wide in both.
const toolbarSearchWDp = 325

// toolbarDepth is the depth of the strip along the window's top: the
// platform's toolbar, or the native title-bar strip where that is deeper.
func toolbarDepth() unit.Dp { return shell.ChromeTop(shell.ToolbarDepth) }

// withToolbar lays the strip of the given depth along the window's top and
// the page below it.
//
// The strip has no fill of its own. Over the outline rail it shows the
// rail's fill and seam, so the rail's fill reaches the window's top edge.
// Over the guide column it shows the page's fill, and the toolbar is in that
// share: the search field at the trailing end, with the toolbar's trailing
// inset after it.
//
// The strip moves the window when the user drags it, except over the
// window's control buttons and over the field.
func withToolbar(depth func() unit.Dp, c tokens.PlatformColors, field, page layout.Widget) layout.Widget {
	inset := desktop.InsetTop(depth, page)
	return func(gtx layout.Context) layout.Dimensions {
		h := gtx.Dp(depth())
		w := gtx.Constraints.Max.X
		rail := min(gtx.Dp(unit.Dp(docsOutlineWidthDp)), w)
		if h > 0 {
			paint.FillShape(gtx.Ops, pageFill(c), clip.Rect{Max: image.Pt(w, h)}.Op())
			paintOutlineRail(gtx, image.Pt(rail, h), c)

			rgtx := gtx
			rgtx.Constraints = layout.Exact(image.Pt(rail, h))
			desktop.DragTop(rgtx, depth)

			st := op.Offset(image.Pt(rail, 0)).Push(gtx.Ops)
			tgtx := gtx
			tgtx.Constraints = layout.Exact(image.Pt(w-rail, h))
			layoutToolbar(tgtx, field)
			st.Pop()
		}
		return inset(gtx)
	}
}

// layoutToolbar lays the toolbar out across the guide column's share of the
// strip. The row centres the field on the strip's middle, so the field
// stands on the line of the window's control buttons.
func layoutToolbar(gtx layout.Context, field layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	gtx.Constraints.Min.Y = 0
	layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, pane.DragFill),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			box := gtx.Dp(toolbarSearchWDp)
			gtx.Constraints.Min = image.Pt(box, 0)
			gtx.Constraints.Max.X = box
			dims := field(gtx)
			return layout.Dimensions{Size: image.Pt(box, dims.Size.Y)}
		}),
		layout.Rigid(pane.DragSpacer(shell.PlatformToolbar.Trail)),
	)
	return layout.Dimensions{Size: size}
}

// pageFill is the fill the page stands on: the platform's content plane.
// The document column and the guide's empty state are drawn on it.
func pageFill(c tokens.PlatformColors) stdcolor.NRGBA {
	return c.ControlBackground
}

// backdropLayer paints the window's own plane, under every other layer.
// Every region that stands paints its own fill over it; what is left showing
// is the window itself. It is the shared mechanism the other workbench
// windows already call, not a fill of this app's own.
func backdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(colors, func(c tokens.PlatformColors) layout.Widget {
		return backdrop.Widget(c.WindowBackground)
	})
}

// onPageFill lays w out over the page's fill across the whole frame. The
// document column draws no fill of its own, so the fill past its reading
// measure is this one.
func onPageFill(c tokens.PlatformColors, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, pageFill(c), clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// docsWindow is the window's content: the toolbar with the search field
// over the page. The page is the outline tree of the one guide document in
// a leading column and the document itself — llms.txt rendered whole by
// vibrantgio/markdown — filling the rest. The document and its parse are
// built once, so an outline click's ScrollToBlock moves the same reader.
//
// The source parameter is the injection seam: tests hand it a fixture so
// no test run can ever reach the checkout file or the network. A failure
// instead of a source keeps both columns: the outline column at its width
// draws its own line, and the content column draws the guide's line and
// the location it was looked for at. The search field then finds nothing.
//
// post is how the window's chords post their messages; it is a parameter
// so a test can read what they post. docsWindow also returns the document
// and the finder, so a test can read the find and where the keyboard is.
//
// Each FocusSearch counts in Model.searchRequests. On the first frame after
// the count changes, the window gives the keyboard to the search field.
func docsWindow(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	source []byte,
	fail *guideFailure,
	bars desktop.ScrollBarSetting,
	post func(gtx layout.Context, msg mvu.Message),
) (rx.Observable[layout.Widget], *markdown.Document, *finder) {
	f := &finder{}
	var page rx.Observable[layout.Widget]
	if fail != nil {
		page = guideUnavailableObservable(th, fail, bars)
	} else {
		blocks := markdown.Parse(source)
		f.doc = markdown.NewDocument(blocks)
		page = docsPage(th, modelObs, blocks, f.doc, bars, f.currentMatch)
	}
	field := f.searchField(th)
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })

	chords := ChordAreas(post)
	// requestsSeen is the FocusSearch count the window last answered. The
	// frame reads and writes it, and nothing else does.
	requestsSeen := 0

	full := rx.CombineLatest4(page, field, colors, modelObs)
	return rx.Map(full, func(n rx.Tuple4[layout.Widget, layout.Widget, tokens.PlatformColors, Model]) layout.Widget {
		c, m := n.Third, n.Fourth
		strip := withToolbar(toolbarDepth, c, n.Second, onPageFill(c, n.First))
		return func(gtx layout.Context) layout.Dimensions {
			for _, a := range chords {
				a(gtx)
			}
			if m.searchRequests != requestsSeen {
				requestsSeen = m.searchRequests
				if f.field != nil {
					gtx.Execute(key.FocusCmd{Tag: f.field})
				}
			}
			return strip(gtx)
		}
	}), f.doc, f
}

// docsPage is the page over a guide that loaded: the outline tree and the
// document. current is asked on each frame which match is current, for the
// scroll bar's track.
func docsPage(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	blocks []markdown.Block,
	doc *markdown.Document,
	bars desktop.ScrollBarSetting,
	current func() int,
) rx.Observable[layout.Widget] {
	stateObs := rx.Map(modelObs, func(m Model) outlineState {
		return outlineState{open: m.outlineOpen, selected: m.selectedHeading}
	})
	sidebarObs := docsOutline(th, guideOutline(blocks), stateObs, doc.ScrollToBlock)
	mainObs := guideDocObservable(th, doc, bars, current)
	return rx.Map(rx.CombineLatest2(sidebarObs, mainObs), func(n rx.Tuple2[layout.Widget, layout.Widget]) layout.Widget {
		tree, column := n.First, n.Second
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(tree),
				layout.Flexed(1, column),
			)
		}
	})
}
