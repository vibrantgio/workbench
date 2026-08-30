package main

import "testing"

// The header anchor names the model in effect and nothing else: the default-ness
// of that model is the menu's story, not the anchor's.
func TestAnchorLabelNamesTheModelOnly(t *testing.T) {
	base := Model{
		Providers: []Provider{
			{Name: "OpenAI", Models: []string{"gpt-5.5", "gpt-5.5-codex"}},
			{Name: "xAI", Models: []string{"grok-4"}},
		},
		DefaultProvider: "OpenAI",
		DefaultModel:    "gpt-5.5",
	}

	fromDefault := anchorKeyOf(base)
	if fromDefault.label != "OpenAI · gpt-5.5" {
		t.Fatalf("default label = %q, want %q", fromDefault.label, "OpenAI · gpt-5.5")
	}

	override := base
	override.CurrentChat.Provider, override.CurrentChat.Model = "xAI", "grok-4"
	if got := anchorKeyOf(override).label; got != "xAI · grok-4" {
		t.Fatalf("override label = %q, want %q", got, "xAI · grok-4")
	}

	// Picking the model that already was the default says the same thing, so
	// it is the same key and the anchor keeps its subscription.
	sameAsDefault := base
	sameAsDefault.CurrentChat.Provider, sameAsDefault.CurrentChat.Model = "OpenAI", "gpt-5.5"
	if anchorKeyOf(sameAsDefault) != fromDefault {
		t.Fatalf("explicit pick of the default = %+v, want %+v", anchorKeyOf(sameAsDefault), fromDefault)
	}

	// Opening the menu no longer parts two keys. The anchor's mark is the
	// component's paired chevrons and they do not flip — on this platform they
	// say "this pops up", never "this is open" — so an open menu draws the
	// identical anchor and rebuilding its subscription for one would be
	// rebuilding for a frame that looks the same.
	open := base
	open.ModelMenu = true
	if anchorKeyOf(open) != fromDefault {
		t.Fatalf("open menu key = %+v, want the closed menu's %+v: the anchor's mark does not move when the menu stands",
			anchorKeyOf(open), fromDefault)
	}
	if got := anchorKeyOf(Model{}).label; got != "No model configured" {
		t.Fatalf("empty label = %q, want %q", got, "No model configured")
	}
}
