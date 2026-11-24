package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/golden"
	complist "github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// boardRig drives the board's page through one router, as the window does.
// The messages the page posts go through the update, and the page is built
// again for each new model, as the window's model stream builds it.
type boardRig struct {
	t      *testing.T
	r      *gioinput.Router
	ops    *op.Ops
	th     themed
	state  *scrollState
	update func(Model, mvu.Message) (Model, mvu.Command)
	model  Model
	// fresh is the new card, made again when the
	// model's Opened count changes, as ContentLayer makes it.
	fresh newCard
	// edit is the edit field, made again when the model's Edits count
	// changes, as ContentLayer makes it.
	edit editCard
	page layout.Widget
	// now is the time of the rig's next pointer event. Each press comes a
	// second after the one before, so two presses are no double-click.
	now time.Duration
	// posted is every message the page posted, in order.
	posted []mvu.Message
}

func newBoardRig(t *testing.T, th themed, f *boardFile, m Model) *boardRig {
	rig := &boardRig{t: t, r: new(gioinput.Router), ops: new(op.Ops), th: th,
		state: newScrollState(), update: newUpdate(f), model: m}
	rig.build()
	rig.drive()
	return rig
}

// build makes the page for the rig's model.
func (rig *boardRig) build() {
	post := func(_ layout.Context, msg mvu.Message) { rig.posted = append(rig.posted, msg) }
	if rig.fresh.field == nil || rig.fresh.opened != rig.model.Opened {
		rig.fresh, _ = openCard(rx.Of(rig.th.components), rig.model.Opened, post).First()
	}
	if m := rig.model; m.IsEditing && rig.edit.edit != m.Edits {
		title := m.Board.Columns[m.Editing.Column].Cards()[m.Editing.Card].Title
		rig.edit, _ = openEdit(rx.Of(rig.th.components), m.Edits, m.Editing, title, post).First()
	}
	board := desktop.InsetTop(func() unit.Dp { return shell.ToolbarDepth }, boardView(rig.th, rig.model, rig.fresh, rig.edit, rig.state, post))
	fill := rig.th.platform.ControlBackground
	rig.page = golden.On(fill, golden.Stack(backdrop.Widget(fill), board))
}

// drive lays the page out once, then gives each message it posted to the
// update and builds the page again for the new model.
func (rig *boardRig) drive() {
	rig.ops.Reset()
	from := len(rig.posted)
	rig.page(layout.Context{
		Constraints: layout.Exact(windowSize),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         rig.ops,
		Source:      rig.r.Source(),
	})
	rig.r.Frame(rig.ops)
	if len(rig.posted) == from {
		return
	}
	for _, msg := range rig.posted[from:] {
		var cmd mvu.Command
		rig.model, cmd = rig.update(rig.model, msg)
		runCmd(rig.t, cmd)
	}
	rig.build()
	rig.drive()
}

func (rig *boardRig) press(at image.Point) {
	rig.pressAfter(at, time.Second)
}

// pressAfter presses at at, after the time gap since the rig's last press.
func (rig *boardRig) pressAfter(at image.Point, gap time.Duration) {
	rig.now += gap
	pos := f32.Pt(float32(at.X), float32(at.Y))
	rig.r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos, Time: rig.now},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos, Time: rig.now + 50*time.Millisecond},
	)
	rig.drive()
	// A press can move the keyboard without a message. The next frame
	// shows what the move did, as the window draws a frame after it.
	rig.drive()
}

// doubleClick presses twice at at, as a double-click does.
func (rig *boardRig) doubleClick(at image.Point) {
	rig.press(at)
	rig.pressAfter(at, 100*time.Millisecond)
}

func (rig *boardRig) key(name key.Name) {
	rig.r.Queue(key.Event{Name: name, State: key.Press})
	rig.drive()
}

// capture is the page as the rig's router has it, keyboard focus included.
func (rig *boardRig) capture() layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Source = rig.r.Source()
		return rig.page(gtx)
	}
}

// firstCard is a point on the first card of column col: under the room
// the focus ring keeps, the group's inset, its title and the gap under the
// title.
func firstCard(col int) image.Point {
	x := int(boardInset) + col*int(columnWidth+columnGap) + int(columnWidth)/2
	y := int(shell.ToolbarDepth) + int(complist.RingWidth) + int(tokens.Spacing.S4) +
		int(tokens.DefaultTypography.SectionHeading.LineHeight) + int(tokens.Spacing.S3) + 8
	return image.Pt(x, y)
}

// A press on a card selects it and gives its column the keyboard. Down then
// moves the selection to the next card in the column, and the model follows
// the list.
func TestAPressSelectsACardAndDownMovesTheSelection(t *testing.T) {
	_, f, m := fixture(t)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(firstCard(0))
	if !rig.model.HasSelected || rig.model.Selected != (Selection{0, 0}) {
		t.Fatalf("after the press the model selects %+v (%v), want column 0, card 0", rig.model.Selected, rig.model.HasSelected)
	}
	if !rig.r.Source().Focused(rig.state.column(0).Focus()) {
		t.Fatal("the press did not give the column's list the keyboard")
	}
	rig.key(key.NameDownArrow)
	if rig.model.Selected != (Selection{0, 1}) {
		t.Fatalf("after Down the model selects %+v, want column 0, card 1", rig.model.Selected)
	}
	rig.press(firstCard(1))
	if rig.model.Selected != (Selection{1, 0}) {
		t.Fatalf("after a press in the second column the model selects %+v, want column 1, card 0", rig.model.Selected)
	}
	if got := rig.state.column(0).Selected(); got != -1 {
		t.Errorf("the first column's list still selects card %d; the board has one selection", got)
	}
	if !rig.r.Source().Focused(rig.state.column(1).Focus()) {
		t.Error("the press did not give the second column's list the keyboard")
	}
}

// Delete and Backspace each delete the selected card and write the file:
// the file loses the card's line and its notes, and keeps every other
// line. After a delete, no card is selected.
func TestDeleteAndBackspaceDeleteTheSelectedCard(t *testing.T) {
	for _, name := range deleteKeys {
		t.Run(string(name), func(t *testing.T) {
			path, f, m := fixture(t)
			rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
			rig.press(firstCard(0))
			rig.key(key.NameDownArrow)
			rig.key(name)
			if rig.model.HasSelected {
				t.Errorf("after the delete the model selects %+v, want no card", rig.model.Selected)
			}
			if got := rig.state.column(0).Selected(); got != -1 {
				t.Errorf("after the delete the list selects card %d, want none", got)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			plugin, _ := os.ReadFile("testdata/plugin.md")
			want := strings.Replace(string(plugin),
				"- [ ] Book the dentist\n\tThursday or Friday morning.\n\tAsk about the cleaning.\n", "", 1)
			if want == string(plugin) {
				t.Fatal("the fixture does not hold the card this test deletes")
			}
			if string(data) != want {
				t.Errorf("the file after the delete is\n%q\nwant\n%q", data, want)
			}
		})
	}
}

// The delete keys do nothing while no card is selected: after a delete,
// a second Delete leaves the board and the file as they are.
func TestDeleteWithNoSelectionDoesNothing(t *testing.T) {
	path, f, m := fixture(t)
	rig := newBoardRig(t, staticThemed(tokens.PlatformLight, sharpRadius), f, m)
	rig.press(firstCard(0))
	rig.key(key.NameDeleteBackward)
	after, _ := os.ReadFile(path)
	n := len(rig.model.Board.Columns[0].Cards())
	rig.key(key.NameDeleteBackward)
	if got := len(rig.model.Board.Columns[0].Cards()); got != n {
		t.Errorf("a Delete with no selection left %d cards, want %d", got, n)
	}
	if again, _ := os.ReadFile(path); string(again) != string(after) {
		t.Error("a Delete with no selection wrote the file")
	}
}

// A delete removes exactly the card's line and its notes, with the line
// ends as the file has them: a CRLF file, and a last card with no line
// end.
func TestADeleteWritesOnlyTheCardsLines(t *testing.T) {
	cases := []struct {
		name, file, want string
		sel              Selection
	}{
		{"notes",
			"## A\n\n- [ ] one\n\tnote\n\n- [x] two\n## B\n- [ ] three\n",
			"## A\n\n\n- [x] two\n## B\n- [ ] three\n", Selection{0, 0}},
		{"crlf",
			"## A\r\n\r\n- [ ] one\r\n- [ ] two\r\n\r\n## B\r\n",
			"## A\r\n\r\n- [ ] one\r\n\r\n## B\r\n", Selection{0, 1}},
		{"no line end",
			"## A\n- [ ] one\n- [ ] two",
			"## A\n- [ ] one\n", Selection{0, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, f, _ := fixture(t)
			m := newModel("Board.md", tc.file)
			m.Selected, m.HasSelected = tc.sel, true
			next, cmd := newUpdate(f)(m, DeleteCard{tc.sel})
			runCmd(t, cmd)
			if next.Text != tc.want {
				t.Errorf("the file after the delete is %q, want %q", next.Text, tc.want)
			}
			if Format(m.Board) != tc.file {
				t.Error("the delete changed the model it was given")
			}
		})
	}
}

// A file whose last card has no line end still ends without one after the
// delete of the card before it.
func TestADeleteOfTheCardBeforeALastLineWithoutEnd(t *testing.T) {
	m := newModel("Board.md", "## A\n- [ ] one\n- [ ] two")
	next := withoutCard(m.Board, Selection{0, 0})
	if got := Format(next); got != "## A\n- [ ] two" {
		t.Errorf("the board after the delete formats to %q", got)
	}
}

// A change on disk that removes the selected card clears the selection; a
// change that keeps it keeps the selection.
func TestAChangeOnDiskClearsASelectionThatNoLongerExists(t *testing.T) {
	_, f, m := fixture(t)
	up := newUpdate(f)
	m, _ = up(m, SelectCard{Selection{0, 2}})
	if !m.HasSelected {
		t.Fatal("SelectCard did not select the card")
	}
	kept, _ := up(m, FileChanged{Data: m.Text + "\n"})
	if !kept.HasSelected || kept.Selected != (Selection{0, 2}) {
		t.Errorf("a change that keeps the card cleared the selection: %+v %v", kept.Selected, kept.HasSelected)
	}
	gone, _ := up(m, FileChanged{Data: "## To Do\n- [ ] one\n"})
	if gone.HasSelected {
		t.Errorf("a change that removes the card keeps the selection %+v", gone.Selected)
	}
	if none, _ := up(m, SelectCard{Selection{0, 9}}); none.Selected != m.Selected {
		t.Errorf("SelectCard of a card that does not exist selects %+v", none.Selected)
	}
}

// TestSelectedCardGolden records or diffs the board with a selected card
// in a column that holds the keyboard, in both schemes. The test reads the
// selected card's fill, and the focus ring above the column's group.
func TestSelectedCardGolden(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel("Kanban.md", fixtureBoard)
			_, f, _ := fixture(t)
			selectedPage := func(radius tokens.RadiusScale) layout.Widget {
				rig := newBoardRig(t, staticThemed(tc.c, radius), f, m)
				rig.press(firstCard(0))
				rig.key(key.NameDownArrow)
				return golden.Stack(rig.capture(), Toolbar(rig.th, m.Name, func() unit.Dp { return shell.ToolbarDepth }))
			}
			page := selectedPage(sharpRadius)
			golden.Render(t, tc.name+"-selected", windowSize, page)
			if *boardDump != "" {
				dumpPNG(t, filepath.Join(*boardDump, "kanban-"+tc.name+"-selected.png"), golden.Capture(t, windowSize, selectedPage(tokens.Radius)))
			}

			img := golden.Capture(t, windowSize, page)
			p := tc.c
			selected := vgcolor.Flatten(p.ContentListSelection, p.CardFill)
			sameRGB(t, img, firstCard(0), p.CardFill, "the card fill")
			sameRGB(t, img, secondCard(img, p), selected, "the selected fill")
			ring := vgcolor.Flatten(p.KeyboardFocusIndicator, p.ControlBackground)
			x := firstCard(0).X
			for y := int(shell.ToolbarDepth); y < int(shell.ToolbarDepth)+int(complist.RingWidth); y++ {
				if at := img.RGBAAt(x, y); !near(at, ring) {
					t.Errorf("the ring's pixel at y %d above the group = %v, want %v", y, at, ring)
				}
			}
			sameRGB(t, img, image.Pt(firstCard(1).X, int(shell.ToolbarDepth)+1), p.ControlBackground, "the content fill above the column that has no keyboard")
		})
	}
}

// secondCard is a point on the second card of the first column, beside its
// title: the first point down the card's leading padding, under the first
// card, that is neither the first card's fill nor the content fill.
func secondCard(img *image.RGBA, p tokens.PlatformColors) image.Point {
	x := int(boardInset) + int(tokens.Spacing.S4) + 4
	y := firstCard(0).Y
	for y < windowSize.Y && img.RGBAAt(x, y) == rgba(p.CardFill) {
		y++
	}
	for y < windowSize.Y && img.RGBAAt(x, y) == rgba(p.ControlBackground) {
		y++
	}
	return image.Pt(x, y+4)
}

// dumpPNG writes img to path.
func dumpPNG(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
}

func rgba(c color.NRGBA) color.RGBA { return color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xff} }

// near reports whether got is want within 2 levels per channel: the ring is
// stroked, and its straight side can differ by a level between GPU
// contexts.
func near(got color.RGBA, want color.NRGBA) bool {
	d := func(a, b uint8) bool { return a-b <= 2 || b-a <= 2 }
	return d(got.R, want.R) && d(got.G, want.G) && d(got.B, want.B)
}
