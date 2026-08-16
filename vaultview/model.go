// model.go defines the MVU model, the message types, and the Update
// function that reduces them.
//
// Vault resolution order: a command-line argument wins; without one the
// stored default is used when it still names a directory; otherwise the
// folder-browser picker asks. Every successful open writes the vault
// back to the store, so the next argument-less launch opens the same
// vault without asking.

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/obsidian"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/toast"
)

// screen selects which top-level surface the window shows.
type screen int

const (
	screenPicker screen = iota
	screenVault
)

// Model is the complete runtime state of the app.
type Model struct {
	Screen screen

	// Picker state: the directory the folder browser shows and its rows.
	PickerDir     string
	PickerEntries []DirEntry

	// Vault state.
	Vault    string // absolute path of the open vault
	Scanning bool   // the scan command is in flight
	ScanErr  string // non-empty when the scan or the note read failed
	Index    *Index // the scanned vault index; nil until scanned

	// Navigation state. Notes caches every note loaded this session,
	// keyed by vault-relative path; the map is replaced, never mutated,
	// when a note is added, so no model aliases another. Current names
	// the note on screen; CurAnchor is the top-level block index the
	// viewport seats at (-1 for the top), and NavSeq counts landings so
	// the view can tell a fresh anchor landing from a re-render.
	Notes     map[string]*Note
	Current   string
	CurAnchor int
	NavSeq    int

	// History is the visited-note stack; Cursor points at the current
	// entry. Navigate pushes and truncates the forward tail; Back and
	// Forward move the cursor.
	History []HistEntry
	Cursor  int

	// Folds is the left tree's disclosure state: vault-relative folder
	// path → open. A missing entry means closed. The map is replaced,
	// never mutated, when a fold toggles, so no model aliases another.
	Folds map[string]bool

	PropsOpen bool        // the properties panel is expanded
	Toasts    toast.Queue // transient notifications, oldest first
}

// HistEntry is one visited note: its path and the anchor block index the
// visit landed on (-1 when the visit started at the top). The live scroll
// position itself belongs to the note's cached document, not the model.
type HistEntry struct {
	Path   string
	Anchor int
}

// CurrentNote returns the note on screen, nil before one is loaded.
func (m Model) CurrentNote() *Note {
	return m.Notes[m.Current]
}

// Note is one loaded note: its frontmatter split off, its body parsed,
// wikilinks lifted into hyperlink spans, and block-id anchors stripped
// into an id → top-level block index map.
type Note struct {
	Path    string // vault-relative, forward slashes
	Title   string // file name without the .md extension
	FM      obsidian.FrontMatter
	Blocks  []markdown.Block
	Anchors map[string]int // block id → top-level block index
}

// BrowseTo points the folder browser at a directory.
type BrowseTo struct{ Dir string }

// OpenVault opens the directory as the vault: the path is stored as the
// default and the scan command starts.
type OpenVault struct{ Path string }

// ToggleProperties expands or collapses the properties panel.
type ToggleProperties struct{}

// ToggleFold opens or closes one folder row of the left tree.
type ToggleFold struct{ Dir string }

// Navigate opens a resolved wikilink target: the note at Path, seated at
// the heading path or block id when one is carried. It pushes onto the
// history stack and truncates any forward tail.
type Navigate struct {
	Path     string
	Headings []string
	BlockID  string
}

// GoBack moves one entry back in the history; at the oldest entry it is
// a no-op.
type GoBack struct{}

// GoForward moves one entry forward in the history; at the newest entry
// it is a no-op.
type GoForward struct{}

// pickerListed delivers the folder browser's rows for a directory.
type pickerListed struct {
	dir     string
	entries []DirEntry
}

// vaultScanned delivers the scan command's result: the index and the
// first note found, or the error that stopped either.
type vaultScanned struct {
	vault string
	index *Index
	note  *Note
	err   string
}

// noteLoaded delivers a navigation target read off disk.
type noteLoaded struct {
	vault string
	nav   Navigate
	note  *Note
	err   string
}

// Init resolves the vault (CLI argument → stored default → the picker)
// and returns the seed model with its startup command.
func Init() (Model, mvu.Command) {
	if v, ok := resolveVault(os.Args); ok {
		return Model{Screen: screenVault, Vault: v, Scanning: true, PropsOpen: true, CurAnchor: -1}, openVaultCmd(v)
	}
	dir := startDir()
	return Model{Screen: screenPicker, PickerDir: dir, CurAnchor: -1}, listDirCmd(dir)
}

// startDir is where the folder browser begins: the home directory, or the
// filesystem root when no home resolves.
func startDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return string(filepath.Separator)
}

// resolveVault applies the resolution order to the process arguments. An
// argument that is not a directory, and a stored default that stopped
// being one, both fall through rather than erroring — a renamed vault is
// an ordinary event.
func resolveVault(args []string) (string, bool) {
	if len(args) > 1 && args[1] != "" {
		if abs, err := filepath.Abs(args[1]); err == nil && isDir(abs) {
			return abs, true
		}
	}
	if p := LoadStoredVault(); p != "" && isDir(p) {
		return p, true
	}
	return "", false
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// Update reduces a message into the next Model.
func Update(model Model, msg mvu.Message) (Model, mvu.Command) {
	switch m := msg.(type) {
	case BrowseTo:
		model.PickerDir = m.Dir
		model.PickerEntries = nil
		return model, listDirCmd(m.Dir)
	case pickerListed:
		// A listing for a directory the browser has already left is stale.
		if m.dir == model.PickerDir {
			model.PickerEntries = m.entries
		}
	case OpenVault:
		model.Screen = screenVault
		model.Vault = m.Path
		model.Scanning = true
		model.ScanErr = ""
		model.Index = nil
		model.Notes = nil
		model.Current = ""
		model.CurAnchor = -1
		model.History = nil
		model.Cursor = 0
		model.Folds = nil
		model.PropsOpen = true
		return model, openVaultCmd(m.Path)
	case vaultScanned:
		if m.vault != model.Vault {
			break // a scan of a vault no longer open
		}
		model.Scanning = false
		model.Index = m.index
		model.ScanErr = m.err
		if m.note != nil {
			model = cacheNote(model, m.note)
			model.Current = m.note.Path
			model.CurAnchor = -1
			model.NavSeq++
			model.History = []HistEntry{{Path: m.note.Path, Anchor: -1}}
			model.Cursor = 0
			model = revealCurrent(model)
		}
	case Navigate:
		if note := model.Notes[m.Path]; note != nil {
			return landOn(model, m, note), mvu.DoNothing()
		}
		return model, loadNoteCmd(model.Vault, m)
	case noteLoaded:
		if m.vault != model.Vault {
			break // a load for a vault no longer open
		}
		if m.err != "" {
			return raiseToast(model, toast.Request(toast.Error, m.err))
		}
		model = cacheNote(model, m.note)
		return landOn(model, m.nav, m.note), mvu.DoNothing()
	case GoBack:
		if model.Cursor > 0 {
			model.Cursor--
			model.Current = model.History[model.Cursor].Path
			model.CurAnchor = -1 // the cached document keeps its scroll
			model = revealCurrent(model)
		}
	case GoForward:
		if model.Cursor+1 < len(model.History) {
			model.Cursor++
			model.Current = model.History[model.Cursor].Path
			model.CurAnchor = -1 // the cached document keeps its scroll
			model = revealCurrent(model)
		}
	case ToggleProperties:
		model.PropsOpen = !model.PropsOpen
	case ToggleFold:
		folds := make(map[string]bool, len(model.Folds)+1)
		for k, v := range model.Folds {
			folds[k] = v
		}
		folds[m.Dir] = !folds[m.Dir]
		model.Folds = folds
	case toast.Requested:
		return raiseToast(model, m)
	case toast.Expired:
		model.Toasts = model.Toasts.Remove(m.ID)
	}
	return model, mvu.DoNothing()
}

// raiseToast queues a toast request and returns the expiry timer that
// will bring its removal back through Update.
func raiseToast(model Model, r toast.Requested) (Model, mvu.Command) {
	q, t := model.Toasts.Add(r)
	model.Toasts = q
	return model, toast.Expire(t.ID, t.Lifetime)
}

// cacheNote returns the model with the note added to the cache. The map
// is replaced, not mutated, so previous models keep the cache they saw.
func cacheNote(model Model, n *Note) Model {
	notes := make(map[string]*Note, len(model.Notes)+1)
	for k, v := range model.Notes {
		notes[k] = v
	}
	notes[n.Path] = n
	model.Notes = notes
	return model
}

// landOn makes a cached note current: the anchor block index is computed
// from the parsed blocks, and the history push truncates any forward
// tail. The history slice is freshly allocated so no model aliases
// another's stack.
func landOn(model Model, nav Navigate, note *Note) Model {
	anchor := -1
	if at, ok := AnchorBlock(note, nav.Headings, nav.BlockID); ok {
		anchor = at
	}
	model.Current = note.Path
	model.CurAnchor = anchor
	model.NavSeq++
	keep := model.History
	if len(keep) > model.Cursor+1 {
		keep = keep[:model.Cursor+1]
	}
	hist := make([]HistEntry, 0, len(keep)+1)
	hist = append(hist, keep...)
	model.History = append(hist, HistEntry{Path: note.Path, Anchor: anchor})
	model.Cursor = len(model.History) - 1
	return revealCurrent(model)
}

// revealCurrent opens every folder on the current note's path, so the
// tree row marking it is visible however the landing happened — link,
// history or tree click. The fold map is replaced, not mutated, so no
// model aliases another's disclosure state.
func revealCurrent(model Model) Model {
	if model.Current == "" {
		return model
	}
	dir := path.Dir(model.Current)
	if dir == "." {
		return model
	}
	folds := make(map[string]bool, len(model.Folds)+2)
	for k, v := range model.Folds {
		folds[k] = v
	}
	cum := ""
	for _, seg := range strings.Split(dir, "/") {
		if cum == "" {
			cum = seg
		} else {
			cum += "/" + seg
		}
		folds[cum] = true
	}
	model.Folds = folds
	return model
}

// loadNoteCmd reads a navigation target off the render goroutine and
// delivers it as a noteLoaded message.
func loadNoteCmd(vault string, nav Navigate) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		n, err := LoadNote(vault, nav.Path)
		if err != nil {
			return noteLoaded{vault: vault, nav: nav, err: err.Error()}, nil
		}
		return noteLoaded{vault: vault, nav: nav, note: n}, nil
	})
}

// listDirCmd lists a directory for the folder browser off the render
// goroutine.
func listDirCmd(dir string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		return pickerListed{dir: dir, entries: ListDir(dir)}, nil
	})
}

// openVaultCmd writes the vault back to the store, scans the vault, and
// loads the first note found — one command, one message back.
func openVaultCmd(path string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		if err := SaveStoredVault(path); err != nil {
			// The store is a convenience; failing to remember the vault
			// must not block opening it.
			fmt.Fprintln(os.Stderr, "vaultview: store:", err)
		}
		idx, err := ScanVault(path)
		if err != nil {
			return vaultScanned{vault: path, err: err.Error()}, nil
		}
		var note *Note
		if len(idx.Files) > 0 {
			note, err = LoadNote(path, idx.Files[0].Path)
			if err != nil {
				return vaultScanned{vault: path, index: idx, err: err.Error()}, nil
			}
		}
		return vaultScanned{vault: path, index: idx, note: note}, nil
	})
}

// LoadNote reads one note from the vault and prepares it for rendering:
// frontmatter split off, body parsed into the public block model, every
// wikilink occurrence lifted into its own hyperlink span, and block-id
// tails stripped into the anchors map the viewport seats on.
func LoadNote(root, rel string) (*Note, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	fm, body := obsidian.SplitFrontMatter(src)
	blocks, anchors := obsidian.BlockAnchors(obsidian.WikiSpans(markdown.Parse(body)))
	base := filepath.Base(rel)
	title := base[:len(base)-len(filepath.Ext(base))]
	return &Note{
		Path:    rel,
		Title:   title,
		FM:      fm,
		Blocks:  blocks,
		Anchors: anchors,
	}, nil
}
