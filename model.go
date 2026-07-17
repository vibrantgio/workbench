package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	openai "github.com/sashabaranov/go-openai"
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

	// Streams tracks in-flight completions by stream id. While a stream's
	// chat is current its deltas apply to CurrentChat.History and the entry
	// holds no buffer; when the user switches away the visible history is
	// stashed into the entry and deltas accumulate there until the stream
	// finishes and saves to ITS chat's file.
	Streams map[int]StreamState
	// NextStream issues stream ids; monotonic, never reused.
	NextStream int
}

// StreamState is one in-flight completion: the chat it belongs to (kept
// up to date across renames) and, while that chat is not current, the
// accumulated history it will save.
type StreamState struct {
	Chat    string
	History []openai.ChatCompletionMessage
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

// StreamFor returns the id of the in-flight completion for the named chat.
func (model Model) StreamFor(name string) (int, bool) {
	for id, s := range model.Streams {
		if s.Chat == name {
			return id, true
		}
	}
	return 0, false
}

// cloneStreams returns a copy of streams for reducer-safe mutation (models
// are values, but maps are shared references).
func cloneStreams(streams map[int]StreamState) map[int]StreamState {
	next := make(map[int]StreamState, len(streams)+1)
	maps.Copy(next, streams)
	return next
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
