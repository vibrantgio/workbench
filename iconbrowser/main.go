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
	"github.com/vibrantgio/mvu/desktop"
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

// The size the window opens at: wide enough for six catalogue cells across,
// tall enough that the first screenful of the Material grid stands under both
// section labels. Named rather than written into the app.Size call so a render
// made outside the running window draws the frame at the size the window
// actually shows.
const (
	winW unit.Dp = 1000
	winH unit.Dp = 700
)

func run() {
	// mvu/desktop's full-size-content treatment (ADR-021 R6): on macOS the
	// content extends behind a transparent title bar with the window control
	// buttons floating over it, so the Background pin this window paints
	// reaches its top edge instead of standing under a native strip. On every
	// other platform FullSizeContent returns no options and the window keeps
	// its normal decorations. app.Title stays even though the treatment hides
	// the title text — Mission Control, the Dock and VoiceOver read it all the
	// same.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Icon browser"),
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

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "iconbrowser:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
