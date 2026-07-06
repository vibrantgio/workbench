// model_g53c_test.go locks down the G5.3c reducer obligations no per-surface
// pixel test catches: selection-set clearing and page clamping on every
// mutation, index-based delete/bulk-delete, rename validation, and the
// delete-watchlist selection fallback. These are the silent-bug surfaces the
// task's edge-case list calls out.
package main

import (
	"reflect"
	"testing"

	"github.com/vibrantgio/mvu"
)

func wl30() Document {
	syms := make([]Symbol, 30)
	for i := range syms {
		syms[i] = Symbol{Symbol: string(rune('A'+i)) + "/USD"}
	}
	return Document{
		Version:  formatVersion,
		Selected: "big",
		Watchlists: []Watchlist{
			{Name: "big", Symbols: syms},
			{Name: "small", Symbols: []Symbol{{Symbol: "BTC/USD"}, {Symbol: "ETH/USD"}}},
		},
	}
}

func TestPageCountAndConditional(t *testing.T) {
	m := initialModel(wl30())
	if got := m.pageCount(); got != 2 {
		t.Fatalf("30 symbols at pageSize=%d: pageCount=%d, want 2", pageSize, got)
	}
	m2, _ := Update(m, SelectWatchlist{Name: "small"})
	if got := m2.pageCount(); got != 1 {
		t.Fatalf("2 symbols: pageCount=%d, want 1 (no pagination)", got)
	}
}

func TestSelectWatchlistClearsSelectionAndPage(t *testing.T) {
	m := initialModel(wl30())
	m, _ = Update(m, ToggleSelect{Row: 3})
	m, _ = Update(m, SetPage{Page: 2})
	if len(m.selection) == 0 || m.currentPage != 2 {
		t.Fatalf("setup failed: selection=%v page=%d", m.selection, m.currentPage)
	}
	m, _ = Update(m, SelectWatchlist{Name: "small"})
	if len(m.selection) != 0 {
		t.Errorf("SelectWatchlist did not clear selection: %v", m.selection)
	}
	if m.currentPage != 1 {
		t.Errorf("SelectWatchlist did not reset page: %d", m.currentPage)
	}
}

func TestDeleteSymbolShiftsAndClearsSelection(t *testing.T) {
	m := initialModel(wl30())
	m, _ = Update(m, ToggleSelect{Row: 5})
	m, _ = Update(m, DeleteSymbol{Row: 0})
	wl, _ := m.selectedWatchlist()
	if len(wl.Symbols) != 29 {
		t.Fatalf("delete did not remove a row: %d symbols", len(wl.Symbols))
	}
	if wl.Symbols[0].Symbol != "B/USD" {
		t.Errorf("wrong row removed: head=%q", wl.Symbols[0].Symbol)
	}
	if len(m.selection) != 0 {
		t.Errorf("delete did not clear selection (stale indices): %v", m.selection)
	}
}

func TestBulkDeleteRemovesSelectedRows(t *testing.T) {
	m := initialModel(wl30())
	m, _ = Update(m, ToggleSelect{Row: 0})
	m, _ = Update(m, ToggleSelect{Row: 2})
	m, _ = Update(m, ToggleSelect{Row: 4})
	m, _ = Update(m, BulkDelete{})
	wl, _ := m.selectedWatchlist()
	if len(wl.Symbols) != 27 {
		t.Fatalf("bulk delete: %d symbols, want 27", len(wl.Symbols))
	}
	// A, C, E removed → first three are now B, D, F.
	want := []string{"B/USD", "D/USD", "F/USD"}
	for i, w := range want {
		if wl.Symbols[i].Symbol != w {
			t.Errorf("row %d = %q, want %q", i, wl.Symbols[i].Symbol, w)
		}
	}
	if len(m.selection) != 0 {
		t.Errorf("bulk delete did not clear selection: %v", m.selection)
	}
}

func TestPageClampsAfterShrink(t *testing.T) {
	m := initialModel(wl30()) // 2 pages
	m, _ = Update(m, SetPage{Page: 2})
	// Select all of page 2 (rows 25..29) and delete them → 25 symbols → 1 page.
	for r := 25; r < 30; r++ {
		m, _ = Update(m, ToggleSelect{Row: r})
	}
	m, _ = Update(m, BulkDelete{})
	wl, _ := m.selectedWatchlist()
	if len(wl.Symbols) != 25 {
		t.Fatalf("expected 25 symbols, got %d", len(wl.Symbols))
	}
	if m.pageCount() != 1 {
		t.Fatalf("expected 1 page, got %d", m.pageCount())
	}
	if m.currentPage != 1 {
		t.Errorf("currentPage not clamped after shrink: %d, want 1", m.currentPage)
	}
}

func TestRenameValidation(t *testing.T) {
	m := initialModel(wl30())
	m, _ = Update(m, OpenRenameWatchlist{Name: "big"})
	// Empty name → error, no change.
	got, _ := Update(m, SubmitRenameWatchlist{Name: "  "})
	if !got.renameError {
		t.Error("empty rename did not raise error")
	}
	if got.renameOpen != true {
		t.Error("empty rename closed the modal")
	}
	// Duplicate name → error.
	got, _ = Update(m, SubmitRenameWatchlist{Name: "small"})
	if !got.renameError {
		t.Error("duplicate rename did not raise error")
	}
	// Valid rename → applies, selection follows.
	got, _ = Update(m, SubmitRenameWatchlist{Name: "majors"})
	if got.renameError || got.renameOpen {
		t.Errorf("valid rename did not succeed: err=%v open=%v", got.renameError, got.renameOpen)
	}
	if got.selected != "majors" {
		t.Errorf("selection did not follow rename: %q", got.selected)
	}
	if got.watchlists[0].Name != "majors" {
		t.Errorf("watchlist not renamed: %q", got.watchlists[0].Name)
	}
}

func TestRenameToSameNameAllowed(t *testing.T) {
	m := initialModel(wl30())
	m, _ = Update(m, OpenRenameWatchlist{Name: "big"})
	got, _ := Update(m, SubmitRenameWatchlist{Name: "big"})
	if got.renameError {
		t.Error("renaming a watchlist to its own name was rejected as duplicate")
	}
}

func TestDeleteWatchlistFallback(t *testing.T) {
	m := initialModel(wl30()) // "big" selected
	m, _ = Update(m, DeleteWatchlist{Name: "big"})
	if len(m.watchlists) != 1 {
		t.Fatalf("delete-watchlist: %d remaining, want 1", len(m.watchlists))
	}
	if m.selected != "small" {
		t.Errorf("selection did not fall back to first remaining: %q", m.selected)
	}
	if m.currentPage != 1 {
		t.Errorf("page not reset after watchlist delete: %d", m.currentPage)
	}
	// Delete the last one → selection empty.
	m, _ = Update(m, DeleteWatchlist{Name: "small"})
	if m.selected != "" {
		t.Errorf("selection not cleared when no watchlists remain: %q", m.selected)
	}
}

func TestDeleteNonSelectedWatchlistKeepsSelection(t *testing.T) {
	m := initialModel(wl30()) // "big" selected
	m, _ = Update(m, DeleteWatchlist{Name: "small"})
	if m.selected != "big" {
		t.Errorf("deleting a non-selected watchlist changed selection: %q", m.selected)
	}
}

func TestPureHelpersNoAlias(t *testing.T) {
	orig := wl30().Watchlists
	out := deleteSymbolAt(orig, "big", 0)
	if len(orig[0].Symbols) != 30 {
		t.Errorf("deleteSymbolAt aliased the input slice: %d", len(orig[0].Symbols))
	}
	if len(out[0].Symbols) != 29 {
		t.Errorf("deleteSymbolAt output wrong length: %d", len(out[0].Symbols))
	}
	out2 := bulkDeleteRows(orig, "big", []int{0, 1})
	if len(orig[0].Symbols) != 30 {
		t.Errorf("bulkDeleteRows aliased the input: %d", len(orig[0].Symbols))
	}
	if len(out2[0].Symbols) != 28 {
		t.Errorf("bulkDeleteRows output wrong length: %d", len(out2[0].Symbols))
	}
	out3 := renameWatchlistTo(orig, "big", "huge")
	if orig[0].Name != "big" {
		t.Errorf("renameWatchlistTo aliased the input: %q", orig[0].Name)
	}
	if out3[0].Name != "huge" {
		t.Errorf("renameWatchlistTo did not rename: %q", out3[0].Name)
	}
	out4 := deleteWatchlistNamed(orig, "big")
	if len(orig) != 2 {
		t.Errorf("deleteWatchlistNamed aliased the input: %d", len(orig))
	}
	if len(out4) != 1 || out4[0].Name != "small" {
		t.Errorf("deleteWatchlistNamed wrong result: %v", out4)
	}
}

// guard against unused import if mvu drops; reflect kept for future deep-equal.
var _ = reflect.DeepEqual
var _ mvu.Command = mvu.DoNothing()
