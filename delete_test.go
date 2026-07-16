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
// opens the undo window (the file survives), and ConfirmDelete's DeleteHist
// command must run and remove the chat's history file from disk. The
// ConfirmDelete is sent explicitly rather than waiting out the 5s timer.
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
	// The delete is soft while the undo window is open; the seed's
	// DeleteGen is 0, so this delete is generation 1.
	in <- ConfirmDelete{Gen: 1}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("chat file %s still exists after DeleteChat", file)
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
