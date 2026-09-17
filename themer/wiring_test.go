package main

import (
	"image"
	stdcolor "image/color"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// TestALateSubscriberReadsTheModelInForce drives the production seam — this
// window's own reducer under mvu.Loop, over the message stream run() merges —
// and subscribes it a second time only after a message has moved the model.
//
// This is the invariant an AutoConnect count used to stand for, and it holds
// without one: the colour field's own theme re-subscribes the model every
// time the live theme re-emits, which is whenever the desktop's appearance
// changes, so no fixed count could be right. A stream without replay leaves
// such a subscriber with no model, and so the field with no colours, until
// the next message.
func TestALateSubscriberReadsTheModelInForce(t *testing.T) {
	messages := make(chan mvu.Message, 1)
	init := func() (Model, mvu.Command) { return judging(), mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Recv(messages), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()

	// One early subscriber, so the loop is connected and draining.
	early := make(chan Model, 8)
	first := models.Subscribe(rx.GoroutineContext(), func(m Model, err error, done bool) {
		if err == nil && !done {
			select {
			case early <- m:
			default:
			}
		}
	})
	defer first.Unsubscribe()
	await(t, early, func(m Model) bool { return m.Scheme == FollowOS }, "the seed")

	messages <- SetScheme{Dark: true}
	await(t, early, func(m Model) bool { return m.Scheme == ShowDark }, "the model the message left")

	// The late subscriber attaches with the switch already flipped.
	late := make(chan Model, 8)
	second := models.Subscribe(rx.GoroutineContext(), func(m Model, err error, done bool) {
		if err == nil && !done {
			select {
			case late <- m:
			default:
			}
		}
	})
	defer second.Unsubscribe()
	await(t, late, func(m Model) bool { return m.Scheme == ShowDark }, "the model in force at a late subscription")
}

// await polls values until one satisfies cond, and fails naming what was
// waited for.
func await(t *testing.T, values <-chan Model, cond func(Model) bool, what string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case m := <-values:
			if cond(m) {
				return
			}
		case <-deadline:
			t.Fatalf("the model stream never carried %s", what)
		}
	}
}

// TestTheContentLayerDraws: the page's layer is the model, the theme and the
// tab strip's own stream combined, and a combination that never emits is a
// window with nothing in it. What is asserted is that a [layout.Widget] arrives
// at all — what it draws is asserted everywhere else in this package, on pixels.
func TestTheContentLayerDraws(t *testing.T) {
	got := make(chan layout.Widget, 1)
	sub := ContentLayer(rx.Of(theme.Default()), rx.Of(judging()), &desktop.ZoneGroup{}).
		Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
			if done || err != nil || w == nil {
				return
			}
			select {
			case got <- w:
			default:
			}
		})
	defer sub.Unsubscribe()
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("the content layer emitted nothing to lay out — the window would open on an empty page")
	}
}

// fieldSize is the box the colour field is captured in: wide enough that the
// trailing end of the field is clear of the placeholder, and a row tall.
var fieldSize = image.Pt(320, 44)

// fieldFill renders [HexField] against a desktop on live with the window's
// switch set by m, and returns the field's own interior — read at the
// trailing end of the row, clear of the placeholder and of the rounded
// corners.
func fieldFill(t *testing.T, live tokens.PlatformColors, m Model) stdcolor.RGBA {
	t.Helper()
	th := theme.Default()
	th.Platform = rx.Of(live)
	th.Typography = rx.Of(tokens.DefaultTypography)

	widgets := make(chan layout.Widget, 1)
	sub := HexField(rx.Of(th), rx.Of(m)).Subscribe(rx.GoroutineContext(),
		func(w layout.Widget, err error, done bool) {
			if err != nil || done || w == nil {
				return
			}
			select {
			case widgets <- w:
			default:
			}
		})
	defer sub.Unsubscribe()

	var field layout.Widget
	select {
	case field = <-widgets:
	case <-time.After(2 * time.Second):
		t.Fatal("the colour field emitted nothing to lay out — it is waiting on a stream that never carried it a value")
	}
	img := golden.Capture(t, fieldSize, func(gtx layout.Context) layout.Dimensions {
		return field(gtx)
	})
	return img.RGBAAt(fieldSize.X-14, fieldSize.Y/2)
}

// TestTheColourFieldWearsTheAppearanceTheSwitchIsOn: the hex field is a
// published live component that reads its colours off the theme it is handed,
// and the theme this window hands it is [WindowTheme] — the platform's set
// for the appearance the switch at the top of the window is on. So with the
// desktop set one way and the switch the other, the field's interior is the
// other side's text background, not the desktop's, in both directions.
func TestTheColourFieldWearsTheAppearanceTheSwitchIsOn(t *testing.T) {
	for _, c := range []struct {
		name   string
		live   tokens.PlatformColors
		scheme Scheme
	}{
		{"a dark window on a light desktop", tokens.PlatformLight, ShowDark},
		{"a light window on a dark desktop", tokens.PlatformDark, ShowLight},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := judging()
			m.Scheme = c.scheme
			got := fieldFill(t, c.live, m)
			want := WindowSet(c.live, m).TextBackground
			if !is(got, want) {
				t.Fatalf("the colour field's interior is %v; the appearance the switch is on paints it %v — the field is reading the desktop's set instead of the window's", got, want)
			}
			if is(got, c.live.TextBackground) {
				t.Fatalf("the colour field's interior is the desktop's %v — the switch moved the window and left the field behind", c.live.TextBackground)
			}
		})
	}
}
