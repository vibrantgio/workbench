// Command themer picks a brand colour out of a picture. Drop an image
// anywhere on the window and the colours the picture is made of come back as
// a row of seed candidates, vivid ones first, each swatch showing the colour
// beside the primary pair a palette derivation makes of it. Click one and the
// whole design system underneath re-draws in it — every component, every
// composition, a page of prose and a specimen of code — which is the
// shortest honest answer to "what would this colour look like". A switch
// beside the picture shows the other side of the pair, because a seed has two
// and both have to be seen.
//
// There is a second door for anybody who has not got a picture in mind. Under
// the drop well, the window opens on a card per syntax style — the ones that
// ship and the ones read out of the styles folder — each showing the style's
// dominant inks and the primary pair its leading one derives, vivid styles
// first, and only the ones fitted to the appearance on screen. One click takes
// both halves of a theme off a card: the seed from its leading ink, and the
// syntax base for each appearance, the style itself on the side its author
// fitted it to and the nearest measured counterpart on the other. Everything
// after that click is what it is after a drop — the row still offers the
// style's other colours, the list beside the code still overrides either
// member, and keeping still writes the lot.
//
// Code is the one surface a palette does not settle on its own, so it gets a
// second choice, at the page's far end where the specimen is: a column beside
// the code lists the syntax bases fitted to the appearance on screen — the
// styles that ship, and any style file dropped into the styles folder beside
// the kept theme — and choosing one re-colours the specimen on the spot, in
// front of you. Keep the two together and they are written where every application
// that adopts a brand looks for one, this window included: the next thing you
// open is already wearing the colour, and its code is already in the base you
// chose.
//
// The whole window is the drop target: file drops arrive as ordinary
// messages through mvu/desktop, resolved against a single zone covering the
// window, and reading, decoding and extracting all happen off the render
// goroutine as one mvu command. A path named on the command line takes the
// same path, which is how the app is started with a picture already open.
//
// Architecturally it is the todos bootstrap plus two things worth copying: a
// window-wide file-drop zone with its hover highlight, and a theme observable
// the application itself re-seeds — the OS still decides light or dark, the
// application decides the colour.
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

// modelObsConsumers: the backdrop and the content layer each subscribe the
// model, because both are themed from the selected candidate rather than
// from the OS palette alone. See llms.txt rule 4 — Publish() multicasts
// without replay, so this count gates when the seed emitted by mvu.Loop
// flows.
const modelObsConsumers = 2

// Window size: wide enough for the candidate row to lay out without
// wrapping and for the widest composition on the embedded page to stand at
// its own width, and tall enough that the page under the row is the biggest
// thing in the window — which it has to be, because it is what is being
// looked at.
const (
	windowW = 1040
	windowH = 820
)

func run() {
	mvuWin := mvu.NewWindow(
		app.Title("Themer"),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)

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
