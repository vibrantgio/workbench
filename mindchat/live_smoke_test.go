package main

// Temporary live smoke test for the Responses transport; guarded by
// MINDCHAT_LIVE so the regular suite never talks to the network.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/mvu"
)

func liveProvider(t *testing.T, name string) Provider {
	t.Helper()
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, "Library", "Application Support", "nl.simpleapps", "mindchat", "config.json"))
	if err != nil {
		t.Skipf("no live config: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, p := range cfg.Providers {
		if p.Name == name && p.APIKey != "" {
			return p
		}
	}
	t.Skipf("no keyed provider %q in live config", name)
	return Provider{}
}

func runExchange(t *testing.T, provider Provider, model, prompt string) []mvu.Message {
	t.Helper()
	logdir := filepath.Join(t.TempDir(), "logs")
	var got []mvu.Message
	cmd := RequestResponse(1, provider, model,
		[]Message{{Role: RoleUser, Content: prompt}}, logdir, "live.jsonl")
	sub := cmd.Observable.Subscribe(rx.GoroutineContext(), func(next mvu.Message, err error, done bool) {
		if !done && next != nil {
			got = append(got, next)
			t.Logf("message: %#v", next)
		}
	})
	sub.Wait()
	entries, _ := os.ReadDir(logdir)
	for _, e := range entries {
		data, _ := os.ReadFile(filepath.Join(logdir, e.Name()))
		t.Logf("wire log %s: %d bytes, %d lines", e.Name(), len(data), countLines(data))
	}
	return got
}

func countLines(data []byte) int {
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}

func TestLiveXAIWebSearch(t *testing.T) {
	if os.Getenv("MINDCHAT_LIVE") == "" {
		t.Skip("set MINDCHAT_LIVE=1 to run")
	}
	provider := liveProvider(t, "xAI")
	provider.WebSearch = true
	got := runExchange(t, provider, "grok-4.5",
		"Search the web and answer in one short sentence: what is the latest stable Go release right now?")

	kinds := map[string]int{}
	var failure string
	for _, m := range got {
		switch v := m.(type) {
		case AssistantDelta:
			kinds["delta"]++
		case ToolStatus:
			kinds["tool"]++
		case CitationAdded:
			kinds["citation"]++
			t.Logf("citation: %s (%s)", v.URL, v.Title)
		case StreamCompleted:
			kinds["completed"]++
		case StreamFailed:
			kinds["failed"]++
			failure = v.Err
		case StreamDone:
			kinds["done"]++
		}
	}
	t.Logf("event kinds: %v", kinds)
	if kinds["failed"] > 0 {
		t.Fatalf("exchange failed: %s", failure)
	}
	if kinds["delta"] == 0 || kinds["completed"] != 1 || kinds["done"] != 1 {
		t.Fatalf("kinds = %v, want deltas plus exactly one completed and one done", kinds)
	}
}

func TestLiveXAIPlain(t *testing.T) {
	if os.Getenv("MINDCHAT_LIVE") == "" {
		t.Skip("set MINDCHAT_LIVE=1 to run")
	}
	provider := liveProvider(t, "xAI")
	got := runExchange(t, provider, "grok-4.5", "Reply with exactly: pong")
	var text string
	completed := false
	for _, m := range got {
		switch v := m.(type) {
		case AssistantDelta:
			text += v.Text
		case StreamCompleted:
			completed = true
		case StreamFailed:
			t.Fatalf("exchange failed: %s", v.Err)
		}
	}
	if !completed || text == "" {
		t.Fatalf("completed=%v text=%q, want a completed non-empty answer", completed, text)
	}
	t.Logf("answer: %q", text)
}
