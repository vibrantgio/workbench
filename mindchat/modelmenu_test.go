package main

import "testing"

// The header chip names the model in effect and nothing else: the default-ness
// of that model is the menu's story, not the anchor's.
func TestChipLabelNamesTheModelOnly(t *testing.T) {
	base := Model{
		Providers: []Provider{
			{Name: "OpenAI", Models: []string{"gpt-5.5", "gpt-5.5-codex"}},
			{Name: "xAI", Models: []string{"grok-4"}},
		},
		DefaultProvider: "OpenAI",
		DefaultModel:    "gpt-5.5",
	}

	fromDefault := chipKeyOf(base)
	if fromDefault.label != "OpenAI · gpt-5.5" {
		t.Fatalf("default label = %q, want %q", fromDefault.label, "OpenAI · gpt-5.5")
	}

	override := base
	override.CurrentChat.Provider, override.CurrentChat.Model = "xAI", "grok-4"
	if got := chipKeyOf(override).label; got != "xAI · grok-4" {
		t.Fatalf("override label = %q, want %q", got, "xAI · grok-4")
	}

	// Picking the model that already was the default says the same thing, so
	// it is the same key and the chip keeps its subscription.
	sameAsDefault := base
	sameAsDefault.CurrentChat.Provider, sameAsDefault.CurrentChat.Model = "OpenAI", "gpt-5.5"
	if chipKeyOf(sameAsDefault) != fromDefault {
		t.Fatalf("explicit pick of the default = %+v, want %+v", chipKeyOf(sameAsDefault), fromDefault)
	}

	// The chevron still parts two keys, and an unconfigured app still says so.
	open := base
	open.ModelMenu = true
	if chipKeyOf(open) == fromDefault {
		t.Fatal("open menu shares the closed menu's chip key")
	}
	if got := chipKeyOf(Model{}).label; got != "No model configured" {
		t.Fatalf("empty label = %q, want %q", got, "No model configured")
	}
}
