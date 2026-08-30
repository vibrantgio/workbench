// Command marketing is a fictional SimpleApps landing: one full-screen
// scrolling page — Hero, features, pricing, testimonials — on a
// single-colour wireframe triangle field over the Background pin, with
// the macOS full-size-content treatment so the traffic lights sit on
// the page. The window title is SimpleApps.
package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

const (
	windowW = 1200
	windowH = 1040
)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers is the number of cold subscriptions that reach modelObs
// when the layers are subscribed once. Publish() multicasts without replay, so
// AutoConnect must fire — letting the seed emitted by mvu.Loop flow — only
// when every consumer is attached. Here the content layer is the single
// consumer; the backdrop and field layers are theme-only.
const modelObsConsumers = 1

func run() {
	// On macOS FullSizeContent extends the content behind a transparent title
	// bar with the traffic lights floating over it; on every other platform it
	// returns no options and the window keeps its normal decorations.
	// app.Title stays even though the treatment hides the title text — Mission
	// Control, the Dock and VoiceOver still read it.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("SimpleApps"),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)...)
	// Gio re-hides the standard window buttons on every configuration rebuild,
	// so ShowWindowButtons re-asserts them on the mvu OnConfigure seam.
	// Post-construction options must go through mvuWin.Option — never
	// mvuWin.Window().Option — or the buttons vanish.
	desktop.ShowWindowButtons(mvuWin)

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second, brand.Kept().Options()...))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(mvuWin.Window(), modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "marketing:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
