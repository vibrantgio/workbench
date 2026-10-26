package main

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/progress"
	"github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// rowTag names a row by who it is from and what kind of line it is, the
// two the pane now reads separately.
func rowTag(m Message) string { return m.Role + "/" + string(m.Kind) }

// TestVisibleHistoryShowsWaitingRowUntilTheFirstToken pins the rule that
// covers the blank gap: from the moment a stream is registered for the
// current chat until the first delta opens the assistant row, the pane
// stands an empty arriving row. Once a delta has landed the answer itself is
// the arriving row, so no second one is added beside it.
func TestVisibleHistoryShowsWaitingRowUntilTheFirstToken(t *testing.T) {
	user := Message{Role: RoleUser, Kind: KindTurn, Content: "explain monoids"}
	partial := Message{Role: RoleAssistant, Kind: KindArriving, Content: "A monoid"}
	settled := Message{Role: RoleAssistant, Kind: KindTurn, Content: "A monoid is a set"}

	for _, tc := range []struct {
		name  string
		model Model
		want  []string
	}{
		{
			name:  "idle chat draws its history and nothing else",
			model: Model{CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}}},
			want:  []string{"user/turn"},
		},
		{
			name: "request sent, no token yet",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl"}},
			},
			want: []string{"user/turn", "assistant/arriving"},
		},
		{
			name: "first delta stands the empty waiting row down",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user, partial}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl"}},
			},
			want: []string{"user/turn", "assistant/arriving"},
		},
		{
			name: "a settled answer with the stream still open gets no second row",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user, settled}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl"}},
			},
			want: []string{"user/turn", "assistant/turn"},
		},
		{
			name: "a tool running before the first token gets both rows",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl", Status: "Searching the web…"}},
			},
			want: []string{"user/turn", "/note", "assistant/arriving"},
		},
		{
			name: "a tool running after the answer started does not resurrect it",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user, partial}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl", Status: "Searching the web…"}},
			},
			want: []string{"user/turn", "assistant/arriving", "/note"},
		},
		{
			name: "another chat's stream draws nothing here",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}},
				Streams:     map[int]StreamState{1: {Chat: "b.jsonl"}},
			},
			want: []string{"user/turn"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := make([]string, 0, len(tc.want))
			for _, msg := range visibleHistory(tc.model) {
				got = append(got, rowTag(msg))
			}
			if len(got) != len(tc.want) {
				t.Fatalf("rows = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("rows = %v, want %v", got, tc.want)
				}
			}
			// The rows the pane adds are the pane's own: the chat's
			// history is what the reducer left, never what was drawn.
			if n := len(tc.model.CurrentChat.History); n > len(tc.want) {
				t.Fatalf("visibleHistory grew the chat's own history to %d rows: %+v", n, tc.model.CurrentChat.History)
			}
			for _, msg := range tc.model.CurrentChat.History {
				if msg.Kind == KindNote {
					t.Fatalf("a note leaked into the model's history: %+v", tc.model.CurrentChat.History)
				}
			}
		})
	}
}

// drawIndicator lays a layout.Widget out at one instant against a live
// input.Source and reports its dimensions and whether it asked to be
// repainted. The wakeup is what op.InvalidateCmd sets, so it answers "does
// this animate?" exactly, with no reference to pixels.
func drawIndicator(now time.Time, w func(gtx layout.Context) layout.Dimensions) (layout.Dimensions, bool) {
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(400, 100)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Now:         now,
		Source:      router.Source(),
		Ops:         &ops,
	}
	dims := w(gtx)
	router.Frame(&ops)
	_, wakeup := router.WakeupTime()
	return dims, wakeup
}

// paintedBox is the box of the pixels inside r that differ from fill by
// more than 8 levels in any channel. An empty box means r is bare.
func paintedBox(img *image.RGBA, r image.Rectangle, fill color.NRGBA) image.Rectangle {
	var box image.Rectangle
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if max(abs(int(p.R)-int(fill.R)), abs(int(p.G)-int(fill.G)), abs(int(p.B)-int(fill.B))) > 8 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// TestArrivingTextIsOnePhraseEndingInTheEllipsis holds the arriving turn's
// text to the phrase with the one ellipsis character, never three full stops.
func TestArrivingTextIsOnePhraseEndingInTheEllipsis(t *testing.T) {
	if arrivingText != "Thinking\u2026" {
		t.Errorf("the arriving turn's text is %q, want %q", arrivingText, "Thinking\u2026")
	}
	if strings.Contains(arrivingText, "...") {
		t.Errorf("the arriving turn's text %q ends with three full stops, want U+2026", arrivingText)
	}
}

// TestTheArrivingTurnDrawsTheIndicatorAtTheBodysTrailingEnd renders an
// assistant turn before its first token in both schemes. The body's trailing
// end holds the indeterminate indicator at its measured 16 × 16, and the
// body's leading end holds the arriving text in the label colour over the
// transcript. No pixel is the accent colour. Once the first text arrives,
// the indicator and the arriving text are removed.
func TestTheArrivingTurnDrawsTheIndicatorAtTheBodysTrailingEnd(t *testing.T) {
	size := image.Pt(720, 80)
	for _, sc := range schemes {
		t.Run(sc.name, func(t *testing.T) {
			th := goldenThemed(t, sc.c)
			fill := th.palette.Transcript
			gtx := layout.Context{Constraints: layout.Exact(size), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
			at, width := columnOf(gtx).body(gtx, true)
			end := at.X + width
			n := int(progress.Size)

			img := golden.Capture(t, size, conversationColumn(th, []Message{{Role: RoleAssistant, Kind: KindArriving}}))
			ind := paintedBox(img, image.Rect(end-n-2, 0, size.X, size.Y), fill)
			if ind.Dx() != n || ind.Dy() != n {
				t.Errorf("the indicator paints %v, want the measured %d × %d", ind, n, n)
			}
			if ind.Max.X != end {
				t.Errorf("the indicator ends at x=%d, want the body's trailing end x=%d", ind.Max.X, end)
			}
			line := paintedBox(img, image.Rect(at.X, 0, end-n-int(progress.Gap), size.Y), fill)
			if line.Empty() {
				t.Fatal("the body's leading end is bare, want the arriving text")
			}
			if lead := line.Min.X - at.X; lead < 0 || lead > 2 {
				t.Errorf("the arriving text begins %d px past the body's leading end, want at most the glyph's own 2 px", lead)
			}
			if got, want := line.Min.Y+line.Max.Y, ind.Min.Y+ind.Max.Y; abs(got-want) > 3 {
				t.Errorf("the arriving text centres at y=%.1f, want the indicator's y=%.1f", float64(got)/2, float64(want)/2)
			}
			want := vgcolor.Flatten(sc.c.Label, fill)
			nearest := 255
			for y := line.Min.Y; y < line.Max.Y; y++ {
				for x := line.Min.X; x < line.Max.X; x++ {
					p := img.RGBAAt(x, y)
					nearest = min(nearest, max(abs(int(p.R)-int(want.R)), abs(int(p.G)-int(want.G)), abs(int(p.B)-int(want.B))))
				}
			}
			if nearest > 16 {
				t.Errorf("the arriving text's nearest pixel lies %d levels off the label colour over the transcript, want within 16", nearest)
			}
			accent := th.palette.Accent
			for y := range size.Y {
				for x := range size.X {
					p := img.RGBAAt(x, y)
					if max(abs(int(p.R)-int(accent.R)), abs(int(p.G)-int(accent.G)), abs(int(p.B)-int(accent.B))) <= 8 {
						t.Fatalf("pixel (%d, %d) is the accent colour %v, want no accent dot", x, y, p)
					}
				}
			}

			img = golden.Capture(t, size, conversationColumn(th, []Message{{Role: RoleAssistant, Kind: KindArriving, Content: "A monoid"}}))
			if box := paintedBox(img, image.Rect(end-n-2, 0, size.X, size.Y), fill); !box.Empty() {
				t.Errorf("after the first text arrives the body's trailing end paints %v, want no indicator", box)
			}
		})
	}
}

// TestTheArrivingTurnAnimatesOnTheFramesClock lays the arriving turn out at a
// fixed instant and checks that the indicator asks for the next frame.
func TestTheArrivingTurnAnimatesOnTheFramesClock(t *testing.T) {
	th := goldenThemed(t, tokens.PlatformLight)
	rows := newDocCache().Rows([]Message{{Role: RoleAssistant, Kind: KindArriving}})
	_, wakeup := drawIndicator(time.Unix(0, 0), func(gtx layout.Context) layout.Dimensions {
		return MessageRow(gtx, th, rows[0])
	})
	if !wakeup {
		t.Error("the arriving turn requested no next frame, want the indicator to step on the frame's clock")
	}
}

// TestAnInFlightSidebarRowDrawsTheIndicatorAtItsTrailingEnd renders one
// sidebar row at rest and selected, in both schemes. While its chat is in
// flight the row draws the indicator at its measured 16 × 16, at the
// trailing end of the title's column. That column ends the controls' own
// 6 dp gap before the row's two reserved controls begin. While the chat is
// not in flight that place is bare.
func TestAnInFlightSidebarRowDrawsTheIndicatorAtItsTrailingEnd(t *testing.T) {
	size := image.Pt(240, int(sidebar.RowHeight))
	n := int(progress.Size)
	// The row's trailing inset is 12 dp, and its two controls are two
	// icons 6 dp apart. The indicator keeps the same 6 dp off them.
	end := size.X - 12 - (2*int(DeleteIconSize) + 6) - 6
	for _, sc := range schemes {
		for _, selected := range []bool{false, true} {
			name := sc.name + "-rest"
			if selected {
				name = sc.name + "-selected"
			}
			t.Run(name, func(t *testing.T) {
				th := railThemed(t, sc.c)
				fill := th.palette.Sidebar
				if selected {
					fill = th.palette.RowSelected
				}
				draw := func(streaming bool) *image.RGBA {
					return golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
						paint.FillShape(gtx.Ops, th.palette.Sidebar, clip.Rect{Max: gtx.Constraints.Max}.Op())
						return ChatRow(gtx, th, "ab.jsonl", selected, false, streaming,
							new(gesture.Click), new(gesture.Click), new(gesture.Click), nil, postMessage)
					})
				}
				look := image.Rect(end-n-12, 0, end+2, size.Y)
				ind := paintedBox(draw(true), look, fill)
				if ind.Dx() != n || ind.Dy() != n {
					t.Errorf("the in-flight row's indicator paints %v, want the measured %d × %d", ind, n, n)
				}
				if ind.Max.X != end {
					t.Errorf("the indicator ends at x=%d, want the title column's trailing end x=%d", ind.Max.X, end)
				}
				if got, want := ind.Min.Y+ind.Max.Y, size.Y; abs(got-want) > 1 {
					t.Errorf("the indicator centres at y=%.1f, want the row's middle y=%.1f", float64(got)/2, float64(want)/2)
				}
				if box := paintedBox(draw(false), look, fill); !box.Empty() {
					t.Errorf("a row not in flight paints %v at the indicator's place, want it bare", box)
				}
			})
		}
	}
}
