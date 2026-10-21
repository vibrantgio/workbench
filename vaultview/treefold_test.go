package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/sidebar"
)

// treeFoldModel is a vault over a nest two folders deep, every fold open:
// the run it flattens to carries a note at depth 2, its folder, that
// folder's own folder and a note at the root, which is every row the outline
// keys are asked about below.
func treeFoldModel() Model {
	m := Model{Screen: screenVault, Vault: "/v", CurAnchor: -1}
	m.Index = treeIndex("top.md", "guide/intro.md", "guide/deep/n.md", "notes/a.md")
	m.Folds = map[string]bool{"guide": true, "guide/deep": true, "notes": true}
	return m
}

// treeFoldFrame lays the folder rail out against a real router and keeps the
// model as the loop does: a message the rail posts is reduced into the model
// the next frame lays out, so the fold state read after a key is the state
// that key left.
type treeFoldFrame struct {
	t      *testing.T
	v      *treeView
	model  Model
	tok    themeTokens
	router *input.Router
	posted []mvu.Message

	// geom is what the rail's column arranged on the last frame, in the
	// column's own coordinates.
	geom shell.PaneColumnLayout

	onRail bool
}

func newTreeFoldFrame(t *testing.T) *treeFoldFrame {
	t.Helper()
	f := &treeFoldFrame{
		t:      t,
		v:      &treeView{list: list.NewState()},
		model:  treeFoldModel(),
		tok:    goldenTokens(),
		router: new(input.Router),
	}
	f.v.post = func(_ layout.Context, msg mvu.Message) { f.posted = append(f.posted, msg) }
	return f
}

func (f *treeFoldFrame) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(treeWidthDp, 700)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         ops,
		Source:      f.router.Source(),
	}
	_, f.geom = railColumn(gtx, f.v, f.model, f.tok, nil, nil)
	f.onRail = gtx.Focused(f.v.list.Focus())
	f.router.Frame(ops)
	for _, msg := range f.posted {
		f.model, _ = Update(f.model, msg)
	}
}

// focus hands the rows the keyboard, which is what the arrows are filtered
// on.
func (f *treeFoldFrame) focus() {
	f.t.Helper()
	f.frame()
	for moves := 0; !f.onRail; moves++ {
		if moves == treeTabStops {
			f.t.Fatalf("no forward focus move inside %d put the keyboard on the rows", treeTabStops)
		}
		f.router.MoveFocus(key.FocusForward)
		f.frame()
	}
}

// press sends one key and answers what the rail posted for it.
func (f *treeFoldFrame) press(name key.Name) []mvu.Message {
	f.t.Helper()
	f.posted = nil
	f.router.Queue(key.Event{Name: name, State: key.Press})
	f.frame()
	return f.posted
}

// rows is the run the rail currently draws.
func (f *treeFoldFrame) rows() []TreeRow {
	return TreeRows(f.model.Index, f.model.Folds, vaultName(f.model))
}

// cursorPath is the path of the row the keys stand on, empty when they stand
// on none.
func (f *treeFoldFrame) cursorPath() string {
	rows := f.rows()
	at := f.v.list.Selected()
	if at < 0 || at >= len(rows) {
		return ""
	}
	return rows[at].Path
}

func (f *treeFoldFrame) readsCursor(what, want string) {
	f.t.Helper()
	if got := f.cursorPath(); got != want {
		f.t.Errorf("%s: the cursor stands on %q, want %q", what, got, want)
	}
}

func (f *treeFoldFrame) readsFold(what, dir string, want bool) {
	f.t.Helper()
	if got := f.model.Folds[dir]; got != want {
		f.t.Errorf("%s: %q reads open=%v, want %v", what, dir, got, want)
	}
}

// TestLeftWalksToTheParentBeforeClosingIt drives the platform's outline keys
// over the folder rail through a real input.Router and reads the fold state
// and the cursor after each.
//
// Left closes only what stands under the cursor: on an open folder it closes
// that folder and stays. On a note or a collapsed folder there is nothing
// under the cursor to close, so it walks to the parent's row and leaves the
// fold alone; a second Left is what closes that parent. The vault's own row
// is the walk's end — it is a folder row like any other, so Left closes it
// and everything it holds leaves the list, and a Left on the closed vault
// stays where it is, there being no parent above it. Right opens a collapsed
// folder under the cursor and steps into an open one.
func TestLeftWalksToTheParentBeforeClosingIt(t *testing.T) {
	f := newTreeFoldFrame(t)
	f.focus()

	// Down four times: the vault, the two folders on the way in, then the
	// note they hold.
	for range 4 {
		f.press(key.NameDownArrow)
	}
	f.readsCursor("Down four times from nothing", "guide/deep/n.md")

	if posted := f.press(key.NameLeftArrow); len(posted) != 0 {
		t.Errorf("Left on a note posted %v; it walks to the parent and closes nothing", posted)
	}
	f.readsFold("Left on a note", "guide/deep", true)
	f.readsCursor("Left on a note", "guide/deep")

	f.press(key.NameLeftArrow)
	f.readsFold("Left again, on the open parent it walked to", "guide/deep", false)
	f.readsCursor("Left again, on the open parent it walked to", "guide/deep")

	if posted := f.press(key.NameLeftArrow); len(posted) != 0 {
		t.Errorf("Left on a collapsed folder posted %v; it walks to the parent and closes nothing", posted)
	}
	f.readsFold("Left on a collapsed folder", "guide", true)
	f.readsCursor("Left on a collapsed folder", "guide")

	f.press(key.NameLeftArrow)
	f.readsFold("Left on an open folder", "guide", false)
	f.readsCursor("Left on an open folder", "guide")

	if posted := f.press(key.NameLeftArrow); len(posted) != 0 {
		t.Errorf("Left on a collapsed folder under the vault posted %v; it walks to the vault's own row and closes nothing", posted)
	}
	f.readsCursor("Left on a collapsed folder under the vault", treeRootPath)

	f.press(key.NameLeftArrow)
	if rows := f.rows(); len(rows) != 1 || rows[0].Path != treeRootPath || rows[0].Open {
		t.Errorf("Left on the open vault left the run %v; the vault closes and everything it holds leaves the list",
			rowSigs(rows))
	}
	f.readsCursor("Left on the open vault", treeRootPath)

	if posted := f.press(key.NameLeftArrow); len(posted) != 0 {
		t.Errorf("Left on the closed vault posted %v; there is no parent above it to walk to", posted)
	}
	f.readsCursor("Left on the closed vault", treeRootPath)

	f.press(key.NameRightArrow)
	if rows := f.rows(); len(rows) < 2 || !rows[0].Open {
		t.Errorf("Right on the closed vault left the run %v; it opens again", rowSigs(f.rows()))
	}
	f.readsCursor("Right on the closed vault", treeRootPath)

	f.press(key.NameRightArrow)
	f.readsCursor("Right on the open vault", "guide")

	f.press(key.NameRightArrow)
	f.readsFold("Right on a collapsed folder", "guide", true)
	f.readsCursor("Right on a collapsed folder", "guide")

	f.press(key.NameRightArrow)
	f.readsFold("Right on an open folder", "guide", true)
	f.readsCursor("Right on an open folder", "guide/deep")

	f.press(key.NameRightArrow)
	f.readsFold("Right on the collapsed folder it stepped into", "guide/deep", true)
	f.readsCursor("Right on the collapsed folder it stepped into", "guide/deep")

	f.press(key.NameRightArrow)
	f.readsCursor("Right on the folder it just opened", "guide/deep/n.md")

	if posted := f.press(key.NameRightArrow); len(posted) != 0 {
		t.Errorf("Right on a note posted %v; a note holds nothing to open", posted)
	}
	f.readsCursor("Right on a note", "guide/deep/n.md")
}

// TestTheOutlineKeysNeverOpenANote reads what the two keys do NOT do: they
// move the cursor and one folder's disclosure, and nothing about them
// navigates — which is Return's work and the pointer's.
func TestTheOutlineKeysNeverOpenANote(t *testing.T) {
	f := newTreeFoldFrame(t)
	f.focus()
	for range 4 {
		f.press(key.NameDownArrow)
	}
	for _, name := range []key.Name{key.NameLeftArrow, key.NameRightArrow, key.NameLeftArrow} {
		for _, msg := range f.press(name) {
			if _, ok := msg.(Navigate); ok {
				t.Errorf("%v navigated to %#v; the outline keys open no note", name, msg)
			}
		}
	}
	if f.model.Current != "" {
		t.Errorf("the outline keys left the vault on note %q; they open none", f.model.Current)
	}
}

// clickRow presses the middle of the run's row at i and answers what the
// rail posted for it. The press is reported on the frame after the one that
// drew the row, which is the frame this draws second.
func (f *treeFoldFrame) clickRow(i int) []mvu.Message {
	f.t.Helper()
	// Cleared before the frame that draws, as press does before the frame
	// that reports: the frame reduces what stands in posted, and a message
	// left there from the last press would be reduced a second time.
	f.posted = nil
	f.frame()
	h := unit.Metric{PxPerDp: 1, PxPerSp: 1}.Dp(sidebar.RowHeight)
	at := f32.Pt(float32(treeWidthDp)/2, float32(f.geom.Rows.Min.Y+i*h+h/2))
	f.posted = nil
	f.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	f.frame()
	return f.posted
}

// TestAClickOnTheVaultRowClosesItLikeAFolder reads the vault's row under the
// pointer: it is a folder row like any other, so a click on it toggles its
// fold rather than opening anything, and the run it leaves is the vault
// alone.
func TestAClickOnTheVaultRowClosesItLikeAFolder(t *testing.T) {
	f := newTreeFoldFrame(t)
	if rows := f.rows(); len(rows) < 2 || rows[0].Path != treeRootPath || !rows[0].Open {
		t.Fatalf("the run opens as %v; the vault's row stands first and open", rowSigs(f.rows()))
	}

	posted := f.clickRow(0)
	if len(posted) != 1 || posted[0] != (ToggleFold{Dir: treeRootPath}) {
		t.Fatalf("a click on the vault's row posted %v, want the fold toggled", posted)
	}
	if rows := f.rows(); len(rows) != 1 || rows[0].Open {
		t.Errorf("after the click the run is %v; the closed vault holds everything under it", rowSigs(f.rows()))
	}

	if posted := f.clickRow(0); len(posted) != 1 || posted[0] != (ToggleFold{Dir: treeRootPath}) {
		t.Fatalf("a second click posted %v, want the fold toggled back", posted)
	}
	if rows := f.rows(); len(rows) < 2 || !rows[0].Open {
		t.Errorf("after the second click the run is %v; the vault opens again", rowSigs(f.rows()))
	}
}
