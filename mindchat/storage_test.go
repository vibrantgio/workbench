package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
