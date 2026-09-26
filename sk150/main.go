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

// The size the window opens at, and the size it refuses to go below. The two
// history charts under the Monitor tab take whatever height is left under the
// header, the tab strip and the readout panel, so the height is set by them:
// 110 dp each is the least at which both read as charts, and the native
// title-bar strip caps 32 dp off the top before any of the page is laid out
// (desktop.TopInset measures 32 on current macOS). 471 is the narrowest width
// that still holds the readout panel at its own 431 dp between the page's
// 20 dp insets. The window opens at its least height: shorter than this the
// charts are crushed.
const (
	winW unit.Dp = 720
	winH unit.Dp = 903
	minW unit.Dp = 471
	minH unit.Dp = 903
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
		app.MinSize(minW, minH),
	)...)
	desktop.ShowWindowButtons(mvuWin)

	w := themewin.New(mvuWin, themesystem.LiveTheme(time.Second))

	models, runner := mvu.Loop(mvuWin.Messages(), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	if err := w.Render(buildLayers(models)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "sk150:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
