// g53b_sim_test.go verifies the G5.3b symbols editor headlessly, against the
// REAL composed shell — the same widget tree the running app renders. Launching
// the Gio app from a shell has no window-server session, so every live
// behaviour is proven here at the pixel level (or, for persistence, at the
// store level — see store_test.go's TestSaveRoundTripPersistsEdits).
//
// Asserted:
//   - a golden of the add/edit modal body (light + dark, with the empty-Symbol
//     alert banner and all four fields) via the static Render paths,
//   - OpenAddSymbol paints the modal scrim over the window,
//   - an empty SubmitSymbol raises the alert band,
//   - a non-empty SubmitSymbol updates the symbols table (a new row appears),
//   - the toast.Notify → toast.Stack render path (the package-global side-channel
//     the model-driven tests cannot reach, since the write+toast fire from the
//     submit callback, not the reducer).
//
// Verification is HEADLESS throughout: there is no GUI driving; clicks are
// modelled by applying messages to the model directly and asserting rendered
// output, mirroring feeds/g52d_sim_test.go.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/alert"
	"github.com/vibrantgio/cadence/card"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/toast"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

var goldenUpdate = flag.Bool("golden.update", false, "overwrite golden images with current output")

// modalCanvas is the canvas the modal golden draws into.
const (
	modalCanvasW = 600
	modalCanvasH = 620
)

var modalSharpRadius = tokens.RadiusScale{}

// staticSymbolModalBody assembles the modal Body from the STATIC Render paths of
// the same components the live addSymbolModal composes: a card wrapping the
// error alert (shown to capture the empty-submit state), the four fields, and
// the Save button. Sharp radii + the static Render paths keep the golden
// deterministic. Field placeholders model an edit-mode pre-population (the
// current row's values shown as placeholders — the G5.3b workaround).
func staticSymbolModalBody(shaper *text.Shaper, colors tokens.ColorTokens) layout.Widget {
	body := func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		gap := gtx.Dp(unit.Dp(symGapDp))
		alertH := gtx.Dp(unit.Dp(symAlertHDp))
		fieldH := gtx.Dp(unit.Dp(symFieldHDp))
		btnH := gtx.Dp(unit.Dp(symBtnHDp))
		y := 0
		place := func(wdg layout.Widget, h int) {
			s := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			cg := gtx
			cg.Constraints = layout.Exact(image.Pt(w, h))
			wdg(cg)
			s.Pop()
			y += h + gap
		}

		place(alert.Render(shaper, alert.Props{Variant: alert.Error, Title: "Symbol is required"},
			colors, tokens.Spacing, modalSharpRadius, tokens.DefaultTypeScale), alertH)
		for _, ph := range []string{"BTC/USD", "Coinbase", "1h", "Notes"} {
			place(input.Render(shaper, ph, colors, tokens.Spacing, modalSharpRadius,
				tokens.DefaultTypeScale, input.RenderState{}), fieldH)
		}
		place(button.Render(shaper, "Save", colors, tokens.Spacing, modalSharpRadius,
			tokens.DefaultTypeScale, button.RenderState{}), btnH)
		y -= gap
		return layout.Dimensions{Size: image.Pt(w, y)}
	}
	return func(gtx layout.Context) layout.Dimensions {
		c := card.Render(card.Props{Body: body}, colors, tokens.Spacing, modalSharpRadius)
		return c(gtx)
	}
}

// TestSymbolModalGolden renders the add/edit modal (open, with the alert banner
// and all four fields) in light and dark token sets.
func TestSymbolModalGolden(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"symbol-modal-light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"symbol-modal-dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := staticSymbolModalBody(shaper, tc.colors)
			m := modal.Render(shaper, modal.Props{Title: "Symbol", Body: body, Shaper: shaper},
				true, tc.colors, tokens.Spacing, modalSharpRadius, tokens.DefaultTypeScale)
			renderGolden(t, tc.name, image.Pt(modalCanvasW, modalCanvasH), scene(m, tc.bg))
		})
	}
}

// scrimRegion samples the centre of the window, where an open modal paints its
// scrim + surface over the shell.
var scrimRegion = image.Rect(shellCanvasW/2-200, shellCanvasH/2-180, shellCanvasW/2+200, shellCanvasH/2+180)

// TestG53bSymbolEditorStatesHeadless renders the real shell at the CRUD model
// states and asserts the pixel-level deltas the G5.3b Measurable describes.
func TestG53bSymbolEditorStatesHeadless(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	send, modelObs := rx.Subject[Model](0, 1, 256)
	storePath := filepath.Join(t.TempDir(), "watchlists.json")
	layer := watchlistShellLayer(rx.Of(theme.Default()), shaper, modelObs, storePath)

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	}, rx.Goroutine)
	defer sub.Unsubscribe()

	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	size := image.Pt(shellCanvasW, shellCanvasH)
	snap := func(what string) *image.RGBA {
		w := awaitStableWidget(t, emissions, what)
		img := capture(t, size, scene(w, bg))
		if img == nil {
			t.Skip("headless rendering unavailable")
		}
		return img
	}

	m := initialModel(testDoc()) // "majors" selected, two symbols
	send.Next(m)
	closed := snap("initial model")

	// OpenAddSymbol paints the modal scrim + surface over the whole window.
	m, _ = Update(m, OpenAddSymbol{})
	send.Next(m)
	modalOpen := snap("OpenAddSymbol")
	if n := regionDiff(closed, modalOpen, scrimRegion); n <= 0 {
		t.Errorf("window unchanged after OpenAddSymbol (diff=%d in scrim region); modal did not open", n)
	}

	// Empty SubmitSymbol raises the alert band inside the modal.
	m, _ = Update(m, SubmitSymbol{Symbol: ""})
	send.Next(m)
	withAlert := snap("SubmitSymbol(empty)")
	if !m.modalError {
		t.Fatal("empty submit did not set modalError")
	}
	if n := regionDiff(modalOpen, withAlert, scrimRegion); n <= 0 {
		t.Errorf("modal unchanged after empty submit (diff=%d); alert did not appear", n)
	}

	// A non-empty SubmitSymbol appends a row and closes the modal — the Main
	// table now shows the new symbol, so the Main region changes.
	m, _ = Update(m, SubmitSymbol{Symbol: "AVAX/USD", Exchange: "Binance"})
	send.Next(m)
	added := snap("SubmitSymbol(non-empty)")
	wl, _ := m.selectedWatchlist()
	if len(wl.Symbols) != 3 || wl.Symbols[2].Symbol != "AVAX/USD" {
		t.Fatalf("non-empty submit did not append the symbol: %+v", wl.Symbols)
	}
	if n := regionDiff(closed, added, mainRegion); n <= 0 {
		t.Errorf("Main table unchanged after add (diff=%d); new row did not appear", n)
	}

	// Edit row 0 in place: reopen pre-populated, change the exchange, submit.
	m, _ = Update(m, OpenEditSymbol{Row: 0})
	send.Next(m)
	_ = snap("OpenEditSymbol(0)")
	if m.editIndex != 0 || m.editSeed.Symbol != "BTC/USD" {
		t.Fatalf("OpenEditSymbol did not seed the edit target: editIndex=%d seed=%+v", m.editIndex, m.editSeed)
	}
	m, _ = Update(m, SubmitSymbol{Symbol: "BTC/USD", Exchange: "Kraken", Timeframe: "1h"})
	send.Next(m)
	edited := snap("SubmitSymbol(edit)")
	wl, _ = m.selectedWatchlist()
	if wl.Symbols[0].Exchange != "Kraken" {
		t.Fatalf("edit did not apply: %+v", wl.Symbols[0])
	}
	if n := regionDiff(added, edited, mainRegion); n <= 0 {
		t.Errorf("Main table unchanged after edit (diff=%d); edited row did not update", n)
	}
}

// TestToastNotifyRendersInStack exercises the toast.Notify → package Subject →
// Stack render path — the package-global side-channel the addSymbolModal submit
// callback fires on a successful save. Driving Update directly never invokes it,
// so this closes that verification gap (copied from feeds/g52d_sim_test.go): an
// empty stack renders no pixels; Notify re-emits the stack widget with a diff.
func TestToastNotifyRendersInStack(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	stackObs := toast.Stack(rx.Of(theme.Default()), toast.Props{
		Position: toast.TopRight,
		Shaper:   shaper,
	})

	emissions := make(chan layout.Widget, 16)
	sub := stackObs.Subscribe(func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	}, rx.Goroutine)
	defer sub.Unsubscribe()

	size := image.Pt(600, 300)
	empty := awaitStableWidget(t, emissions, "seeded empty stack")
	before := capture(t, size, scene(empty, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))
	if before == nil {
		t.Skip("headless rendering unavailable")
	}

	toast.Notify(toast.Success, "Saved")
	after := awaitStableWidget(t, emissions, "Notify ping")
	got := capture(t, size, scene(after, color.NRGBA{R: 240, G: 240, B: 240, A: 255}))
	if got == nil {
		t.Skip("headless rendering unavailable")
	}
	if n := pixelDiff(before, got); n <= 0 {
		t.Errorf("stack frame unchanged after toast.Notify (diff=%d); toast did not render", n)
	}
}

// ----- inlined golden harness (mirrors feeds/feeds_test.go) -----

func renderGolden(t *testing.T, name string, size image.Point, draw layout.Widget) {
	t.Helper()
	img := capture(t, size, draw)
	if img == nil {
		t.Skip("headless rendering unavailable")
		return
	}
	path := filepath.Join("testdata", name+".png")
	if *goldenUpdate {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := writePNG(path, img); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	stored, err := readPNG(path)
	if err != nil {
		t.Skipf("%s not found; run go test -golden.update to create (err=%v)", path, err)
		return
	}
	// n != 0 (not n > 0): pixelDiff returns -1 on a bounds mismatch, which a
	// >0 check would pass silently — a size change must fail too.
	if n := pixelDiff(stored, img); n != 0 {
		t.Errorf("golden %s differs in %d pixels; run go test -golden.update to refresh", name, n)
	}
}

// pixelDiff counts differing pixels across the whole image (0 when identical).
func pixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	n := 0
	for i := 0; i < len(a.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] ||
			a.Pix[i+2] != b.Pix[i+2] || a.Pix[i+3] != b.Pix[i+3] {
			n++
		}
	}
	return n
}

func writePNG(path string, img *image.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func readPNG(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)
	return rgba, nil
}
