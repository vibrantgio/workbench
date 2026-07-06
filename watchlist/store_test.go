package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoadOrInitWritesStarterWhenAbsent verifies the first-run path: an absent
// file is created with the starter document and returned. Uses t.TempDir() so
// the real ~/Library/Application Support is never touched.
func TestLoadOrInitWritesStarterWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "watchlists.json")

	doc, err := loadOrInitStore(path)
	if err != nil {
		t.Fatalf("loadOrInitStore: %v", err)
	}
	if doc.Version != formatVersion {
		t.Errorf("version = %d; want %d", doc.Version, formatVersion)
	}
	if len(doc.Watchlists) != 1 || doc.Watchlists[0].Name != "default" {
		t.Fatalf("starter watchlists = %+v; want single \"default\"", doc.Watchlists)
	}
	if got := len(doc.Watchlists[0].Symbols); got != 3 {
		t.Errorf("starter symbols = %d; want 3", got)
	}
	wantSyms := []string{"BTC/USD", "ETH/USD", "SOL/USD"}
	for i, s := range doc.Watchlists[0].Symbols {
		if i < len(wantSyms) && s.Symbol != wantSyms[i] {
			t.Errorf("symbol[%d] = %q; want %q", i, s.Symbol, wantSyms[i])
		}
	}

	// The file must now exist on disk and round-trip to the same document.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("starter file not written: %v", err)
	}
	reloaded, err := loadStore(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Watchlists[0].Name != "default" || len(reloaded.Watchlists[0].Symbols) != 3 {
		t.Errorf("reloaded document differs from starter: %+v", reloaded)
	}
}

// TestLoadOrInitLeavesPresentFileUntouched verifies a present file is loaded
// as-is and never overwritten by the starter — including a present-but-empty
// document (a valid empty-state file).
func TestLoadOrInitLeavesPresentFileUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlists.json")
	custom := Document{
		Version:  formatVersion,
		Selected: "alts",
		Watchlists: []Watchlist{
			{Name: "majors", Symbols: []Symbol{{Symbol: "BTC/USD"}}},
			{Name: "alts", Symbols: []Symbol{{Symbol: "SOL/USD"}, {Symbol: "AVAX/USD"}}},
		},
	}
	if err := saveStore(path, custom); err != nil {
		t.Fatalf("saveStore: %v", err)
	}

	doc, err := loadOrInitStore(path)
	if err != nil {
		t.Fatalf("loadOrInitStore: %v", err)
	}
	if len(doc.Watchlists) != 2 || doc.Watchlists[0].Name != "majors" || doc.Watchlists[1].Name != "alts" {
		t.Errorf("present file not loaded as-is (order/content changed): %+v", doc.Watchlists)
	}
	if doc.Selected != "alts" {
		t.Errorf("selected = %q; want \"alts\"", doc.Selected)
	}
}

// TestLoadOrInitEmptyDocumentIsValid verifies a present-but-empty document
// loads cleanly (empty watchlists array) and is NOT replaced by the starter.
func TestLoadOrInitEmptyDocumentIsValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlists.json")
	if err := saveStore(path, Document{Version: formatVersion, Watchlists: []Watchlist{}}); err != nil {
		t.Fatalf("saveStore: %v", err)
	}
	doc, err := loadOrInitStore(path)
	if err != nil {
		t.Fatalf("loadOrInitStore: %v", err)
	}
	if len(doc.Watchlists) != 0 {
		t.Errorf("empty document replaced by starter: %+v", doc.Watchlists)
	}
}

// TestOrderIsPreservedOnRoundTrip guards the format's ordering contract: the
// watchlists array order survives a write/read cycle (the reason the top level
// is an array, not a map).
func TestOrderIsPreservedOnRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlists.json")
	names := []string{"zeta", "alpha", "mike", "bravo"}
	var wls []Watchlist
	for _, n := range names {
		wls = append(wls, Watchlist{Name: n, Symbols: []Symbol{{Symbol: "BTC/USD"}}})
	}
	if err := saveStore(path, Document{Version: formatVersion, Watchlists: wls}); err != nil {
		t.Fatalf("saveStore: %v", err)
	}
	doc, err := loadStore(path)
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	for i, n := range names {
		if doc.Watchlists[i].Name != n {
			t.Fatalf("order not preserved at %d: got %q want %q", i, doc.Watchlists[i].Name, n)
		}
	}
}

// TestStarterJSONMatchesDocumentedExample sanity-checks that the on-disk JSON
// of the starter contains the documented fields so WATCHLIST-FORMAT.md's
// example stays in sync with what is actually written.
func TestStarterJSONMatchesDocumentedExample(t *testing.T) {
	data, err := json.MarshalIndent(starterDocument(), "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"version": 1`, `"selected": "default"`, `"name": "default"`, `"symbol": "BTC/USD"`, `"exchange": "Coinbase"`, `"timeframe": "1h"`} {
		if !containsSub(string(data), want) {
			t.Errorf("starter JSON missing %q\n%s", want, data)
		}
	}
}

// TestSaveRoundTripPersistsEdits is the G5.3b persistence proof: applyEdit's
// mutation (the SAME helper the reducer and the submit callback both call) is
// written via saveStore and reloaded by a FRESH loadStore — modelling
// "persists across restart" at the store level (no GUI, no driven callback).
// Both an edit-in-place and an append are exercised, then a deep-equal check.
func TestSaveRoundTripPersistsEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlists.json")
	start := Document{
		Version:  formatVersion,
		Selected: "majors",
		Watchlists: []Watchlist{
			{Name: "majors", Symbols: []Symbol{
				{Symbol: "BTC/USD", Exchange: "Coinbase", Timeframe: "1h"},
				{Symbol: "ETH/USD"},
			}},
			{Name: "alts", Symbols: []Symbol{{Symbol: "SOL/USD"}}},
		},
	}
	if err := saveStore(path, start); err != nil {
		t.Fatalf("saveStore(seed): %v", err)
	}

	// Edit row 1 of "majors" in place (mirrors OpenEditSymbol{1} + SubmitSymbol).
	wls := applyEdit(start.Watchlists, "majors", 1, Symbol{Symbol: "ETH/USD", Exchange: "Kraken", Timeframe: "4h", Notes: "rotated"})
	// Append a new row to "majors" (mirrors OpenAddSymbol + SubmitSymbol).
	wls = applyEdit(wls, "majors", -1, Symbol{Symbol: "AVAX/USD", Exchange: "Binance"})
	if err := saveStore(path, documentOf(wls, "majors")); err != nil {
		t.Fatalf("saveStore(edited): %v", err)
	}

	// Fresh load == restart.
	got, err := loadStore(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	want := documentOf(wls, "majors")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\n got = %+v\nwant = %+v", got, want)
	}
	// Spot-check the edited and appended rows survived precisely.
	majors := got.Watchlists[0]
	if majors.Symbols[1].Exchange != "Kraken" || majors.Symbols[1].Timeframe != "4h" || majors.Symbols[1].Notes != "rotated" {
		t.Errorf("edited row not persisted: %+v", majors.Symbols[1])
	}
	if len(majors.Symbols) != 3 || majors.Symbols[2].Symbol != "AVAX/USD" {
		t.Errorf("appended row not persisted: %+v", majors.Symbols)
	}
	// "alts" untouched.
	if got.Watchlists[1].Symbols[0].Symbol != "SOL/USD" {
		t.Errorf("untouched watchlist changed: %+v", got.Watchlists[1])
	}
}

// TestSaveStoreLeavesNoTempOnSuccess is the atomic-write crash-safety property:
// a successful saveStore renames the temp file over the target, so no ".tmp"
// debris is left behind.
func TestSaveStoreLeavesNoTempOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchlists.json")
	if err := saveStore(path, starterDocument()); err != nil {
		t.Fatalf("saveStore: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file left behind after successful save (stat err = %v)", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("target file missing after save: %v", err)
	}
}

// TestApplyEditDoesNotAliasPreviousModel guards the reducer-purity contract:
// applyEdit copies before mutating, so the previous Model's slice is never
// observed changing under it.
func TestApplyEditDoesNotAliasPreviousModel(t *testing.T) {
	orig := []Watchlist{{Name: "w", Symbols: []Symbol{{Symbol: "A"}}}}
	next := applyEdit(orig, "w", 0, Symbol{Symbol: "B"})
	if orig[0].Symbols[0].Symbol != "A" {
		t.Errorf("applyEdit mutated the input slice: %+v", orig[0].Symbols)
	}
	if next[0].Symbols[0].Symbol != "B" {
		t.Errorf("applyEdit did not apply the edit: %+v", next[0].Symbols)
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestInitialModelSelection verifies seed selection: honour Selected when
// present, fall back to the first watchlist, and select nothing when empty.
func TestInitialModelSelection(t *testing.T) {
	cases := []struct {
		name string
		doc  Document
		want string
	}{
		{"honours selected", Document{Selected: "b", Watchlists: []Watchlist{{Name: "a"}, {Name: "b"}}}, "b"},
		{"falls back to first", Document{Selected: "missing", Watchlists: []Watchlist{{Name: "a"}, {Name: "b"}}}, "a"},
		{"empty selects none", Document{Watchlists: nil}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := initialModel(tc.doc).selected; got != tc.want {
				t.Errorf("selected = %q; want %q", got, tc.want)
			}
		})
	}
}
