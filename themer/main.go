// Command themer picks a brand colour out of a picture. Drop an image
// anywhere on the window and the colours the picture is made of come back as
// a row of seed candidates, vivid ones first, each swatch showing the colour
// beside the primary pair a palette derivation makes of it. Click one and the
// whole design system underneath re-draws in it — every component, every
// composition, a page of prose and a specimen of code — which is the
// shortest honest answer to "what would this colour look like". A switch at
// the trailing end of the window's title row shows the other side of the pair,
// because a seed has two and both have to be seen.
//
// That system is drawn a tab at a time — the theme this seed derives, then
// the catalogue's own groups — so what a colour does to buttons is one click
// away rather than several screens down, and the tab a reader is on, with the
// place they scrolled it to, is where the next pick leaves them.
//
// There is a second door for anybody who has not got a picture in mind. Under
// the drop well, the window opens on a card per syntax style — the ones that
// ship and the ones read out of the styles folder — each led by its name and
// showing under it the style's dominant inks and the primary pair its leading
// one derives, in name order, and only the ones fitted to the appearance on
// screen. One click takes both halves of a theme off a card: the seed from its
// leading ink, and the syntax base for each appearance, the style itself on the
// side its author fitted it to and the nearest measured counterpart on the
// other. Everything after that click is what it is after a drop — the row
// still offers the style's other colours, the list beside the code still
// overrides either member, and keeping still writes the lot.
//
// Code is the one surface a palette does not settle on its own, so it gets a
// second choice, on the Markdown tab where the specimen is: a column beside
// the code lists the syntax bases fitted to the appearance on screen — the
// styles that ship, and any style file dropped into the styles folder beside
// the kept theme — and choosing one re-colours the specimen on the spot, in
// front of you. The same seat holds a two-name plate for the face the fence
// wears: Roboto Mono or JetBrains Mono. Keep the lot and they are written
// where every application that adopts a brand looks for one, this window
// included: the next thing you open is already wearing the colour, its code
// is already in the base you chose, and the fence is already in that face.
//
// The whole window is the drop target: file drops arrive as ordinary
// messages through mvu/desktop, resolved against a single zone covering the
// window, and reading, decoding and extracting all happen off the render
// goroutine as one mvu command. A path named on the command line takes the
// same path, which is how the app is started with a picture already open.
//
// The window has no title bar of its own. Its content extends behind the
// platform's, so the title row is the top of the window rather than a second
// band under an empty one, and the platform's own control buttons stand in that
// row beside the name — on the row's centre line, and with the row starting
// where they end. Away from macOS none of that applies: the window keeps the
// decorations the platform gives it and the row leads at the page's margin.
//
// Architecturally it is the todos bootstrap plus three things worth copying: a
// window-wide file-drop zone with its hover highlight, a theme observable the
// application itself re-seeds — the OS still decides light or dark, the
// application decides the colour — and a title row that has the window's own
// strip, drag included.
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
	"github.com/vibrantgio/theme/brand"
	specsystem "github.com/vibrantgio/theme/system"
	specwin "github.com/vibrantgio/theme/window"
)

func main() {
	go run()
	app.Main()
}

// modelObsConsumers: how many streams subscribe the model. The backdrop is
// one, because it is themed from the selected candidate rather than from the
// OS palette alone; the content layer is the other three — the page itself,
// the palette the embedded tab strip is drawn from, and the strip's selected
// cell. See llms.txt rule 4 — Publish() multicasts without replay, so this
// count gates when the seed emitted by mvu.Loop flows, and a subscriber more
// than it names is a subscriber that misses the first one.
const modelObsConsumers = 4

// Window size: wide enough for the candidate row to lay out without
// wrapping and for the widest composition on the embedded page to stand at
// its own width, and tall enough that the page under the row is the biggest
// thing in the window — which it has to be, because it is what is being
// looked at.
const (
	windowW = 1040
	windowH = 820
)

// WindowOptions is the window this application opens: the full-size-content
// treatment, the window's name and its size.
//
// The treatment is what lets the title row have the strip the platform would
// otherwise keep for itself. On macOS the content extends behind a transparent
// title bar, so the row stands at the very top of the window and no empty band
// stands above it; everywhere else the treatment contributes no options at all
// and the window keeps the decorations the platform gives it. That is why the
// two options here are appended to whatever it returns rather than written out
// beside a platform test.
//
// The name is passed even where the treatment hides the title text, because
// Mission Control, the Dock and the screen reader all still read it.
func WindowOptions() []app.Option {
	return append(desktop.FullSizeContent(),
		app.Title(AppName),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)
}

func run() {
	mvuWin := mvu.NewWindow(WindowOptions()...)

	// The three standard window buttons are hidden by the treatment and Gio
	// re-hides them on every rebuild of the window's configuration, so what is
	// registered here is a re-assertion rather than a one-off unhide. The
	// placement rides on that same re-assertion, which is why stating it once
	// at startup holds for the window's life — including across a resize, which
	// changes no option and so raises no rebuild of its own.
	desktop.ShowWindowButtons(mvuWin)
	buttons := WindowButtons()
	desktop.PlaceWindowButtonsAt(buttons.Leading, buttons.Center)

	// The drop target is constructed before the window renders a frame: it
	// claims the window's view-event stream, and its messages join the
	// window's own on the way into the loop.
	zones := &desktop.ZoneGroup{}
	drops := desktop.NewDropTarget(mvuWin, zones)

	// The window opens in the theme that was kept, if one was: this is the
	// same one-line adoption every other application makes, and making it
	// here too is what lets a session start where the last one stopped.
	// With nothing kept the options are empty and the stream is the OS's
	// own, which is what this window has always opened in.
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second, brand.Kept().Options()...))

	models, runner := mvu.Loop(rx.Merge(mvuWin.Messages(), drops.Messages()), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	modelObs := models.Publish().AutoConnect(modelObsConsumers)

	if err := w.Render(buildLayers(modelObs, zones)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "themer:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
