package main

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// A card added to a column goes after the column's last card and its
// notes, or after the blank line under the heading of a column without a
// card, and before the lines after the cards. The new line takes the
// file's line end, and a line before it without a line end takes one.
func TestAnAddedCardWritesOnlyItsLine(t *testing.T) {
	plugin, err := os.ReadFile("testdata/plugin.md")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, file string
		col        int
		want       string
	}{
		{"after the last card", string(plugin), 0,
			strings.Replace(string(plugin), "- [ ] Water the plants\n", "- [ ] Water the plants\n- [ ] New\n", 1)},
		{"after the last card's notes", string(plugin), 1,
			strings.Replace(string(plugin), "  Pages 4 to 9 still.\n", "  Pages 4 to 9 still.\n- [ ] New\n", 1)},
		{"before the settings block", string(plugin), 3,
			strings.Replace(string(plugin), "- [x] Return the library books\n", "- [x] Return the library books\n- [ ] New\n", 1)},
		{"empty column", newBoard, 0,
			strings.Replace(newBoard, "## To Do\n\n", "## To Do\n\n- [ ] New\n", 1)},
		{"empty last column before the settings block", newBoard, 2,
			strings.Replace(newBoard, "## Done\n\n", "## Done\n\n- [ ] New\n", 1)},
		{"crlf", "## A\r\n\r\n- [ ] one\r\n\r\n## B\r\n", 0,
			"## A\r\n\r\n- [ ] one\r\n- [ ] New\r\n\r\n## B\r\n"},
		{"crlf empty column", "## A\r\n\r\n- [ ] one\r\n## B\r\n\r\n", 1,
			"## A\r\n\r\n- [ ] one\r\n## B\r\n\r\n- [ ] New\r\n"},
		{"last card without line end", "## A\n\n- [ ] one", 0,
			"## A\n\n- [ ] one\n- [ ] New\n"},
		{"crlf last card without line end", "## A\r\n\r\n- [ ] one", 0,
			"## A\r\n\r\n- [ ] one\r\n- [ ] New\r\n"},
		{"last note without line end", "## A\n- [ ] one\n  note", 0,
			"## A\n- [ ] one\n  note\n- [ ] New\n"},
		{"heading without line end", "## A\n- [ ] one\n## B", 1,
			"## A\n- [ ] one\n## B\n- [ ] New\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.want == tc.file {
				t.Fatal("the fixture does not hold the line this case adds after")
			}
			_, f, _ := fixture(t)
			m := newModel("Board.md", tc.file)
			update := newUpdate(f)
			m, _ = update(m, OpenAdd{Column: tc.col})
			next, cmd := update(m, AddCard{Title: "  New "})
			runCmd(t, cmd)
			if next.Text != tc.want {
				t.Errorf("the file after the add is\n%q\nwant\n%q", next.Text, tc.want)
			}
			if Format(m.Board) != tc.file {
				t.Error("the add changed the model it was given")
			}
			cards := Parse(next.Text).Columns[tc.col].Cards()
			if len(cards) == 0 || cards[len(cards)-1].Title != "New" {
				t.Errorf("the file read again ends column %d with %+v, want the new card", tc.col, cards)
			}
			if !next.IsAdding || next.Adding != tc.col {
				t.Error("the add closed the field")
			}
		})
	}
}

// A title that is empty or only spaces adds nothing, and an add with no
// field open adds nothing.
func TestAnEmptyTitleAddsNothing(t *testing.T) {
	_, f, m := fixture(t)
	update := newUpdate(f)
	if next, _ := update(m, AddCard{Title: "Water"}); next.Text != m.Text {
		t.Error("an add with no field open changed the board")
	}
	m, _ = update(m, OpenAdd{Column: 0})
	for _, title := range []string{"", "   ", "\t"} {
		if next, _ := update(m, AddCard{Title: title}); next.Text != m.Text || next.Seq != m.Seq {
			t.Errorf("the title %q added a card", title)
		}
	}
}

// Each opening of the field counts up, Escape's CloseAdd closes it, and a
// change on disk that removes the field's column closes it.
func TestTheFieldOpensAndCloses(t *testing.T) {
	_, f, m := fixture(t)
	update := newUpdate(f)
	m, _ = update(m, OpenAdd{Column: 3})
	if !m.IsAdding || m.Adding != 3 || m.Opened != 1 {
		t.Fatalf("after OpenAdd the model is adding %v in column %d, opened %d", m.IsAdding, m.Adding, m.Opened)
	}
	if closed, _ := update(m, CloseAdd{}); closed.IsAdding {
		t.Error("CloseAdd left the field open")
	}
	if again, _ := update(m, OpenAdd{Column: 0}); again.Opened != 2 {
		t.Errorf("a second opening counts %d, want 2", again.Opened)
	}
	if gone, _ := update(m, FileChanged{Data: "## A\n"}); gone.IsAdding {
		t.Error("the field stays open in a column that the file no longer has")
	}
}

// A toggle changes only the character in the card's box, and a second
// toggle gives back the file, with a lower-case "x" in a completed box.
func TestAToggleWritesOnlyTheBox(t *testing.T) {
	cases := []struct {
		name, file, want, back string
		sel                    Selection
	}{
		{"complete", "## A\n- [ ] one\n\tnote\n- [ ] two\n", "## A\n- [ ] one\n\tnote\n- [x] two\n", "", Selection{0, 1}},
		{"not complete", "## A\r\n* [X] one\r\n", "## A\r\n* [ ] one\r\n", "## A\r\n* [x] one\r\n", Selection{0, 0}},
		{"no line end", "## A\n- [x] one", "## A\n- [ ] one", "", Selection{0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, f, _ := fixture(t)
			m := newModel("Board.md", tc.file)
			update := newUpdate(f)
			next, cmd := update(m, ToggleCard{tc.sel})
			runCmd(t, cmd)
			if next.Text != tc.want {
				t.Errorf("the file after the toggle is %q, want %q", next.Text, tc.want)
			}
			if Format(m.Board) != tc.file {
				t.Error("the toggle changed the model it was given")
			}
			back := tc.back
			if back == "" {
				back = tc.file
			}
			if again, _ := update(next, ToggleCard{tc.sel}); again.Text != back {
				t.Errorf("a second toggle gives %q, want %q", again.Text, back)
			}
		})
	}
}

// boardFileWith writes data to a new file and opens it.
func boardFileWith(t *testing.T, data string) (string, *boardFile, Model) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Kanban.md")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	text, seen, err := openBoard(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, newBoardFile(path, seen), newModel(filepath.Base(path), text)
}

// addButtonAt is a point on the "Add a card" button of column col when the
// column has no card: the button stands where the first card would.
func addButtonAt(col int) image.Point {
	p := firstCard(col)
	return image.Pt(int(boardInset)+col*int(columnWidth+columnGap)+int(tokens.Spacing.S4)+20, p.Y)
}

// boxAt is a point on the checkbox of the first card of column col: the
// card's inset in from its leading edge, beside the title's first line.
func boxAt(col int) image.Point {
	p := firstCard(col)
	x := int(boardInset) + col*int(columnWidth+columnGap) + 2*int(tokens.Spacing.S4) + 11
	return image.Pt(x, p.Y-8+int(tokens.Spacing.S4)+8)
}

// typeText gives text to the field that holds the keyboard and puts the
// caret after it, as typing does.
func (rig *boardRig) typeText(text string) {
	rig.r.Queue(key.EditEvent{Text: text})
	rig.drive()
	rig.key(key.NameEnd)
}

// open opens the field in column col, as the column's button does.
func (rig *boardRig) open(col int) {
	rig.model, _ = rig.update(rig.model, OpenAdd{Column: col})
	rig.build()
	rig.drive()
}

// readFile is the file at path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A press on "Add a card" opens the field, and the field takes the
// keyboard. Return adds the card to the file and keeps the field open and
// empty for the next card. Escape closes the field, and the next opening
// gives an empty field.
func TestTheFieldAddsCardsUntilEscape(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureEmptyColumn)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(addButtonAt(1))
	if !rig.model.IsAdding || rig.model.Adding != 1 {
		t.Fatalf("the press opened no field in column 1: %v, %d", rig.model.IsAdding, rig.model.Adding)
	}
	if !rig.r.Source().Focused(rig.field.state.tag) {
		t.Fatal("the field does not hold the keyboard")
	}
	rig.typeText("Call the plumber")
	rig.key(key.NameReturn)
	rig.typeText("Fix the tap")
	rig.key(key.NameReturn)
	rig.key(key.NameReturn)
	want := strings.Replace(fixtureEmptyColumn, "## In Progress\n\n", "## In Progress\n\n- [ ] Call the plumber\n- [ ] Fix the tap\n", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the file after two adds is\n%q\nwant\n%q", got, want)
	}
	if !rig.model.IsAdding || !rig.r.Source().Focused(rig.field.state.tag) {
		t.Error("Return closed the field or took its keyboard")
	}
	rig.key(key.NameEscape)
	if rig.model.IsAdding {
		t.Fatal("Escape left the field open")
	}
	rig.open(0)
	rig.typeText("Sweep")
	rig.key(key.NameReturn)
	cards := rig.model.Board.Columns[0].Cards()
	if got := cards[len(cards)-1].Title; got != "Sweep" {
		t.Errorf("the field opened again adds %q, want an empty field's \"Sweep\"", got)
	}
}

// With the field open, Down and End move the selection to the column's
// last card and no further: the field is not a row of the list.
func TestDownAndEndStopAtTheLastCard(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureBoard)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.open(0)
	rig.press(firstCard(0))
	last := len(rig.model.Board.Columns[0].Cards()) - 1
	for _, name := range []key.Name{key.NameEnd, key.NameDownArrow, key.NameDownArrow} {
		rig.key(name)
		if got := rig.state.column(0).Selected(); got != last {
			t.Errorf("after %s the list selects %d, want the last card %d", name, got, last)
		}
	}
}

// A press on a card's checkbox marks the task completed in the file, and a
// second press marks it not completed.
func TestTheCheckboxTogglesTheTask(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureBoard)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(boxAt(0))
	want := strings.Replace(fixtureBoard, "- [ ] Draft", "- [x] Draft", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the file after the press is\n%q\nwant\n%q", got, want)
	}
	rig.press(boxAt(0))
	if got := readFile(t, path); got != fixtureBoard {
		t.Errorf("the file after the second press is\n%q\nwant\n%q", got, fixtureBoard)
	}
}

// TestFieldGolden records or diffs the board with the field open in the
// empty column, with a title being typed, in both schemes.
func TestFieldGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			fieldPage := func(radius tokens.RadiusScale) layout.Widget {
				_, f, m := boardFileWith(t, fixtureEmptyColumn)
				rig := newBoardRig(t, staticThemed(tc.c, radius), f, m)
				rig.press(addButtonAt(1))
				rig.typeText("Call the plumber")
				return golden.Stack(rig.capture(), Toolbar(rig.th, m.Name, func() unit.Dp { return shell.ToolbarDepth }))
			}
			golden.Render(t, tc.name+"-field", windowSize, fieldPage(sharpRadius))
			if *boardDump != "" {
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-field.png"), golden.Capture(t, windowSize, fieldPage(tokens.Radius)))
			}
		})
	}
}

// staticTheme is the theme.Theme of p with the stored renders' radius.
func staticTheme(p tokens.PlatformColors) theme.Theme {
	return staticThemed(p, sharpRadius).components
}

// The content layer keeps the field's text through a change on disk and
// through a change of scheme: Return after both adds the whole title.
func TestTheFieldKeepsItsTextThroughAFileChangeAndAScheme(t *testing.T) {
	_, f, m := boardFileWith(t, fixtureEmptyColumn)
	update := newUpdate(f)
	m, _ = update(m, OpenAdd{Column: 1})

	themes, themeObs := rx.Subject[theme.Theme](0, 1)
	models, modelObs := rx.Subject[Model](0, 1)
	var mu sync.Mutex
	var page layout.Widget
	var posted []mvu.Message
	built := make(chan struct{}, 64)
	post := func(_ layout.Context, msg mvu.Message) { posted = append(posted, msg) }
	sub := contentLayer(themeObs, modelObs, desktop.ScrollBarsWhenScrolling, post).
		Tap(func(w layout.Widget, err error, done bool) {
			if !done {
				mu.Lock()
				page = w
				mu.Unlock()
				built <- struct{}{}
			}
		}).Go()
	defer sub.Unsubscribe()
	// settle waits for a page and then for the layer to stop building
	// pages, and gives the last page.
	settle := func() layout.Widget {
		t.Helper()
		select {
		case <-built:
		case <-time.After(5 * time.Second):
			t.Fatal("the content layer built no page")
		}
		for {
			select {
			case <-built:
			case <-time.After(200 * time.Millisecond):
				mu.Lock()
				defer mu.Unlock()
				return page
			}
		}
	}
	r := new(gioinput.Router)
	ops := new(op.Ops)
	frame := func(w layout.Widget) {
		ops.Reset()
		w(layout.Context{Constraints: layout.Exact(windowSize), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Ops: ops, Source: r.Source()})
		r.Frame(ops)
	}

	themes.Next(staticTheme(tokens.PlatformLight))
	models.Next(m)
	p := settle()
	frame(p)
	r.Queue(key.EditEvent{Text: "Call the plumber"})
	frame(p)

	m, _ = update(m, FileChanged{Data: fixtureBoard})
	models.Next(m)
	p = settle()
	frame(p)
	themes.Next(staticTheme(tokens.PlatformDark))
	p = settle()
	frame(p)

	r.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	frame(p)
	if len(posted) != 1 || posted[0] != (AddCard{Title: "Call the plumber"}) {
		t.Errorf("Return after the change on disk and the scheme posted %v, want the typed title", posted)
	}
}
