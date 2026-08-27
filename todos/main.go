// Command todos is the minimal canonical Vibrant Gio MVU application: an
// in-memory todo list with add, edit, toggle, and delete. It demonstrates the
// full bootstrap in its smallest honest form — mvu.NewWindow, a theme
// window with a live OS theme (dark mode follows the system), a Model
// observable driven by mvu.Loop, and widgets that route every event
// through mvu.MessageOp. Start here before reading the larger
// apps (sitedocs, feeds, mindchat).
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

// modelObsConsumers is the number of cold subscriptions that reach modelObs
// when the layers are subscribed once. Publish() multicasts WITHOUT replay,
// so AutoConnect must fire — letting the seed emitted by mvu.Loop flow —
// only when every
// consumer is attached. Here the content layer is the single consumer; the
// backdrop layer is theme-only.
const modelObsConsumers = 1

// The size the window opens at, named because the whole-window render test
// draws the composition at exactly this size and a frame is only worth judging
// where somebody actually looks at it.
const (
	winW unit.Dp = 650
	winH unit.Dp = 600
)

func run() {
	// mvu/desktop's full-size-content treatment (ADR-021 R6): on macOS the
	// content extends behind a transparent title bar with the window control
	// buttons floating over it, so the ground this window paints reaches its
	// top edge instead of standing under a native strip. On every other
	// platform FullSizeContent returns no options and the window keeps its
	// normal decorations. app.Title stays even though the treatment hides the
	// title text — Mission Control, the Dock and VoiceOver read it all the
	// same.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Todos"),
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
		fmt.Fprintln(os.Stderr, "todos:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
