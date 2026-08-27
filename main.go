// Command workbench is the front door to the example apps: a hero screen
// whose backdrop is a live seen 3D triangular field (animated with simplex
// noise, colour-keyed to the live components theme) with the eight apps
// floating on it as patterns cards. Clicking Launch runs one and tracks its
// process through the MVU loop.
//
// This command is the package at the repository root, and the apps are not
// part of it. Each keeps a module of its own beside it, built, tested and
// released on its own cadence — a nested module stands outside its parent by
// Go's own rules, so building this command never builds an app and releasing
// it never releases one. A launch therefore names the app's own latest
// release rather than anything this build was compiled alongside; see
// launch.go for the two ways that resolves.
//
// Architecturally it is the todos bootstrap plus two demonstrations: a seen
// 3D scene composited as an ordinary mvu background layer that re-keys its
// palette on every theme theme change, and a single streaming mvu.Command
// that emits Started and later Exited for one launched process.
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

// winW, winH size the window to the card grid: wide enough for a full row of
// them with a margin either side, tall enough for the hero and the rows the
// roster fills. The seventh app is what last moved these; the eighth fills
// the last row.
const winW, winH = unit.Dp(1340), unit.Dp(760)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers: the content layer is the single modelObs consumer; the
// backdrop and field layers are theme-only. See llms.txt rule 4 — Publish()
// multicasts without replay, so this count gates when the seed emitted by
// mvu.Loop flows.
const modelObsConsumers = 1

func run() {
	// mvu/desktop's full-size-content treatment (ADR-021 R6): on macOS the
	// content extends behind a transparent title bar with the window control
	// buttons floating over it, so the ground and the field this window paints
	// reach its top edge instead of standing under a native strip. On every
	// other platform FullSizeContent returns no options and the window keeps
	// its normal decorations. app.Title stays even though the treatment hides
	// the title text — Mission Control, the Dock and VoiceOver read it all the
	// same.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Vibrant Gio Workbench"),
		app.Size(winW, winH),
	)...)
	// Gio re-hides the standard window buttons on every configuration rebuild,
	// so ShowWindowButtons registers a re-assertion on the mvu OnConfigure
	// seam. Post-construction options must therefore go through mvuWin.Option —
	// never mvuWin.Window().Option — or the buttons vanish.
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
