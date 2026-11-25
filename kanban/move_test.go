package main

import (
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// shownCardMenu is what the Card menu shows for m, one item a line: "Move
// Item" and "Move to Column", each followed by its items in brackets, and
// "off" after each disabled item.
func shownCardMenu(m Model) string {
	text := func(it desktop.MenuItem) string {
		if it.Disabled {
			return it.Title + " off"
		}
		return it.Title
	}
	var lines []string
	for _, it := range cardMenu(m) {
		var subs []string
		for _, sub := range it.Items {
			subs = append(subs, text(sub))
		}
		lines = append(lines, text(it)+" ["+strings.Join(subs, ", ")+"]")
	}
	return strings.Join(lines, "\n")
}

// The Card menu's declaration is the platform's: "Move Item" holds "Up" on
// Ctrl-Cmd-Up and "Down" on Ctrl-Cmd-Down, and "Move to Column" has no key
// equivalent. Each item stands after the Edit menu's, at its index.
func TestTheCardMenuIsDeclaredAsThePlatformsMenusAre(t *testing.T) {
	bar := menuBar()
	if len(bar) != moveToColumnItem+1 {
		t.Fatalf("the menu bar declares %d items, want %d", len(bar), moveToColumnItem+1)
	}
	mv, col := bar[moveItem], bar[moveToColumnItem]
	if mv.Menu != "Card" || mv.Title != "Move Item" || !mv.Submenu || !mv.Disabled ||
		col.Menu != "Card" || col.Title != "Move to Column" || !col.Submenu || !col.Disabled || col.Key != "" {
		t.Fatalf("the Card menu declares %+v and %+v", mv, col)
	}
	if len(mv.Items) != 2 {
		t.Fatalf("Move Item holds %d items, want Up and Down", len(mv.Items))
	}
	up, down := mv.Items[0], mv.Items[1]
	if up.Title != "Up" || up.Key != desktop.UpArrow || !up.Control || up.Msg != (MoveUp{}) ||
		down.Title != "Down" || down.Key != desktop.DownArrow || !down.Control || down.Msg != (MoveDown{}) {
		t.Fatalf("Move Item holds %+v and %+v", up, down)
	}
	if bar[undoItem].Title != "Undo" || bar[redoItem].Title != "Redo" {
		t.Fatalf("the Edit menu's items are not first: %+v", bar[:2])
	}
}

// With no selected card, every item of the Card menu is disabled. "Up" is
// disabled on a column's first card and "Down" on its last, and the card's
// own column is disabled. A column alone with one card has both disabled.
func TestTheCardMenuFollowsTheSelectedCard(t *testing.T) {
	m := newModel("Kanban.md", fixtureBoard)
	cases := []struct {
		name string
		sel  *Selection
		want string
	}{
		{"no selection", nil,
			"Move Item off [Up off, Down off]\nMove to Column off [To Do off, In Progress off, Done off]"},
		{"the first card", &Selection{0, 0},
			"Move Item [Up off, Down]\nMove to Column [To Do off, In Progress, Done]"},
		{"a middle card", &Selection{0, 1},
			"Move Item [Up, Down]\nMove to Column [To Do off, In Progress, Done]"},
		{"the last card", &Selection{2, 1},
			"Move Item [Up, Down off]\nMove to Column [To Do, In Progress, Done off]"},
		{"a column's one card", &Selection{1, 0},
			"Move Item [Up off, Down off]\nMove to Column [To Do, In Progress off, Done]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := m
			if tc.sel != nil {
				m.Selected, m.HasSelected = *tc.sel, true
			}
			if got := shownCardMenu(m); got != tc.want {
				t.Errorf("the Card menu shows\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// The "Move to Column" submenu follows the file: a column added on disk
// adds an item in the file's order. A column whose heading has no text has
// no item, and the items after it keep their columns' indices.
func TestMoveToColumnListsTheFilesColumns(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m = apply(t, update, m, SelectCard{Selection{0, 0}})
	added := strings.Replace(fixtureBoard, "## Done\n", "## Later\n\n- [ ] Plan the trip\n\n## \n\n## Done\n", 1)
	m = apply(t, update, m, FileChanged{Data: added})
	if want := "Move Item [Up off, Down]\nMove to Column [To Do off, In Progress, Later, Done]"; shownCardMenu(m) != want {
		t.Fatalf("after a column is added the Card menu shows\n%s\nwant\n%s", shownCardMenu(m), want)
	}
	cols := cardMenu(m)[1].Items
	for i, want := range []int{0, 1, 2, 4} {
		if cols[i].Msg != (MoveToColumn{Column: want}) {
			t.Errorf("item %q posts %+v, want column %d", cols[i].Title, cols[i].Msg, want)
		}
	}
}

// menuMoveCase is one move: the card selected before it, the message, the file
// after it, and the place of the card after it.
type menuMoveCase struct {
	name string
	from Selection
	msg  mvu.Message
	file string
	to   Selection
}

var moveMenuCases = []menuMoveCase{
	{"up", Selection{0, 1}, MoveUp{},
		strings.Replace(fixtureBoard,
			"- [ ] Draft the release notes\n- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n",
			"- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n- [ ] Draft the release notes\n", 1),
		Selection{0, 0}},
	{"down", Selection{0, 1}, MoveDown{},
		strings.Replace(fixtureBoard,
			"- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n- [ ] Water the plants\n",
			"- [ ] Water the plants\n- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n", 1),
		Selection{0, 2}},
	{"to a column", Selection{0, 0}, MoveToColumn{Column: 1},
		"---\n\nkanban-plugin: board\n\n---\n\n" +
			"## To Do\n\n" +
			"- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n" +
			"- [ ] Water the plants\n\n\n" +
			"## In Progress\n\n" +
			"- [ ] Paint the hallway\n" +
			"- [ ] Draft the release notes\n\n\n" +
			"## Done\n\n" +
			"- [x] Buy oat milk\n" +
			"- [x] Renew the passport\n",
		Selection{1, 1}},
	{"to the last column", Selection{1, 0}, MoveToColumn{Column: 2},
		"---\n\nkanban-plugin: board\n\n---\n\n" +
			"## To Do\n\n" +
			"- [ ] Draft the release notes\n" +
			"- [ ] Book the dentist for a cleaning on a Thursday or a Friday morning\n" +
			"- [ ] Water the plants\n\n\n" +
			"## In Progress\n\n\n\n" +
			"## Done\n\n" +
			"- [x] Buy oat milk\n" +
			"- [x] Renew the passport\n" +
			"- [ ] Paint the hallway\n",
		Selection{2, 2}},
}

// Each item of the Card menu moves the selected card: Up and Down one
// place in its column, and Move to Column to the end of that column. The
// move is one "Move Card" step, the selection stays on the card, and the
// column that holds the card holds the keyboard. Undo puts the card back.
func TestTheCardMenuMovesTheSelectedCard(t *testing.T) {
	for _, tc := range moveMenuCases {
		t.Run(tc.name, func(t *testing.T) {
			path, f, m := boardFileWith(t, fixtureBoard)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			rig.selectAt(tc.from)
			title := titleAt(rig.model.Board, tc.from)
			rig.choose(tc.msg)
			if got := readFile(t, path); got != tc.file {
				t.Fatalf("the file after the move is\n%q\nwant\n%q", got, tc.file)
			}
			rig.selects(tc.to, title, "the move")
			if n := len(rig.model.Undo); n != 1 || rig.model.Undo[0].Name != stepMove {
				t.Fatalf("the undo stack is %d steps, want one %q step", n, stepMove)
			}
			rig.menuChord(false)
			if got := readFile(t, path); got != fixtureBoard {
				t.Errorf("Undo wrote\n%q\nwant the fixture", got)
			}
			rig.selects(tc.from, title, "Undo")
		})
	}
}

// Ctrl-Cmd-Up and Ctrl-Cmd-Down that reach the window move the selected
// card as the menu's Up and Down do. A key whose item is disabled reaches
// the window too, and then changes nothing: not the file, not the undo
// stack and not the selection.
func TestTheMoveKeysMoveTheSelectedCard(t *testing.T) {
	for _, tc := range moveMenuCases[:2] {
		t.Run(tc.name, func(t *testing.T) {
			path, f, m := boardFileWith(t, fixtureBoard)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			rig.selectAt(tc.from)
			title := titleAt(rig.model.Board, tc.from)
			name := key.Name(key.NameUpArrow)
			if tc.msg == (MoveDown{}) {
				name = key.NameDownArrow
			}
			rig.chord(name, moveModifiers)
			if got := readFile(t, path); got != tc.file {
				t.Fatalf("the file after the key is\n%q\nwant\n%q", got, tc.file)
			}
			rig.selects(tc.to, title, "the key")
			if n := len(rig.model.Undo); n != 1 || rig.model.Undo[0].Name != stepMove {
				t.Fatalf("the undo stack is %d steps, want one %q step", n, stepMove)
			}
		})
	}
	disabled := []struct {
		name string
		sel  *Selection
		key  key.Name
	}{
		{"up on the first card", &Selection{0, 0}, key.NameUpArrow},
		{"down on the last card", &Selection{0, 2}, key.NameDownArrow},
		{"up with no selection", nil, key.NameUpArrow},
		{"down with no selection", nil, key.NameDownArrow},
	}
	for _, tc := range disabled {
		t.Run(tc.name, func(t *testing.T) {
			path, f, m := boardFileWith(t, fixtureBoard)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			if tc.sel != nil {
				rig.selectAt(*tc.sel)
			}
			before := rig.model
			rig.chord(tc.key, moveModifiers)
			if got := readFile(t, path); got != fixtureBoard {
				t.Errorf("the key wrote\n%q\nwant the fixture", got)
			}
			if len(rig.model.Undo) != 0 || rig.model.Selected != before.Selected || rig.model.HasSelected != before.HasSelected {
				t.Errorf("the key changed the model: undo %d steps, selection %+v (%v)", len(rig.model.Undo), rig.model.Selected, rig.model.HasSelected)
			}
		})
	}
}

// Two moves that the update gets before the next model act on the
// selection as each finds it, because the menu lags the model.
func TestTwoMovesActOnTheSelectionAsTheyFindIt(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	update := newUpdate(f)
	m = apply(t, update, m, SelectCard{Selection{0, 2}}, MoveUp{}, MoveUp{}, MoveUp{})
	if m.Selected != (Selection{0, 0}) || titleAt(m.Board, m.Selected) != "Water the plants" {
		t.Fatalf("after three moves up the model selects %+v", m.Selected)
	}
	if n := len(m.Undo); n != 2 {
		t.Fatalf("the undo stack holds %d steps, want 2: the third move found the column's top", n)
	}
	m = apply(t, update, m, MoveToColumn{Column: 0}, MoveToColumn{Column: 7})
	if len(m.Undo) != 2 {
		t.Errorf("a move to the card's own column or to no column added a step")
	}
}

// selectAt selects card s with a press on it, so its column holds the
// keyboard.
func (rig *boardRig) selectAt(s Selection) {
	rig.t.Helper()
	rig.press(spots(rig).upper(s.Column, s.Card))
	if !rig.model.HasSelected || rig.model.Selected != s {
		rig.t.Fatalf("a press selects %+v (%v), want %+v", rig.model.Selected, rig.model.HasSelected, s)
	}
}

// choose chooses the item of the Card menu that posts msg, as a person
// does: only an enabled item can be chosen.
func (rig *boardRig) choose(msg mvu.Message) {
	rig.t.Helper()
	for _, it := range cardMenu(rig.model) {
		for _, sub := range it.Items {
			if sub.Msg != msg {
				continue
			}
			if it.Disabled || sub.Disabled {
				rig.t.Fatalf("the item that posts %+v is disabled", msg)
			}
			var cmd mvu.Command
			rig.model, cmd = rig.update(rig.model, msg)
			runCmd(rig.t, cmd)
			rig.build()
			rig.drive()
			return
		}
	}
	rig.t.Fatalf("the Card menu holds no item that posts %+v", msg)
}
