package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
)

// TestDeleteChatRemovesFileThroughLoop drives the real mvu.Loop: the
// DeleteChat message reduces the model AND its DeleteHist command must run
// and remove the chat's history file from disk.
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

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("chat file %s still exists after DeleteChat", file)
}
