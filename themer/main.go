// Command themer picks a brand colour out of a picture. Drop an image
// anywhere on the window and the colours the picture is made of come back as
// a row of seed candidates, vivid ones first, each swatch showing the colour
// beside the primary pair a palette derivation makes of it. Click one and
// the window itself re-themes to it, which is the shortest honest answer to
// "what would this colour look like".
//
// The whole window is the drop target: file drops arrive as ordinary
// messages through mvu/desktop, resolved against a single zone covering the
// window, and reading, decoding and extracting all happen off the render
// goroutine as one mvu command. A path named on the command line takes the
// same path, which is how the app is started with a picture already open.
//
// Architecturally it is the todos bootstrap plus two things worth copying: a
// window-wide file-drop zone with its hover highlight, and a theme observable
// the application itself re-seeds — the OS still decides light or dark, the
// application decides the colour.
package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers: the backdrop and the content layer each subscribe the
// model, because both are themed from the selected candidate rather than
// from the OS palette alone. See llms.txt rule 4 — Publish() multicasts
// without replay, so this count gates when the seed emitted by mvu.Loop
// flows.
const modelObsConsumers = 2

// Window size: wide enough for the candidate row to lay out without
// wrapping and tall enough for the picture above it to be worth looking at.
const (
	windowW = 980
	windowH = 680
)

func run() {
	mvuWin := mvu.NewWindow(
		app.Title("Themer"),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)

	// The drop target is constructed before the window renders a frame: it
	// claims the window's view-event stream, and its messages join the
	// window's own on the way into the loop.
	zones := &desktop.ZoneGroup{}
	drops := desktop.NewDropTarget(mvuWin, zones)

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	models, runner := mvu.Loop(rx.Merge(mvuWin.Messages(), drops.Messages()), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs, zones)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "themer:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
