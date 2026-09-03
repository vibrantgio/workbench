// Command sk150 is a desktop control panel for the XY-SK150 buck-boost
// converter over Modbus RTU: a monitor screen with the panel's live
// voltage/current/power readouts and setpoint controls, and a setup screen
// for the protection registers the device's own limited menu does not
// reach. It follows the canonical Vibrant Gio bootstrap (see todos/).
package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	themesystem "github.com/vibrantgio/theme/system"
	themewin "github.com/vibrantgio/theme/window"
)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers is the number of cold subscriptions that reach modelObs
// when the layers are subscribed once: the content layer's CombineLatest4,
// the tab strip's Selected observable, and the LVP override modal's Open.
// The backdrop layer is theme-only.
const modelObsConsumers = 3

const (
	winW unit.Dp = 720
	winH unit.Dp = 760
)

func run() {
	// Optional arguments: a tab name opens the app on that tab (sk150
	// presets); "demo" starts against the simulated device (sk150 demo).
	for _, arg := range os.Args[1:] {
		if arg == "demo" {
			startDemo = true
			setBus(newSimDevice())
			continue
		}
		// "M3" opens the Presets tab with that memory in the editor.
		if len(arg) == 2 && arg[0] == 'M' && arg[1] >= '0' && arg[1] <= '9' {
			startScreen = "presets"
			startEditPreset = int(arg[1] - '0')
			continue
		}
		for _, s := range tabScreens {
			if arg == s {
				startScreen = s
			}
		}
	}
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("SK150 Control"),
		app.Size(winW, winH),
	)...)
	desktop.ShowWindowButtons(mvuWin)

	w := themewin.New(mvuWin, themesystem.LiveTheme(time.Second))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "sk150:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
