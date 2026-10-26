package main

import (
	"image"
	"strings"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// menuFrameSize is the size of the frame the tests below lay out, at one
// pixel per dp.
var menuFrameSize = image.Pt(900, 600)

// driveFrame lays one frame of w out through r.
func driveFrame(w layout.Widget, ops *op.Ops, r *gioinput.Router) {
	ops.Reset()
	w(layout.Context{
		Constraints: layout.Exact(menuFrameSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      r.Source(),
	})
	r.Frame(ops)
}

// postedByChord drives the key areas ChordAreas builds and returns every
// message they post for one press of name with the modifiers mods.
func postedByChord(t *testing.T, name key.Name, mods key.Modifiers) []mvu.Message {
	t.Helper()
	var posted []mvu.Message
	areas := ChordAreas(func(_ layout.Context, msg mvu.Message) { posted = append(posted, msg) })
	frame := func(gtx layout.Context) layout.Dimensions {
		for _, a := range areas {
			a(gtx)
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(frame, ops, r)
	r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	driveFrame(frame, ops, r)
	return posted
}

// The bar holds Find in Edit on Command-F, posting Find.
func TestTheBarHoldsFind(t *testing.T) {
	items := MenuItems()
	if len(items) != 1 {
		t.Fatalf("the menu declares %d items, want 1: %+v", len(items), items)
	}
	item := items[0]
	if item.Menu != "Edit" || item.Title != "Find" || item.Key != "f" || item.Msg != (Find{}) {
		t.Errorf("the item is %+v, want Edit Find on f posting Find{}", item)
	}
}

// Each item with a key equivalent posts the same message as the chord the
// window lays out for that key. The bare key posts nothing.
func TestTheMenuItemsPostTheSameMessagesAsTheChords(t *testing.T) {
	chords := 0
	for _, item := range MenuItems() {
		if item.Key == "" {
			continue
		}
		chords++
		name := key.Name(strings.ToUpper(item.Key))
		posted := postedByChord(t, name, key.ModShortcut)
		if len(posted) != 1 || posted[0] != item.Msg {
			t.Errorf("the chord on %q posts %#v, the item %q posts %#v", name, posted, item.Title, item.Msg)
		}
		if bare := postedByChord(t, name, 0); len(bare) != 0 {
			t.Errorf("the bare %q posts %#v, want nothing", name, bare)
		}
	}
	if got := len(ChordAreas(func(layout.Context, mvu.Message) {})); got != chords {
		t.Errorf("the window lays out %d key areas, want one per key equivalent (%d)", got, chords)
	}
}

// Find counts a request and keeps the query.
func TestFindCountsARequest(t *testing.T) {
	got := ReduceModel(Model{Query: "arrow"}, Find{})
	if got.Finds != 1 || got.Query != "arrow" {
		t.Errorf("Find gives %+v, want one request and the query kept", got)
	}
}

// focusAfterFrames lays out two frames of the page for model and reports
// whether a control holds the keyboard after them.
func focusAfterFrames(t *testing.T, model Model) bool {
	t.Helper()
	th := rx.Of(golden.StaticTheme(tokens.PlatformLight))
	var page layout.Widget
	var bars desktop.ScrollBarSetting
	sub := ContentLayer(th, rx.Of(model), bars).Subscribe(rx.GoroutineContext(),
		func(next layout.Widget, err error, done bool) {
			if !done && err == nil {
				page = next
			}
		})
	t.Cleanup(func() { sub.Unsubscribe() })
	sub.Wait()
	if page == nil {
		t.Fatal("the page emitted nothing")
	}
	r := new(gioinput.Router)
	ops := new(op.Ops)
	driveFrame(page, ops, r)
	driveFrame(page, ops, r)
	return !r.Source().Focused(nil)
}

// The page gives the keyboard to the search field on the first frame after
// the Find count changes, and not while the count stays at zero.
func TestFindGivesTheKeyboardToTheSearchField(t *testing.T) {
	if focusAfterFrames(t, Model{}) {
		t.Errorf("with no Find request a control holds the keyboard")
	}
	if !focusAfterFrames(t, Model{Finds: 1}) {
		t.Errorf("after a Find request no control holds the keyboard, want the search field")
	}
}
