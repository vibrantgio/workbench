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

func run() {
	mvuWin := mvu.NewWindow(
		app.Title("Feeds"),
		app.Size(unit.Dp(1200), unit.Dp(800)),
	)
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	// Build the model observable by scanning over mvu messages. The window's
	// collector registers on each FrameEvent so MessageOp.Add(gtx.Ops) calls
	// made during layout are collected and delivered here on the same frame.
	//
	// mvuWin.Messages() drains a channel via rx.Recv, so each emitted message
	// reaches exactly one subscriber. feedsShellLayer derives several cold
	// streams from modelObs; without multicast each cold subscription would
	// re-drain the channel and split the messages between them.
	// Publish().AutoConnect(N) shares one upstream subscription across exactly
	// those N consumers. See the consumer count documented on feedsShellLayer
	// — the N here is load-bearing and must match it.
	seed := initialModel()
	modelObs := rx.Scan(mvuWin.Messages(), seed, func(model Model, msg mvu.Message) Model {
		next, _ := Update(model, msg)
		return next
	}).StartWith(seed).Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
