package main

// A whole-window render, headless. The app has no offscreen mode of its own —
// it is a native window binary — but the layers the window renders are plain
// observables of layout.Widget, so composing them over a frozen theme and
// drawing them into a headless canvas produces the same frame the window
// would show, at the size the window opens at.
//
// A composition can only be judged as a composition: a render of one widget
// in isolation cannot see that a window reads darker in the middle than at
// its edges, and this can. Run it with -window.dump=<dir> to write the frames
// out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/mindchat
//
// It renders both colour schemes AND both pane states — four frames — because
// the composition this window is judged on is the pair: the pane standing and
// the pane away have to hold one line between them, and a picture of either
// alone cannot show that. Without the flag it still renders all four every
// run, which makes it a smoke test of the whole layer stack: a panic anywhere
// in the pane, the transcript, the chrome row's picker or the prompt field
// fails it.

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

var windowDump = flag.String("window.dump", "", "directory to write whole-window renders into")

// windowSize is the size MindChat's window opens at (main.go), and the only
// size these frames are drawn at: a composition is worth looking at where
// somebody actually looks at it.
var windowSize = image.Pt(1024, 768)

// staticTheme freezes one colour scheme into a Theme whose every field emits
// once — the shape theme/window feeds the layers, minus the live OS poll.
func staticTheme(c tokens.ColorTokens) theme.Theme {
	return theme.Theme{
		Color:      rx.Of(c),
		Typography: rx.Of(tokens.DefaultTypography),
		Density:    rx.Of(tokens.Comfortable),
		Motion:     rx.Of(tokens.Motion),
		Spacing:    rx.Of(tokens.Spacing),
		Radius:     rx.Of(tokens.Radius),
		Elevation:  rx.Of(tokens.Elevation),
	}
}

// demoModel is a settled conversation with the shapes a transcript actually
// grows: a couple of exchanges, a fenced code block, an inline code span, a
// bulleted list with a nested sublist, a numbered list, and an answer
// carrying web-search citations. Both list kinds are here because the
// document draws their markers, hanging indent and item rhythm itself, and a
// composition is the only place that rhythm can be judged against the prose
// it sits between.
func demoModel() Model {
	return Model{
		CurrentChat: Chat{
			Name:   "reactive layouts.jsonl",
			Loaded: true,
			History: []Message{
				{Role: RoleUser, Content: "In MVU, how does a button click reach the update function?"},
				{Role: RoleAssistant, Content: "The widget records a `MessageOp`; after the frame, " +
					"the window drains the operation list into the loop:\n\n" +
					"```go\nfor _, msg := range frame.Messages() {\n\tmodel = Update(model, msg)\n}\n```"},
				{Role: RoleUser, Content: "So state never lives in the widget?"},
				{Role: RoleAssistant, Content: "Only ephemeral gesture state:\n\n" +
					"- press tracking\n" +
					"- an editor's cursor\n" +
					"    - the caret's blink phase\n" +
					"    - the live selection\n\n" +
					"Everything else reduces into the Model:\n\n" +
					"1. the message reaches `Update`\n" +
					"2. `Update` returns the next Model\n" +
					"3. the view re-renders from it\n",
					Citations: []Citation{
						{URL: "https://gioui.org/doc/architecture", Title: "Gio — Architecture"},
					}},
			},
		},
		ChatList: ChatList{
			"reactive layouts.jsonl",
			"cassowary layout.jsonl",
			"simplex noise.jsonl",
			"popover dismissal.jsonl",
		},
		Providers: []Provider{
			{Name: "OpenAI", Models: []string{"gpt-5.5", "gpt-5.5-codex"}},
			{Name: "xAI", BaseURL: "https://api.x.ai/v1", Models: []string{"grok-4"}},
		},
		DefaultProvider: "OpenAI",
		DefaultModel:    "gpt-5.5",
		Streams:         map[int]StreamState{},
	}
}

// frame composes the window's layers for one scheme into a single widget: the
// backdrop first, the content over it, exactly as theme/window stacks them.
func frame(t *testing.T, c tokens.ColorTokens, model Model) layout.Widget {
	t.Helper()
	layers := buildLayers(rx.Of(model))(rx.Of(staticTheme(c)))

	widgets := make([]layout.Widget, len(layers))
	for i, layer := range layers {
		// The callback runs on rx's own goroutine, not this one, so the
		// handoff goes through an atomic cell rather than a bare variable —
		// the same bridge theme/window's own layers use to cross from a
		// live stream onto a slot read at frame time (view.go, settings.go).
		var cell atomic.Value
		sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
			if !done && err == nil {
				cell.Store(w)
			}
		})
		var latest layout.Widget
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				latest = w
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		sub.Unsubscribe()
		if latest == nil {
			t.Fatalf("layer %d never emitted a widget", i)
		}
		widgets[i] = latest
	}

	return func(gtx layout.Context) layout.Dimensions {
		for _, w := range widgets {
			w(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// The macOS window controls, as this window will actually carry them: the
// leading inset and centre line the app states, and the diameter and pitch the
// stored platform reference measured (19 in, 14 across, 23 apart, so the third
// circle's trailing edge lands at 79).
//
// A headless frame has no window behind it, so it is told none of this unless
// it says so: LeadingInset reports 0 with no window, and the buttons the OS
// draws over the top-left corner do not exist at all. Both are stated here —
// the measurement so the brand row reserves the run it will really have to
// reserve, and the circles so a composition is judged with the things that
// will be standing in it. They are stand-ins drawn at the measured geometry,
// not the platform's own controls: right in size and place, plain discs rather
// than the real glyphs and gradients.
const (
	buttonLeadDp     = 19
	buttonDiameterDp = 14
	buttonPitchDp    = 23
	buttonsEndDp     = buttonLeadDp + buttonDiameterDp + 2*buttonPitchDp
)

// defaultPickerCentre is where the settings dialog's default-model trigger
// stands in this window: the dialog is 560 dp wide and centred, its body's
// trailing edge is the picker's, and the row is the last one in the body.
// Read off the rendered frame rather than recomputed from the modal's
// internals, and asserted by the open capture differing from the closed one.
var defaultPickerCentre = f32.Pt(642, 508)

var buttonHues = []color.NRGBA{
	{R: 0xff, G: 0x5f, B: 0x57, A: 0xff},
	{R: 0xfe, G: 0xbc, B: 0x2e, A: 0xff},
	{R: 0x28, G: 0xc8, B: 0x40, A: 0xff},
}

// withWindowControls draws w and then puts the window's control buttons over
// its top-left corner, where the platform will.
func withWindowControls(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		dims := w(gtx)
		d := gtx.Dp(buttonDiameterDp)
		r := d / 2
		for i, hue := range buttonHues {
			x := gtx.Dp(buttonLeadDp) + i*gtx.Dp(buttonPitchDp)
			y := gtx.Dp(WindowButtonCenter) - r
			circle := clip.RRect{
				Rect: image.Rect(x, y, x+d, y+d),
				NE:   r, NW: r, SE: r, SW: r,
			}
			paint.FillShape(gtx.Ops, hue, circle.Op(gtx.Ops))
		}
		return dims
	}
}

// TestWholeWindowRender draws the composed window in both schemes, and writes
// the frames out when -window.dump names a directory.
func TestWholeWindowRender(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	states := []struct {
		name   string
		hidden bool
	}{
		{"pane", false},
		{"hidden", true},
	}
	for _, tc := range schemes {
		for _, st := range states {
			t.Run(tc.name+"-"+st.name, func(t *testing.T) {
				renderPane(t, tc.name+"-"+st.name, tc.c, st.hidden)
			})
		}
	}
}

// TestWholeWindowPickerRender draws the window's two pickers where a reader
// meets them — the header menu standing open over the transcript, and the
// settings dialog at its default-model row — in both schemes. Both are
// compositions rather than controls: what the header menu has to clear is the
// transcript under it, and what the dialog's row has to clear is the action
// row below it.
func TestWholeWindowPickerRender(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	menuOpen := demoModel()
	menuOpen.ModelMenu = true
	settings, _ := Update(demoModel(), OpenSettings{})

	for _, tc := range schemes {
		t.Run(tc.name+"-header-menu", func(t *testing.T) {
			renderWindow(t, tc.name+"-header-menu", tc.c, menuOpen)
		})
		t.Run(tc.name+"-settings", func(t *testing.T) {
			renderWindow(t, tc.name+"-settings", tc.c, settings)
		})
		t.Run(tc.name+"-settings-open", func(t *testing.T) {
			closed := dumpFrame(t, "", withWindowControls(frame(t, tc.c, settings)))
			w := withWindowControls(frame(t, tc.c, settings))
			open := dumpFrame(t, tc.name+"-settings-open", clicked(w, windowSize, defaultPickerCentre))
			if n := golden.PixelDiff(closed, open); n == 0 {
				t.Errorf("clicking the default-model trigger at %v changed nothing: the menu did not open, or the point missed it", defaultPickerCentre)
			}
		})
	}
}

// renderPane draws one scheme in one pane state and writes it out when the
// dump flag names a directory.
func renderPane(t *testing.T, name string, c tokens.ColorTokens, hidden bool) {
	t.Helper()
	m := demoModel()
	m.SidebarHidden = hidden
	renderWindow(t, name, c, m)
}

// renderWindow draws one Model in one scheme and writes it out when the dump
// flag names a directory.
func renderWindow(t *testing.T, name string, c tokens.ColorTokens, m Model) {
	t.Helper()
	dumpFrame(t, name, withWindowControls(frame(t, c, m)))
}

// clicked drives w through two headless frames with a click queued at pos and
// returns a widget drawing from the state those frames left behind. It is how
// a surface whose open state lives INSIDE a component — a picker field's menu
// — is captured standing open: the Model cannot be posed into it.
func clicked(w layout.Widget, size image.Point, pos f32.Point) layout.Widget {
	r := new(gioinput.Router)
	drive := func() {
		var ops op.Ops
		w(layout.Context{
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(size),
			Ops:         &ops,
			Source:      r.Source(),
		})
		r.Frame(&ops)
	}
	drive()
	r.Queue(
		pointer.Event{Kind: pointer.Press, Position: pos, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
		pointer.Event{Kind: pointer.Release, Position: pos, Buttons: pointer.ButtonPrimary, Source: pointer.Mouse},
	)
	drive()
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Source = r.Source()
		return w(gtx)
	}
}

// dumpFrame captures one composed frame, returns it, and writes it out when
// the dump flag names a directory and the frame is named.
func dumpFrame(t *testing.T, name string, w layout.Widget) *image.RGBA {
	t.Helper()
	img := golden.Capture(t, windowSize, w)
	if img.Bounds().Size() != windowSize {
		t.Fatalf("frame size = %v, want %v", img.Bounds().Size(), windowSize)
	}
	if *windowDump == "" || name == "" {
		return img
	}
	if err := os.MkdirAll(*windowDump, 0o755); err != nil {
		t.Fatalf("dump dir: %v", err)
	}
	path := filepath.Join(*windowDump, "mindchat-"+name+".png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
	return img
}

// TestTheBackdropShowsAroundThePane reads the composed window down its
// leading edge: the pane is set in one margin from the window's edges, and
// what shows in that margin is the window's own plane at the backdrop level.
// Nothing is drawn at the backdrop, so a frame that painted the transcript's
// paper across the whole window — as this one once did — would leave the pane
// standing on the document instead of being set into the window.
func TestTheBackdropShowsAroundThePane(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := dumpFrame(t, "", frame(t, tc.c, demoModel()))
			want := tc.c.SurfaceAt(tokens.LevelBackdrop)
			for x := 0; x < PaneMargin; x++ {
				for y := 0; y < windowSize.Y; y++ {
					got := img.RGBAAt(x, y)
					if got.R != want.R || got.G != want.G || got.B != want.B {
						t.Fatalf("the gap at (%d,%d) draws %v, want the backdrop %v", x, y, got, want)
					}
				}
			}
		})
	}
}
