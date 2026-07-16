package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
)

// TestDeleteChatRemovesFileThroughLoop drives the real mvu.Loop: DeleteChat
// must move the chat's history file into the trash directory (undoable for
// the whole session), and UndoDelete must move it back.
func TestDeleteChatRemovesFileThroughLoop(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "chats"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "chats", "alpha.json")
	if err := os.WriteFile(file, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	seed := Model{
		DataDir:     dir,
		CurrentChat: Chat{Name: "beta.json"},
		ChatList:    ChatList{"alpha.json", "beta.json"},
	}
	in := make(chan mvu.Message, 4)
	init := func() (Model, mvu.Command) { return seed, mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Recv(in), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	sub := models.Subscribe(func(Model, error, bool) {}, rx.Goroutine)
	defer sub.Unsubscribe()

	in <- DeleteChat{Name: "alpha.json"}

	trashed := filepath.Join(dir, "chats", ".trash", "alpha.json")
	deadline := time.Now().Add(2 * time.Second)
	moved := false
	for time.Now().Before(deadline) {
		_, liveErr := os.Stat(file)
		_, trashErr := os.Stat(trashed)
		if os.IsNotExist(liveErr) && trashErr == nil {
			moved = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !moved {
		t.Fatalf("chat file was not moved to the trash after DeleteChat")
	}

	// Undo brings it back.
	in <- UndoDelete{}
	for time.Now().Before(deadline.Add(2 * time.Second)) {
		_, liveErr := os.Stat(file)
		_, trashErr := os.Stat(trashed)
		if liveErr == nil && os.IsNotExist(trashErr) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("chat file was not restored from the trash after UndoDelete")
}

// TestRenameChatMovesFileThroughLoop drives the real mvu.Loop: OpenRename +
// RenameChat must run the RenameHist command and move the history file.
func TestRenameChatMovesFileThroughLoop(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "chats"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(dir, "chats", "alpha.json")
	newFile := filepath.Join(dir, "chats", "ideas.json")
	if err := os.WriteFile(oldFile, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	seed := Model{
		DataDir:     dir,
		CurrentChat: Chat{Name: "alpha.json"},
		ChatList:    ChatList{"alpha.json"},
	}
	in := make(chan mvu.Message, 4)
	init := func() (Model, mvu.Command) { return seed, mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Recv(in), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	sub := models.Subscribe(func(Model, error, bool) {}, rx.Goroutine)
	defer sub.Unsubscribe()

	in <- OpenRename{Name: "alpha.json"}
	in <- RenameChat{To: "ideas"}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, oldErr := os.Stat(oldFile)
		_, newErr := os.Stat(newFile)
		if os.IsNotExist(oldErr) && newErr == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("history file was not moved: old=%v new=%v", oldFile, newFile)
}
