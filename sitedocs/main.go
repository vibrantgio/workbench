// Command sitedocs is the Vibrant Gio documentation desktop app ("Site
// Docs"). Routing and accordion state live in the canonical MVU
// Model/Update/Messages loop; MessageOp emissions fire within the same
// frame that originated the click.
//
// The window renders one shell per route family, all built once and kept
// subscribed so scroll positions survive navigation:
//
//   - Home  → cadence/shell StackedPage: pinned full-width navbar over
//     the marketing sections (landing.go).
//   - Docs  → cadence/shell ThreeColumn with a nil aside: full-width
//     navbar, accordion sidebar in the leading column, routed page in
//     the main slot.
//   - About → StackedPage with a single prose section.
//
// routedShellLayer selects among the three on every model emission.

package main

import (
	"fmt"
	"image/color"
	"os"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/navbar"
	"github.com/vibrantgio/cadence/shell"
	"github.com/vibrantgio/mvu"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	specsystem "github.com/vibrantgio/spectrum/system"
	specwin "github.com/vibrantgio/spectrum/window"
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
	mvuWin := mvu.NewWindow(
		app.Title("Site Docs"),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)

	w := specwin.New(mvuWin, themeObservable())

	// Build the model observable with mvu.Loop over mvu messages. The
	// window's collector registers on each FrameEvent so MessageOp.Add(gtx.Ops)
	// calls made during layout are collected and delivered here on the same
	// frame; Loop also runs the commands Update returns (this app returns
	// DoNothing everywhere) and emits the seed model first.
	//
	// mvuWin.Messages() drains a channel via rx.Recv, so each emitted message
	// reaches exactly one subscriber. Three streams derive from modelObs —
	// the router's current-page stream plus the docs shell's open-sections
	// and current-page streams — so without multicast those cold
	// subscriptions would each re-drain the channel and split the messages
	// between them. Publish().AutoConnect(3) shares one upstream
	// subscription across exactly those three consumers. NOTE: the count 3
	// is load-bearing — adding another modelObs consumer requires bumping it.
	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
	models, runner := mvu.Loop(mvuWin.Messages(), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(3)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

// themeObservable returns the live system-driven theme stream. The 5 s poll
// interval is the intended low-CPU default, not a workaround: each darwin
// Appearance read is a `defaults` fork+exec (~5.5 ms), so 5 s polling costs
// ~0.1% CPU at idle while keeping dark-mode response well under a second of a
// toggle. See spectrum/system for the dark/accent cadence split and the
// measured cost table.
func themeObservable() rx.Observable[theme.Theme] {
	return specsystem.LiveTheme(5 * time.Second)
}

// buildLayers returns a function that spectrum/window.Render passes the
// per-window theme to. It returns the two rendering layers: a backdrop and
// the routed shell. The model observable drives routing and accordion state.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			routedShellLayer(th, shaper, modelObs),
		}
	}
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

// routedShellLayer builds the three route-family shells once and selects
// among them on every model emission. CombineLatest keeps all three
// subscribed, so switching routes is a pure selection — scroll positions
// and accordion state survive navigation in both directions.
func routedShellLayer(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	modelObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	currentPageObs := rx.Map(modelObs, func(m Model) string { return m.currentPage })
	home := homeShellLayer(th, shaper)
	docs := docsShellLayer(th, shaper, modelObs)
	about := aboutShellLayer(th, shaper)
	combined := rx.CombineLatest4(currentPageObs, home, docs, about)
	return rx.Map(combined, func(n rx.Tuple4[string, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
		switch n.First {
		case pageHome:
			return n.Second
		case pageAbout:
			return n.Fourth
		default: // every docs route, and any unrecognised route
			return n.Third
		}
	})
}

// docsShellLayer composes the accordion sidebar and the routed docs page
// into a ThreeColumn shell (nil aside): the navbar spans the full window
// width and the sidebar sits below it in the leading column.
//
// cadence/shell exposes Sidebar as an rx.Observable[layout.Widget] but Main
// as a static layout.Widget, and the shell re-emits (driving
// spectrum/window's Invalidate) only when one of its input streams emits. So
// the routed page widget is folded onto the sidebar stream: mainObs is
// combined into the sidebar-driving observable, and the selected page widget
// is published into mainCell — a layer-boundary adapter read by the static
// Main slot at frame time. A SetRoute message therefore re-emits the sidebar
// stream, which makes the shell re-emit and the window repaint on the same
// frame.
func docsShellLayer(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	modelObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	openSectionsObs := rx.Map(modelObs, func(m Model) map[int]bool { return m.openSections })
	currentPageObs := rx.Map(modelObs, func(m Model) string { return m.currentPage })

	// Pre-build every docs page once so its state (theme token
	// subscriptions, list scroll position) survives navigation
	// back-and-forth. docsPages is the single source of truth, so the
	// sidebar can never route to a page that is not built here.
	defs := docsPages()
	pageIdx := make(map[string]int, len(defs))
	pageObs := make([]rx.Observable[layout.Widget], len(defs))
	for i, def := range defs {
		pageIdx[def.ID] = i
		pageObs[i] = docsPage(th, shaper, def.Content)
	}

	// mainObs re-emits whenever the current page changes or any page
	// re-renders (theme change). CombineLatest holds the latest widget of
	// every page, so switching routes is a pure selection — no
	// re-subscription, no lost scroll state.
	pagesCombined := rx.CombineLatest(pageObs...)
	mainObs := rx.Map(
		rx.CombineLatest2(currentPageObs, pagesCombined),
		func(n rx.Tuple2[string, []layout.Widget]) layout.Widget {
			if i, ok := pageIdx[n.First]; ok {
				return n.Second[i]
			}
			return n.Second[0] // non-docs routes keep the first page warm
		},
	)

	// mainCell bridges the layer boundary: the combined map below stores
	// the selected Main widget synchronously, and the static Main slot
	// reads it at frame time — the same atomic hand-off mvu/window.go uses
	// for its layer snapshot.
	var mainCell atomic.Value
	sidebarObs := docsSidebar(th, shaper, openSectionsObs)
	sidebarDriven := rx.Map(rx.CombineLatest2(sidebarObs, mainObs), func(n rx.Tuple2[layout.Widget, layout.Widget]) layout.Widget {
		mainCell.Store(n.Second)
		return n.First
	})

	mainSlot := func(gtx layout.Context) layout.Dimensions {
		if w, ok := mainCell.Load().(layout.Widget); ok && w != nil {
			return w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	return shell.Shell(th, shell.Props{
		Layout:  shell.ThreeColumn,
		Sidebar: sidebarDriven,
		Navbar:  navbarProps(th, shaper, pageDocsDefault),
		Main:    mainSlot,
	})
}

// aboutShellLayer renders the About page: a StackedPage with a single
// prose section and the shared footer.
func aboutShellLayer(th rx.Observable[theme.Theme], shaper *text.Shaper) rx.Observable[layout.Widget] {
	return shell.Shell(th, shell.Props{
		Layout: shell.StackedPage,
		Navbar: navbarProps(th, shaper, pageAbout),
		Sections: []rx.Observable[layout.Widget]{
			aboutSection(th, shaper),
			footerSection(th, shaper),
		},
	})
}

// aboutSection is the About page prose: headline plus paragraphs, theme-aware.
func aboutSection(th rx.Observable[theme.Theme], shaper *text.Shaper) rx.Observable[layout.Widget] {
	paragraphs := []string{
		"Site Docs is the documentation and marketing example for Vibrant Gio — a design system for building native desktop applications in Go with Gio.",
		"It is one of the workbench apps that exercise the system end to end, alongside the launcher, feeds, todos, watchlist, iconbrowser and mindchat.",
		"Every layer — prism, cadence, spectrum, pulse, mvu — is MIT licensed and developed in the open at github.com/vibrantgio.",
	}
	type pair struct {
		col tokens.ColorTokens
		typ tokens.TypeScale
	}
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	combined := rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.TypeScale]) pair {
		return pair{col: t.First, typ: t.Second}
	})
	return rx.Map(combined, func(p pair) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			inset := pllayout.Inset(docsOuterInsetDp)
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawLabel(gtx, shaper, "About Vibrant Gio", unit.Sp(p.typ.HeadlineSmall), p.col.OnSurface)
					}),
					layout.Rigid(pllayout.VSpacer(docsCardGapDp)),
				}
				for _, para := range paragraphs {
					children = append(children,
						layout.Rigid(paragraphWidget(shaper, para, p.col.OnSurface, p.typ)),
						layout.Rigid(pllayout.VSpacer(docsProseGapDp)),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}
	})
}

// navbarProps builds the shared navbar for a shell. active names the route
// family whose link renders in the Active state, so each shell's navbar is
// correct by construction. The brand label tracks the theme through the
// same atomic token adapter the code cards use.
func navbarProps(th rx.Observable[theme.Theme], shaper *text.Shaper, active string) navbar.Props {
	isDocs := active != pageHome && active != pageAbout
	return navbar.Props{
		Brand:  brandWidget(th, shaper),
		Shaper: shaper,
		Links: []navbar.Link{
			{Label: "Home", Active: active == pageHome, OnClick: func(gtx layout.Context) {
				mvu.MessageOp{Message: SetRoute{Page: pageHome}}.Add(gtx.Ops)
			}},
			{Label: "Docs", Active: isDocs, OnClick: func(gtx layout.Context) {
				mvu.MessageOp{Message: SetRoute{Page: pageDocsDefault}}.Add(gtx.Ops)
			}},
			{Label: "About", Active: active == pageAbout, OnClick: func(gtx layout.Context) {
				mvu.MessageOp{Message: SetRoute{Page: pageAbout}}.Add(gtx.Ops)
			}},
		},
	}
}

// brandWidget renders the "Vibrant Gio" wordmark in the theme's OnSurface
// colour (the previous hardcoded black was invisible on the dark scheme).
func brandWidget(th rx.Observable[theme.Theme], shaper *text.Shaper) layout.Widget {
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	var cell atomic.Value
	cell.Store(tokens.DefaultLight)
	_ = colObs.Subscribe(rx.GoroutineContext(), func(c tokens.ColorTokens, _ error, done bool) {
		if !done {
			cell.Store(c)
		}
	})
	return func(gtx layout.Context) layout.Dimensions {
		c := cell.Load().(tokens.ColorTokens)
		return drawLabel(gtx, shaper, "Vibrant Gio", unit.Sp(18), c.OnSurface)
	}
}

// drawLabel paints a single-line text label at the current offset.
func drawLabel(gtx layout.Context, shaper *text.Shaper, msg string, size unit.Sp, c color.NRGBA) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	wl := widget.Label{MaxLines: 1}
	return wl.Layout(gtx, shaper, font.Font{}, size, msg, material)
}

// Compile-time anchor.
var _ = widget.Clickable{}
