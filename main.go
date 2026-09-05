// Command workbench is the front door to the example apps: a hero screen whose
// backdrop is a live seen 3D triangular field, colour-keyed to the live
// components theme, with the apps floating on it as patterns cards. Clicking
// Launch runs one and tracks its process through the MVU loop.
//
// Each app keeps a module of its own beside this one. A nested module stands
// outside its parent by Go's rules, so building this command never builds an
// app and releasing it never releases one; a launch therefore names the app's
// own latest release rather than anything this build was compiled alongside.
package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

// winW, winH size the window to the app grid: wide enough for a full row of
// cells with a margin either side, tall enough for the hero plus every row the
// roster fills.
const winW, winH = unit.Dp(1340), unit.Dp(966)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers is the number of layers subscribing to modelObs: the
// content layer only, since the backdrop and field layers are theme-only.
// Publish() multicasts without replay, so this count gates when the seed
// emitted by mvu.Loop flows.
const modelObsConsumers = 1

func run() {
	// On macOS FullSizeContent extends the content behind a transparent title
	// bar so the ground and the field reach the window's top edge; on every
	// other platform it returns no options. app.Title stays even though the
	// treatment hides the title text — Mission Control, the Dock and VoiceOver
	// still read it.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Vibrant Gio Workbench"),
		app.Size(winW, winH),
	)...)
	// Gio re-hides the standard window buttons on every configuration rebuild,
	// so ShowWindowButtons re-asserts them on the mvu OnConfigure seam.
	// Post-construction options must go through mvuWin.Option — never
	// mvuWin.Window().Option — or the buttons vanish.
	desktop.ShowWindowButtons(mvuWin)

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(mvuWin.Window(), modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "workbench:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
