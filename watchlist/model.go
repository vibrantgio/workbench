// model.go defines the MVU model for the watchlist editor, its messages, and
// the pure Update reducer. Built on the post-GX.8/GX.10 architecture from the
// start (see feeds/model.go): every interactive callback lands an
// mvu.MessageOp, Update is pure, and there are no rx.Subject controllers or
// atomic interaction mirrors.
//
// G5.3b adds the symbols editor: a table of the selected watchlist's symbols
// and an add/edit modal. Messages:
//   - SelectWatchlist{Name}        — select a watchlist by name (drives Main).
//   - OpenAddSymbol{}              — open the modal in add mode (editIndex -1).
//   - OpenEditSymbol{Row}          — open the modal pre-populated with a row.
//   - CloseModal{}                 — close the modal (close button / backdrop).
//   - SubmitSymbol{Symbol …}       — reduce the modal submit (validate + mutate).
//
// G5.3c adds delete + bulk + pagination + rename/delete-watchlist:
//   - DeleteSymbol{Row}            — remove one row (trash-icon confirm popover).
//   - ToggleSelect{Row}            — toggle a row's bulk-select checkbox.
//   - BulkDelete{}                 — remove every selected row (navbar Delete N).
//   - SetPage{Page}               — paginate when a watchlist has > pageSize rows.
//   - OpenRenameWatchlist{}/CloseRenameWatchlist{}/SubmitRenameWatchlist{Name}
//                                  — the sidebar context-menu rename modal.
//   - DeleteWatchlist{Name}        — remove a watchlist (context-menu confirm).
//
// SELECTION-SET POLICY (logged in FEEDBACK-G5.3.md): selection is a set of
// ABSOLUTE indices into the selected watchlist's full Symbols slice (NOT
// page-relative, NOT Symbol-string identity — duplicate symbols are legal so a
// string key is ambiguous). Because indices shift under deletion, the reducer
// CLEARS the selection on EVERY mutation (add/edit/delete/bulk-delete) and on
// SelectWatchlist; it also clamps currentPage to the new page count. A paginated
// checkbox maps a page-relative row to its absolute index via pageOffset+row.
//
// The DISK WRITE on save lives in the submit/confirm CALLBACK, not the reducer:
// the reducer stays pure and returns no Commands (mvu.Loop in run() would run
// them; the callback keeps the write synchronous with the confirming click),
// so the callback reads a model mirror, applies the SAME pure helper the reducer
// uses, and writes the resulting Document atomically. (Rationale logged in
// FEEDBACK-G5.3.md.) Per-mutation helpers — applyEdit, deleteSymbolAt,
// bulkDeleteRows, renameWatchlistTo, deleteWatchlistNamed — are each called by
// BOTH the reducer (in-memory) and the callback (to build the saved Document),
// keeping the two in lockstep.

package main

import (
	"strings"

	"github.com/vibrantgio/mvu"
)

// Model is the runtime state of the watchlist editor: the watchlists loaded
// from disk (ordered), the selected name, and the add/edit modal state.
type Model struct {
	watchlists []Watchlist // ordered, as loaded from the on-disk file
	selected   string      // name of the selected watchlist ("" if none)

	// Modal state. modalOpen toggles the add/edit modal; editIndex is the
	// 0-based row index being edited, or -1 in add mode; modalError raises the
	// empty-Symbol alert band; modalEpoch increments on every open so the
	// uncontrolled TextField can be rebuilt fresh per open (see app.go).
	modalOpen  bool
	editIndex  int
	modalError bool
	modalEpoch int
	// editSeed carries the row's current values into the view so the modal can
	// pre-populate (the uncontrolled TextField has no initial-value prop — see
	// FEEDBACK-G5.3.md). In add mode it is the zero Symbol.
	editSeed Symbol

	// selection is the bulk-select set: ABSOLUTE indices into the selected
	// watchlist's full Symbols slice. Cleared on every symbol mutation and on
	// SelectWatchlist (indices shift, so a stale set would target wrong rows).
	selection map[int]bool
	// currentPage is the 1-indexed table page (clamped to [1, pageCount]).
	currentPage int

	// Rename-watchlist modal state. Same uncontrolled-field workaround as the
	// symbol modal (renameEpoch bumps on open to rebuild the field fresh;
	// renameSeed is the old name shown as placeholder; renameError flags an
	// empty/duplicate name). renameTarget is the watchlist being renamed.
	renameOpen   bool
	renameTarget string
	renameSeed   string
	renameError  bool
	renameEpoch  int
}

// pageSize is the row count per table page. A watchlist with more than pageSize
// symbols paginates; at or below it the pagination row is not rendered.
const pageSize = 25

// initialModel builds the seed Model from a loaded Document. Selection follows
// the document's Selected field when it names a present watchlist; otherwise it
// falls back to the first watchlist (or "" when the document is empty — the
// empty-state render branch).
func initialModel(doc Document) Model {
	m := Model{watchlists: doc.Watchlists, editIndex: -1, currentPage: 1}
	if hasWatchlist(doc.Watchlists, doc.Selected) {
		m.selected = doc.Selected
	} else if len(doc.Watchlists) > 0 {
		m.selected = doc.Watchlists[0].Name
	}
	return m
}

// pageCount returns the number of table pages for the selected watchlist's
// symbol count, with a minimum of 1 so page 1 always exists.
func (m Model) pageCount() int {
	wl, _ := m.selectedWatchlist()
	n := (len(wl.Symbols) + pageSize - 1) / pageSize
	if n < 1 {
		return 1
	}
	return n
}

// clampPage returns p constrained to [1, pageCount].
func (m Model) clampPage(p int) int {
	if p < 1 {
		return 1
	}
	if max := m.pageCount(); p > max {
		return max
	}
	return p
}

// hasWatchlist reports whether any watchlist has the given name.
func hasWatchlist(wls []Watchlist, name string) bool {
	if name == "" {
		return false
	}
	for _, w := range wls {
		if w.Name == name {
			return true
		}
	}
	return false
}

// selectedWatchlist returns the currently-selected watchlist and whether one
// is selected (false when selection is empty or names a missing watchlist).
func (m Model) selectedWatchlist() (Watchlist, bool) {
	for _, w := range m.watchlists {
		if w.Name == m.selected {
			return w, true
		}
	}
	return Watchlist{}, false
}

// SelectWatchlist selects the watchlist with the given name. The sidebar row
// click lands this message; Main then shows the selected name.
type SelectWatchlist struct{ Name string }

// OpenAddSymbol opens the modal in add mode (editIndex -1, empty seed). It
// clears any stale alert and bumps the epoch so the field rebuilds empty.
type OpenAddSymbol struct{}

// OpenEditSymbol opens the modal pre-populated with the selected watchlist's
// row at Row (0-based). The row's current values flow into editSeed so the
// view can seed its text cells and placeholders.
type OpenEditSymbol struct{ Row int }

// CloseModal hides the modal (close button, backdrop press, Escape). It clears
// the alert so a later open never starts with a stale banner.
type CloseModal struct{}

// SubmitSymbol reduces the modal submit. An empty Symbol raises the alert and
// changes nothing; otherwise applyEdit mutates the in-memory watchlists (add
// when editIndex is -1, replace otherwise) and the modal closes. The disk
// write + success toast fire from the submit callback (the reducer is pure).
type SubmitSymbol struct {
	Symbol    string
	Exchange  string
	Timeframe string
	Notes     string
}

// DeleteSymbol removes the row at Row (absolute index) from the selected
// watchlist. Fired by the per-row trash-icon confirm popover. Clears the
// selection set and clamps the page (the row count shrank).
type DeleteSymbol struct{ Row int }

// ToggleSelect flips the bulk-select checkbox for the row at Row (absolute
// index) in the selected watchlist.
type ToggleSelect struct{ Row int }

// BulkDelete removes every selected row from the selected watchlist (the navbar
// "Delete N" confirm). Clears the selection and clamps the page.
type BulkDelete struct{}

// SetPage navigates the symbols table to the given 1-indexed page (clamped).
type SetPage struct{ Page int }

// OpenRenameWatchlist opens the rename modal for Name, seeding the field with
// the current name and bumping the epoch so the uncontrolled field rebuilds.
type OpenRenameWatchlist struct{ Name string }

// CloseRenameWatchlist hides the rename modal (close button, backdrop, cancel).
type CloseRenameWatchlist struct{}

// SubmitRenameWatchlist reduces the rename submit. An empty or duplicate name
// raises the alert and changes nothing; otherwise the target watchlist is
// renamed (selection follows if it was the selected one).
type SubmitRenameWatchlist struct{ Name string }

// DeleteWatchlist removes the watchlist named Name (the context-menu delete
// confirm). If it was the selected one, selection falls back to the first
// remaining watchlist (page resets, selection clears).
type DeleteWatchlist struct{ Name string }

// Update reduces a message into the next Model. Pure; always returns
// mvu.DoNothing() — the disk write on save is performed in the submit callback
// by design, not here (mvu.Loop in run() would run a returned Command; the
// callback keeps the write synchronous with the confirming click).
func Update(model Model, msg mvu.Message) (Model, mvu.Command) {
	switch m := msg.(type) {
	case SelectWatchlist:
		// Switching watchlists invalidates the index-based selection and the
		// page (the new watchlist has a different symbol slice).
		model.selected = m.Name
		model.selection = nil
		model.currentPage = 1
	case OpenAddSymbol:
		model.modalOpen = true
		model.editIndex = -1
		model.modalError = false
		model.editSeed = Symbol{}
		model.modalEpoch++
	case OpenEditSymbol:
		wl, ok := model.selectedWatchlist()
		if !ok || m.Row < 0 || m.Row >= len(wl.Symbols) {
			break
		}
		model.modalOpen = true
		model.editIndex = m.Row
		model.modalError = false
		model.editSeed = wl.Symbols[m.Row]
		model.modalEpoch++
	case CloseModal:
		model.modalOpen = false
		model.modalError = false
	case SubmitSymbol:
		if strings.TrimSpace(m.Symbol) == "" {
			// Empty Symbol: raise the alert, keep the modal open, change nothing.
			model.modalError = true
			break
		}
		sym := Symbol{
			Symbol:    strings.TrimSpace(m.Symbol),
			Exchange:  strings.TrimSpace(m.Exchange),
			Timeframe: strings.TrimSpace(m.Timeframe),
			Notes:     strings.TrimSpace(m.Notes),
		}
		model.watchlists = applyEdit(model.watchlists, model.selected, model.editIndex, sym)
		model.modalOpen = false
		model.modalError = false
		// A new/edited row shifts nothing for an edit but a new row grows the
		// slice; clear selection either way so stale indices never apply.
		model.selection = nil
		model.currentPage = model.clampPage(model.currentPage)
	case DeleteSymbol:
		model.watchlists = deleteSymbolAt(model.watchlists, model.selected, m.Row)
		model.selection = nil
		model.currentPage = model.clampPage(model.currentPage)
	case ToggleSelect:
		wl, ok := model.selectedWatchlist()
		if !ok || m.Row < 0 || m.Row >= len(wl.Symbols) {
			break
		}
		sel := make(map[int]bool, len(model.selection)+1)
		for k, v := range model.selection {
			sel[k] = v
		}
		if sel[m.Row] {
			delete(sel, m.Row)
		} else {
			sel[m.Row] = true
		}
		model.selection = sel
	case BulkDelete:
		rows := selectedRows(model.selection)
		model.watchlists = bulkDeleteRows(model.watchlists, model.selected, rows)
		model.selection = nil
		model.currentPage = model.clampPage(model.currentPage)
	case SetPage:
		model.currentPage = model.clampPage(m.Page)
	case OpenRenameWatchlist:
		model.renameOpen = true
		model.renameTarget = m.Name
		model.renameSeed = m.Name
		model.renameError = false
		model.renameEpoch++
	case CloseRenameWatchlist:
		model.renameOpen = false
		model.renameError = false
	case SubmitRenameWatchlist:
		name := strings.TrimSpace(m.Name)
		if name == "" || nameTaken(model.watchlists, name, model.renameTarget) {
			model.renameError = true
			break
		}
		model.watchlists = renameWatchlistTo(model.watchlists, model.renameTarget, name)
		if model.selected == model.renameTarget {
			model.selected = name
		}
		model.renameOpen = false
		model.renameError = false
	case DeleteWatchlist:
		model.watchlists = deleteWatchlistNamed(model.watchlists, m.Name)
		if model.selected == m.Name {
			model.selected = firstWatchlistName(model.watchlists)
			model.selection = nil
			model.currentPage = 1
		}
	}
	return model, mvu.DoNothing()
}

// selectedRows returns the selection set's keys as a slice (unordered).
func selectedRows(sel map[int]bool) []int {
	out := make([]int, 0, len(sel))
	for r, on := range sel {
		if on {
			out = append(out, r)
		}
	}
	return out
}

// nameTaken reports whether name collides with an EXISTING watchlist other than
// except (the watchlist being renamed, which may legally keep its own name).
func nameTaken(wls []Watchlist, name, except string) bool {
	for _, w := range wls {
		if w.Name == name && w.Name != except {
			return true
		}
	}
	return false
}

// firstWatchlistName returns the first watchlist's name, or "" when none remain.
func firstWatchlistName(wls []Watchlist) string {
	if len(wls) == 0 {
		return ""
	}
	return wls[0].Name
}

// applyEdit returns a copy of wls with sym applied to the watchlist named
// selected: appended when editIndex is -1, replacing the row at editIndex
// otherwise. The watchlist slice and the target's symbol slice are copied
// before mutation so the previous Model is never aliased. This is the SINGLE
// pure mutation both the reducer (in-memory) and the submit callback (to build
// the Document it writes to disk) call — keeping the two in lockstep.
func applyEdit(wls []Watchlist, selected string, editIndex int, sym Symbol) []Watchlist {
	out := make([]Watchlist, len(wls))
	copy(out, wls)
	for i := range out {
		if out[i].Name != selected {
			continue
		}
		syms := append([]Symbol(nil), out[i].Symbols...)
		if editIndex < 0 || editIndex >= len(syms) {
			syms = append(syms, sym)
		} else {
			syms[editIndex] = sym
		}
		out[i].Symbols = syms
		break
	}
	return out
}

// deleteSymbolAt returns a copy of wls with the symbol at row removed from the
// watchlist named selected. Out-of-range rows are a no-op. The watchlist slice
// and the target's symbol slice are copied before mutation (no aliasing). The
// SINGLE pure helper both the reducer and the confirm callback call.
func deleteSymbolAt(wls []Watchlist, selected string, row int) []Watchlist {
	return bulkDeleteRows(wls, selected, []int{row})
}

// bulkDeleteRows returns a copy of wls with every row in rows removed from the
// watchlist named selected. rows are ABSOLUTE indices into the target's Symbols
// slice; out-of-range indices are ignored. Both slices are copied before
// mutation. The SINGLE pure helper both the reducer and the confirm callback
// call for row and bulk deletes.
func bulkDeleteRows(wls []Watchlist, selected string, rows []int) []Watchlist {
	drop := make(map[int]bool, len(rows))
	for _, r := range rows {
		drop[r] = true
	}
	out := make([]Watchlist, len(wls))
	copy(out, wls)
	for i := range out {
		if out[i].Name != selected {
			continue
		}
		kept := make([]Symbol, 0, len(out[i].Symbols))
		for j, s := range out[i].Symbols {
			if !drop[j] {
				kept = append(kept, s)
			}
		}
		out[i].Symbols = kept
		break
	}
	return out
}

// renameWatchlistTo returns a copy of wls with the watchlist named oldName
// renamed to newName. The slice is copied before mutation. The SINGLE pure
// helper both the reducer and the rename callback call.
func renameWatchlistTo(wls []Watchlist, oldName, newName string) []Watchlist {
	out := make([]Watchlist, len(wls))
	copy(out, wls)
	for i := range out {
		if out[i].Name == oldName {
			out[i].Name = newName
			break
		}
	}
	return out
}

// deleteWatchlistNamed returns a copy of wls with the watchlist named name
// removed. The SINGLE pure helper both the reducer and the confirm callback call.
func deleteWatchlistNamed(wls []Watchlist, name string) []Watchlist {
	out := make([]Watchlist, 0, len(wls))
	for _, w := range wls {
		if w.Name != name {
			out = append(out, w)
		}
	}
	return out
}

// documentOf rebuilds the full on-disk Document from the model's watchlists and
// selection. Used by the submit callback to write the whole file back.
func documentOf(wls []Watchlist, selected string) Document {
	return Document{Version: formatVersion, Selected: selected, Watchlists: wls}
}
