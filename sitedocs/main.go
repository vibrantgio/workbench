// Command sitedocs is the Vibrant Gio documentation desktop app ("Site
// Docs"). Routing and accordion state live in the canonical MVU
// Model/Update/Messages loop; MessageOp emissions fire within the same
// frame that originated the click.
//
// The window renders one shell per route family, all built once and kept
// subscribed so scroll positions survive navigation:
//
//   - Home  → patterns/shell StackedPage: pinned full-width navbar over
//     the marketing sections (landing.go).
//   - Docs  → patterns/shell ThreeColumn with a nil aside: full-width
//     navbar, the guide's outline tree in the leading column, the one
//     guide document (llms.txt) in the main slot.
//   - About → StackedPage with a single prose section.
//
// routedShellLayer selects among the three on every model emission.

package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
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
	// Dock and VoiceOver read it all the same.
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

	w := specwin.New(mvuWin, themeObservable())

	// Build the model observable with mvu.Loop over mvu messages. The
	// window's collector registers on each FrameEvent so MessageOp.Add(gtx.Ops)
	// calls made during layout are collected and delivered here on the same
	// frame; Loop also runs the commands Update returns (this app returns
	// DoNothing everywhere) and emits the seed model first.
	//
	// mvuWin.Messages() drains a channel via rx.Recv, so each emitted message
	// reaches exactly one subscriber. Two streams derive from modelObs —
	// the router's current-page stream plus the docs shell's outline-state
	// stream — so without multicast those cold subscriptions would each
	// re-drain the channel and split the messages between them.
	// Publish().AutoConnect(2) shares one upstream subscription across
	// exactly those two consumers. NOTE: the count 2 is load-bearing —
	// adding another modelObs consumer requires bumping it.
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

// themeObservable returns the live system-driven theme stream, dressed in
// the brand this user kept if they kept one. The 5 s poll interval is the
// intended low-CPU default, not a workaround: each darwin Appearance read
// is a `defaults` fork+exec (~5.5 ms), so 5 s polling costs ~0.1% CPU at
// idle while keeping dark-mode response well under a second of a toggle.
func themeObservable() rx.Observable[theme.Theme] {
	return specsystem.LiveTheme(5*time.Second, brand.Kept().Options()...)
}

// themeTokens is the colour/typography snapshot the app's own drawing code
// reads at frame time. The shaper is the theme's cached Typography shaper
// (F1.4): the app builds none of its own, so the typefaces — Roboto, plus
// the Roboto Mono face the docs pages' code style names — come from the
// theme.
type themeTokens struct {
	col    tokens.ColorTokens
	typ    tokens.Typography
	shaper *text.Shaper
}

// mirrorTokens subscribes the theme's Color and Typography streams into an
// atomic cell and returns a frame-time loader. It is the layer-boundary
// adapter for closures that run outside any rx scope (static component
// slots, navbar widgets) — the same hand-off pattern feeds and vaultview
// use.
func mirrorTokens(th rx.Observable[theme.Theme]) func() themeTokens {
	var cell atomic.Value
	// First-frame typography follows the kept brand so a JetBrains Mono
	// theme cannot flash Roboto Mono on the navbar before the stream emits,
	// and so the first emoji cannot flash tofu: Brand.Typography is
	// CodeFace then WithEmoji, the same value the stream emits.
	// Goldens and unit tests go through theme.Default(), which is Roboto Mono.
	opening := brand.Kept().Typography()
	cell.Store(themeTokens{
		col:    tokens.DefaultLight,
		typ:    opening,
		shaper: opening.Shaper(),
	})
	colorObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	_ = rx.CombineLatest2(colorObs, typObs).Subscribe(rx.GoroutineContext(), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography], _ error, done bool) {
		if !done {
			typ := t.Second
			cell.Store(themeTokens{col: t.First, typ: typ, shaper: typ.Shaper()})
		}
	})
	return func() themeTokens { return cell.Load().(themeTokens) }
}

// buildLayers returns a function that theme/window.Render passes the
// per-window theme to. It returns the two rendering layers: a backdrop and
// the routed shell. The model observable drives routing and accordion state.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			underTitleBar(routedShellLayer(th, modelObs)),
		}
	}
}

// underTitleBar pads the routed shell down by the native title-bar strip's
// measured height on a full-size-content window. desktop.TopInset is read at
// frame time: it reports 0 until the window's first frame, in headless tests,
// and on every platform but macOS, so away from the treatment (goldens
// included) the wrapper is an exact no-op. The strip itself is paint-only —
// what shows through the transparent title bar is the backdrop layer's
// Surface fill, the same colour the navbar paints, so the header reads as one
// band extending up behind the traffic lights — and because the whole shell
// starts below the strip, no interactive control ever sits in it, leading
// ~80 dp (the window buttons' territory) included.
func underTitleBar(shellObs rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(shellObs, func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			inset := gtx.Dp(desktop.TopInset())
			if inset <= 0 {
				return w(gtx)
			}
			size := gtx.Constraints.Max
			defer op.Offset(image.Pt(0, inset)).Push(gtx.Ops).Pop()
			gtx.Constraints.Max.Y -= inset
			if gtx.Constraints.Min.Y > gtx.Constraints.Max.Y {
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			}
			w(gtx)
			return layout.Dimensions{Size: size}
		}
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

// routedShellLayer builds the route-family shells once and selects
// among them on every model emission. CombineLatest keeps all of them
// subscribed, so switching routes is a pure selection — scroll positions
// and accordion state survive navigation in both directions.
func routedShellLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	currentPageObs := rx.Map(modelObs, func(m Model) string { return m.currentPage })
	home := homeShellLayer(th)
	docs := docsShellLayer(th, modelObs)
	about := aboutShellLayer(th)
	gallery := galleryShellLayer(th)
	combined := rx.CombineLatest5(currentPageObs, home, docs, about, gallery)
	return rx.Map(combined, func(n rx.Tuple5[string, layout.Widget, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
		switch n.First {
		case pageHome:
			return n.Second
		case pageAbout:
			return n.Fourth
		case pageGallery:
			return n.Fifth
		default: // every docs route, and any unrecognised route
			return n.Third
		}
	})
}

// docsShellLayer composes the Docs experience into a ThreeColumn shell
// (nil aside): the navbar spans the full window width; the leading column
// is the outline tree of the one guide document; the main slot is that
// document — llms.txt rendered whole by vibrantgio/markdown. The document
// and its parse are built once, so scroll position survives navigation
// and an outline click's ScrollToBlock moves the same reader.
//
// patterns/shell exposes Sidebar as an rx.Observable[layout.Widget] but Main
// as a static layout.Widget, and the shell re-emits (driving
// theme/window's Invalidate) only when one of its input streams emits. So
// the document widget is folded onto the sidebar stream: mainObs is
// combined into the sidebar-driving observable, and the latest document
// widget is published into mainCell — a layer-boundary adapter read by the
// static Main slot at frame time. A ToggleOutline or SelectHeading message
// therefore re-emits the sidebar stream, which makes the shell re-emit and
// the window repaint on the same frame.
func docsShellLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
) rx.Observable[layout.Widget] {
	return docsShellLayerFrom(th, modelObs, loadGuide())
}

// docsShellLayerFrom is docsShellLayer's source-injected core: tests hand
// it a fixture so no test run can ever reach the checkout file or the
// network.
func docsShellLayerFrom(
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

	// mainCell bridges the layer boundary: the combined map below stores
	// the selected Main widget synchronously, and the static Main slot
	// reads it at frame time — the same atomic hand-off mvu/window.go uses
	// for its layer snapshot.
	var mainCell atomic.Value
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
		Navbar:  navbarProps(mirrorTokens(th), pageDocsDefault),
		Main:    mainSlot,
	})
}

// aboutShellLayer renders the About page: a StackedPage with a single
// prose section and the shared footer.
func aboutShellLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	return shell.Shell(th, shell.Props{
		Layout: shell.StackedPage,
		Navbar: navbarProps(mirrorTokens(th), pageAbout),
		Sections: []rx.Observable[layout.Widget]{
			aboutSection(th),
			footerSection(th),
		},
	})
}

// aboutSection is the About page prose: headline plus paragraphs, theme-aware.
// The heading sits in HeadlineSmall on the Text pin; the paragraphs in
// BodyMedium on the body-text ramp step. Both shape with the theme's shaper.
func aboutSection(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	paragraphs := []string{
		"Site Docs is the documentation and marketing example for Vibrant Gio — a design system for building native desktop applications in Go with Gio.",
		"It is one of the workbench apps that exercise the system end to end, alongside the launcher, feeds, todos, iconbrowser, mindchat and vaultview.",
		"Every layer — components, patterns, theme, effects, mvu — is MIT licensed and developed in the open at github.com/vibrantgio.",
	}
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	combined := rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themeTokens {
		typ := t.Second
		return themeTokens{col: t.First, typ: typ, shaper: typ.Shaper()}
	})
	return rx.Map(combined, func(p themeTokens) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			inset := complayout.Inset(docsOuterInsetDp)
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawLabel(gtx, p.shaper, "About Vibrant Gio", p.typ.HeadlineSmall, p.col.Text)
					}),
					layout.Rigid(complayout.VSpacer(docsCardGapDp)),
				}
				for _, para := range paragraphs {
					children = append(children,
						layout.Rigid(paragraphWidget(p.shaper, para, p.col.Ramps.Neutral.Step(900), p.typ.BodyMedium)),
						layout.Rigid(complayout.VSpacer(docsProseGapDp)),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}
	})
}

// navbarProps builds the shared navbar for a shell. active names the route
// family whose link renders in the Active state, so each shell's navbar is
// correct by construction. The brand label is the app's own text, so it
// reads the theme snapshot from loadTok at frame time: TitleMedium on the
// Text pin, shaped with the theme's shaper. No Shaper prop is passed — the
// navbar shapes its links with the theme's Typography.Shaper().
func navbarProps(loadTok func() themeTokens, active string) navbar.Props {
	isDocs := active != pageHome && active != pageAbout
	brand := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		return drawLabel(gtx, s.shaper, "Vibrant Gio", s.typ.TitleMedium, s.col.Text)
	}
	return navbar.Props{
		Brand: brand,
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

// drawLabel paints a single-line text label at the current offset in the
// given Typography role: typeface, weight, size and line height all come
// from the theme's TextStyle.
func drawLabel(gtx layout.Context, shaper *text.Shaper, msg string, style tokens.TextStyle, c color.NRGBA) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	return typeset.Layout(gtx, shaper, typeset.Label(style, 1),
		typeset.Font(style, font.Normal), unit.Sp(style.Size), msg, material)
}

// Compile-time anchor.
var _ = widget.Clickable{}
