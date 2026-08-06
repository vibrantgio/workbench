package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/spectrum/tokens"
)

// TestVisibleHistoryShowsWaitingRowUntilTheFirstToken pins the rule that
// covers the blank gap: from the moment a stream is registered for the
// current chat until the first delta opens the assistant row, the pane draws
// a pending row. Before it, the four-plus seconds a reasoning model spends
// thinking drew nothing whatever.
func TestVisibleHistoryShowsWaitingRowUntilTheFirstToken(t *testing.T) {
	user := Message{Role: RoleUser, Content: "explain monoids"}
	partial := Message{Role: RoleAssistant, Content: "A monoid"}

	for _, tc := range []struct {
		name  string
		model Model
		want  []string
	}{
		{
			name:  "idle chat draws its history and nothing else",
			model: Model{CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}}},
			want:  []string{RoleUser},
		},
		{
			// The gap this task exists to fill.
			name: "request sent, no token yet",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl"}},
			},
			want: []string{RoleUser, RolePending},
		},
		{
			name: "first delta stands the waiting row down",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user, partial}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl"}},
			},
			want: []string{RoleUser, RoleAssistant},
		},
		{
			name: "a tool running before the first token gets both rows",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl", Status: "Searching the web…"}},
			},
			want: []string{RoleUser, RoleStatus, RolePending},
		},
		{
			name: "a tool running after the answer started does not resurrect it",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user, partial}},
				Streams:     map[int]StreamState{1: {Chat: "a.jsonl", Status: "Searching the web…"}},
			},
			want: []string{RoleUser, RoleAssistant, RoleStatus},
		},
		{
			name: "another chat's stream draws nothing here",
			model: Model{
				CurrentChat: Chat{Name: "a.jsonl", History: []Message{user}},
				Streams:     map[int]StreamState{1: {Chat: "b.jsonl"}},
			},
			want: []string{RoleUser},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := make([]string, 0, len(tc.want))
			for _, msg := range visibleHistory(tc.model) {
				got = append(got, msg.Role)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("rows = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("rows = %v, want %v", got, tc.want)
				}
			}
			// Nothing transient may reach the model, and so the history file.
			for _, msg := range tc.model.CurrentChat.History {
				if msg.Role == RolePending || msg.Role == RoleStatus {
					t.Fatalf("transient row leaked into the model's history: %+v", tc.model.CurrentChat.History)
				}
			}
		})
	}
}

// motionThemed is the themed snapshot the indicators read, for one motion
// scale. Everything else is the default theme.
func motionThemed(m tokens.MotionScale) themed {
	return themed{
		palette: PaletteFrom(tokens.DefaultLight),
		typ:     tokens.DefaultTypography,
		motion:  m,
	}
}

// drawIndicator lays a widget out at one instant against a live input.Source
// and reports its dimensions and whether it asked to be repainted. The
// wakeup is what op.InvalidateCmd sets, so it answers "does this animate?"
// exactly, with no reference to pixels.
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

// TestWaitingIndicatorsHonourTheThemesMotionScale is the reduced-motion
// contract for both in-flight indicators, and the reason neither carries a
// duration of its own. The theme composes the OS preference (E3.2): while
// Reduce Motion is on it emits tokens.Motion.Reduced(), every duration stop
// zero. A zero stop must mean a still indicator that schedules no further
// frame — not a slower spin, and not the same spin.
func TestWaitingIndicatorsHonourTheThemesMotionScale(t *testing.T) {
	// Two instants a third of a second apart: far enough into any cycle
	// derived from the stops that a moving indicator differs between them.
	first := time.Unix(0, 0)
	second := first.Add(333 * time.Millisecond)

	for _, tc := range []struct {
		name    string
		scale   tokens.MotionScale
		animate bool
	}{
		{"default scale animates", tokens.Motion, true},
		{"reduced scale is still", tokens.Motion.Reduced(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := motionThemed(tc.scale)

			dims, wakeup := drawIndicator(first, func(gtx layout.Context) layout.Dimensions {
				return WaitingDots(gtx, th)
			})
			if wakeup != tc.animate {
				t.Errorf("WaitingDots requested a repaint = %v, want %v", wakeup, tc.animate)
			}
			if dims.Size.X <= 0 || dims.Size.Y <= 0 {
				t.Fatalf("WaitingDots dimensions = %v, want a drawn row", dims.Size)
			}

			later, _ := drawIndicator(second, func(gtx layout.Context) layout.Dimensions {
				return WaitingDots(gtx, th)
			})
			if later.Size != dims.Size {
				t.Errorf("WaitingDots size moved between frames: %v then %v; the row must not resize while it waits", dims.Size, later.Size)
			}

			_, wakeup = drawIndicator(first, func(gtx layout.Context) layout.Dimensions {
				StreamDot(gtx, th, image.Pt(gtx.Dp(StreamDotSlot), gtx.Dp(DeleteIconSize)))
				return layout.Dimensions{Size: image.Pt(1, 1)}
			})
			if wakeup != tc.animate {
				t.Errorf("StreamDot requested a repaint = %v, want %v", wakeup, tc.animate)
			}
		})
	}
}

// TestMotionPhaseTravelsAcrossTheDots checks the arithmetic the wave rests
// on: over one cycle each dot leads the next by exactly one stop, so their
// phases are evenly spread rather than moving in lockstep — and a zero cycle
// (the reduced scale) reports that there is no phase at all.
func TestMotionPhaseTravelsAcrossTheDots(t *testing.T) {
	stop := tokens.Motion.DurXSlow
	cycle := WaitingDotCount * stop
	now := time.Unix(0, 0)

	var phases []float64
	for i := range WaitingDotCount {
		p, ok := motionPhase(now, cycle, time.Duration(i)*stop)
		if !ok {
			t.Fatalf("dot %d: motionPhase over a %v cycle reported no phase", i, cycle)
		}
		if p < 0 || p >= 1 {
			t.Fatalf("dot %d: phase = %v, want [0,1)", i, p)
		}
		phases = append(phases, p)
	}
	for i := 1; i < len(phases); i++ {
		gap := phases[i-1] - phases[i]
		if gap < 0 {
			gap += 1
		}
		if want := 1.0 / float64(WaitingDotCount); gap < want-1e-9 || gap > want+1e-9 {
			t.Errorf("dots %d and %d are %v of a cycle apart, want %v — the pulse must travel, not blink in unison", i-1, i, gap, want)
		}
	}

	if _, ok := motionPhase(now, 0, 0); ok {
		t.Error("motionPhase over a zero cycle reported a phase; a reduced motion scale must yield no animation at all")
	}
}
