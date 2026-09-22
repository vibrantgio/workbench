package main

import (
	"fmt"
	"image"
	"path/filepath"
	"reflect"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/components/breadcrumb"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/theme/tokens"
)

// switchedModel is a vault on screen with the switch dialog raised over it,
// as SwitchVault leaves things where the platform offers no panel of its
// own. Every test of the dialog stands on that, not on the platform the
// test run happens to be on: the dialog is the fallback, and on macOS
// Switch Vault raises the platform's panel instead.
func switchedModel(t *testing.T) Model {
	t.Helper()
	withoutPlatformPanel(t)
	m := Model{Screen: screenVault, Vault: "/vaults/Second Brain", CurAnchor: -1, PropsOpen: true}
	m.Index = treeIndex("Sources.md", "guide/Reading list.md")
	m.Current = "guide/Reading list.md"
	m.History = []HistEntry{{Path: "guide/Reading list.md", Anchor: -1}}
	m.Folds = map[string]bool{"guide": true}
	m, _ = Update(m, SwitchVault{})
	return m
}

// TestTheSwitchDialogStartsAtTheOpenVault requires the browser to open
// INSIDE the vault on screen rather than beside it. It is what makes Open
// with nothing changed a safe answer: the directory the dialog offers is
// the one the reader already has.
func TestTheSwitchDialogStartsAtTheOpenVault(t *testing.T) {
	m := switchedModel(t)
	if !m.PickerOpen {
		t.Fatal("SwitchVault did not open the dialog")
	}
	if m.Screen != screenVault {
		t.Error("SwitchVault left the vault screen; the dialog stands over it")
	}
	if m.PickerDir != "/vaults/Second Brain" {
		t.Errorf("the browser opened at %q, want the open vault %q", m.PickerDir, "/vaults/Second Brain")
	}
	if m.PickerDir == filepath.Dir("/vaults/Second Brain") {
		t.Error("the browser opened at the vault's parent")
	}
}

// TestCancellingTheSwitchChangesNothing requires Cancel to leave the vault
// and every piece of its state exactly as the dialog found them. Escape is
// the same answer — the dialog is a decision, which binds Escape to Cancel
// — so this is what both keys leave behind.
func TestCancellingTheSwitchChangesNothing(t *testing.T) {
	before := switchedModel(t)
	after, cmd := Update(before, CancelSwitch{})

	if after.PickerOpen {
		t.Error("Cancel left the dialog open")
	}
	if msg, err := cmd.First(); err == nil && msg != nil {
		t.Errorf("Cancel started work: %T", msg)
	}
	// The vault itself, read field by field: what a reader would lose if
	// Cancel were a switch in disguise.
	if after.Screen != before.Screen || after.Vault != before.Vault {
		t.Errorf("Cancel left screen=%v vault=%q, want %v and %q",
			after.Screen, after.Vault, before.Screen, before.Vault)
	}
	if after.Index != before.Index || after.Current != before.Current {
		t.Error("Cancel disturbed the scanned index or the note on screen")
	}
	if !reflect.DeepEqual(after.History, before.History) || after.Cursor != before.Cursor {
		t.Error("Cancel disturbed the history")
	}
	if !reflect.DeepEqual(after.Folds, before.Folds) {
		t.Error("Cancel disturbed the tree's folds")
	}
}

// TestTheSwitchDialogAnswersWithCancelAndOpen reads the dialog's two
// bindings back off the props it is built with: Escape invokes
// Decision.Cancel and Return invokes Decision.DefaultAction, so what those
// two send is what those two keys do. The keys themselves are
// patterns/modal's to deliver and are tested there.
func TestTheSwitchDialogAnswersWithCancelAndOpen(t *testing.T) {
	m := switchedModel(t)
	m, _ = Update(m, BrowseTo{Dir: "/vaults/Other"})

	var sent []mvu.Message
	post := func(_ layout.Context, msg mvu.Message) { sent = append(sent, msg) }
	cancel, openVault := vaultPickerAnswers(func() Model { return m }, post)
	props := modal.Props{Decision: vaultPickerDecision(cancel, openVault)}

	if got := props.Purpose(); got != modal.PurposeDecision {
		t.Errorf("the dialog is a %v; a switch with Cancel and Open is a decision — "+
			"a panel would keep a close X and a backdrop that dismisses it", got)
	}
	if props.Decision.Cancel == nil {
		t.Fatal("the decision names no Cancel, so Escape has nothing to invoke")
	}
	props.Decision.Cancel(layout.Context{})

	act := props.Decision.DefaultAction()
	if act == nil {
		t.Fatal("Return reaches nothing; Open is the default action")
	}
	act(layout.Context{})

	want := []mvu.Message{CancelSwitch{}, OpenVault{Path: "/vaults/Other"}}
	if !reflect.DeepEqual(sent, want) {
		t.Errorf("the dialog answered with %v, want %v", sent, want)
	}
}

// TestTheFirstLaunchWithNoVaultKeepsTheFullScreenPicker requires a launch
// that resolves no vault to take the picker SCREEN and not the dialog:
// there is no window for a dialog to stand over then.
func TestTheFirstLaunchWithNoVaultKeepsTheFullScreenPicker(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("HOME", root)

	m, _ := Init()
	if m.Screen != screenPicker {
		t.Errorf("a launch with no vault took screen %v, want the full-screen picker", m.Screen)
	}
	if m.PickerOpen {
		t.Error("a launch with no vault raised the dialog; there is no window for it to stand over")
	}
}

// TestTheSwitchDialogShowsEightRows pins the dialog's stated height. No
// stored capture holds the platform's own open panel, so its size has no
// reading and this one is stated instead: the trail, eight rows, and the
// footer below them — which is what puts the whole dialog at 560 by 340 dp,
// the size window-switch-{light,dark}.png record it at.
//
// A folder with four times the children is the same dialog: the list
// scrolls inside the eight rows rather than growing the surface past them.
func TestTheSwitchDialogShowsEightRows(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	tok := themeTokens{col: tokens.PlatformLight, typ: tokens.DefaultTypography,
		sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper}

	// The room the modal gives its body: 560 dp of surface less the 20 dp
	// inset on each side, and a height it cannot reach.
	room := image.Pt(520, 1000)
	newGtx := func(ops *op.Ops) layout.Context {
		return layout.Context{
			Constraints: layout.Constraints{Max: room},
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
		}
	}
	// A trail of its own per layout: the value NewTrail returns owns the
	// segments' clickables, and nothing here presses one.
	trailFor := func() breadcrumb.TrailLayout {
		return breadcrumb.NewTrail(shaper, breadcrumb.TrailProps{Chevron: trailChevronDp},
			tok.col, tok.sp, tok.typ.TitleSmall)
	}

	folders := func(n int) []DirEntry {
		out := make([]DirEntry, n)
		for i := range out {
			out[i] = DirEntry{Idx: i, Name: fmt.Sprintf("Folder %d", i), Path: fmt.Sprintf("/v/%d", i)}
		}
		return out
	}
	bodyHeight := func(n int) int {
		var ops op.Ops
		gtx := newGtx(&ops)
		m := Model{PickerOpen: true, PickerDir: "/vaults/Second Brain", PickerEntries: folders(n)}
		v := &pickerView{list: list.NewState()}
		return vaultPickerBody(gtx, v, m, tok, trailFor()).Size.Y
	}

	var ops op.Ops
	gtx := newGtx(&ops)
	trailH := trailFor()(gtx, trailSegments(dirPlaces("/vaults/Second Brain", place{label: "Macintosh HD", path: "/"}), browseTo)).Size.Y
	want := trailH + pickerGapDp + vaultPickerRows*int(tokens.Comfortable.ControlHeight)

	if got := bodyHeight(10); got != want {
		t.Errorf("the body stands %d dp tall, want %d — the trail, %d rows and the gap between them",
			got, want, vaultPickerRows)
	}
	if short, long := bodyHeight(10), bodyHeight(40); short != long {
		t.Errorf("ten folders give a %d dp body and forty give %d; the list scrolls inside the dialog, it does not grow it",
			short, long)
	}
}
