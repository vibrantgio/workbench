// Command mindchat is a ChatGPT-style chat client on the Vibrant Gio stack:
// mvu.NewWindow wrapped in a theme window with a live OS theme, a Model
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
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

func main() {
	go MindChat()
	app.Main()
}

// modelObsConsumers is the number of cold subscriptions that reach modelObs
// when the layers are subscribed once. Publish() multicasts WITHOUT replay,
// so AutoConnect must fire — letting the seed emitted by mvu.Loop flow —
// only when every consumer is attached. The consumers: the content layer's
// CombineLatest (1), the split-pane ratio derivation (1), the rename
// modal's open and edit derivations (2), the settings modal's open, field
// and body derivations (3), the model menu's open, data and chip-key
// derivations (3), and the settings dropdown's open derivation (1); the
// backdrop layer is theme-only. The chip key is the third of the menu's
// three because components/chip takes its label as a static prop: the
// picker derives a deduplicated key from the Model and subscribes a new
// chip when it changes. Measured by TestModelObsConsumerCountMatchesConst.
const modelObsConsumers = 11

// MindChat drives the MindChat window; one function per window, so further
// windows get sibling functions with their own theme and loop.
func MindChat() {
	// The full-size-content treatment: on macOS the window's content extends
	// behind a transparent title bar, so the sidebar's surface and the
	// transcript's ground each reach the window's top edge and there is no
	// native white strip standing above either of them. Everywhere else the
	// treatment contributes no options and the window keeps the decorations
	// its platform gives it. The title is passed all the same — the treatment
	// hides the text, but Mission Control, the Dock and VoiceOver read it.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("MindChat"),
		app.Size(unit.Dp(1024), unit.Dp(768)),
		app.MinSize(unit.Dp(575), unit.Dp(256)),
	)...)
	// The treatment hides the three standard window buttons and Gio re-hides
	// them on every rebuild of the window's configuration, so this registers a
	// re-assertion rather than a one-off unhide — and the placement rides on
	// the same seam, which is why stating it once here holds for the window's
	// life.
	//
	// The placement is stated rather than defaulted because the default is
	// wrong for this window: left alone the buttons land at the inset the
	// platform's compact windows use, which would put them high in a band deep
	// enough to centre them. The sidebar's brand row is the band this window
	// gives them, and the two numbers below are that row's height read through
	// the platform's own centring rule.
	desktop.ShowWindowButtons(mvuWin)
	desktop.PlaceWindowButtonsAt(WindowButtonInset, WindowButtonCenter)

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second, brand.Kept().Options()...))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "mindchat:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
