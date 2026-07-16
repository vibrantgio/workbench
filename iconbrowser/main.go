// Command iconbrowser is a browsable catalogue of the Material Design icons
// the VibrantGio apps draw from (golang.org/x/exp/shiny/materialdesign/icons,
// rendered through ivg/raster/gio — see llm.txt §Icons). A search field
// filters the scrolling grid live; every glyph is captioned with the exported
// name to import.
//
// Architecturally it is the todos bootstrap plus two demonstrations: a prism
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

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	specsystem "github.com/vibrantgio/spectrum/system"
	specwin "github.com/vibrantgio/spectrum/window"
)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers: the content layer is the single modelObs consumer; the
// backdrop layer is theme-only. See llm.txt rule 4 — Publish() multicasts
// without replay, so this count gates when StartWith(seed) flows.
const modelObsConsumers = 1

func run() {
	mvuWin := mvu.NewWindow(
		app.Title("Icon browser"),
		app.Size(unit.Dp(1000), unit.Dp(700)),
	)
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	seed := Init()
	modelObs := rx.Scan(mvuWin.Messages(), seed, func(model Model, msg mvu.Message) Model {
		next, _ := Update(model, msg)
		return next
	}).StartWith(seed).Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "iconbrowser:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
