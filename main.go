// Command mindchat is a ChatGPT-style chat client on the VibrantGio stack:
// mvu.NewWindow wrapped in a spectrum window with a live OS theme, a Model
// observable driven by mvu.Loop — whose command runner feeds side-effect
// messages (config/history I/O, the streaming OpenAI completion) back into
// the update scan — and widgets that route every event through
// mvu.MessageOp.
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

func main() {
	go MindChat()
	app.Main()
}

// modelObsConsumers is the number of cold subscriptions that reach modelObs
// when the layers are subscribed once. Publish() multicasts WITHOUT replay,
// so AutoConnect must fire — letting the seed emitted by mvu.Loop flow —
// only when every consumer is attached. The consumers: the content layer's
// CombineLatest (1) plus the rename modal's open and edit derivations (2);
// the backdrop layer is theme-only. Measured by
// TestModelObsConsumerCountMatchesConst.
const modelObsConsumers = 3

// MindChat drives the MindChat window; one function per window, so further
// windows get sibling functions with their own theme and loop.
func MindChat() {
	mvuWin := mvu.NewWindow(
		app.Title("MindChat"),
		app.Size(unit.Dp(1024), unit.Dp(768)),
		app.MinSize(unit.Dp(575), unit.Dp(256)),
	)
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "mindchat:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
