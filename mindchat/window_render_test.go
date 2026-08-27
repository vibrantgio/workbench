package main

// A whole-window render, headless. The app has no offscreen mode of its own —
// it is a native window binary — but the layers the window renders are plain
// observables of layout.Widget, so composing them over a frozen theme and
// drawing them into a headless canvas produces the same frame the window
// would show, at the size the window opens at.
//
// It is here because a composition can only be judged as a composition. A
// render of one widget in isolation cannot see that a window reads darker in
// the middle than at its edges; this can. Run it with -window.dump=<dir> to
// write the frames out for a pair of eyes:
//
//	go test ./ -run TestWholeWindowRender -window.dump=/tmp/mindchat
//
// Without the flag it still renders both schemes every run, which makes it a
// smoke test of the whole layer stack: a panic anywhere in the sidebar, the
// transcript, the header picker or the prompt field fails it.

import (
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/layout"

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
// bulleted list, and an answer carrying web-search citations.
func demoModel() Model {
	return Model{
		CurrentChat: Chat{
			Name:   "reactive layouts.jsonl",
			Loaded: true,
			History: []Message{
				{Role: RoleUser, Content: "In MVU, how does a button click reach the update function?"},
				{Role: RoleAssistant, Content: "The widget never calls back into your code — it records a message. " +
					"The clickable's layout writes a `MessageOp` into the frame's operation list; after the frame, " +
					"the window collects every one of them and feeds them to the loop:\n\n" +
					"```go\nfor _, msg := range frame.Messages() {\n\tmodel = Update(model, msg)\n}\n```\n\n" +
					"One direction, no callbacks: view → ops → message → update → model → view."},
				{Role: RoleUser, Content: "So state never lives in the widget?"},
				{Role: RoleAssistant, Content: "Only ephemeral gesture state:\n\n" +
					"- press tracking\n- scroll position\n- an editor's cursor\n\n" +
					"Everything the app must remember reduces into the Model — the sidebar split, " +
					"the open popover, even an in-flight completion stream.",
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
		var latest layout.Widget
		sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
			if !done && err == nil {
				latest = w
			}
		})
		deadline := time.Now().Add(2 * time.Second)
		for latest == nil && time.Now().Before(deadline) {
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

// TestWholeWindowRender draws the composed window in both schemes, and writes
// the frames out when -window.dump names a directory.
func TestWholeWindowRender(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			img := golden.Capture(t, windowSize, frame(t, tc.c, demoModel()))
			if img.Bounds().Size() != windowSize {
				t.Fatalf("frame size = %v, want %v", img.Bounds().Size(), windowSize)
			}
			if *windowDump == "" {
				return
			}
			if err := os.MkdirAll(*windowDump, 0o755); err != nil {
				t.Fatalf("dump dir: %v", err)
			}
			path := filepath.Join(*windowDump, "mindchat-"+tc.name+".png")
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("create %s: %v", path, err)
			}
			defer f.Close()
			if err := png.Encode(f, img); err != nil {
				t.Fatalf("encode %s: %v", path, err)
			}
			t.Logf("wrote %s", path)
		})
	}
}
