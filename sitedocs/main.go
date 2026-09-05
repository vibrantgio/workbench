// Command sitedocs is the Vibrant Gio documentation desktop app ("Site
// Docs"). Tab selection, outline disclosure and heading selection live in
// the canonical MVU Model/Update/Messages loop; MessageOp emissions fire
// within the same frame that originated the click.
//
// The window is a patterns/tabs shell with five pages, all built once
// and kept subscribed so scroll positions survive switching:
//
//   - Docs       → the application guide (the workbench root's llms.txt)
//     as one markdown document, its ##/### outline tree in a leading
//     column.
//   - Theme      → the seed the palette grew from, the themer's palette
//     section and the inventory's type ladder, following the live theme
//     (theme_tab.go).
//   - Components → components/gallery/inventory's Components group as
//     live controls in one scrolling column (inventory_tabs.go).
//   - Patterns   → the same for the inventory's Patterns group.
//   - Markdown   → the same for the inventory's Markdown group.
//
// tabbedShellLayer folds the five content streams into the tab shell on
// every emission.

package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"gioui.org/app"
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
	"github.com/vibrantgio/patterns/tabs"
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
	// Dock and VoiceOver read it all the same. A docs app wears the
	// treatment as well as any other: underTitleBar paints the title-bar
	// strip in the tab strip's own fill so the two read as one band, and
	// keeps the strip itself (and so the Docs tree under it) clear of the
	// window buttons.
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

	// The kept brand is read once: the theme stream is dressed in it, and
	// the Theme tab names its seed from the same reading.
	kept := brand.Kept()
	w := specwin.New(mvuWin, themeObservable(kept))

	// The window's collector registers on each FrameEvent so MessageOp.Add(gtx.Ops)
	// calls made during layout are collected and delivered here on the same
	// frame.
	//
	// mvuWin.Messages() drains a channel via rx.Recv, so each emitted message
	// reaches exactly one subscriber. Two streams derive from modelObs —
	// the tab strip's selected-index stream plus the docs outline-state
	// stream — so without multicast those cold subscriptions would each
	// re-drain the channel and split the messages between them.
	// Publish().AutoConnect(2) shares one upstream subscription across
	// exactly those two consumers. NOTE: the count 2 is load-bearing —
	// adding another modelObs consumer requires bumping it.
	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
	models, runner := mvu.Loop(mvuWin.Messages(), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(2)

	if err := w.Render(buildLayers(modelObs, seedOf(kept))).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

// themeObservable returns the live system-driven theme stream, dressed in
// the brand this user kept if they kept one. The 5 s poll interval is the
// intended low-CPU default, not a workaround: each darwin Appearance read
// is a `defaults` fork+exec (~5.5 ms), so 5 s polling costs ~0.1% CPU at
// idle while keeping dark-mode response well under a second of a toggle.
func themeObservable(b brand.Brand) rx.Observable[theme.Theme] {
	return specsystem.LiveTheme(5*time.Second, b.Options()...)
}

// seedOf is the colour the Theme tab offers as this window's seed: the
// brand this user kept, else the palette's own default. It is a candidate
// and not a claim — an OS accent outranks the default, and the theme
// stream publishes the tokens it derived without saying which colour they
// came from, so the seed row checks the candidate against the palette it
// is drawing before naming it.
func seedOf(b brand.Brand) stdcolor.NRGBA {
	if b.Chosen() {
		return b.Seed
	}
	return tokens.DefaultSeed
}

// themeTokens is the colour/typography snapshot the app's own drawing code
// reads at frame time. The shaper is the theme's cached Typography shaper:
// the app builds none of its own, so the typefaces — Roboto, plus
// the Roboto Mono face the guide's code style names — come from the
// theme.
type themeTokens struct {
	col    tokens.ColorTokens
	typ    tokens.Typography
	shaper *text.Shaper
}

// buildLayers returns a function that theme/window.Render passes the
// per-window theme to. It returns the two rendering layers: a backdrop and
// the tabbed shell. The model observable drives tab selection and the
// docs outline state.
func buildLayers(modelObs rx.Observable[Model], seed stdcolor.NRGBA) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			underTitleBar(th, tabbedShellLayer(th, modelObs, seed)),
		}
	}
}

// underTitleBar pads the tab shell down by the native title-bar strip's
// measured height on a full-size-content window and paints that strip in the
// fill of the region it caps. desktop.TopInset is read at frame time: it
// reports 0 until the window's first frame, in headless tests, and on every
// platform but macOS, so away from the treatment (goldens included) the
// wrapper is an exact no-op. Because the whole shell starts below the strip,
// neither the tab strip nor the Docs tree ever sits under the buttons,
// leading ~80 dp (the window buttons' territory) included.
//
// The strip carries no widget of its own, so what it shows is whatever was
// painted there, and the window ground is the Background pin: the agreement
// has to be made rather than inherited. The region this band caps is the tab
// strip, which patterns/tabs fills one rung over its panel, so on this
// window's level-0 panel the band is level 1. What is required is the
// region's fill at the window's top edge, not the region's widget reaching
// it — which is what lets the shell stay inset off the buttons.
//
// The cap claims that same strip for the window's own drag: without that
// claim the window could not be moved by its top edge at all.
func underTitleBar(th rx.Observable[theme.Theme], shellObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(rx.CombineLatest2(shellObs, colors), func(n rx.Tuple2[layout.Widget, tokens.ColorTokens]) layout.Widget {
		return bandedCap(desktop.TopInset, titleBandFill(n.Second), n.First)
	})
}

// titleBandFill is the fill the title-bar strip wears. The region it caps is
// the tab strip, and patterns/tabs fills that strip with the raise walked
// from its panel; this window's panel takes the pattern's default ground,
// the content, so this is the raise off the content. Named once because two
// callers have to agree on it — the window, and the whole-window render that
// photographs the window.
func titleBandFill(c tokens.ColorTokens) stdcolor.NRGBA {
	return c.RaisedOn(c.SurfaceAt(tokens.Level0)).Fill
}

// bandedCap is desktop.CapTop with a fill under it: the strip is painted, and
// then capped as any plain page's strip is — claimed for the window's drag and
// held open above the page. The fill is this app's half and stays here, since
// what a band is painted with is a question about colour and the cap answers
// only in geometry and input; a zero-height strip paints none of it, which is
// the same no-op the cap itself is at that height.
//
// The height is stated rather than read from the window — the same split
// desktop.InsetTop's own height parameter already makes — so a test can state
// a strip it has no window to measure.
func bandedCap(height func() unit.Dp, band stdcolor.NRGBA, w layout.Widget) layout.Widget {
	capped := desktop.CapTop(height, w)
	return func(gtx layout.Context) layout.Dimensions {
		if h := gtx.Dp(height()); h > 0 {
			paint.FillShape(gtx.Ops, band, clip.Rect{
				Max: image.Pt(gtx.Constraints.Max.X, h),
			}.Op())
		}
		return capped(gtx)
	}
}

// backdropLayer is the window's ground: the Background pin, which is what the
// expanse a window exists to show wears. It is the shared mechanism the other
// workbench windows already call, not a fill of this app's own.
func backdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
		return backdrop.Widget(c.Background)
	})
}

// tabbedShellLayer composes the window: a patterns/tabs strip whose five
// pages are Docs, Theme, Components, Patterns and Markdown. The content
// streams are built once and kept subscribed, so scroll positions — one
// per tab — and outline state survive switching tabs in both directions.
//
// tabs.Props.Tabs carries static content widgets, while the five pages
// are streams (theme changes restyle them; model changes move the docs
// outline). So each Tab.Content reads an atomic cell at frame time, and
// the combined map below stores every stream's latest widget into its
// cell before re-emitting the strip. Any input emitting therefore re-emits
// this layer, which drives theme/window's Invalidate and the same-frame
// repaint after a click.
//
// The contents are combined as one homogeneous []layout.Widget rather
// than through a CombineLatestN tuple: rx tops out at five sources and
// the shell needs the strip plus one per page. Combining the pages by
// tabPages order and pairing that slice with the strip has no ceiling, so
// a sixth tab is a line in tabPages and nothing here.
func tabbedShellLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	seed stdcolor.NRGBA,
) rx.Observable[layout.Widget] {
	selectedObs := rx.Map(modelObs, func(m Model) int { return tabIndex(m.currentPage) })

	cells := make([]atomic.Value, len(tabPages))
	fromCell := func(cell *atomic.Value) layout.Widget {
		return contentSlot(func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	}

	pages := make([]rx.Observable[layout.Widget], len(tabPages))
	strip := make([]tabs.Tab, len(tabPages))
	for i, page := range tabPages {
		switch page {
		case pageDocs:
			pages[i] = docsTabFrom(th, modelObs, loadGuide())
		case pageTheme:
			pages[i] = themeTabLayer(th, seed)
		default:
			pages[i] = groupTabLayer(th, tabGroups[page])
		}
		strip[i] = tabs.Tab{Label: tabLabels[i], Content: fromCell(&cells[i])}
	}

	shell := tabs.Tabs(th, tabs.Props{
		Tabs:     strip,
		Selected: selectedObs,
		OnSelect: func(gtx layout.Context, idx int) {
			mvu.MessageOp{Message: SetRoute{Page: tabPages[idx]}}.Add(gtx.Ops)
		},
	})

	combined := rx.CombineLatest2(shell, rx.CombineLatest(pages...))
	return rx.Map(combined, func(n rx.Tuple2[layout.Widget, []layout.Widget]) layout.Widget {
		for i, w := range n.Second {
			cells[i].Store(w)
		}
		return n.First
	})
}

// contentGap is the air the shell keeps between the tab strip and
// whatever the selected tab shows: S4, eight times the 2 dp underline it
// has to separate. S2 — 8 dp, measured — is not enough: the underline reads
// as camouflaged against the inventory's full-width banner, which is Primary,
// the underline's own colour to the byte in both schemes. The banner cannot
// move, so the air is the only variable and it is spent generously. sitedocs
// never overrides the spacing scale — the strip's own cell padding is the
// theme's S3 — so the value reads the published scale directly.
var contentGap = unit.Dp(tokens.Spacing.S4)

// contentSlot is the tab shell's content slot: a tab's content, pushed
// down by contentGap. The gap exposes the panel's own ground — the window
// paper, since patterns/tabs fills its panel at the caller's ground and this
// app takes the default — so the active tab's Primary underline has quiet
// ground on both sides and reads as a line rather than as the top edge of
// whatever begins below it. The strip's lower edge is a rung change as well
// as an underline.
//
// The gap lives here, in the shell, rather than in any one tab: the collision
// is structural. The underline is the strip's bottom two pixels, so any
// content whose first row is a filled band merges with it. That no tab
// currently opens on such a band is an accident of the present content and
// not a contract, and a gap that appeared only on the tab of the day would
// shift every page's first line as the user switches tabs. One slot for all
// five keeps the strip a fixed band and costs each page 8 dp.
//
// Applied by tabbedShellLayer to the live tabs and by the review capture
// to the static ones, so the camera photographs the composition the app
// actually draws.
func contentSlot(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: contentGap}.Layout(gtx, w)
	}
}

// docsTabFrom is the Docs page: the outline tree of the one guide
// document in a leading column, the document itself — llms.txt rendered
// whole by vibrantgio/markdown — filling the rest. The document and its
// parse are built once, so scroll position survives tab switches and an
// outline click's ScrollToBlock moves the same reader.
//
// The source parameter is the injection seam: tests hand it a fixture
// so no test run can ever reach the checkout file or the network. The
// outline stream carries the model's disclosure and selection, so a
// ToggleOutline or SelectHeading message re-emits the combined widget and
// the window repaints on the same frame.
func docsTabFrom(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	source []byte,
) rx.Observable[layout.Widget] {
	blocks := markdown.Parse(source)
	doc := markdown.NewDocument(blocks)
	entries := guideOutline(blocks)

	stateObs := rx.Map(modelObs, func(m Model) outlineState {
		return outlineState{open: m.outlineOpen, selected: m.selectedHeading}
	})
	sidebarObs := docsOutline(th, entries, stateObs, doc.ScrollToBlock)
	mainObs := guideDocObservable(th, doc)

	return rx.Map(rx.CombineLatest2(sidebarObs, mainObs), func(n rx.Tuple2[layout.Widget, layout.Widget]) layout.Widget {
		tree, docW := n.First, n.Second
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(tree),
				layout.Flexed(1, docW),
			)
		}
	})
}
