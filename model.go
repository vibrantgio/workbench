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

	// Pending is the chat whose delete is awaiting its undo window; the
	// zero value means none. The history file stays on disk until
	// ConfirmDelete finalises it.
	Pending PendingDelete
	// DeleteGen counts deletes monotonically and never resets, so a stale
	// ConfirmDelete timer from an undone delete can never finalise a later
	// one that reused its slot.
	DeleteGen int
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

// FreshChatName returns the first of new.json, new-2.json, new-3.json, …
// not taken by existing.
func FreshChatName(existing ChatList) string {
	name := "new.json"
	for i := 2; slices.Contains(existing, name); i++ {
		name = fmt.Sprintf("new-%d.json", i)
	}
	return name
}
