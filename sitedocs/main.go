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
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
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
	// Dock and VoiceOver read it all the same. underTitleBar paints the
	// title-bar strip in the fills of the page's columns under it, and keeps
	// the page (and so the outline tree) clear of the window buttons.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Site Docs"),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)...)
	// Gio re-hides the standard window buttons on every configuration
	// rebuild, so ShowWindowButtons registers a re-assertion on the mvu
	// OnConfigure seam. Post-construction options must therefore go through
	// mvuWin.Option — never mvuWin.Window().Option — or the buttons vanish.
	desktop.ShowWindowButtons(mvuWin)

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
// the page. The model observable drives the outline state.
func buildLayers(modelObs rx.Observable[Model], bars desktop.ScrollBarSetting) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			underTitleBar(th, pageLayer(th, modelObs, bars)),
		}
	}
}

// underTitleBar pads the page down by the native title-bar strip's measured
// height on a full-size-content window and paints that strip in the fills of
// the columns it caps. desktop.TopInset is read at frame time: it reports 0
// until the window's first frame, in headless tests, and on every platform
// but macOS, so away from the treatment (goldens included) the wrapper is an
// exact no-op. Because the whole page starts below the strip, the outline
// tree never sits under the window buttons.
//
// The cap claims that same strip for the window's own drag: without that
// claim the window could not be moved by its top edge at all.
func underTitleBar(th rx.Observable[theme.Theme], pageObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(rx.CombineLatest2(pageObs, colors), func(n rx.Tuple2[layout.Widget, tokens.PlatformColors]) layout.Widget {
		return bandedCap(desktop.TopInset, n.Second, n.First)
	})
}

// paintTitleBand paints the title-bar strip across size. The strip draws
// nothing of its own, so the window paints in it the fill of each region
// under it, continued upward: the outline rail's fill and seam over the
// rail's width, and the page's fill over the rest. The stored reference
// measures the platform's band the same way: it is the sidebar's fill over
// the sidebar and the content's fill over the content.
func paintTitleBand(gtx layout.Context, size image.Point, c tokens.PlatformColors) {
	paint.FillShape(gtx.Ops, pageFill(c), clip.Rect{Max: size}.Op())
	rail := min(gtx.Dp(unit.Dp(docsOutlineWidthDp)), size.X)
	paintOutlineRail(gtx, image.Pt(rail, size.Y), c)
}

// pageFill is the fill the page stands on: the platform's content plane.
// The document column and the guide's empty state are drawn on it.
func pageFill(c tokens.PlatformColors) stdcolor.NRGBA {
	return c.ControlBackground
}

// bandedCap is desktop.CapTop with the title band painted under it: the strip
// is painted, and then capped as any plain page's strip is — claimed for the
// window's drag and held open above the page. The paint is this app's half
// and stays here, since what a band is painted with is a question about
// colour and the cap answers only in geometry and input; a zero-height strip
// paints nothing, which is the same no-op the cap itself is at that height.
//
// The height is stated rather than read from the window — the same split
// desktop.InsetTop's own height parameter already makes — so a test can state
// a strip it has no window to measure.
func bandedCap(height func() unit.Dp, c tokens.PlatformColors, w layout.Widget) layout.Widget {
	capped := desktop.CapTop(height, w)
	return func(gtx layout.Context) layout.Dimensions {
		if h := gtx.Dp(height()); h > 0 {
			paintTitleBand(gtx, image.Pt(gtx.Constraints.Max.X, h), c)
		}
		return capped(gtx)
	}
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

// pageLayer composes the window's one page on the page's fill. Any input
// emitting re-emits this layer, which drives theme/window's Invalidate and
// the same-frame repaint after a click.
func pageLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	bars desktop.ScrollBarSetting,
) rx.Observable[layout.Widget] {
	guide, guideFail := loadGuide()
	page := docsPageFrom(th, modelObs, guide, guideFail, bars)
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(rx.CombineLatest2(page, colors), func(n rx.Tuple2[layout.Widget, tokens.PlatformColors]) layout.Widget {
		return onPageFill(n.Second, n.First)
	})
}

// docsPageFrom is the page: the outline tree of the one guide document in a
// leading column, the document itself — llms.txt rendered whole by
// vibrantgio/markdown — filling the rest. The document and its parse are
// built once, so an outline click's ScrollToBlock moves the same reader.
//
// The source parameter is the injection seam: tests hand it a fixture
// so no test run can ever reach the checkout file or the network. The
// outline stream carries the model's disclosure and selection, so a
// ToggleOutline or SelectHeading message re-emits the combined layer and
// the window repaints on the same frame.
//
// A failure instead of a source keeps both columns: the outline column at
// its width draws its own line, and the content column draws the guide's
// line and the location it was looked for at.
func docsPageFrom(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	source []byte,
	fail *guideFailure,
	bars desktop.ScrollBarSetting,
) rx.Observable[layout.Widget] {
	if fail != nil {
		return guideUnavailableObservable(th, fail, bars)
	}
	post := func(gtx layout.Context, msg mvu.Message) { mvu.MessageOp{Message: msg}.Add(gtx.Ops) }
	page, _, _ := docsPage(th, modelObs, source, bars, post)
	return page
}

// docsPage is docsPageFrom over a guide that loaded. It also returns the
// document and the find bar, so a test can read where the keyboard is. post
// is how the page's chords, Escape and Done post their messages; it is a
// parameter so a test can read what they post.
//
// The find bar stands over the guide column only. The outline rail keeps its
// fill and its height, and the guide moves down by the bar's height.
//
// Edit ▸ Find… (⌘F) opens the bar, and Escape and Done close it. Each open
// and close request counts in Model.findMoves. On the first frame after the
// count changes, the page gives the keyboard to the field if the bar is open
// and to the guide if it is closed.
func docsPage(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	source []byte,
	bars desktop.ScrollBarSetting,
	post func(gtx layout.Context, msg mvu.Message),
) (rx.Observable[layout.Widget], *markdown.Document, *findBar) {
	blocks := markdown.Parse(source)
	doc := markdown.NewDocument(blocks)
	entries := guideOutline(blocks)

	stateObs := rx.Map(modelObs, func(m Model) outlineState {
		return outlineState{open: m.outlineOpen, selected: m.selectedHeading}
	})
	sidebarObs := docsOutline(th, entries, stateObs, doc.ScrollToBlock)
	mainObs := guideDocObservable(th, doc, bars)
	bar := newFindBar(th, post)
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })

	chords := ChordAreas(post)
	escape := OnKey(key.NameEscape, 0, func(gtx layout.Context) { post(gtx, CloseFind{}) })
	// movesSeen is the find request count the page last answered. The frame
	// reads and writes it, and nothing else does.
	movesSeen := 0

	columns := rx.CombineLatest2(sidebarObs, mainObs)
	full := rx.CombineLatest4(columns, bar.parts, colors, modelObs)
	return rx.Map(full, func(n rx.Tuple4[rx.Tuple2[layout.Widget, layout.Widget], rx.Tuple3[layout.Widget, layout.Widget, layout.Widget], tokens.PlatformColors, Model]) layout.Widget {
		tree, docW := n.First.First, n.First.Second
		field, steps, done := n.Second.First, n.Second.Second, n.Second.Third
		c, m := n.Third, n.Fourth
		strip := func(gtx layout.Context) layout.Dimensions {
			return layoutFindBar(gtx, c, field, steps, done)
		}
		column := withFindBar(m.findOpen, strip, docW)
		return func(gtx layout.Context) layout.Dimensions {
			for _, a := range chords {
				a(gtx)
			}
			if m.findOpen {
				escape(gtx)
			}
			if m.findMoves != movesSeen {
				movesSeen = m.findMoves
				if !m.findOpen {
					gtx.Execute(key.FocusCmd{Tag: doc.Focus()})
				} else if bar.field != nil {
					gtx.Execute(key.FocusCmd{Tag: bar.field})
				}
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(tree),
				layout.Flexed(1, column),
			)
		}
	}), doc, bar
}
