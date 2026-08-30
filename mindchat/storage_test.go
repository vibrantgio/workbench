package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/mvu"
)

// run subscribes a command and returns its messages and its error. It is the
// runner in main.go reduced to what a test needs: the same Observable, drained
// to completion on this goroutine.
func run(t *testing.T, cmd mvu.Command) ([]mvu.Message, error) {
	t.Helper()
	var msgs []mvu.Message
	var failure error
	cmd.Observable.Subscribe(rx.GoroutineContext(), func(next mvu.Message, err error, done bool) {
		switch {
		case !done:
			msgs = append(msgs, next)
		case err != nil:
			failure = err
		}
	}).Wait()
	return msgs, failure
}

// TestFirstRunOnAnEmptyDataDir is the fresh-install test: it drives the
// application's OWN startup sequence (InitIn, exactly what Init runs once the
// OS data directory is resolved) against a directory that contains nothing,
// and then Load Chat List, Load History and Append Prompt.
//
// It creates no directory of its own, and that restraint is load-bearing:
// every other storage test calls os.MkdirAll(dir, "chats") in its setup,
// standing in for the application. Do not "fix" a failure here by adding a
// MkdirAll to the test.
func TestFirstRunOnAnEmptyDataDir(t *testing.T) {
	datadir := t.TempDir()
	model, startup := InitIn(datadir, "")
	startupMsgs, err := run(t, startup)
	if err != nil {
		t.Fatalf("startup sequence on an empty data dir: %v", err)
	}

	// The application, not the test, is what must have made this.
	if info, err := os.Stat(model.ChatDir()); err != nil || !info.IsDir() {
		t.Fatalf("chats/ after startup: %v (isdir=%v); nothing in the app creates it, "+
			"so every read and write below fails on a fresh install", err, err == nil && info.IsDir())
	}

	// The startup sequence ends in a Config, and the reducer answers it with
	// Load Chat List and (when there is a last chat) Load History. Both are
	// driven here from the reducer's own output rather than hand-built, so a
	// fallback config naming a chat no fresh install has fails here rather
	// than only on a real first run.
	if len(startupMsgs) != 1 {
		t.Fatalf("startup emitted %d messages, want one Config", len(startupMsgs))
	}
	config, ok := startupMsgs[0].(Config)
	if !ok {
		t.Fatalf("startup emitted %#v, want a Config", startupMsgs[0])
	}
	if config.LastChat != "" {
		t.Fatalf("fallback config's LastChat = %q; a fresh install has no chats, "+
			"so naming one makes Load History fail on every first run", config.LastChat)
	}
	model, follow := Update(model, config)
	if _, err := run(t, follow); err != nil {
		t.Fatalf("the reducer's answer to the first-run config: %v", err)
	}

	// Load Chat List over the empty directory. An empty directory yields no
	// entries and so no ChatList message at all — the scan has nothing to
	// accumulate — which leaves the model's nil list alone and is the right
	// first-run sidebar. A MISSING directory is what errored.
	msgs, err := run(t, LoadChatList(model.ChatDir()))
	if err != nil {
		t.Fatalf("Load Chat List: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("Load Chat List over an empty chats/ = %#v, want no emission", msgs)
	}

	// Append Prompt: the first message a new user types. The reducer issues
	// this before the request runs, so it is what stands between the composer
	// accepting the text and the text existing anywhere.
	name := FreshChatName(model.ChatList)
	if _, err := run(t, AppendChatEvent(model.ChatFile(name), ChatEvent{Type: "user", Text: "hello"})); err != nil {
		t.Fatalf("Append Prompt: %v", err)
	}

	// Load Chat List again: the chat the first prompt created is now listed.
	msgs, err = run(t, LoadChatList(model.ChatDir()))
	if err != nil {
		t.Fatalf("Load Chat List after the first prompt: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("Load Chat List emitted %d messages, want one ChatList", len(msgs))
	}
	if list, ok := msgs[0].(ChatList); !ok || len(list) != 1 || list[0] != name {
		t.Fatalf("chat list = %#v, want just %q", msgs[0], name)
	}

	// Load History: the same chat reopened. The prompt must come back.
	msgs, err = run(t, LoadHist(name, model.ChatFile(name)))
	if err != nil {
		t.Fatalf("Load History: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("Load History emitted %d messages, want one HistLoaded", len(msgs))
	}
	loaded, ok := msgs[0].(HistLoaded)
	if !ok {
		t.Fatalf("Load History = %#v, want HistLoaded", msgs[0])
	}
	if len(loaded.History) != 1 || loaded.History[0].Role != RoleUser || loaded.History[0].Content != "hello" {
		t.Fatalf("reloaded history = %+v, want the one prompt back", loaded.History)
	}
}

// TestMigrateChatsConvertsLegacyFiles runs the startup migration command
// against both legacy formats: the originals must be parked under
// .migrated and the .jsonl conversions must replay identically.
func TestMigrateChatsConvertsLegacyFiles(t *testing.T) {
	chatdir := filepath.Join(t.TempDir(), "chats")
	if err := os.MkdirAll(chatdir, 0o755); err != nil {
		t.Fatal(err)
	}
	wrapped := `{"Provider":"xAI","Model":"grok-4","History":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"}]}`
	if err := os.WriteFile(filepath.Join(chatdir, "old.json"), []byte(wrapped), 0o644); err != nil {
		t.Fatal(err)
	}
	bare := `[{"role":"user","content":"solo"}]`
	if err := os.WriteFile(filepath.Join(chatdir, "bare.json"), []byte(bare), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateChats(chatdir).Wait(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(chatdir, "old.json")); !os.IsNotExist(err) {
		t.Fatalf("old.json still in the chat dir; it would list as a chat")
	}
	if _, err := os.Stat(filepath.Join(chatdir, ".migrated", "old.json")); err != nil {
		t.Fatalf("backup missing from .migrated: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(chatdir, "old.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	cf, err := ParseChatFile(data)
	if err != nil || cf.Provider != "xAI" || cf.Model != "grok-4" || len(cf.History) != 2 {
		t.Fatalf("old.jsonl replay = %+v, %v; want the override and both rows", cf, err)
	}
	data, err = os.ReadFile(filepath.Join(chatdir, "bare.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if cf, err = ParseChatFile(data); err != nil || len(cf.History) != 1 || cf.History[0].Content != "solo" {
		t.Fatalf("bare.jsonl replay = %+v, %v", cf, err)
	}

	// A second sweep over the already-converted directory is a no-op.
	if err := MigrateChats(chatdir).Wait(); err != nil {
		t.Fatal(err)
	}
}

// TestAppendChatEventRoundTrip appends the event kinds a live exchange
// produces and replays the file the way the chat loader does.
func TestAppendChatEventRoundTrip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "chat.jsonl")
	events := []ChatEvent{
		{Type: "meta", Provider: "xAI", Model: "grok-4.5"},
		{Type: "user", Text: "search this"},
		{Type: "assistant", Text: "found it", Citations: []Citation{{URL: "https://x.ai", Title: "xAI"}}},
		{Type: "error", Error: "HTTP 410: Gone"},
	}
	for _, e := range events {
		if err := AppendChatEvent(file, e).Wait(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := ParseChatFile(data)
	if err != nil || cf.Provider != "xAI" || cf.Model != "grok-4.5" {
		t.Fatalf("replay = %+v, %v; want the meta override applied", cf, err)
	}
	if len(cf.History) != 3 || cf.History[0].Role != RoleUser || cf.History[2].Role != RoleError {
		t.Fatalf("history = %+v, want user+assistant+error rows", cf.History)
	}
	if len(cf.History[1].Citations) != 1 || cf.History[1].Citations[0].URL != "https://x.ai" {
		t.Fatalf("citations = %+v, want the source preserved", cf.History[1].Citations)
	}
}
