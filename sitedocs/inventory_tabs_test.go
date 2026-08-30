package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// groupCanvasSize is an inventory tab's content area at the app's default
// window: the first screen of the column, which is what the goldens pin.
var groupCanvasSize = image.Pt(1180, 760)

// schemeCases is the light/dark pair every golden here is taken in, with
// the ground the capture is laid over.
var schemeCases = []struct {
	name   string
	colors tokens.ColorTokens
	bg     color.NRGBA
}{
	{"light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
	{"dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
}

// TestGroupTabGoldens pins each inventory tab's first screen in both
// schemes: the sections' headings and bodies, laid out by the same column
// widget the app scrolls — and, at the top of each, a section heading
// rather than the group banner a tab of one group does not need.
func TestGroupTabGoldens(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	for _, page := range []string{pageComponents, pagePatterns, pageMarkdown} {
		for _, tc := range schemeCases {
			t.Run(page+"/"+tc.name, func(t *testing.T) {
				w := renderGroupTab(shaper, tabGroups[page], tc.colors, tokens.DefaultTypography)
				golden.Render(t, page+"-tab-"+tc.name, groupCanvasSize, scene(w, tc.bg))
			})
		}
	}
}

// TestGroupTabsFollowScheme is the standing hunt for an inventory surface
// drawn from something other than the tokens it was handed: the same
// column in the two schemes must not come out the same bytes.
func TestGroupTabsFollowScheme(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	for _, page := range []string{pageComponents, pagePatterns, pageMarkdown} {
		t.Run(page, func(t *testing.T) {
			light := renderGroupTab(shaper, tabGroups[page], tokens.DefaultLight, tokens.DefaultTypography)
			dark := renderGroupTab(shaper, tabGroups[page], tokens.DefaultDark, tokens.DefaultTypography)
			a := golden.Capture(t, groupCanvasSize, scene(light, bg))
			b := golden.Capture(t, groupCanvasSize, scene(dark, bg))
			if golden.PixelDiff(a, b) == 0 {
				t.Fatalf("%s tab renders identically in light and dark — the column is not following its tokens", page)
			}
		})
	}
}

// TestEveryTabNamesALiveGroup guards the wiring against a group renamed
// upstream: TabItems looks its group up by name and returns nothing when
// no group answers, which on screen is a blank tab and in a golden a flat
// rectangle nobody reads twice.
func TestEveryTabNamesALiveGroup(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.DefaultLight
	for _, page := range []string{pageComponents, pagePatterns, pageMarkdown} {
		group := tabGroups[page]
		rows := inv.TabItems(c, group)
		if len(rows) == 0 {
			t.Fatalf("the %s tab asks the inventory for a group named %q and gets nothing", page, group)
		}
	}
}

// TestGroupTabDropsTheBanner asserts the whole reason these tabs build
// their own rows: a tab showing one group carries no banner repeating the
// name on the strip cell above it. GroupItems leads with that banner, so
// the tab's rows are its rows less one, plus the closing line.
func TestGroupTabDropsTheBanner(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.DefaultLight
	for _, grp := range inv.Groups(c) {
		if grp.Name == groupFoundations {
			continue
		}
		want := 2 * len(grp.Sections) // heading + body per section
		got := len(inv.TabItems(c, grp.Name)) - 1
		if got != want {
			t.Errorf("the %s tab lays out %d rows before its closing line, want %d — the group banner is still in",
				grp.Name, got, want)
		}
	}
}

// TestNoInventorySectionIsLost is the phase's own arithmetic: every
// section the published inventory builds is on exactly one tab, save the
// two colour sections the Theme tab tells better. A section added
// upstream lands on a tab or fails here; it does not quietly vanish
// because sitedocs picks its groups by name.
func TestNoInventorySectionIsLost(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.DefaultLight

	// dropped names the sections sitedocs deliberately does not show.
	dropped := map[string]bool{"foundations-roles": true, "foundations-ramps": true}

	// shown counts the sections each surface accounts for: the three group
	// tabs by their groups, the Theme tab by the one section the palette
	// story borrows for its type ladder.
	shown := map[string]int{typeSection: 1}
	for page, group := range tabGroups {
		found := false
		for _, grp := range inv.Groups(c) {
			if grp.Name != group {
				continue
			}
			found = true
			for _, s := range grp.Sections {
				shown[s.Name]++
			}
		}
		if !found {
			t.Fatalf("no group named %q for the %s tab", group, page)
		}
	}

	for _, grp := range inv.Groups(c) {
		for _, s := range grp.Sections {
			switch {
			case dropped[s.Name]:
				if shown[s.Name] != 0 {
					t.Errorf("section %q is dropped in sitedocs but shown %d times", s.Name, shown[s.Name])
				}
			case shown[s.Name] != 1:
				t.Errorf("section %q is on %d tabs, want exactly 1", s.Name, shown[s.Name])
			}
		}
	}
}
