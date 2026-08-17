// main.go is the app's entry point and its layer composition: the
// canonical MVU shape — mvu.NewWindow, a theme window with a live OS
// theme (dark mode follows the system), a Model observable driven by
// mvu.Loop, and an app-local column frame for the vault screen. The
// vault scan runs as an mvu.Do command off the render goroutine.
//
// The package's own documentation — what the app is, and what a link
// means when it is clicked — lives in doc.go.

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
	// one subscriber. Exactly three streams derive from modelObs — the
	// routed layer's CombineLatest, the chooser layer's open flag, and the
	// toast layer's queue map — so AutoConnect(3) shares the single
	// upstream subscription. NOTE: the count is load-bearing; adding
	// another modelObs consumer requires bumping it.
	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(3)

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
// closures that run outside any rx scope (the vault frame's chrome row
// and note column, the picker's frame closure).
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

// buildLayers returns the rendering layers, back to front: a backdrop,
// the routed screen (picker or vault), the ambiguity chooser, and the
// toast stack over everything.
//
// The model cell and the token mirror are built here and shared by every
// layer's frame-time closures, so the chooser reads the same snapshot the
// screen beneath it was laid out from. The routed layer owns the store,
// since its emission is the one that selects a screen.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		loadTok := mirrorTokens(th)
		var modelCell atomic.Value
		modelCell.Store(Model{})
		loadModel := func() Model { return modelCell.Load().(Model) }

		toastsObs := rx.Map(modelObs, func(m Model) []toast.Toast { return m.Toasts.Items() })
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			underTitleBar(routedLayer(th, modelObs, &modelCell, loadModel, loadTok)),
			underChrome(chooserLayer(th, modelObs, loadModel, loadTok)),
			underChrome(toast.Stack(th, toast.Props{Position: toast.TopRight, Toasts: toastsObs})),
		}
	}
}

// routedLayer builds the picker and vault screens once and selects
// between them on every model emission. The model is stored into the
// shared cell as part of the same emission, so the frame-time closures
// of both screens read the state that selected them.
func routedLayer(
	th rx.Observable[theme.Theme],
	modelObs rx.Observable[Model],
	modelCell *atomic.Value,
	loadModel func() Model,
	loadTok func() themeTokens,
) rx.Observable[layout.Widget] {
	picker := pickerLayer(th, loadModel, loadTok)
	vault := vaultLayer(th, loadModel, loadTok)

	combined := rx.CombineLatest3(modelObs, picker, vault)
	return rx.Map(combined, func(t rx.Tuple3[Model, layout.Widget, layout.Widget]) layout.Widget {
		modelCell.Store(t.First)
		if t.First.Screen == screenPicker {
			placeWindowButtons(0)
			return t.Second
		}
		placeWindowButtons(toolbarHeight(loadTok()) / 2)
		return t.Third
	})
}

// buttonLine remembers the line the window buttons were last asked to sit
// on, so the request is sent when it changes and not on every emission.
var buttonLine atomic.Value

// placeWindowButtons asks the window to centre its control buttons on the
// given line — which is the middle of the vault screen's chrome row, so
// that the row and the buttons share one band, and zero on the picker,
// which has no such row and lets the platform keep its own geometry.
//
// Screen by screen rather than once at startup, because the two screens
// answer differently: taking the top strip is the chrome row's doing, and
// where there is no row the strip is not the application's to take.
func placeWindowButtons(center unit.Dp) {
	if prev, ok := buttonLine.Load().(unit.Dp); ok && prev == center {
		return
	}
	buttonLine.Store(center)
	desktop.PlaceWindowButtons(center)
}

// underTitleBar pads a layer down by the native title-bar strip's
// measured height, which is what a full-size-content window leaves above
// content the application has not claimed — the picker screen's case.
// Where the chrome row has taken the strip, and away from the treatment
// altogether, the measurement is zero and this is an exact no-op.
func underTitleBar(content rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return insetTop(content, desktop.TopInset)
}

// underChrome pads a layer down past everything at the top of the window
// that is not document: the native strip where it still stands, and the
// chrome row's own height where the row has taken the strip instead. It
// is what the overlays use, because a toast or a dialog landing on the
// chrome row would cover controls that are live under it.
func underChrome(content rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return insetTop(content, chromeHeight)
}

// chromeHeight is the window's top band that belongs to chrome rather
// than to the document, from whichever of the two owns it.
func chromeHeight() unit.Dp {
	if inset := desktop.TopInset(); inset > 0 {
		return inset
	}
	// The buttons are centred in the chrome row, so the line they were
	// placed on is half its height.
	if line, ok := buttonLine.Load().(unit.Dp); ok {
		return 2 * line
	}
	return 0
}

// insetTop offsets a layer down by a height measured afresh each frame.
func insetTop(content rx.Observable[layout.Widget], height func() unit.Dp) rx.Observable[layout.Widget] {
	return rx.Map(content, func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			inset := gtx.Dp(height())
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
