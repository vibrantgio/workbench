// Command sitedocs is the VibrantGIO documentation desktop app. GX.9
// migrates routing and accordion state to the canonical MVU
// Model/Update/Messages loop; MessageOp emissions replace the former
// rx.Subject + atomic.Pointer workaround so every interactive callback
// fires within the same frame that originated the click.

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
		app.Title("VibrantGIO Docs"),
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
	// reaches exactly one subscriber. docsShellLayer derives two streams from
	// modelObs (open-sections and current-page), so without multicast those two
	// cold subscriptions would each re-drain the channel and split the messages
	// between them. Publish().AutoConnect(2) shares one upstream subscription
	// across exactly those two consumers. NOTE: the count 2 is load-bearing —
	// adding a third modelObs consumer requires bumping it.
	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
	models, runner := mvu.Loop(mvuWin.Messages(), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(2)

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
// toggle. See spectrum/system (GX.11) for the dark/accent cadence split and the
// measured cost table.
func themeObservable() rx.Observable[theme.Theme] {
	return specsystem.LiveTheme(5 * time.Second)
}

// buildLayers returns a function that spectrum/window.Render passes the
// per-window theme to. It returns the two rendering layers: a backdrop and the
// docs shell. The model observable drives routing and accordion state.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			docsShellLayer(th, shaper, modelObs),
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

// docsShellLayer composes the docs sidebar, the cadence/navbar header, and the
// routed Main slot into a SidebarHeaderMain shell. Routing and accordion open
// state are driven by modelObs; theme tokens flow independently through th.
//
// cadence/shell exposes Sidebar as an rx.Observable[layout.Widget] but Main as
// a static layout.Widget, and Shell re-emits (driving spectrum/window's
// Invalidate) only when its Sidebar or Navbar stream emits. So the routed Main
// widget is folded onto the sidebar stream: mainObs is combined into the
// sidebar-driving observable, and the selected page widget is published into
// mainCell — a layer-boundary adapter read by the static Main slot at frame
// time. A SetRoute message therefore re-emits the sidebar stream, which makes
// Shell re-emit and the window repaint on the same frame. (The previous
// routedMain mirrored page + selection into atomic pointers disconnected from
// the layer chain, so navigation never triggered Invalidate — the
// FEEDBACK-G5.1 "click does nothing until the mouse moves" defect.)
func docsShellLayer(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	modelObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	openSectionsObs := rx.Map(modelObs, func(m Model) map[int]bool { return m.openSections })
	currentPageObs := rx.Map(modelObs, func(m Model) string { return m.currentPage })

	// Pre-build all page observables once so their state (theme token
	// subscriptions, list scroll positions) survives navigation back-and-forth.
	gotoDocs := func(gtx layout.Context) {
		mvu.MessageOp{Message: SetRoute{Page: pageDocsGettingStarted}}.Add(gtx.Ops)
	}
	landing := landingMain(th, shaper, gotoDocs)
	gettingStarted := docsPage(th, shaper, gettingStartedContent())
	phasesOverview := docsPage(th, shaper, phasesOverviewContent())
	componentRef := docsPage(th, shaper, componentReferenceContent())

	// mainObs re-emits whenever the current page changes or any page's content
	// re-renders (theme change). CombineLatest holds the latest widget of each
	// page, so switching routes is a pure selection — no re-subscription, no
	// lost scroll state.
	mainObs := rx.Map(
		rx.CombineLatest5(currentPageObs, landing, gettingStarted, phasesOverview, componentRef),
		func(n rx.Tuple5[string, layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			switch n.First {
			case pageDocsGettingStarted:
				return n.Third
			case pageDocsPhasesOverview:
				return n.Fourth
			case pageDocsComponentRef:
				return n.Fifth
			default: // pageHome and any unrecognised route fall back to landing.
				return n.Second
			}
		},
	)

	// mainCell bridges the layer boundary: the combined map below stores the
	// selected Main widget synchronously, and the static Main slot reads it at
	// frame time — the same atomic hand-off mvu/window.go uses for its layer
	// snapshot. (It is an atomic.Value holding a layout.Widget, distinct from
	// the per-widget atomic mirrors GX.9 removed, which severed state from the
	// layer chain.)
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
		Layout:  shell.SidebarHeaderMain,
		Sidebar: sidebarDriven,
		Navbar:  navbarProps(shaper),
		Main:    mainSlot,
	})
}

func navbarProps(shaper *text.Shaper) navbar.Props {
	brand := func(gtx layout.Context) layout.Dimensions {
		return drawLabel(gtx, shaper, "VibrantGIO", unit.Sp(18), color.NRGBA{A: 0xff})
	}
	return navbar.Props{
		Brand:  brand,
		Shaper: shaper,
		Links: []navbar.Link{
			{Label: "Home", Active: true, OnClick: func(gtx layout.Context) {
				mvu.MessageOp{Message: SetRoute{Page: pageHome}}.Add(gtx.Ops)
			}},
			{Label: "Docs", OnClick: func(gtx layout.Context) {
				mvu.MessageOp{Message: SetRoute{Page: pageDocsGettingStarted}}.Add(gtx.Ops)
			}},
			{Label: "About", OnClick: func(_ layout.Context) {}},
		},
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
