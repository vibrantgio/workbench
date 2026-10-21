// Command mindchat is a ChatGPT-style chat client on the Vibrant Gio stack:
// mvu.NewWindow wrapped in a theme window with a live OS theme, a Model
// observable driven by mvu.Loop — whose command runner feeds side-effect
// messages (config/history I/O, the streaming OpenAI completion) back into
// the update scan — and layout.Widgets that route every event through
// mvu.MessageOp.
//
// A message says two things separately: its role, the chat API's word for
// who it is from (user or assistant), and its kind — a turn, a turn still
// arriving, a failed turn, or a note the application itself wrote.
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
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

func main() {
	go MindChat()
	app.Main()
}

// MindChat drives the MindChat window; one function per window, so further
// windows get sibling functions with their own theme and loop.
func MindChat() {
	// The full-size-content treatment: on macOS the window's content extends
	// behind a transparent title bar, so the sidebar's surface and the
	// transcript's fill each reach the window's top edge and there is no
	// native white strip standing above either of them. Everywhere else the
	// treatment contributes no options and the window keeps the decorations
	// its platform gives it. The title is passed all the same — the treatment
	// hides the text, but Mission Control, the Dock and VoiceOver read it.
	mvuWin := mvu.NewWindow(shell.PaneWindow(
		app.Title("MindChat"),
		app.Size(unit.Dp(1024), unit.Dp(768)),
		app.MinSize(unit.Dp(575), unit.Dp(256)),
	)...)
	// Gio re-hides the standard window buttons on every rebuild of the
	// window's configuration; the shell registers the re-assertion on the
	// mvu configure seam and places them where the pane's own strip keeps
	// them, measured from the window's glass and from nothing drawn beneath
	// them — which is what lets the pane be sent away without the window's
	// own controls moving.
	shell.StandWindowButtons(mvuWin, nil)

	// The arrangement inside the window is this application's own to keep:
	// how wide the conversation pane stands, and whether it stands at all.
	// A nil memory is a working one — the window opens on the defaults and
	// keeps nothing.
	widths, err := shell.RememberArrangement(filepath.Join(appOrg, appName), defaultArrangement(), frameLimits)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mindchat: the window will not remember its pane:", err)
	}

	// The application menu carries what the window does: New Chat, the
	// conversations pane's switch, and Settings — which with the pane away
	// has no control anywhere in the window and is the reason this exists.
	// The items post the same messages as the window's own chords, from the
	// same table (menu.go); away from macOS the declaration is inert and the
	// chords carry the actions alone.
	menu := desktop.NewMenuBar(mvuWin, MenuItems()...)

	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second, brand.Kept().Options()...))

	models, runner := mvu.Loop(rx.Merge(mvuWin.Messages(), menu.Messages()), opensOn(Init, widths.Arrangement()), Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	renderErr := w.Render(buildLayers(models, widths)).Wait()
	// Render returns once the window is destroyed, which is the last moment
	// a width moved inside the write delay can still be kept — and the
	// change the reader most expects to find again is the one they made
	// just before quitting.
	widths.Stop()
	if renderErr != nil {
		fmt.Fprintln(os.Stderr, "mindchat:", renderErr)
		os.Exit(1)
	}
	os.Exit(0)
}

// opensOn seats the window's first model on the arrangement that was kept:
// the pane stands where the reader left it, which is what the kept shown
// state is for — a reader who sent the pane away and found it back on the
// next launch would be reading a defect.
//
// The width is the shell's own to apply and is not the model's; the shown
// state is the model's, because the pane is sent away by a message and
// nothing but the model may say whether it stands.
func opensOn(init func() (Model, mvu.Command), a shell.Arrangement) func() (Model, mvu.Command) {
	return func() (Model, mvu.Command) {
		m, cmd := init()
		m.SidebarHidden = !a.PaneShown
		return m, cmd
	}
}
