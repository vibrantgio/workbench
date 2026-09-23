package main

// A whole-window render, headless. The app has no offscreen mode of its own —
// it is a native window binary — but the layers the window renders are plain
// observables of layout.Widget, so composing them over a frozen theme and
// drawing them into a headless image produces the same frame the window
// would show, at the size the window opens at.
//
// A composition can only be judged as a composition: a render of one component
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
	"image/draw"
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
func staticTheme(c tokens.PlatformColors) theme.Theme {
	return theme.Theme{
		Platform:   rx.Of(c),
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
				{Role: RoleUser, Kind: KindTurn, Content: "In MVU, how does a button click reach the update function?"},
				{Role: RoleAssistant, Kind: KindTurn, Content: "The component records a `MessageOp`; after the frame, " +
					"the window drains the operation list into the loop:\n\n" +
					"```go\nfor _, msg := range frame.Messages() {\n\tmodel = Update(model, msg)\n}\n```"},
				{Role: RoleUser, Kind: KindTurn, Content: "So state never lives in the component?"},
				{Role: RoleAssistant, Kind: KindTurn, Content: "Only ephemeral gesture state:\n\n" +
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

// streamingModel is demoModel's window over a shorter conversation, caught
// part way through: two settled exchanges, one of which failed, and a third
// prompt whose answer has not started — a server-side tool reporting itself
// under it while the request is out. It is the one state the pane draws
// every kind of line in at once, and a composition of the four can be judged
// nowhere else. The history is its own rather than demoModel's because the
// four have to fit one viewport together.
func streamingModel() Model {
	m := demoModel()
	m.CurrentChat.History = []Message{
		{Role: RoleUser, Kind: KindTurn, Content: "In MVU, how does a button click reach the update function?"},
		{Role: RoleAssistant, Kind: KindTurn, Content: "The component records a `MessageOp`; after the frame, " +
			"the window drains the operation list into the loop:\n\n" +
			"```go\nfor _, msg := range frame.Messages() {\n\tmodel = Update(model, msg)\n}\n```"},
		{Role: RoleUser, Kind: KindTurn, Content: "So state never lives in the component? I mean the gesture state as well, or only what the model would have to reduce."},
		{Role: RoleAssistant, Kind: KindFailed, Content: "HTTP 410: Gone"},
		{Role: RoleUser, Kind: KindTurn, Content: "Show me the current release notes."},
	}
	m.Streams = map[int]StreamState{1: {Chat: m.CurrentChat.Name, Status: "Searching the web…"}}
	return m
}

// frame composes the window's layers for one scheme into a single
// layout.Widget: the backdrop first, the content over it, exactly as
// theme/window stacks them.
func frame(t *testing.T, c tokens.PlatformColors, model Model) layout.Widget {
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
			t.Fatalf("layer %d never emitted a layout.Widget", i)
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
// stands in this window: the dialog is 560 dp wide and centred, the trigger
// runs from the form's field column to the body's trailing edge, and its row
// is the last one in the form. Read off the rendered frame rather than
// recomputed from the modal's internals — x 514-771, y 474-497 in
// window-settings-light.png — and asserted by the open capture differing from
// the closed one.
var defaultPickerCentre = f32.Pt(642, 485)

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
		t.Run(tc.name+"-stream", func(t *testing.T) {
			renderWindow(t, tc.name+"-stream", tc.c, streamingModel())
		})
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

// TestWholeWindowVerdictRender draws the settings dialog with the selected
// provider's key already answered for — the one state the key-check verdict
// is drawn in, and the one the dialogs rendered above cannot show, since a
// dialog opened on a keyless catalogue reserves the verdict's box and leaves
// it empty.
func TestWholeWindowVerdictRender(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	verdicts := []struct {
		name string
		m    Model
	}{
		{"key-ok", settingsWithKey("")},
		{"key-bad", settingsWithKey("HTTP 401: invalid_api_key")},
	}
	for _, tc := range schemes {
		for _, v := range verdicts {
			t.Run(tc.name+"-"+v.name, func(t *testing.T) {
				renderWindow(t, tc.name+"-settings-"+v.name, tc.c, v.m)
			})
		}
	}
}

// settingsWithKey opens the settings dialog on a catalogue whose selected
// provider carries a key the /models fetch has already answered for: an
// empty failure is the live key, a non-empty one the failed key. The
// verdict's status is read off that answer, so this is what decides which
// disc the key row draws.
func settingsWithKey(failure string) Model {
	m, _ := Update(demoModel(), OpenSettings{})
	sel := m.Settings.Draft[m.Settings.Selected]
	sel.APIKey = "sk-demo-key"
	m.Settings.Draft[m.Settings.Selected] = sel
	m.Settings.Errors = map[string]string{sel.Name: failure}
	return m
}

// renderPane draws one scheme in one pane state and writes it out when the
// dump flag names a directory.
func renderPane(t *testing.T, name string, c tokens.PlatformColors, hidden bool) {
	t.Helper()
	m := demoModel()
	m.SidebarHidden = hidden
	renderWindow(t, name, c, m)
}

// renderWindow draws one Model in one scheme and writes it out when the dump
// flag names a directory.
func renderWindow(t *testing.T, name string, c tokens.PlatformColors, m Model) {
	t.Helper()
	dumpFrame(t, name, withWindowControls(frame(t, c, m)))
}

// clicked drives w through two headless frames with a click queued at pos and
// returns a layout.Widget drawing from the state those frames left behind. It
// is how a surface whose open state lives INSIDE a component — a picker
// field's menu — is captured standing open: the Model cannot be posed into it.
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

// TestTheRailIsSetIntoTheWindow reads the composed window down its leading
// edge: the rail is a panel set one margin in from the window's leading, top
// and bottom edges, with the window's own plane showing in the margin and
// the panel's rim at the end of it. MEASURED,
// voicememos-multi-folder-2026-09-18.png and
// finder-window-untinted-dark.png: eight pixels of the window's plane stand
// between the window's bound and the panel on those three sides.
func TestTheRailIsSetIntoTheWindow(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := dumpFrame(t, "", frame(t, tc.c, demoModel()))
			// The window's middle row, clear of the panel's rounded corners.
			y := windowSize.Y / 2
			fill := tc.c.SidebarMaterial
			// The margin carries the window's own plane under the panel's
			// shadow, so it is never lighter than the plane. It is not told
			// from the panel's fill here: in the dark appearance the
			// platform's chrome material and its shadowed plane are two of
			// 255 apart, which is why the boundary is the rim below.
			plane := tc.c.WindowBackground
			for x := 0; x < PaneMargin; x++ {
				if got := img.RGBAAt(x, y); got.R > plane.R || got.G > plane.G || got.B > plane.B {
					t.Fatalf("the window's leading margin at (%d,%d) draws %v, lighter than its own plane %v; a shadow only darkens", x, y, got, plane)
				}
			}
			rim := tc.c.PaneRim
			if got := img.RGBAAt(PaneMargin, y); got.R != rim.R || got.G != rim.G || got.B != rim.B {
				t.Fatalf("the panel's leading edge at (%d,%d) draws %v, want the platform's rim %v", PaneMargin, y, got, rim)
			}
			if got := img.RGBAAt(PaneMargin+1, y); got.R != fill.R || got.G != fill.G || got.B != fill.B {
				t.Fatalf("one pixel inside the panel's leading rim draws %v, want the chrome material %v", got, fill)
			}
		})
	}
}

// switchBand is the run of the window's top band the sidebar switch stands
// in, at the band's full height and wide enough to hold both placements: the
// panel's own marks at its top trailing corner while it stands, and the
// chrome row's two controls after the window's buttons once it is away.
var switchBand = image.Rect(0, 0, 320, 52)

// TestTheSidebarSwitchGolden pins the sidebar switch itself: the control that
// puts the panel away and the control that brings it back, drawn at the same
// size on the same line, in both schemes.
//
// It is a crop of the whole-window frame rather than a render of the control
// alone, because what is being pinned is each half where it stands — the
// panel's half bare at the panel's top trailing corner, the row's half in
// the platform's bordered toolbar control, and the air each leaves around
// it. The whole-window frames above are compositions for a pair
// of eyes and store nothing; this stores the one run of them a change to the
// switch has to move.
func TestTheSidebarSwitchGolden(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	for _, tc := range schemes {
		for _, st := range []struct {
			name   string
			hidden bool
		}{{"pane", false}, {"hidden", true}} {
			name := "switch-" + tc.name + "-" + st.name
			t.Run(name, func(t *testing.T) {
				m := demoModel()
				m.SidebarHidden = st.hidden
				full := dumpFrame(t, "", withWindowControls(frame(t, tc.c, m)))
				golden.Compare(t, name, crop(full, switchBand))
			})
		}
	}
}

// TestTheWholeWindowGolden stores the composed window itself, in both
// schemes: the pane standing, the transcript under the chrome row, the prompt
// field at the foot. The four frames TestWholeWindowRender draws are for a
// pair of eyes and store nothing; this is the one composition a change
// anywhere in the layer stack has to move, and it is kept in the shape
// vaultview keeps its own window in — one render at the size the window opens
// at, compared whole.
//
// The switch crop above stays beside it. A crop pins one run of the band at
// the byte where a whole-window image would drown it; the window pins what
// only a composition carries — the band's controls against the columns under
// them, the shadows they cast on those columns included.
func TestTheWholeWindowGolden(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	for _, tc := range schemes {
		name := "window-" + tc.name
		t.Run(name, func(t *testing.T) {
			golden.Render(t, name, windowSize, withWindowControls(frame(t, tc.c, demoModel())))
		})
	}
}

// crop copies r out of img into an image of its own, at the origin: a golden
// is compared by its bounds as well as its pixels, and a sub-image carries the
// offset it was cut at.
func crop(img *image.RGBA, r image.Rectangle) *image.RGBA {
	out := image.NewRGBA(image.Rectangle{Max: r.Size()})
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

// TestTheSettingsDialogGolden records the settings dialog standing open over
// the window it interrupts, in both schemes. It is the only stored picture of
// the dialog as a composition — the providers well beside the form, the
// labels in their column, the templates as one control and the default-model
// row at the foot — and the only one that can show the sheet against the
// window it stands on. It is kept in the shape vaultview keeps its own
// vault-switch dialog in: one render at the size the window opens at,
// compared whole.
//
// The catalogue is posed with the selected provider's key already answered
// for, because the verdict disc is part of what the composition is judged on
// and a dialog opened on a keyless catalogue reserves its box and leaves it
// empty.
func TestTheSettingsDialogGolden(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	m := settingsWithKey("")
	for _, tc := range schemes {
		name := "window-settings-" + tc.name
		t.Run(name, func(t *testing.T) {
			golden.Render(t, name, windowSize, withWindowControls(frame(t, tc.c, m)))
		})
	}
}
