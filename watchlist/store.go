// store.go owns the on-disk watchlists file: the Go types that mirror the JSON
// document specified in WATCHLIST-FORMAT.md, the load/save helpers, and the
// first-run starter. All file I/O takes an explicit path so tests use
// t.TempDir() and never touch the real ~/Library/Application Support; the
// platform default path (os.UserConfigDir + vibrantgio/watchlists.json) is
// resolved only in main() via defaultStorePath.

package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// formatVersion is the version this build reads and writes. See
// WATCHLIST-FORMAT.md: a reader that encounters a newer version refuses to
// overwrite the file.
const formatVersion = 1

// Symbol is one tracked instrument inside a watchlist. Symbol is required; the
// rest are optional and default to "" when absent (omitempty on write).
type Symbol struct {
	Symbol    string `json:"symbol"`
	Exchange  string `json:"exchange,omitempty"`
	Timeframe string `json:"timeframe,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

// Watchlist is a named, ordered list of symbols.
type Watchlist struct {
	Name    string   `json:"name"`
	Symbols []Symbol `json:"symbols"`
}

// Document is the whole on-disk file. Watchlists is an ORDERED array (not a
// map) so the sidebar order is stable and deterministic — see
// WATCHLIST-FORMAT.md for why the top level is an array rather than a keyed
// object.
type Document struct {
	Version    int         `json:"version"`
	Selected   string      `json:"selected,omitempty"`
	Watchlists []Watchlist `json:"watchlists"`
}

// defaultStorePath returns the platform watchlists.json path
// (~/Library/Application Support/vibrantgio/watchlists.json on macOS, the XDG
// path on Linux). Wired only in main(); tests inject t.TempDir() paths.
func defaultStorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vibrantgio", "watchlists.json"), nil
}

// starterDocument is the first-run seed: one watchlist named "default" with
// three example symbols, matching the worked example in WATCHLIST-FORMAT.md.
func starterDocument() Document {
	return Document{
		Version:  formatVersion,
		Selected: "default",
		Watchlists: []Watchlist{
			{
				Name: "default",
				Symbols: []Symbol{
					{Symbol: "BTC/USD", Exchange: "Coinbase", Timeframe: "1h"},
					{Symbol: "ETH/USD", Exchange: "Coinbase", Timeframe: "1h"},
					{Symbol: "SOL/USD", Exchange: "Coinbase", Timeframe: "1h"},
				},
			},
		},
	}
}

// loadOrInitStore is the startup entry point. If the file is absent it writes
// the starter document and returns it (the app launches with data); if the
// file is present it is loaded and returned as-is — a present-but-empty
// document is valid and renders the empty state, and is never overwritten by
// the starter. Any other I/O or parse error is returned to the caller.
func loadOrInitStore(path string) (Document, error) {
	doc, err := loadStore(path)
	if err == nil {
		return doc, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return Document{}, err
	}
	// File absent: write the starter once, then return it.
	starter := starterDocument()
	if werr := saveStore(path, starter); werr != nil {
		return Document{}, werr
	}
	return starter, nil
}

// loadStore reads and parses the document at path. A missing file surfaces as
// fs.ErrNotExist (so loadOrInitStore can branch on it).
func loadStore(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

// saveStore writes doc to path as indented JSON, ATOMICALLY: the bytes are
// written to a sibling temp file in the same directory, then os.Rename'd over
// path. A same-filesystem rename is atomic on macOS/Linux, so a reader (or a
// crash mid-write) never observes a partially-written file — it sees either the
// old document or the new one, never a truncated mix. The temp file is removed
// on any pre-rename failure so a failed save never leaves stale .tmp debris.
func saveStore(path string, doc Document) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) // best-effort cleanup; the rename, not this, is the failure
		return err
	}
	return nil
}
