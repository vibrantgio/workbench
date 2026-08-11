// sim_test.go verifies G5.3a's live behaviours headlessly, at the pixel level,
// against the REAL composed shell — the same widget tree the running app
// renders. Launching the Gio app from a shell has no window-server session, so
// "the app opens with the sidebar populated" is proven here: a Subject-driven
// model into watchlistShellLayer, captured via the headless GPU, with pixel
// assertions on the sidebar and Main regions.
//
// Asserted:
//   - an empty-state seed (no watchlists) renders a sidebar visibly different
//     from a loaded seed (names populate the sidebar),
//   - applying SelectWatchlist changes the Main region (the selected name).
package main

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/theme"
)

// Region windows. The sidebar is the leading 192 dp column; Main starts after
// it, below the 64 dp navbar.
var (
	sidebarRegion = image.Rect(0, 0, wlSidebarWidthDp, shellCanvasH-20)
	mainRegion    = image.Rect(wlSidebarWidthDp+10, 80, shellCanvasW-10, shellCanvasH-20)
)

// TestWatchlistShellHeadless renders the real shell at three model states and
// asserts the pixel-level deltas G5.3a's Measurable describes.
func TestWatchlistShellHeadless(t *testing.T) {
	send, modelObs := rx.Subject[Model](0, 1, 256)
	layer := watchlistShellLayer(rx.Of(theme.Default()), modelObs, filepath.Join(t.TempDir(), "watchlists.json"))

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	size := image.Pt(shellCanvasW, shellCanvasH)
	snap := func(what string) *image.RGBA {
		w := awaitStableWidget(t, emissions, what)
		img := golden.Capture(t, size, scene(w, bg))
		return img
	}

	// Empty-state seed: no watchlists (a present-but-empty file). Sidebar shows
	// the empty-state message.
	send.Next(initialModel(Document{Version: formatVersion, Watchlists: nil}))
	empty := snap("empty-state model")

	// Loaded seed: two watchlists, "majors" selected. The sidebar must now show
	// names — its region differs from the empty state.
	loaded := initialModel(testDoc())
	send.Next(loaded)
	populated := snap("loaded model")
	if n := regionDiff(empty, populated, sidebarRegion); n <= 0 {
		t.Errorf("sidebar unchanged between empty-state and loaded seeds (diff=%d); names did not populate", n)
	}

	// Selecting the other watchlist must change the Main region (selected name).
	next, _ := Update(loaded, SelectWatchlist{Name: "alts"})
	send.Next(next)
	selected := snap("SelectWatchlist(alts)")
	if n := regionDiff(populated, selected, mainRegion); n <= 0 {
		t.Errorf("Main unchanged after SelectWatchlist (diff=%d); selection did not reach Main", n)
	}
}

// ----- headless test helpers -----

// awaitStableWidget drains the emission channel until it has been silent for
// quiet, returning the LAST widget seen. Model changes fan out through
// CombineLatest chains, so a single send can produce several intermediate
// emissions; pixel assertions must run against the settled one.
func awaitStableWidget(t *testing.T, emissions <-chan layout.Widget, what string) layout.Widget {
	t.Helper()
	const quiet = 150 * time.Millisecond
	var last layout.Widget
	deadline := time.Now().Add(2 * time.Second)
	for {
		select {
		case w := <-emissions:
			last = w
		case <-time.After(quiet):
			if last != nil {
				return last
			}
			if time.Now().After(deadline) {
				t.Fatalf("no widget emission after %s", what)
				return nil
			}
		}
	}
}

func scene(w layout.Widget, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

// regionDiff counts differing pixels between a and b inside r. a and b must
// have equal bounds; it panics if they do not, for the reason given on
// pixelDiff.
func regionDiff(a, b *image.RGBA, r image.Rectangle) int {
	if a.Bounds() != b.Bounds() {
		panic(fmt.Sprintf("regionDiff: images must have equal bounds, got %v and %v",
			a.Bounds(), b.Bounds()))
	}
	r = r.Intersect(a.Bounds())
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			off := (y-a.Bounds().Min.Y)*a.Stride + (x-a.Bounds().Min.X)*4
			if a.Pix[off] != b.Pix[off] ||
				a.Pix[off+1] != b.Pix[off+1] ||
				a.Pix[off+2] != b.Pix[off+2] ||
				a.Pix[off+3] != b.Pix[off+3] {
				n++
			}
		}
	}
	return n
}
