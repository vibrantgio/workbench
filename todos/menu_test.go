package main

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/tokens"
)

// menuFrameSize is the size of the frame the tests below lay out, at one
// pixel per dp.
var menuFrameSize = image.Pt(400, 300)

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

// The bar holds New Todo in File on Command-N and Delete in Edit. Each item
// posts the message of its control: New Todo the add button's, Delete the
// message the list's Delete key posts.
func TestTheBarHoldsNewTodoAndDelete(t *testing.T) {
	want := map[string]struct {
		menu string
		key  string
		msg  mvu.Message
	}{
		"New Todo": {menu: "File", key: "n", msg: SetRoute{Route: "add.todo"}},
		"Delete":   {menu: "Edit", key: "", msg: DeleteSelected{}},
	}
	for _, item := range MenuItems() {
		w, ok := want[item.Title]
		if !ok {
			t.Fatalf("unexpected menu item %q", item.Title)
		}
		if item.Menu != w.menu {
			t.Errorf("%q is in menu %q, want %q", item.Title, item.Menu, w.menu)
		}
		if item.Key != w.key {
			t.Errorf("%q has the key equivalent %q, want %q", item.Title, item.Key, w.key)
		}
		if item.Msg != w.msg {
			t.Errorf("%q posts %#v, want %#v", item.Title, item.Msg, w.msg)
		}
		delete(want, item.Title)
	}
	for title := range want {
		t.Errorf("the menu does not declare %q", title)
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

// DeleteSelected deletes the selected todo and only that one. With no todo
// selected, or with a dialog open, it changes nothing.
func TestDeleteSelectedDeletesTheSelectedTodo(t *testing.T) {
	list := TodoList{{Id: 0, Text: "a"}, {Id: 1, Text: "b"}, {Id: 2, Text: "c"}}
	m := ReduceModel(Model{List: list}, SelectTodo{Id: 1})
	got := ReduceModel(m, DeleteSelected{})
	if len(got.List) != 2 || got.List[0].Id != 0 || got.List[1].Id != 2 {
		t.Fatalf("DeleteSelected with todo 1 selected leaves %+v, want todos 0 and 2", got.List)
	}
	if got.HasSelected {
		t.Errorf("the deleted todo is still selected")
	}

	none := Model{List: list}
	if got := ReduceModel(none, DeleteSelected{}); len(got.List) != 3 {
		t.Errorf("DeleteSelected with no selection leaves %d todos, want 3", len(got.List))
	}

	for _, route := range []string{"add.todo", "edit.todo"} {
		open := ReduceModel(m, SetRoute{Route: route})
		if got := ReduceModel(open, DeleteSelected{}); len(got.List) != 3 {
			t.Errorf("DeleteSelected on route %q leaves %d todos, want 3", route, len(got.List))
		}
	}
}

// A todo deleted by its row's mark leaves the selection on the selected todo
// when it is another todo, and clears it when it is the selected todo.
func TestDeleteTodoKeepsTheSelectionOnItsTodo(t *testing.T) {
	list := TodoList{{Id: 0, Text: "a"}, {Id: 1, Text: "b"}}
	m := ReduceModel(Model{List: list}, SelectTodo{Id: 1})
	if got := ReduceModel(m, DeleteTodo{Id: 0}); !got.HasSelected || got.Selected != 1 {
		t.Errorf("deleting todo 0 leaves selection %v/%d, want todo 1", got.HasSelected, got.Selected)
	}
	if got := ReduceModel(m, DeleteTodo{Id: 1}); got.HasSelected {
		t.Errorf("deleting the selected todo leaves it selected")
	}
}

// New Todo from the menu does not replace an open dialog, so the text in
// the dialog's field stays.
func TestADialogRouteDoesNotReplaceAnOpenDialog(t *testing.T) {
	m := Model{Route: "edit.todo", List: TodoList{{Id: 0, Text: "a"}}}
	if got := ReduceModel(m, SetRoute{Route: "add.todo"}); got.Route != "edit.todo" {
		t.Errorf("New Todo over the edit dialog sets route %q, want edit.todo", got.Route)
	}
	if got := ReduceModel(m, SetRoute{}); got.Route != "" {
		t.Errorf("closing the dialog sets route %q, want the list", got.Route)
	}
}

// listRig is a scrollingList over list, with a text field under it, both
// driven through one router. posted records what the list posts.
type listRig struct {
	r      *gioinput.Router
	ops    *op.Ops
	state  *complist.State
	editor *widget.Editor
	posted []mvu.Message
	frame  layout.Widget
}

const rigRowHeight = 40

func newListRig(t *testing.T, model Model) *listRig {
	t.Helper()
	rig := &listRig{r: new(gioinput.Router), ops: new(op.Ops), state: complist.NewState(), editor: new(widget.Editor)}
	rig.editor.SingleLine = true
	rig.editor.SetText("abc")
	rows := make([]rowLayout, len(model.List))
	ids := make([]int, len(model.List))
	selected := -1
	for i, todo := range model.List {
		rows[i] = func(gtx layout.Context, _ rowSelection) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		ids[i] = todo.Id
		if model.HasSelected && todo.Id == model.Selected {
			selected = i
		}
	}
	th := staticThemed(tokens.PlatformLight)
	list := scrollingList(rows, ids, selected, rigRowHeight, th.palette.RowSelection, th.listBar(), rig.state,
		func(_ layout.Context, msg mvu.Message) { rig.posted = append(rig.posted, msg) })
	rig.frame = func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		lgtx := gtx
		lgtx.Constraints = layout.Exact(image.Pt(size.X, 200))
		list(lgtx)
		defer op.Offset(image.Pt(0, 220)).Push(gtx.Ops).Pop()
		egtx := gtx
		egtx.Constraints = layout.Exact(image.Pt(size.X, 40))
		rig.editor.Layout(egtx, th.typ.Shaper, th.typ.Field.Font, th.typ.Field.Size, op.CallOp{}, op.CallOp{})
		return layout.Dimensions{Size: size}
	}
	rig.drive()
	return rig
}

func (rig *listRig) drive() { driveFrame(rig.frame, rig.ops, rig.r) }

func (rig *listRig) press(at f32.Point) {
	rig.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
	)
	rig.drive()
}

func (rig *listRig) key(name key.Name) {
	rig.r.Queue(key.Event{Name: name, State: key.Press})
	rig.drive()
}

// A press on the third row selects it and posts its todo to the update. The
// Delete key then posts DeleteSelected, and the update deletes that todo
// and no other.
func TestTheDeleteKeyDeletesTheSelectedTodo(t *testing.T) {
	model := Model{List: TodoList{{Id: 0, Text: "a"}, {Id: 1, Text: "b"}, {Id: 7, Text: "c"}, {Id: 9, Text: "d"}}}
	rig := newListRig(t, model)
	rig.press(f32.Pt(100, 2*rigRowHeight+rigRowHeight/2))
	rig.key(deleteKey)

	want := []mvu.Message{SelectTodo{Id: 7}, DeleteSelected{}}
	if len(rig.posted) != len(want) || rig.posted[0] != want[0] || rig.posted[1] != want[1] {
		t.Fatalf("the press and the Delete key post %#v, want %#v", rig.posted, want)
	}
	for _, msg := range rig.posted {
		model = ReduceModel(model, msg)
	}
	if _, ok := model.List.Find(7); ok || len(model.List) != 3 {
		t.Fatalf("after the Delete key the list is %+v, want todo 7 gone and three left", model.List)
	}
}

// With the text field holding the keyboard, the Delete key deletes the
// field's text and posts nothing to the list's todos.
func TestTheDeleteKeyInATextFieldDeletesNoTodo(t *testing.T) {
	model := Model{List: TodoList{{Id: 0, Text: "a"}, {Id: 1, Text: "b"}}, Selected: 1, HasSelected: true}
	rig := newListRig(t, model)
	rig.press(f32.Pt(100, 230))
	if !rig.r.Source().Focused(rig.editor) {
		t.Fatalf("a press on the text field did not give it the keyboard")
	}
	rig.key(deleteKey)
	for _, msg := range rig.posted {
		if _, ok := msg.(DeleteSelected); ok {
			t.Fatalf("the Delete key in the text field posted DeleteSelected")
		}
	}
	if got := rig.editor.Text(); got == "abc" {
		t.Errorf("the Delete key did not reach the text field: its text is still %q", got)
	}
}

// The first frame selects the row of Model's selected todo, and a Model
// with no selection selects no row.
func TestTheListSelectsModelsSelectedTodo(t *testing.T) {
	list := TodoList{{Id: 0, Text: "a"}, {Id: 4, Text: "b"}}
	if got := newListRig(t, Model{List: list, Selected: 4, HasSelected: true}).state.Selected(); got != 1 {
		t.Errorf("with todo 4 selected the list selects row %d, want 1", got)
	}
	if got := newListRig(t, Model{List: list}).state.Selected(); got != -1 {
		t.Errorf("with no selection the list selects row %d, want none", got)
	}
}
