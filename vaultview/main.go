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
	"math"
	"os"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/toast"
	"github.com/vibrantgio/theme/brand"
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

	// The brand this user kept, if they kept one. It pins the palette the
	// theme stream flips between; which side shows is still the desktop's
	// decision, live, as it always was. With nothing kept the options are
	// empty and the stream is exactly the one this line made before.
	//
	// The same value seeds the token mirror's first cell below, so the
	// opening frames are already in the kept palette rather than flashing
	// the default one at somebody who chose against it.
	kept := brand.Kept()
	opening, _ := kept.Colors()

	w := specwin.New(mvuWin, specsystem.LiveTheme(5*time.Second, kept.Options()...))

	// mvuWin.Messages() drains a channel, so each message reaches exactly
	// one subscriber. Exactly three streams derive from modelObs — the
	// routed layer's CombineLatest, the chooser layer's open flag, and the
	// toast layer's queue map — so AutoConnect(3) shares the single
	// upstream subscription. NOTE: the count is load-bearing; adding
	// another modelObs consumer requires bumping it.
	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(3)

	if err := w.Render(buildLayers(modelObs, opening)).Wait(); err != nil {
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
//
// opening is the palette the cell holds until the streams first emit, a
// moment that spans the first frames. It is the caller's business because
// the caller is the one that knows which palette this run is in: a window
// that seeds it with the package default while its stream is about to emit
// something else opens on a colour nobody chose.
func mirrorTokens(th rx.Observable[theme.Theme], opening tokens.ColorTokens) func() themeTokens {
	var cell atomic.Value
	cell.Store(themeTokens{
		col:    opening,
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
//
// The toasts stand on the midpoint of the window's bottom edge. What a
// rescan reports is a confirmation of something the reader just did, and
// it belongs where they are looking — the foot of the column they are
// reading — rather than in a corner they have no reason to watch. At the
// width this window opens, the column the stack is wide falls entirely
// inside the reading column: the sidebar and the actions at its foot end
// well to the leading side of it, and the backlinks panel begins well to
// the trailing side, so nothing live is under a toast. The layer keeps
// the chrome inset it always had, which now bounds the stack from above:
// a queue tall enough to climb the window stops at the chrome row's foot
// instead of covering the controls standing in it.
func buildLayers(modelObs rx.Observable[Model], opening tokens.ColorTokens) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		loadTok := mirrorTokens(th, opening)
		var modelCell atomic.Value
		modelCell.Store(Model{})
		loadModel := func() Model { return modelCell.Load().(Model) }

		toastsObs := rx.Map(modelObs, func(m Model) []toast.Toast { return m.Toasts.Items() })
		return []rx.Observable[layout.Widget]{
			backdropLayer(th),
			underTitleBar(routedLayer(th, modelObs, &modelCell, loadModel, loadTok)),
			underChrome(chooserLayer(th, modelObs, loadModel, loadTok)),
			underChrome(toast.Stack(th, toast.Props{Position: toast.BottomCenter, Toasts: toastsObs})),
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
		m := t.First
		modelCell.Store(m)
		if m.Screen == screenPicker {
			topBand.Store(unit.Dp(0))
			placeWindowButtons(buttonPlacementFor(m))
			return t.Second
		}
		row := toolbarHeight(loadTok())
		topBand.Store(row)
		placeWindowButtons(buttonPlacementFor(m))
		return t.Third
	})
}

// buttonPlacement is a full placement request for the window buttons:
// where their leading edge sits and the line they are centred on, both in
// dp from the window's top-leading corner, zero per axis meaning the
// platform's own geometry there.
type buttonPlacement struct {
	leading, center unit.Dp
}

// buttonPlacementFor answers where the screen on show puts the window's
// control buttons.
//
// The vault screen states one placement and states it from the window's
// own top and leading edges. The buttons are part of the window, seen
// through whatever the application draws under them, so nothing the
// application draws may move them: not the pane arriving, and not the
// pane leaving. There is deliberately no rail state in this answer —
// asking the buttons back to a different geometry when the pane goes is
// the movement that reading forbids, and the pane is what moved, not
// them.
//
// The picker asks for no placement at all. It claims no band of its own
// and lays its content out below the native strip, so the whole strip is
// the platform's there — the buttons included, at whatever geometry the
// platform keeps them.
func buttonPlacementFor(m Model) buttonPlacement {
	if m.Screen == screenPicker {
		return buttonPlacement{}
	}
	return buttonPlacement{leading: buttonInsetDp, center: buttonCenterDp}
}

// buttonPlace remembers the placement the window buttons were last asked
// for, so the request is sent when it changes and not on every emission.
var buttonPlace atomic.Value

// topBand remembers how tall the band at the top of the window is that
// the screen on show lays out itself: the vault screen's chrome row, and
// zero on the picker, which lays out under the native strip instead. It
// is what tells the two questions below apart — how much is mine, and how
// much is not document.
var topBand atomic.Value

// placeWindowButtons asks the window to place its control buttons where
// the screen on show says, and sends the request only when the answer
// changes — which, once the vault screen is up, is never again.
//
// Screen by screen rather than once at startup, because the two screens
// answer differently: the vault screen takes the top of the window and
// states the placement, and the picker leaves the strip to the platform.
func placeWindowButtons(p buttonPlacement) {
	if prev, ok := buttonPlace.Load().(buttonPlacement); ok && prev == p {
		return
	}
	buttonPlace.Store(p)
	desktop.PlaceWindowButtonsAt(p.leading, p.center)
}

// underTitleBar pads a layer down by the native title-bar strip's
// measured height, which is what a full-size-content window leaves above
// content the application has not claimed — the picker screen's case.
// Where the screen lays out its own top band, and away from the treatment
// altogether, this is an exact no-op: the vault screen's chrome row and
// sidebar strip are drawn in the strip on purpose, and padding them down
// would put the band back that this window exists without.
func underTitleBar(content rx.Observable[layout.Widget]) rx.Observable[layout.Widget] {
	return insetTop(content, screenTopInset)
}

// screenTopInset is the native strip's height where the screen on show
// has not taken the top of the window for itself, and zero where it has.
func screenTopInset() unit.Dp {
	if band, ok := topBand.Load().(unit.Dp); ok && band > 0 {
		return 0
	}
	return desktop.TopInset()
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
// than to the document: the band the screen lays out itself, and the
// native strip where that is taller — with the rail hidden the buttons
// are back on the platform's own line, and the strip they stand in can
// reach below a row shorter than it. An overlay clears the larger of the
// two, since either would put a toast on top of a live control.
func chromeHeight() unit.Dp {
	band, _ := topBand.Load().(unit.Dp)
	if inset := desktop.TopInset(); inset > band {
		return inset
	}
	return band
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

// Mark sizes. The set the marks come from was drawn and weight-matched
// against the platform's own symbols at 16, 20 and 24 dp, so every call
// site here asks for one of those and none for a size in between.
const (
	// markSmallDp is the size a mark takes beside a line of text: the
	// disclosure marks, which stand next to a fourteen-point label, and
	// the history controls, which stand next to the breadcrumb.
	markSmallDp = 16
	// markMediumDp is the size a mark takes as a control in its own
	// right, with no text on its row to answer to. Nothing in this
	// window is one — every mark here either sits in a text row or is
	// the window's own sidebar control — so the middle of the set is
	// named, and taken by nothing.
	markMediumDp = 20
	// markLargeDp is the top of the set's range, and what a mark with
	// axis-aligned edges takes. Those edges land on whole device pixels
	// at 16 and 24 dp and between them at 20, where the sidebar figure's
	// faint list lines smear into one grey column.
	markLargeDp = 24
)

// drawMark paints one of the design system's marks into a square of
// sizeDp at the current offset, in the colour the caller chose, and
// reports that square as its dimensions. A name the set does not carry
// paints nothing and still measures, so a missing mark leaves a gap
// rather than moving everything around it.
//
// The marks are drawn rather than typeset, so what the window shows does
// not depend on which faces the host carries — the app used to typeset
// them and had glyphs resolve through system fallback.
func drawMark(gtx layout.Context, name icons.Name, sizeDp unit.Dp, c color.NRGBA) layout.Dimensions {
	px := gtx.Dp(sizeDp)
	if mark := icons.Mark(name); mark != nil {
		mark(gtx, px, c)
	}
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// drawDisclosure paints the disclosure mark for a row that is open or
// closed. There is one drawing, not two: the set draws it as the row
// stands closed, and an open row turns it a quarter turn — which is what
// the platform does, and what keeps the mark recognisable through the
// turn. The mark occupies the whole square, so the turn is about the
// square's own centre.
func drawDisclosure(gtx layout.Context, open bool, sizeDp unit.Dp, c color.NRGBA) layout.Dimensions {
	if open {
		half := float32(gtx.Dp(sizeDp)) / 2
		defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), math.Pi/2)).Push(gtx.Ops).Pop()
	}
	return drawMark(gtx, icons.Disclosure, sizeDp, c)
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
