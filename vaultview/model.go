// model.go defines the MVU model, the message types, and the Update
// function that reduces them.
//
// Vault resolution order: a command-line argument wins; without one the
// stored default is used when it still names a directory; otherwise the
// folder-browser picker asks. Every successful open writes the vault
// back to the store, so the next argument-less launch opens the same
// vault without asking.
//
// Freshness without a file watcher: a landing re-stats the note it is
// opening and reads it again when the file moved on, while Rescan
// re-walks the vault for the changes one file cannot show.

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

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

	// Arrival is the NavSeq of the landing a followed link made, and it is
	// what arms the arrival highlight; zero means no link has been followed
	// yet. Only a link lands here: the note the app opens on and the one a
	// rescan finds were not sought, and a step back through the history is a
	// return to a note already read rather than an arrival at one. A rebuilt
	// note — a task box written, a file re-read — is neither, so it leaves
	// this where it was and nothing re-arms.
	Arrival int

	// History is the visited-note stack; Cursor points at the current
	// entry. Navigate pushes and truncates the forward tail; Back and
	// Forward move the cursor.
	History []HistEntry
	Cursor  int

	// Folds is the left tree's disclosure state: vault-relative folder
	// path → open. A missing entry means closed. The map is replaced,
	// never mutated, when a fold toggles, so no model aliases another.
	Folds map[string]bool

	// Filter is the note-name filter typed above the tree. While it is
	// non-empty the tree shows the matching notes as a flat list instead
	// of the folder hierarchy — a filter over the scanned names, nothing
	// more: it reads no file and searches no prose.
	Filter string

	// SidebarHidden hides the folder rail, giving the note column the
	// freed width. It is window state, not vault state: it survives
	// navigation and a switch to another vault for as long as the app
	// runs.
	SidebarHidden bool

	PropsOpen bool        // the properties panel is expanded
	Toasts    toast.Queue // transient notifications, oldest first

	// Chooser state: an ambiguous wikilink's raw body and the candidate
	// paths the resolver refused to pick between. The chooser modal is
	// open exactly while ChooserCandidates is non-empty.
	ChooserBody       string
	ChooserCandidates []string
}

// ChooserOpen reports whether the ambiguity chooser modal is up.
func (m Model) ChooserOpen() bool { return len(m.ChooserCandidates) > 0 }

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
//
// Src is the file bytes as they were read. A task-checkbox click splices
// one marker character in a copy of these bytes and writes that copy;
// the original is never mutated, and neither is the Note.
//
// Mod and Size record what the file looked like when it was read, so a
// later navigation can tell a cached note that is still current from one
// the vault's owner has edited since. A toggle uses the same stamp: a
// file that has moved on is not written.
//
// Lines is the file's own line count, taken off the same bytes and kept
// beside them for the same reason: it is a fact about the file as it was
// read, not about anything the window later does with it.
type Note struct {
	Path    string // vault-relative, forward slashes
	Title   string // file name without the .md extension
	FM      obsidian.FrontMatter
	Blocks  []markdown.Block
	Anchors map[string]int // block id → top-level block index
	Src     []byte         // file bytes at read
	Mod     time.Time      // modification time at read
	Size    int64          // byte size at read
	Lines   int            // source lines at read
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

// OpenChooser raises the ambiguity chooser: the raw wikilink body that
// refused to resolve and the candidate paths it matched.
type OpenChooser struct {
	Body       string
	Candidates []string
}

// ChooseCandidate answers the chooser: navigate to the chosen path,
// carrying the refused link's own anchor parts.
type ChooseCandidate struct{ Path string }

// CloseChooser dismisses the chooser without navigating.
type CloseChooser struct{}

// SwitchVault leaves the vault screen for the folder-browser picker, so
// another vault can be opened. The vault state itself is untouched until
// an OpenVault lands.
type SwitchVault struct{}

// RevealFolder opens every fold on the way to a folder, the folder
// itself included, so its contents are visible in the tree.
type RevealFolder struct{ Dir string }

// RootTree collapses every fold, returning the tree to its root state.
type RootTree struct{}

// ToggleSidebar shows or hides the folder rail.
type ToggleSidebar struct{}

// SetFilter carries the note-name filter typed above the tree, one
// message per keystroke.
type SetFilter struct{ Text string }

// Rescan re-walks the vault. Navigation already re-reads a note whose
// file changed, so this is for the changes a single file cannot show:
// notes added, renamed or removed while the app was open.
type Rescan struct{}

// ToggleTask is a click on a GFM task checkbox. Item is the *ListItem
// Parse produced — the same pointer OnTaskClick received — and Path is
// the note it was drawn from. The command that handles it writes the
// marker before it returns; a toggle that has not yet hit disk has not
// happened.
type ToggleTask struct {
	Path string
	Item *markdown.ListItem
}

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

// vaultRescanned delivers a rescan's result: the fresh index and a fresh
// read of the note on screen, or the error that stopped either. Unlike
// vaultScanned it disturbs nothing else — the history, the folds and the
// note being read stay exactly where they were.
type vaultRescanned struct {
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

// taskToggled delivers the result of a toggle command: the note as
// re-read after the write (or after a refused write), and err when the
// write was refused. The file on disk already holds the new marker
// before this message exists.
type taskToggled struct {
	vault string
	path  string
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
		model.Arrival = 0
		model.History = nil
		model.Cursor = 0
		model.Folds = nil
		model.PropsOpen = true
		model.ChooserBody = ""
		model.ChooserCandidates = nil
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
		// Landing re-stats the target: a note edited outside the app since
		// it was cached is read again, everything else is served from the
		// cache with its scroll position and link state intact.
		if note := model.Notes[m.Path]; note != nil && unchangedOnDisk(model.Vault, note) {
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
	case OpenChooser:
		model.ChooserBody = m.Body
		model.ChooserCandidates = m.Candidates
	case ChooseCandidate:
		ref := ParseRef(model.ChooserBody)
		model.ChooserBody = ""
		model.ChooserCandidates = nil
		return Update(model, Navigate{Path: m.Path, Headings: ref.Headings, BlockID: ref.BlockID})
	case CloseChooser:
		model.ChooserBody = ""
		model.ChooserCandidates = nil
	case SwitchVault:
		model.Screen = screenPicker
		dir := startDir()
		if model.Vault != "" {
			dir = filepath.Dir(model.Vault)
		}
		model.PickerDir = dir
		model.PickerEntries = nil
		return model, listDirCmd(dir)
	case ToggleTask:
		path := m.Path
		if path == "" {
			path = model.Current
		}
		note := model.Notes[path]
		if note == nil || m.Item == nil || !m.Item.Task {
			break
		}
		return model, toggleTaskCmd(model.Vault, note, m.Item)
	case taskToggled:
		if m.vault != model.Vault {
			break
		}
		if m.note != nil {
			// Replace, never mutate: a different pointer at the same path
			// is what makes the document rebuild. NavSeq is not bumped, so
			// the rebuild seats at the outgoing viewport rather than at
			// the landing anchor.
			model = cacheNote(model, m.note)
		}
		if m.err != "" {
			return raiseToast(model, toast.Request(toast.Warning, m.err))
		}
	case Rescan:
		if model.Vault == "" || model.Scanning {
			break
		}
		model.Scanning = true
		return model, rescanCmd(model.Vault, model.Current)
	case vaultRescanned:
		if m.vault != model.Vault {
			break // a rescan of a vault no longer open
		}
		model.Scanning = false
		if m.err != "" {
			return raiseToast(model, toast.Request(toast.Warning, m.err))
		}
		model.Index = m.index
		model.ScanErr = ""
		if m.note != nil {
			// A note the rescan found unchanged keeps its cached value, so
			// the reader's scroll position survives a rescan that had
			// nothing to say about the note on screen.
			if cur := model.Notes[m.note.Path]; cur == nil || cur.Size != m.note.Size || !cur.Mod.Equal(m.note.Mod) {
				model = cacheNote(model, m.note)
			}
			if model.Current == "" {
				model.Current = m.note.Path
				model.CurAnchor = -1
				model.NavSeq++
				model.History = []HistEntry{{Path: m.note.Path, Anchor: -1}}
				model.Cursor = 0
				model = revealCurrent(model)
			}
		}
		return raiseToast(model, toast.Request(toast.Info, rescanSummary(m.index)))
	case SetFilter:
		model.Filter = m.Text
	case RevealFolder:
		model.Folds = openFolds(model.Folds, m.Dir)
	case RootTree:
		model.Folds = nil
	case ToggleSidebar:
		model.SidebarHidden = !model.SidebarHidden
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
	model.Arrival = model.NavSeq
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
	model.Folds = openFolds(model.Folds, dir)
	return model
}

// openFolds returns the fold map with every folder on the way to dir
// opened, dir itself included. The map is replaced, not mutated, so no
// model aliases another's disclosure state; "" and "." open nothing.
func openFolds(prev map[string]bool, dir string) map[string]bool {
	if dir == "" || dir == "." {
		return prev
	}
	folds := make(map[string]bool, len(prev)+2)
	for k, v := range prev {
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
	return folds
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

// rescanCmd re-walks the vault off the render goroutine and re-reads the
// note on screen (or the first note found, when none is), so a rescan
// picks up an edit to the open note as well as the vault's new shape.
func rescanCmd(vault, current string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		idx, err := ScanVault(vault)
		if err != nil {
			return vaultRescanned{vault: vault, err: err.Error()}, nil
		}
		rel := current
		if rel == "" && len(idx.Files) > 0 {
			rel = idx.Files[0].Path
		}
		if rel == "" {
			return vaultRescanned{vault: vault, index: idx}, nil
		}
		n, err := LoadNote(vault, rel)
		if err != nil {
			// The index is good even when the note has gone; keep it and
			// let the refusal speak for the note alone.
			return vaultRescanned{vault: vault, index: idx}, nil
		}
		return vaultRescanned{vault: vault, index: idx, note: n}, nil
	})
}

// rescanSummary is the line a finished rescan reports, since a rescan
// that changed nothing must still say it happened.
func rescanSummary(idx *Index) string {
	n := 0
	if idx != nil {
		n = len(idx.Files)
	}
	if n == 1 {
		return "Rescanned: 1 note"
	}
	return fmt.Sprintf("Rescanned: %d notes", n)
}

// unchangedOnDisk reports whether a cached note still matches the file it
// was read from. A file that cannot be stat'ed — a vault on a detached
// volume, a note deleted since — counts as unchanged: a viewer showing
// what it last read beats one blanking the page.
func unchangedOnDisk(vault string, n *Note) bool {
	if vault == "" || n == nil {
		return true
	}
	fi, err := os.Stat(filepath.Join(vault, filepath.FromSlash(n.Path)))
	if err != nil {
		return true
	}
	return fi.Size() == n.Size && fi.ModTime().Equal(n.Mod)
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
// tails stripped into the anchors map the viewport seats on. The file's
// own lines are counted here, off the bytes that were read, because this
// is the only place that has them.
func LoadNote(root, rel string) (*Note, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	src, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	// Stat after the read, so the recorded stamp can never claim a note is
	// current when the read raced an edit: a write between the two makes
	// the note look stale and it is read again on the next landing.
	var mod time.Time
	var size int64
	if fi, serr := os.Stat(full); serr == nil {
		mod, size = fi.ModTime(), fi.Size()
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
		Src:     src,
		Mod:     mod,
		Size:    size,
		Lines:   sourceLines(src),
	}, nil
}
