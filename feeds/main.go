package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/shell"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

// appName is the directory this application keeps its own files in, inside
// the OS config directory. The arrangement the shell keeps is the first
// thing in it.
const appName = "feeds"

func main() {
	go run()
	app.Main()
}

func run() {
	// The full-size-content treatment: on macOS the window's content extends
	// behind a transparent title bar, so the rail's pane on the leading side
	// and the toolbar's two shares beside it each reach the window's own top
	// edge and no native strip stands above any of them. Everywhere else the
	// treatment contributes no options and the window keeps the decorations
	// its platform gives it. The title is passed all the same — the treatment
	// hides the text, but Mission Control, the Dock and VoiceOver read it.
	mvuWin := mvu.NewWindow(shell.PaneWindow(
		app.Title("Feeds"),
		app.Size(unit.Dp(1200), unit.Dp(800)),
	)...)
	// Gio re-hides the standard window buttons on every rebuild of the
	// window's configuration; the shell registers the re-assertion on the
	// mvu configure seam and places them where the pane's own strip keeps
	// them, measured from the window's glass and from nothing drawn beneath
	// them — which is what lets the pane be sent away without the window's
	// own controls moving.
	shell.StandWindowButtons(mvuWin, nil)

	// The arrangement inside the window is this application's own to keep:
	// how wide the rail's pane and the article list stand, and whether the
	// pane stands at all. A nil memory is a working one — the window opens
	// on the defaults and keeps nothing.
	widths, err := shell.RememberArrangement(appName, defaultArrangement(), frameLimits)
	if err != nil {
		fmt.Fprintln(os.Stderr, "feeds: the window will not remember its columns:", err)
	}

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	// Build the model observable with mvu.Loop over mvu messages. The
	// window's collector registers on each FrameEvent so MessageOp.Add(gtx.Ops)
	// calls made during layout are collected and delivered here on the same
	// frame; Loop also runs the commands Update returns and emits the seed
	// model first.
	//
	// feedsShellLayer derives a couple of dozen streams from the model, and
	// mvu.Loop's models observable is a replay-latest multicast: they all
	// share one upstream subscription, and each reads the model in force when
	// it attaches, whenever that is.
	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
	models, runner := mvu.Loop(mvuWin.Messages(), openedOn(init, widths.Arrangement()), Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	renderErr := w.Render(buildLayers(models, widths)).Wait()
	// Render returns once the window is destroyed, which is the last moment
	// a width moved inside the write delay can still be kept — and the
	// change the reader most expects to find again is the one they made
	// just before quitting.
	widths.Stop()
	if renderErr != nil {
		fmt.Fprintln(os.Stderr, renderErr)
		os.Exit(1)
	}
	os.Exit(0)
}
