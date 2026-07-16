package main

import (
	"strings"
	"testing"
)

func TestSetQueryReduces(t *testing.T) {
	seed, _ := Init()
	m := ReduceModel(seed, SetQuery{Text: "play"})
	if m.Query != "play" {
		t.Fatalf("Query = %q, want play", m.Query)
	}
}

func TestUnknownMessageIsIdentity(t *testing.T) {
	m := ReduceModel(Model{Query: "x"}, struct{ Unrelated int }{1})
	if m.Query != "x" {
		t.Fatalf("Query = %q, want x", m.Query)
	}
}

func TestFilterEmptyQueryMatchesAll(t *testing.T) {
	if got := len(FilterIcons("")); got != len(IconTable) {
		t.Fatalf("matches = %d, want %d", got, len(IconTable))
	}
	if got := len(FilterIcons("   ")); got != len(IconTable) {
		t.Fatalf("whitespace query matches = %d, want %d", got, len(IconTable))
	}
}

func TestFilterIsCaseInsensitiveSubstring(t *testing.T) {
	for _, query := range []string{"playarrow", "PlayArrow", "PLAYARROW", "layarro"} {
		matches := FilterIcons(query)
		found := false
		for _, i := range matches {
			if IconTable[i].Name == "AVPlayArrow" {
				found = true
			}
		}
		if !found {
			t.Fatalf("query %q did not match AVPlayArrow (got %d matches)", query, len(matches))
		}
	}
}

func TestFilterExcludesNonMatches(t *testing.T) {
	matches := FilterIcons("settings")
	if len(matches) == 0 {
		t.Fatal("no matches for settings")
	}
	for _, i := range matches {
		name := IconTable[i].Name
		if !strings.Contains(strings.ToLower(name), "settings") {
			t.Fatalf("%q does not contain settings", name)
		}
	}
	if got := len(FilterIcons("zzzzzz")); got != 0 {
		t.Fatalf("nonsense query matched %d icons", got)
	}
}

func TestIconTableIsWellFormed(t *testing.T) {
	if len(IconTable) == 0 {
		t.Fatal("IconTable is empty")
	}
	seen := map[string]bool{}
	for _, icon := range IconTable {
		if icon.Name == "" || len(icon.Data) == 0 {
			t.Fatalf("malformed entry: %+v", icon.Name)
		}
		if seen[icon.Name] {
			t.Fatalf("duplicate icon name %s", icon.Name)
		}
		seen[icon.Name] = true
	}
}
