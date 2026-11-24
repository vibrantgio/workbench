// Command kanban shows a board of tasks kept in a markdown file, in the
// format of the Obsidian Kanban plugin: each level-2 heading is a column and
// each task list item under it is a card. The app writes the file after each
// change and reads it again when it changes on disk.
package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// The size the window opens at, which the stored renders are drawn at too.
// The window is wide enough to show three columns at [columnWidth], with
// the gaps between them and the board's inset on each side.
var (
	winW = 2*boardInset + 3*columnWidth + 2*columnGap
	winH = unit.Dp(600)
)

// defaultPath is the file the app opens when its command line names none.
func defaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents", "Kanban.md"), nil
}

func run() {
	if err := runWindow(); err != nil {
		fmt.Fprintln(os.Stderr, "kanban:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runWindow() error {
	path := ""
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		p, err := defaultPath()
		if err != nil {
			return err
		}
		path = p
	}
	data, seen, err := openBoard(path)
	if err != nil {
		return err
	}
	file := newBoardFile(path, seen)
	name := filepath.Base(path)

	// On macOS FullSizeContent extends the content behind a transparent
	// title bar with the window control buttons over it, so the content
	// fill reaches the window's top edge. app.Title stays: Mission Control,
	// the Dock and VoiceOver read it.
	mvuWin := mvu.NewWindow(append(desktop.FullSizeContent(),
		app.Title(name),
		app.Size(winW, winH),
	)...)
	// Options after construction go through mvuWin.Option, never
	// mvuWin.Window().Option, or the control buttons vanish.
	shell.StandWindowButtons(mvuWin, nil)

	// The platform's "Show scroll bars" setting, read once when the window
	// is built, never on a frame.
	bars := desktop.ScrollBars()

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second))
	init := func() (Model, mvu.Command) { return newModel(name, data), mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Merge(mvuWin.Messages(), watch(file, pollInterval)), init, newUpdate(file))
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	return w.Render(buildLayers(models, bars)).Wait()
}
