// Command themer chooses the theme colour, and the two things a theme says
// about code: the syntax highlighter style a fence is coloured from and the
// typeface it is set in.
//
// The theme colour is the one colour a person may choose. On macOS it is the
// accent colour from the system's Appearance settings unless it is chosen
// here; on other platforms this window is where it comes from. It stands in
// wherever the platform uses its accent — the default button, the selection,
// the sidebar's pill, the focus ring — and nothing derives from it.
//
// So the window offers three ways to settle that colour and one place to see
// what it does. The platform's own accent colour leads the row, being the
// default. Drop a picture anywhere on the window and the colours it is made
// of come back as swatches beside it, vivid first, one click each. Or write
// the colour into the field. Keeping it writes the file every application
// that adopts a brand reads — which holds choices this window does not make,
// and they are written back untouched.
//
// The window is a column of titled groups, in the order somebody settling a
// theme works through them: the picture and the colours it gave, the theme
// colour in force, and the code face. Under them stands the row the choices
// are judged in — the list of highlighter styles beside the preview — and
// under that the one default button this window has.
//
// The style list holds every style chroma ships that is fitted to the
// appearance on screen, at the platform's own list row height with the
// platform's scrollbar beside it. A style is a pair, one member per
// appearance, so a flip of the appearance swaps the list, the marked row and
// the colours under the code together. Keeping writes both beside the colour
// and the code face.
//
// What is previewed is a picture of an application in the platform's set with
// the chosen colour standing in for the accent: a sidebar with the selection
// pill, a toolbar with the platform's window controls, a heading with a badge
// beside it, prose, a fenced code block in the chosen face and style, a text
// field and two push buttons. Every choice above it moves something in it.
// The colour set itself, every name the platform answers for beside its
// value, is what the gallery's colour board is for and is not repeated here.
//
// One appearance at a time, and one switch to change it, at the top of the
// window: it moves the window's own plane, every group on it, the style list
// and the preview together. The window opens on the appearance the desktop is
// set to and is the window's own answer from then on — a theme has two
// appearances and both have to be settled, which cannot wait for the desktop
// to change its mind.
//
// The whole window is the drop target: file drops arrive as ordinary messages
// through mvu/desktop, resolved against a single zone covering the window,
// and reading, decoding and extracting all happen off the render goroutine as
// one mvu command. A path named on the command line takes the same path,
// which is how the app is started with a picture already open.
//
// The window has no title bar of its own. Its content extends behind the
// platform's, so the title row is the top of the window rather than a second
// band under an empty one, and the platform's own control buttons stand in
// that row beside the name — on the row's centre line, and with the row
// starting where they end. Away from macOS none of that applies: the window
// keeps the decorations the platform gives it and the row leads at the page's
// margin.
package main

import (
	"fmt"
	stdcolor "image/color"
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

// Window size: wide enough for the swatch row to lay out without wrapping and
// for the style list to stand beside a picture of a window rather than beside
// a strip; tall enough for the choices, the list, the preview and the footer
// to close without the window scrolling. The column is laid out to that height
// exactly — see the box heights in view.go — so a change to either wants the
// other checked.
const (
	windowW = 1200
	windowH = 900
)

// WindowOptions is the window this application opens: the full-size-content
// treatment, the window's name and its size.
//
// The treatment is what lets the title row have the strip the platform would
// otherwise keep for itself. On macOS the content extends behind a
// transparent title bar, so the row stands at the very top of the window and
// no empty band stands above it; everywhere else the treatment contributes no
// options at all and the window keeps the decorations the platform gives it.
//
// The name is passed even where the treatment hides the title text, because
// Mission Control, the Dock and the screen reader all still read it.
func WindowOptions() []app.Option {
	return append(desktop.FullSizeContent(),
		app.Title(AppName),
		app.Size(unit.Dp(windowW), unit.Dp(windowH)),
	)
}

// platformColors is the accent colour the platform reports, as messages: the
// appearance stream mapped through the one question this window asks it,
// emitting at startup and again whenever the setting changes.
//
// A platform that reports none emits the zero colour, which is the row
// drawing no cell for it.
func platformColors(interval time.Duration) rx.Observable[mvu.Message] {
	return rx.Map(specsystem.Live(interval), func(a specsystem.Appearance) mvu.Message {
		col, ok := specsystem.PlatformColor(a)
		if !ok {
			col = stdcolor.NRGBA{}
		}
		return PlatformColorChanged{Color: col}
	})
}

func run() {
	mvuWin := mvu.NewWindow(WindowOptions()...)

	// The three standard window buttons are hidden by the treatment and Gio
	// re-hides them on every rebuild of the window's configuration, so what
	// is registered here is a re-assertion rather than a one-off unhide. The
	// placement rides on that same re-assertion, which is why stating it once
	// at startup holds for the window's life — including across a resize,
	// which changes no option and so raises no rebuild of its own.
	desktop.ShowWindowButtons(mvuWin)
	buttons := WindowButtons()
	desktop.PlaceWindowButtonsAt(buttons.Leading, buttons.Center)

	// The drop target is constructed before the window renders a frame: it
	// claims the window's view-event stream, and its messages join the
	// window's own on the way into the loop.
	zones := &desktop.ZoneGroup{}
	drops := desktop.NewDropTarget(mvuWin, zones)

	// The window opens in the theme that was kept, if one was: this is the
	// same one-line adoption every other application makes.
	w := specwin.New(mvuWin, specsystem.LiveTheme(time.Second, brand.Kept().Options()...))

	// The model stream is subscribed by the backdrop, by the content, and
	// again by the colour field's own theme whenever the live theme re-emits
	// — the backdrop and the field both read the model because the switch at
	// the top of the window moves the window's plane and everything drawn on
	// it. mvu.Loop carries the current model to every one of those
	// subscriptions whenever it attaches, so there is no consumer count here
	// to keep right.
	models, runner := mvu.Loop(rx.Merge(mvuWin.Messages(), drops.Messages(), platformColors(time.Second)), Init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	if err := w.Render(buildLayers(models, zones)).Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "themer:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
