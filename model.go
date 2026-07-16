package main

import (
	"fmt"
	"path/filepath"
	"slices"
)

type Model struct {
	DataDir     string
	AuthToken   string
	CurrentChat Chat
	ChatList    ChatList

	// Pending is the delete currently advertised by the undo bar; the zero
	// value means the bar is hidden. Its ConfirmDelete timer only HIDES the
	// bar — the chat stays undoable via Trash for the whole session.
	Pending PendingDelete
	// Trash is the session's undo stack, most recent last. Deleted chats'
	// history files live in ChatDir()/.trash until undone; leftovers are
	// restored into the chat list on next startup.
	Trash []PendingDelete
	// DeleteGen counts deletes monotonically and never resets, so a stale
	// ConfirmDelete timer from an undone delete can never hide the bar of a
	// later one that reused its slot.
	DeleteGen int

	// Rename is the chat whose rename modal is open; the zero value means
	// closed.
	Rename RenameState
}

// RenameState drives the rename modal: Target is the chat filename being
// renamed ("" = modal closed); Epoch is bumped on every open and keys the
// rebuild of the modal's uncontrolled text field, so reopening — even for
// the same chat — reseeds it.
type RenameState struct {
	Target string
	Epoch  int
}

// PendingDelete remembers everything needed to undo a chat delete: the name,
// where in the list it sat, and whether it was the selected chat.
type PendingDelete struct {
	Name       string
	Index      int
	WasCurrent bool
	Gen        int
}

func (model Model) ConfigFile() string {
	return filepath.Join(model.DataDir, "config.json")
}

func (model Model) ChatDir() string {
	return filepath.Join(model.DataDir, "chats")
}

func (model Model) ChatFile(name string) string {
	return filepath.Join(model.DataDir, "chats", name)
}

// TrashDir holds deleted chats' history files while they are undoable.
func (model Model) TrashDir() string {
	return filepath.Join(model.DataDir, "chats", ".trash")
}

func (model Model) TrashFile(name string) string {
	return filepath.Join(model.DataDir, "chats", ".trash", name)
}

// TakenNames returns every chat name that must not be reused: listed chats
// plus everything still undoable in the trash (restoring must never
// collide).
func (model Model) TakenNames() ChatList {
	names := slices.Clone(model.ChatList)
	for _, pending := range model.Trash {
		names = append(names, pending.Name)
	}
	return names
}

// FreshChatName returns the first of new.json, new-2.json, new-3.json, …
// not taken by existing.
func FreshChatName(existing ChatList) string {
	name := "new.json"
	for i := 2; slices.Contains(existing, name); i++ {
		name = fmt.Sprintf("new-%d.json", i)
	}
	return name
}
