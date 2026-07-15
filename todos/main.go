// Command todos is the minimal canonical VibrantGio MVU application: an
// in-memory todo list with add, edit, toggle, and delete. It demonstrates the
// full bootstrap in its smallest honest form — mvu.NewWindow, a spectrum
// window with a live OS theme (dark mode follows the system), a Model
// observable built by scanning the message stream, and widgets that route
// every event through mvu.MessageOp. Start here before reading the larger
// apps (sitedocs, feeds, watchlist).
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

// modelObsConsumers is the number of cold subscriptions that reach modelObs
// when the layers are subscribed once. Publish() multicasts WITHOUT replay,
// so AutoConnect must fire — letting StartWith(seed) flow — only when every
// consumer is attached. Here the content layer is the single consumer; the
// backdrop layer is theme-only.
const modelObsConsumers = 1

func run() {
	mvuWin := mvu.NewWindow(
		app.Title("Todos"),
		app.Size(unit.Dp(650), unit.Dp(600)),
	)
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	seed := Init()
	modelObs := rx.Scan(mvuWin.Messages(), seed, func(model Model, msg mvu.Message) Model {
		next, _ := Update(model, msg)
		return next
	}).StartWith(seed).Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "todos:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
