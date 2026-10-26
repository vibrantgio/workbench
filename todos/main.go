// Command todos is the minimal canonical Vibrant Gio MVU application: an
// in-memory todo list with add, edit, toggle, and delete. It demonstrates the
// full bootstrap in its smallest honest form — mvu.NewWindow, a theme window
// with a live OS theme, a Model observable driven by mvu.Loop, and
// layout.Widgets that route every event through mvu.MessageOp.
package main

import (
	"fmt"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

func main() {
	go run()
	app.Main()
}

// The size the window opens at, named because the whole-window render test
// draws the composition at exactly this size.
const (
	winW unit.Dp = 650
	winH unit.Dp = 600
)

func run() {
	// On macOS FullSizeContent extends the content behind a transparent title
	// bar with the window control buttons floating over it, so the fill this
	// window paints reaches its top edge; on every other platform it returns no
	// options and the window keeps its normal decorations. app.Title stays even
	// though the treatment hides the title text — Mission Control, the Dock and
	// VoiceOver still read it.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title("Todos"),
		app.Size(winW, winH),
	)...)
	// Gio re-hides the standard window buttons on every configuration rebuild,
	// so StandWindowButtons re-asserts them on the mvu OnConfigure seam. It
	// also centres them on the toolbar's band, which this window's every
	// screen lays out in.
	// Post-construction options must go through mvuWin.Option — never
	// mvuWin.Window().Option — or the buttons vanish.
	shell.StandWindowButtons(mvuWin, nil)

	// The platform's "Show scroll bars" setting, read once here and passed
	// to the composition: a window asks the platform when it is built,
	// never on a frame.
	bars := desktop.ScrollBars()

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))

	// The menu bar holds the window's actions, from the same table as the
	// window's chords (menu.go). Away from macOS the declaration is inert.
	menu := desktop.NewMenuBar(mvuWin, MenuItems()...)

	models, runner := mvu.Loop(rx.Merge(mvuWin.Messages(), menu.Messages()), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	if err := w.Render(buildLayers(models, bars)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "todos:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
