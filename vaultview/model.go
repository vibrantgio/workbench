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
	"path/filepath"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/obsidian"
	"github.com/vibrantgio/mvu"
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
	Vault     string // absolute path of the open vault
	Scanning  bool   // the scan command is in flight
	ScanErr   string // non-empty when the scan or the note read failed
	Index     *Index // the scanned vault index; nil until scanned
	Note      *Note  // the rendered note; nil until loaded
	PropsOpen bool   // the properties panel is expanded
}

// Note is one loaded note: its frontmatter split off and its body parsed.
type Note struct {
	Path   string // vault-relative, forward slashes
	Title  string // file name without the .md extension
	FM     obsidian.FrontMatter
	Blocks []markdown.Block
}

// BrowseTo points the folder browser at a directory.
type BrowseTo struct{ Dir string }

// OpenVault opens the directory as the vault: the path is stored as the
// default and the scan command starts.
type OpenVault struct{ Path string }

// ToggleProperties expands or collapses the properties panel.
type ToggleProperties struct{}

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

// Init resolves the vault (CLI argument → stored default → the picker)
// and returns the seed model with its startup command.
func Init() (Model, mvu.Command) {
	if v, ok := resolveVault(os.Args); ok {
		return Model{Screen: screenVault, Vault: v, Scanning: true, PropsOpen: true}, openVaultCmd(v)
	}
	dir := startDir()
	return Model{Screen: screenPicker, PickerDir: dir}, listDirCmd(dir)
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
		model.Note = nil
		model.PropsOpen = true
		return model, openVaultCmd(m.Path)
	case vaultScanned:
		if m.vault != model.Vault {
			break // a scan of a vault no longer open
		}
		model.Scanning = false
		model.Index = m.index
		model.Note = m.note
		model.ScanErr = m.err
	case ToggleProperties:
		model.PropsOpen = !model.PropsOpen
	}
	return model, mvu.DoNothing()
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
// frontmatter split off, body parsed into the public block model.
func LoadNote(root, rel string) (*Note, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	fm, body := obsidian.SplitFrontMatter(src)
	base := filepath.Base(rel)
	title := base[:len(base)-len(filepath.Ext(base))]
	return &Note{
		Path:   rel,
		Title:  title,
		FM:     fm,
		Blocks: markdown.Parse(body),
	}, nil
}
