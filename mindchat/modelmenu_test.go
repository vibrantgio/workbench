package main

import (
	"testing"

	"github.com/vibrantgio/theme/tokens"
)

// standsOn is the fill the header anchor is drawn on in these checks: the
// platform's content plane, which is what the chrome row is painted with.
var standsOn = tokens.PlatformLight.ControlBackground

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

	fromDefault := anchorKeyOf(base, standsOn)
	if fromDefault.label != "OpenAI · gpt-5.5" {
		t.Fatalf("default label = %q, want %q", fromDefault.label, "OpenAI · gpt-5.5")
	}

	override := base
	override.CurrentChat.Provider, override.CurrentChat.Model = "xAI", "grok-4"
	if got := anchorKeyOf(override, standsOn).label; got != "xAI · grok-4" {
		t.Fatalf("override label = %q, want %q", got, "xAI · grok-4")
	}

	// Picking the model that already was the default says the same thing, so
	// it is the same key and the anchor keeps its subscription.
	sameAsDefault := base
	sameAsDefault.CurrentChat.Provider, sameAsDefault.CurrentChat.Model = "OpenAI", "gpt-5.5"
	if anchorKeyOf(sameAsDefault, standsOn) != fromDefault {
		t.Fatalf("explicit pick of the default = %+v, want %+v", anchorKeyOf(sameAsDefault, standsOn), fromDefault)
	}

	// The fill the anchor stands on is part of the key: the trigger takes it
	// once per subscription, so a scheme change has to rebuild it.
	otherScheme := anchorKeyOf(base, tokens.PlatformDark.ControlBackground)
	if otherScheme == fromDefault {
		t.Fatalf("the two appearances give one key %+v; the trigger would keep flattening onto the wrong plane", otherScheme)
	}

	// Opening the menu does not part two keys. The anchor's mark is the
	// component's down chevron and it does not flip — on this platform it says
	// "a menu opens below this", never "this is open" — so an open menu draws
	// the identical anchor and rebuilding its subscription for one would be
	// rebuilding for a frame that looks the same.
	open := base
	open.ModelMenu = true
	if anchorKeyOf(open, standsOn) != fromDefault {
		t.Fatalf("open menu key = %+v, want the closed menu's %+v: the anchor's mark does not move when the menu stands",
			anchorKeyOf(open, standsOn), fromDefault)
	}
	if got := anchorKeyOf(Model{}, standsOn).label; got != "No model configured" {
		t.Fatalf("empty label = %q, want %q", got, "No model configured")
	}
}
