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

func run() {
	// The full-size-content treatment: on macOS the window's content extends
	// behind a transparent title bar, so the sidebar's surface on the leading
	// side and the navbar's on the other each reach the window's own top edge
	// and no native strip stands above either of them. Everywhere else the
	// treatment contributes no options and the window keeps the decorations
	// its platform gives it. The title is passed all the same — the treatment
	// hides the text, but Mission Control, the Dock and VoiceOver read it.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Feeds"),
		app.Size(unit.Dp(1200), unit.Dp(800)),
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
	// enough to centre them. The band this window gives them is the strip the
	// sidebar and the navbar hold open across its top, and windowButtonRun is
	// that strip's height read through the platform's own centring rule.
	desktop.ShowWindowButtons(mvuWin)
	desktop.PlaceWindowButtonsAt(windowButtonRun.Leading, windowButtonRun.Center)

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	// Build the model observable with mvu.Loop over mvu messages. The
	// window's collector registers on each FrameEvent so MessageOp.Add(gtx.Ops)
	// calls made during layout are collected and delivered here on the same
	// frame; Loop also runs the commands Update returns (this app returns
	// DoNothing everywhere) and emits the seed model first.
	//
	// mvuWin.Messages() drains a channel via rx.Recv, so each emitted message
	// reaches exactly one subscriber. feedsShellLayer derives several cold
	// streams from modelObs; without multicast each cold subscription would
	// re-drain the channel and split the messages between them.
	// Publish().AutoConnect(N) shares one upstream subscription across exactly
	// those N consumers. See the consumer count documented on feedsShellLayer
	// — the N here is load-bearing and must match it.
	init := func() (Model, mvu.Command) { return initialModel(), mvu.DoNothing() }
	models, runner := mvu.Loop(mvuWin.Messages(), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
