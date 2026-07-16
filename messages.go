package main

import openai "github.com/sashabaranov/go-openai"

type LoadState struct{}

type Config struct {
	LastChat string
}

type Prompt struct {
	Content string
}

type ChatList []string

type SelectChat struct {
	Name string
}

type DeleteChat struct {
	Name string
}

// UndoDelete reverses the pending chat delete while its undo window is open.
type UndoDelete struct{}

// ConfirmDelete closes a delete's undo window; it only takes effect while
// Gen matches the pending delete's generation.
type ConfirmDelete struct {
	Gen int
}

// NewChat starts a fresh, empty chat and selects it.
type NewChat struct{}

// OpenRename opens the rename modal for the named chat.
type OpenRename struct {
	Name string
}

// CloseRename dismisses the rename modal without renaming.
type CloseRename struct{}

// RenameChat renames the modal's target chat to the given name (extension
// optional). The reducer validates; an invalid name leaves the modal open.
type RenameChat struct {
	To string
}

// OpenSettings will open the settings surface (OPENAI_API_KEY
// configuration); it reduces to a no-op until that surface exists.
type OpenSettings struct{}

type Chat struct {
	Name    string
	History []openai.ChatCompletionMessage
}
