// Command iconbrowser is a browsable catalogue of the glyphs the Vibrant Gio
// apps draw from, in two labelled sets: the design system's own marks
// (components/icons, each shown at the sizes a control draws it at) above the
// Material Design icons everything else comes from
// (golang.org/x/exp/shiny/materialdesign/icons, rendered through
// ivg/raster/gio — see llms.txt §Icons). The set comes first so an author sees
// which marks already exist before drawing another. A search field filters
// both live; every glyph is captioned with the name to write.
//
// Architecturally it is the todos bootstrap plus two demonstrations: a components
// TextField driving the Model through mvu.MessageOp on every keystroke, and
// subscription-scoped widget state (the grid's scroll position and the
// field's editor) surviving the per-keystroke view rebuilds.
package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers: the content layer is the single modelObs consumer; the
// backdrop layer is theme-only. See llms.txt rule 4 — Publish() multicasts
// without replay, so this count gates when the seed emitted by mvu.Loop
// flows.
const modelObsConsumers = 1

func run() {
	mvuWin := mvu.NewWindow(
		app.Title("Icon browser"),
		app.Size(unit.Dp(1000), unit.Dp(700)),
	)
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "iconbrowser:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
