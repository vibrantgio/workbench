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
	// treatment as well as any other: the tab strip's Surface band reads as
	// one piece with the title-bar strip, and underTitleBar keeps the strip
	// itself (and so the Docs tree under it) clear of the window buttons.
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
	// OnConfigure re-assertion (H1.1/H1.2), so this keeps the invariant
	// reproducible: run with the variable set and watch the buttons survive.
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

	// Build the model observable with mvu.Loop over mvu messages. The
	// window's collector registers on each FrameEvent so MessageOp.Add(gtx.Ops)
	// calls made during layout are collected and delivered here on the same
	// frame; Loop also runs the commands Update returns (this app returns
	// DoNothing everywhere) and emits the seed model first.
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
// is drawing before naming it (see theme_seed.go).
func seedOf(b brand.Brand) stdcolor.NRGBA {
	if b.Chosen() {
		return b.Seed
	}
	return tokens.DefaultSeed
}

// themeTokens is the colour/typography snapshot the app's own drawing code
// reads at frame time. The shaper is the theme's cached Typography shaper
// (F1.4): the app builds none of its own, so the typefaces — Roboto, plus
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
			underTitleBar(tabbedShellLayer(th, modelObs, seed)),
		}
	}
}

// underTitleBar pads the tab shell down by the native title-bar strip's
// measured height on a full-size-content window. desktop.TopInset is read at
// frame time: it reports 0 until the window's first frame, in headless tests,
// and on every platform but macOS, so away from the treatment (goldens
// included) the wrapper is an exact no-op. The strip itself is paint-only —
// what shows through the transparent title bar is the backdrop layer's
// Surface fill, the same colour the tab strip paints, so the header reads as
// one band extending up behind the traffic lights — and because the whole
// shell starts below the strip, neither the tab strip nor the Docs tree ever
// sits under the buttons, leading ~80 dp (the window buttons' territory)
// included.
func underTitleBar(shellObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(shellObs, func(w layout.Widget) layout.Widget {
		return desktop.InsetTop(desktop.TopInset, w)
	})
}

// backdropLayer paints a full-canvas rectangle in the theme Surface colour.
func backdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
		fill := c.Surface
		return func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}
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
// cell before re-emitting the strip — the same layer-boundary hand-off
// mvu/window.go uses for its layer snapshot. Any input emitting therefore
// re-emits this layer, which drives theme/window's Invalidate and the
// same-frame repaint after a click.
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
// has to separate. The first cut of this was S2; the fresh-eyes reviewer
// measured that 8 dp and still read the underline as camouflaged against
// the inventory's full-width banner, which is Primary — the underline's
// own colour, to the byte, in both schemes. The banner is ruled
// separately and cannot move, so the air is the only variable this task
// holds, and it is spent generously. sitedocs never overrides the
// spacing scale — the strip's own cell padding is the theme's S3 — so
// the value reads the published scale directly, the way the review
// camera already hands tokens.Spacing to tabs.Render.
var contentGap = unit.Dp(tokens.Spacing.S4)

// contentSlot is the tab shell's content slot: a tab's content, pushed
// down by contentGap. The gap exposes the strip's own Surface fill (the
// tabs pattern paints the whole panel Surface before the content draws
// over it), so the active tab's Primary underline has quiet ground on
// both sides and reads as a line rather than as the top edge of whatever
// begins below it.
//
// The gap lives here, in the shell, rather than in the one tab that first
// showed the defect. The collision is structural, not a property of the
// tab that reported it: the underline is the strip's bottom two pixels,
// so any content whose first row is a filled band merges with it. The
// case that reported it — the inventory's full-width Primary group banner
// — is no longer drawn, because a single-group tab drops its banner
// (inventory_tabs.go); every tab now opens on a quiet surface. That is an
// accident of the present content and not a contract, and a gap that
// appeared only on the tab of the day would shift every page's first line
// as the user switches tabs. One slot for all five keeps the strip a
// fixed band and costs each page 8 dp.
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
