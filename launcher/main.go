// Command launcher is the workbench's front door: a hero screen whose
// backdrop is a live seen 3D triangular field (animated with simplex noise,
// colour-keyed to the live prism theme) with the five example apps floating
// on it as cadence cards. Clicking Launch runs `go run ./<app>/` at the
// workbench root and tracks the process through the MVU loop.
//
// Architecturally it is the todos bootstrap plus two demonstrations: a seen
// 3D scene composited as an ordinary mvu background layer that re-keys its
// palette on every spectrum theme change, and a single streaming mvu.Command
// that emits Started and later Exited for one launched process.
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

const winW, winH = unit.Dp(1100), unit.Dp(760)

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
	mvuWin := mvu.NewWindow(
		app.Title("VibrantGio Workbench"),
		app.Size(winW, winH),
	)
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(mvuWin.Window(), modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "launcher:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
