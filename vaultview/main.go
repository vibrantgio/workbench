// Command vaultview is a read-only desktop viewer for Obsidian-style
// markdown vaults. Point it at a folder of notes — a vault path on the
// command line wins; without one the last-used vault opens, and on the
// first run an in-app folder browser asks — and the first note renders
// with its frontmatter split into a collapsible properties panel above
// the prose.
//
// The app is the canonical MVU shape: mvu.NewWindow, a theme window with
// a live OS theme (dark mode follows the system), a Model observable
// driven by mvu.Loop, and a patterns/shell ThreeColumn for the vault
// screen. The vault scan runs as an mvu.Do command off the render
// goroutine.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
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

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/toast"
	specsystem "github.com/vibrantgio/theme/system"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
	specwin "github.com/vibrantgio/theme/window"
)

const (
	windowW = 1100
	windowH = 800
)

func main() {
	go run()
	app.Main()
}

func run() {
	// mvu/desktop's full-size-content treatment: on macOS the content
	// extends behind a transparent title bar; elsewhere FullSizeContent
	// returns no options and the window keeps its decorations.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Vault View"),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)...)
	// Gio re-hides the standard window buttons on every configuration
	// rebuild; ShowWindowButtons registers the re-assertion on the mvu
	// OnConfigure seam.
	desktop.ShowWindowButtons(mvuWin)

	w := specwin.New(mvuWin, specsystem.LiveTheme(5*time.Second))

	// mvuWin.Messages() drains a channel, so each message reaches exactly
	// one subscriber. Exactly two streams derive from modelObs — the
	// routed layer's CombineLatest and the toast layer's queue map — so
	// AutoConnect(2) shares the single upstream subscription. NOTE: the
	// count is load-bearing; adding another modelObs consumer requires
	// bumping it.
	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(2)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "vaultview:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// themeTokens is the colour/typography/spacing/density snapshot the app's
// own drawing code reads at frame time. The shaper is the theme's cached
// Typography shaper: the app builds none of its own.
type themeTokens struct {
	col    tokens.ColorTokens
	typ    tokens.Typography
	sp     tokens.SpacingScale
	den    tokens.Density
	shaper *text.Shaper
}

// mirrorTokens subscribes the theme's token streams into an atomic cell
// and returns a frame-time loader — the layer-boundary adapter for
// closures that run outside any rx scope (the shell's static Main slot,
// the picker's frame closure, the navbar brand).
func mirrorTokens(th rx.Observable[theme.Theme]) func() themeTokens {
	var cell atomic.Value
	cell.Store(themeTokens{
		col:    tokens.DefaultLight,
		typ:    tokens.DefaultTypography,
		sp:     tokens.Spacing,
		den:    tokens.Comfortable,
		shaper: tokens.DefaultTypography.Shaper(),
	})
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	spObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.SpacingScale] { return t.Spacing })
	denObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Density] { return t.Density })
	_ = rx.CombineLatest4(colObs, typObs, spObs, denObs).Subscribe(rx.GoroutineContext(),
		func(t rx.Tuple4[tokens.ColorTokens, tokens.Typography, tokens.SpacingScale, tokens.Density], _ error, done bool) {
			if !done {
				typ := t.Second
				cell.Store(themeTokens{col: t.First, typ: typ, sp: t.Third, den: t.Fourth, shaper: typ.Shaper()})
			}
		})
	return func() themeTokens { return cell.Load().(themeTokens) }
}

// buildLayers returns the rendering layers: a backdrop, the routed screen
// (picker or vault), and the toast stack over everything.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		toastsObs := rx.Map(modelObs, func(m Model) []toast.Toast { return m.Toasts.Items() })
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			underTitleBar(routedLayer(th, modelObs)),
			underTitleBar(toast.Stack(th, toast.Props{Position: toast.TopRight, Toasts: toastsObs})),
		}
	}
}

// routedLayer builds the picker and vault screens once and selects
// between them on every model emission. The model is stored into an
// atomic cell as part of the same emission, so the frame-time closures
// of both screens read the state that selected them.
func routedLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	loadTok := mirrorTokens(th)
	var modelCell atomic.Value
	modelCell.Store(Model{})
	loadModel := func() Model { return modelCell.Load().(Model) }

	picker := pickerLayer(th, loadModel, loadTok)
	vault := vaultLayer(th, loadModel, loadTok)

	combined := rx.CombineLatest3(modelObs, picker, vault)
	return rx.Map(combined, func(t rx.Tuple3[Model, layout.Widget, layout.Widget]) layout.Widget {
		modelCell.Store(t.First)
		if t.First.Screen == screenPicker {
			return t.Second
		}
		return t.Third
	})
}

// underTitleBar pads the content down by the native title-bar strip's
// measured height on a full-size-content window; away from the treatment
// it is an exact no-op (TopInset reports 0).
func underTitleBar(content rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return rx.Map(content, func(w layout.Widget) layout.Widget {
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
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	return rx.Map(colors, func(c tokens.ColorTokens) layout.Widget {
		fill := c.Surface
		return func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			paint.FillShape(gtx.Ops, fill, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}
	})
}

// drawLabel paints a single-line text label at the current offset in the
// given Typography role, truncated with an ellipsis when it overflows.
func drawLabel(gtx layout.Context, shaper *text.Shaper, msg string, style tokens.TextStyle, c color.NRGBA) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	return typeset.Layout(gtx, shaper, typeset.Label(style, 1),
		typeset.Font(style, font.Normal), unit.Sp(style.Size), msg, material)
}

// drawText paints multi-line wrapped text in the given Typography role.
func drawText(gtx layout.Context, shaper *text.Shaper, msg string, style tokens.TextStyle, c color.NRGBA) layout.Dimensions {
	mat := op.Record(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	material := mat.Stop()
	wl := typeset.Label(style, 0)
	wl.Alignment = text.Start
	return typeset.Layout(gtx, shaper, wl,
		typeset.Font(style, font.Normal), unit.Sp(style.Size), msg, material)
}
