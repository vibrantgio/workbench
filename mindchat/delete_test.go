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
	file := filepath.Join(dir, "chats", "alpha.jsonl")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	seed := Model{
		DataDir:     dir,
		CurrentChat: Chat{Name: "beta.jsonl"},
		ChatList:    ChatList{"alpha.jsonl", "beta.jsonl"},
	}
	in := make(chan mvu.Message, 4)
	init := func() (Model, mvu.Command) { return seed, mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Recv(in), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	sub := models.Subscribe(rx.GoroutineContext(), func(Model, error, bool) {})
	defer sub.Unsubscribe()

	in <- DeleteChat{Name: "alpha.jsonl"}

	trashed := filepath.Join(dir, "chats", ".trash", "alpha.jsonl")
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
	oldFile := filepath.Join(dir, "chats", "alpha.jsonl")
	newFile := filepath.Join(dir, "chats", "ideas.jsonl")
	if err := os.WriteFile(oldFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	seed := Model{
		DataDir:     dir,
		CurrentChat: Chat{Name: "alpha.jsonl"},
		ChatList:    ChatList{"alpha.jsonl"},
	}
	in := make(chan mvu.Message, 4)
	init := func() (Model, mvu.Command) { return seed, mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Recv(in), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	sub := models.Subscribe(rx.GoroutineContext(), func(Model, error, bool) {})
	defer sub.Unsubscribe()

	in <- OpenRename{Name: "alpha.jsonl"}
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

// TestBackgroundStreamSavesToOwningChatFile drives the real mvu.Loop: an
// exchange still streaming for alpha while beta is current must append its
// final answer to ALPHA's file when it completes — never to the currently
// selected chat's.
func TestBackgroundStreamSavesToOwningChatFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "chats"), 0o755); err != nil {
		t.Fatal(err)
	}

	seed := Model{
		DataDir:     dir,
		CurrentChat: Chat{Name: "beta.jsonl"},
		ChatList:    ChatList{"alpha.jsonl", "beta.jsonl"},
		NextStream:  1,
		Streams: map[int]StreamState{
			1: {Chat: "alpha.jsonl", History: []Message{
				{Role: RoleUser, Content: "tell me a story"},
			}},
		},
	}
	in := make(chan mvu.Message, 4)
	init := func() (Model, mvu.Command) { return seed, mvu.DoNothing() }
	models, runner := mvu.Loop(rx.Recv(in), init, Update)
	defer func() { runner.Unsubscribe(); runner.Wait() }()
	sub := models.Subscribe(rx.GoroutineContext(), func(Model, error, bool) {})
	defer sub.Unsubscribe()

	in <- AssistantDelta{Stream: 1, Text: "Once upon a time"}
	in <- StreamCompleted{Stream: 1} // → append the answer to alpha's file

	alphaFile := filepath.Join(dir, "chats", "alpha.jsonl")
	betaFile := filepath.Join(dir, "chats", "beta.jsonl")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(alphaFile); err == nil {
			if _, betaErr := os.Stat(betaFile); !os.IsNotExist(betaErr) {
				t.Fatalf("beta's file was written by alpha's stream")
			}
			cf, err := ParseChatFile(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(cf.History) != 1 || cf.History[0].Content != "Once upon a time" {
				t.Fatalf("alpha.jsonl = %+v, want the streamed reply appended", cf.History)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("alpha.jsonl was never written by the background stream")
}
