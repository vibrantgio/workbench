package main

import (
	"image"
	"image/color"
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
			if !next.IsAdding || next.Adding != tc.col || next.Opened != m.Opened+1 {
				t.Error("the add opened no next new card in its column")
			}
		})
	}
}

// A title that is empty or only spaces adds nothing, and an add with no
// new card open adds nothing.
func TestAnEmptyTitleAddsNothing(t *testing.T) {
	_, f, m := fixture(t)
	update := newUpdate(f)
	if next, _ := update(m, AddCard{Title: "Water"}); next.Text != m.Text {
		t.Error("an add with no new card open changed the board")
	}
	m, _ = update(m, OpenAdd{Column: 0})
	for _, title := range []string{"", "   ", "\t"} {
		if next, _ := update(m, AddCard{Title: title}); next.Text != m.Text || next.Seq != m.Seq || next.Opened != m.Opened {
			t.Errorf("the title %q added a card", title)
		}
	}
}

// Each opening of a new card counts up, Escape's CloseAdd removes it, and a
// change on disk that removes the new card's column removes it.
func TestTheNewCardOpensAndCloses(t *testing.T) {
	_, f, m := fixture(t)
	update := newUpdate(f)
	m, _ = update(m, OpenAdd{Column: 3})
	if !m.IsAdding || m.Adding != 3 || m.Opened != 1 {
		t.Fatalf("after OpenAdd the model is adding %v in column %d, opened %d", m.IsAdding, m.Adding, m.Opened)
	}
	if closed, _ := update(m, CloseAdd{}); closed.IsAdding {
		t.Error("CloseAdd left the new card open")
	}
	if again, _ := update(m, OpenAdd{Column: 0}); again.Opened != 2 {
		t.Errorf("a second opening counts %d, want 2", again.Opened)
	}
	if gone, _ := update(m, FileChanged{Data: "## A\n"}); gone.IsAdding {
		t.Error("the new card stays open in a column that the file no longer has")
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

// addButtonAt is a point on the "Add Card" button of column col when the
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

// open opens a new card in column col, as the column's button does.
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

// A press on "Add Card" opens a new card, and its field takes the keyboard.
// Return adds the card to the file and opens the next new card, empty and
// with the keyboard. Escape removes the new card, and the next opening
// gives an empty field.
func TestTheNewCardAddsCardsUntilEscape(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureEmptyColumn)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(addButtonAt(1))
	if !rig.model.IsAdding || rig.model.Adding != 1 {
		t.Fatalf("the press opened no new card in column 1: %v, %d", rig.model.IsAdding, rig.model.Adding)
	}
	if !rig.r.Source().Focused(rig.fresh.state.tag) {
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
	if !rig.model.IsAdding || !rig.r.Source().Focused(rig.fresh.state.tag) {
		t.Error("Return opened no next new card or left it without the keyboard")
	}
	rig.key(key.NameEscape)
	if rig.model.IsAdding {
		t.Fatal("Escape left the new card open")
	}
	rig.open(0)
	rig.typeText("Sweep")
	rig.key(key.NameReturn)
	cards := rig.model.Board.Columns[0].Cards()
	if got := cards[len(cards)-1].Title; got != "Sweep" {
		t.Errorf("the new card opened again adds %q, want an empty field's \"Sweep\"", got)
	}
}

// With a new card open, Down and End move the selection to the column's
// last card and no further: the new card is not a row of the list.
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

// TestNewCardGolden records or diffs the board with a new card open in the
// empty column, its title empty, in both schemes. With -board.dump it also
// writes the new card with a title typed, and the board after Return.
func TestNewCardGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			page := func(radius tokens.RadiusScale) layout.Widget {
				_, f, m := boardFileWith(t, fixtureEmptyColumn)
				rig := newBoardRig(t, staticThemed(tc.c, radius), f, m)
				rig.press(addButtonAt(1))
				return golden.Stack(rig.capture(), Toolbar(rig.th, m.Name, func() unit.Dp { return shell.ToolbarDepth }))
			}
			golden.Render(t, tc.name+"-new-card", windowSize, page(sharpRadius))
			if *boardDump != "" {
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-new-card.png"), golden.Capture(t, windowSize, page(tokens.Radius)))
				_, f, m := boardFileWith(t, fixtureEmptyColumn)
				rig := newBoardRig(t, staticThemed(tc.c, tokens.Radius), f, m)
				rig.press(addButtonAt(1))
				rig.typeText("Call the plumber")
				bar := Toolbar(rig.th, m.Name, func() unit.Dp { return shell.ToolbarDepth })
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-new-card-typed.png"), golden.Capture(t, windowSize, golden.Stack(rig.capture(), bar)))
				rig.key(key.NameReturn)
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-new-card-added.png"), golden.Capture(t, windowSize, golden.Stack(rig.capture(), bar)))
			}
		})
	}
}

// cardRows is the rows of card n of column col in img, and the card's
// fill. It reads them down a line of pixels inside the cards' trailing
// inset, which no title reaches.
func cardRows(img *image.RGBA, th themed, col, n int) (top, bottom int, fill color.RGBA) {
	x := int(boardInset) + col*int(columnWidth+columnGap) + int(columnWidth) - 3*int(th.spacing.S4)/2
	fill = img.RGBAAt(x, firstCard(col).Y)
	run, in := -1, false
	for y := firstCard(col).Y - 12; y < img.Bounds().Max.Y; y++ {
		on := img.RGBAAt(x, y) == fill
		switch {
		case on && !in:
			run++
			if run == n {
				top = y
			}
		case !on && in && run == n:
			return top, y, fill
		}
		in = on
	}
	return top, top, fill
}

// titleArea is the part of the card in rows top to bottom of column col
// that holds the title: the columns from the title's leading edge to the
// card's inset at its trailing edge.
func titleArea(img *image.RGBA, th themed, col, top, bottom int, fill color.RGBA) image.Rectangle {
	colX := int(boardInset) + col*int(columnWidth+columnGap)
	mid := (top + bottom) / 2
	left, right := colX, colX+int(columnWidth)
	for left < right && img.RGBAAt(left, mid) != fill {
		left++
	}
	for right > left && img.RGBAAt(right-1, mid) != fill {
		right--
	}
	pad := int(th.spacing.S4)
	row := int(max(th.density.CheckboxRowHeight, checkboxBox))
	return image.Rect(left+pad+row+int(th.spacing.S2), top, right-pad, bottom)
}

// drawnIn is the box of the pixels in r that are not fill, leaving out the
// columns of skip.
func drawnIn(img *image.RGBA, fill color.RGBA, r, skip image.Rectangle) image.Rectangle {
	var box image.Rectangle
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if x >= skip.Min.X && x < skip.Max.X {
				continue
			}
			if img.RGBAAt(x, y) != fill {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// The title's text stands at the same pixels while it is typed in the new
// card and after Return, when the card is an ordinary card. Inside the
// card, the two renders differ only in the caret. The cases are the first
// card of an empty column, a card after the cards of a column, and a title
// that wraps.
func TestTheTitleStandsStillOnReturn(t *testing.T) {
	cases := []struct {
		name       string
		col, lines int
		title      string
	}{
		{"empty column", 1, 1, "Call the plumber"},
		{"after the cards", 0, 1, "Call the plumber"},
		{"wrapped", 1, 2, "Call the plumber about the tap in the bathroom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, f, m := boardFileWith(t, fixtureEmptyColumn)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			n := len(rig.model.Board.Columns[tc.col].Cards())
			rig.open(tc.col)
			rig.typeText(tc.title)
			typed := golden.Capture(t, windowSize, rig.capture())
			rig.key(key.NameReturn)
			cards := rig.model.Board.Columns[tc.col].Cards()
			if len(cards) != n+1 || cards[n].Title != tc.title {
				t.Fatalf("Return added %d cards, want the title %q as card %d", len(cards)-n, tc.title, n)
			}
			after := golden.Capture(t, windowSize, rig.capture())
			if typed == nil || after == nil {
				return
			}
			top, bottom, fill := cardRows(after, rig.th, tc.col, n)
			if lines := (bottom - top - 2*int(rig.th.spacing.S4)) / int(rig.th.typ.Body.LineHeight); lines != tc.lines {
				t.Fatalf("the added card stands in rows %d to %d, %d lines, want %d", top, bottom, lines, tc.lines)
			}
			area := titleArea(after, rig.th, tc.col, top, bottom, fill)
			var caret image.Rectangle
			for y := area.Min.Y; y < area.Max.Y; y++ {
				for x := area.Min.X; x < area.Max.X; x++ {
					if typed.RGBAAt(x, y) != after.RGBAAt(x, y) {
						caret = caret.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			if caret.Dx() > 3 {
				t.Errorf("Return changed the pixels inside %v; only the caret may change", caret)
			}
			was, is := drawnIn(typed, fill, area, caret), drawnIn(after, fill, area, caret)
			if is.Empty() {
				t.Fatal("the added card draws no title; this measures nothing")
			}
			t.Logf("in %v the text stands at %v, and the caret at %v", area, is, caret)
			if was != is {
				t.Errorf("the text stands at %v while it is typed and at %v after Return", was, is)
			}
		})
	}
}

// Return on an empty title adds nothing and keeps the new card open, with
// the keyboard.
func TestReturnOnAnEmptyTitleAddsNothing(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureEmptyColumn)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.open(1)
	opened := rig.model.Opened
	rig.typeText("  ")
	rig.key(key.NameReturn)
	if got := readFile(t, path); got != fixtureEmptyColumn {
		t.Errorf("Return on an empty title wrote\n%q", got)
	}
	if !rig.model.IsAdding || rig.model.Opened != opened || !rig.r.Source().Focused(rig.fresh.state.tag) {
		t.Error("Return on an empty title closed the new card or took its keyboard")
	}
}

// Escape removes the new card and discards the text typed in it. The file
// does not change.
func TestEscapeRemovesTheNewCard(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureEmptyColumn)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.open(1)
	rig.typeText("Call the plumber")
	rig.key(key.NameEscape)
	if rig.model.IsAdding {
		t.Fatal("Escape left the new card open")
	}
	if got := readFile(t, path); got != fixtureEmptyColumn {
		t.Errorf("Escape wrote\n%q", got)
	}
	rig.open(1)
	rig.key(key.NameReturn)
	if got := readFile(t, path); got != fixtureEmptyColumn {
		t.Errorf("Return after Escape and a new opening added the discarded text:\n%q", got)
	}
}

// A line break in the title becomes a space, so the card is one line of
// the file.
func TestALineBreakInTheTitleBecomesASpace(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureEmptyColumn)
	update := newUpdate(f)
	m, _ = update(m, OpenAdd{Column: 1})
	m, cmd := update(m, AddCard{Title: "Call\nthe\r\nplumber\r"})
	runCmd(t, cmd)
	want := strings.Replace(fixtureEmptyColumn, "## In Progress\n\n", "## In Progress\n\n- [ ] Call the plumber\n", 1)
	if got := readFile(t, path); got != want {
		t.Errorf("the file is\n%q\nwant\n%q", got, want)
	}
}

// The new card's checkbox does not operate: a press on it posts nothing,
// and the new card stays open.
func TestTheNewCardsCheckboxDoesNotOperate(t *testing.T) {
	path, f, m := boardFileWith(t, fixtureEmptyColumn)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.open(1)
	from := len(rig.posted)
	rig.press(boxAt(1))
	if len(rig.posted) != from {
		t.Errorf("a press on the new card's checkbox posted %v", rig.posted[from:])
	}
	if got := readFile(t, path); got != fixtureEmptyColumn {
		t.Errorf("a press on the new card's checkbox wrote\n%q", got)
	}
	if !rig.model.IsAdding {
		t.Error("a press on the new card's checkbox closed it")
	}
}

// staticTheme is the theme.Theme of p with the stored renders' radius.
func staticTheme(p tokens.PlatformColors) theme.Theme {
	return staticThemed(p, sharpRadius).components
}

// The content layer keeps the new card's text through a change on disk and
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

// The list scrolls to its end when a new card opens and after each add. A
// column whose cards fit keeps its first card at the top.
func TestTheListScrollsToItsEndForTheNewCard(t *testing.T) {
	var b strings.Builder
	b.WriteString("## To Do\n\n")
	for range 30 {
		b.WriteString("- [ ] Water the plants\n")
	}
	b.WriteString("## Done\n\n- [x] Buy oat milk\n")
	_, f, m := boardFileWith(t, b.String())
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	long := rig.state.column(0)
	if long.Position().BeforeEnd == false {
		t.Fatal("thirty cards fit the column; the test needs a list that scrolls")
	}
	rig.open(0)
	if long.Position().BeforeEnd {
		t.Errorf("after a new card opens the list is at %+v, want its end", long.Position())
	}
	long.ScrollToStart()
	rig.drive()
	rig.typeText("Sweep")
	rig.key(key.NameReturn)
	if long.Position().BeforeEnd {
		t.Errorf("after an add the list is at %+v, want its end", long.Position())
	}

	rig.open(1)
	rig.typeText("Renew the passport")
	rig.key(key.NameReturn)
	if p := rig.state.column(1).Position(); p.First != 0 || p.Offset != 0 {
		t.Errorf("after an add to a column whose cards fit the list is at %+v, want its start", p)
	}
}
