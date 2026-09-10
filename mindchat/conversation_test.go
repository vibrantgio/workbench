package main

// The conversation's own goldens: the four kinds of line this pane draws,
// in one column, in both schemes.
//
// The column is captured on its own rather than inside the whole window
// because that is what CA2.1 shaped — a picture of the window would carry
// the pane, the chrome row and the composer through every future diff and
// bury a moved card in them. The whole window still has its render, and it
// is a smoke test rather than a stored image (window_render_test.go).

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// conversationFrame is the size the column is captured at: the transcript's
// own reading width, and deep enough to hold every kind of line at once.
var conversationFrame = image.Pt(int(ChatPaneWidth), 720)

// goldenConversation is the history the goldens are drawn from: two settled
// exchanges, then a third prompt whose answer is still in flight — a note
// reporting the tool it is running and the arriving row under it — and
// finally a failed turn. One picture holding all four kinds of line is the
// only place they can be judged against each other.
func goldenConversation() []Message {
	return []Message{
		{Role: RoleUser, Kind: KindTurn, Content: "In MVU, how does a button click reach the update function?"},
		{Role: RoleAssistant, Kind: KindTurn, Content: "The component records a `MessageOp`; after the frame, " +
			"the window drains the operation list into the loop:\n\n" +
			"```go\nfor _, msg := range frame.Messages() {\n\tmodel = Update(model, msg)\n}\n```"},
		{Role: RoleUser, Kind: KindTurn, Content: "So state never lives in the component? I mean the gesture state as well, or only what the model would have to reduce."},
		{Role: RoleAssistant, Kind: KindTurn, Content: "Only ephemeral gesture state — press tracking and an editor's cursor. Everything else reduces into the Model."},
		{Role: RoleUser, Kind: KindTurn, Content: "Show me the current release notes."},
		{Kind: KindNote, Content: "Searching the web…"},
		{Role: RoleAssistant, Kind: KindArriving},
		{Role: RoleAssistant, Kind: KindFailed, Content: "HTTP 410: Gone"},
	}
}

// goldenThemed is the themed snapshot the stored images are drawn with: the
// pinned shaper, so the same words shape to the same pixels on every
// machine, and the shipped scales, so the card and the banner carry the
// insets and corners the live pane gives them.
func goldenThemed(t *testing.T, c tokens.ColorTokens) themed {
	t.Helper()
	th := schemeThemed(t, c)
	th.sp, th.rad = tokens.Spacing, tokens.Radius
	return th
}

// conversationColumn draws the history as the pane's list would, over the
// transcript's own fill.
func conversationColumn(th themed, history []Message) layout.Widget {
	rows := newDocCache().Rows(history)
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, th.palette.Transcript, clip.Rect{Max: gtx.Constraints.Max}.Op())
		y := 0
		for _, row := range rows {
			stack := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			inner := gtx
			inner.Constraints.Max.Y = gtx.Constraints.Max.Y - y
			inner.Constraints.Min = image.Point{}
			y += MessageRow(inner, th, row).Size.Y
			stack.Pop()
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// TestConversationGolden stores the column in both schemes.
func TestConversationGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			th := goldenThemed(t, tc.c)
			golden.Render(t, "conversation-"+tc.name, conversationFrame, conversationColumn(th, goldenConversation()))
		})
	}
}

// TestTheUserTurnStopsAtItsMeasure holds the card to the two halves of the
// rule it was given: it is as wide as its words up to UserCardMeasure and no
// wider, and it is set against the band's trailing edge, which is the
// trailing edge the answer above it reads to.
func TestTheUserTurnStopsAtItsMeasure(t *testing.T) {
	th := goldenThemed(t, tokens.DefaultLight)
	sentinel := color.NRGBA{R: 255, G: 0, B: 255, A: 255}

	// The card's own fill, which is the one thing on the row that is not the
	// transcript's.
	fill := tokens.DefaultLight.RaisedOn(th.palette.Transcript).Fill

	measure := func(body string) (lo, hi, rowW int) {
		rows := newDocCache().Rows([]Message{{Role: RoleUser, Kind: KindTurn, Content: body}})
		var dims layout.Dimensions
		img := golden.Capture(t, conversationFrame, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, sentinel, clip.Rect{Max: gtx.Constraints.Max}.Op())
			dims = MessageRow(gtx, th, rows[0])
			return dims
		})
		lo, hi = -1, -1
		y := dims.Size.Y / 2
		for x := 0; x < conversationFrame.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			got := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
			if got == sentinel {
				t.Fatalf("pixel (%d,%d) is the sentinel; the row painted no fill", x, y)
			}
			if got == fill {
				if lo < 0 {
					lo = x
				}
				hi = x
			}
		}
		return lo, hi, dims.Size.X
	}

	short, shortEnd, rowW := measure("Hi.")
	long, longEnd, _ := measure("So state never lives in the component? I mean the gesture " +
		"state as well, or only what the model would have to reduce into the next one.")

	if short < 0 || long < 0 {
		t.Fatal("no card fill found on a user row; the prompt is not standing on a card")
	}
	if shortEnd != longEnd {
		t.Errorf("a short card ends at %d and a long one at %d; both are set against the band's trailing edge", shortEnd, longEnd)
	}
	if shortEnd-short >= longEnd-long {
		t.Errorf("the short prompt's card is %d wide and the long one's %d; a card is as wide as its words", shortEnd-short, longEnd-long)
	}
	if got, want := longEnd-long+1, int(UserCardMeasure); got > want {
		t.Errorf("the long prompt's card is %d wide, past the %d measure", got, want)
	}
	if rowW != conversationFrame.X {
		t.Errorf("the row reports %d wide, want the pane's %d — every row spans the column", rowW, conversationFrame.X)
	}
}

// TestTurnsShareOneTrailingEdge is what makes the transcript one column
// rather than two runs of text drifting apart: the answer's measure and the
// prompt's card read to the same edge.
func TestTurnsShareOneTrailingEdge(t *testing.T) {
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Constraints{Max: conversationFrame},
		Metric:      unitMetric(),
		Ops:         &ops,
	}
	col := columnOf(gtx)
	at, width := col.body(gtx, true)
	if got, want := at.X+width, col.lead+col.width; got != want {
		t.Errorf("the answer reads to %d and the band ends at %d", got, want)
	}
	if got, want := width, gtx.Dp(TurnMeasure); got != want {
		t.Errorf("the answer's measure is %d, want %d", got, want)
	}
}

// unitMetric is the one-pixel-per-dp metric the headless captures use.
func unitMetric() unit.Metric { return unit.Metric{PxPerDp: 1, PxPerSp: 1} }
