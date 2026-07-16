// Command watchlist is the vibrantgio watchlists editor. G5.3a is the read +
// display skeleton: it loads the watchlists file from disk (writing a starter
// on first run), opens a window, and renders a SidebarHeaderMain shell — the
// navbar brand "Watchlist editor" + a no-op "New watchlist" action, a sidebar
// listing watchlist names (or an empty-state message), and a Main pane showing
// the selected name.
//
// Bootstrap mirrors feeds/main.go: mvu.NewWindow + spectrum/window.New +
// spectrum/system.LiveTheme. (The plan's "prism/initial" citation is stale —
// logged in FEEDBACK-G5.3.md.) Persistence is read-on-startup only; the
// platform path is resolved here and passed in, so all file logic is testable
// against t.TempDir() without touching the real config dir.

package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	specsystem "github.com/vibrantgio/spectrum/system"
	specwin "github.com/vibrantgio/spectrum/window"
)

func main() {
	go run()
	app.Main()
}

func run() {
	// Resolve the platform path and load (or first-run-init) the store BEFORE
	// building the model seed. A load/init failure is fatal: the app has no
	// data to display.
	path, err := defaultStorePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "watchlist: resolve config path:", err)
		os.Exit(1)
	}
	doc, err := loadOrInitStore(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "watchlist: load store:", err)
		os.Exit(1)
	}

	mvuWin := mvu.NewWindow(
		app.Title("Watchlist editor"),
		app.Size(unit.Dp(1100), unit.Dp(760)),
	)
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	// Build the model observable with mvu.Loop over mvu messages. See feeds:
	// Publish().AutoConnect(modelObsConsumers) shares the loop's upstream scan
	// across exactly the consumers watchlistShellLayer derives; N is
	// load-bearing and measured by TestModelObsConsumerCountMatchesConst.
	init := func() (Model, mvu.Command) { return initialModel(doc), mvu.DoNothing() }
	models, runner := mvu.Loop(mvuWin.Messages(), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs, path)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
