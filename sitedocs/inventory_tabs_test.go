package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// groupFrameSize is an inventory tab's content area at the app's default
// window: the first screen of the column, which is what the goldens pin.
var groupFrameSize = image.Pt(1180, 760)

// schemeCases is the light/dark pair every golden here is taken in, with
// the fill the capture is laid over.
var schemeCases = []struct {
	name   string
	colors tokens.PlatformColors
	bg     color.NRGBA
}{
	{"light", tokens.PlatformLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
	{"dark", tokens.PlatformDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
}

// TestGroupTabGoldens pins each inventory tab's first screen in both
// schemes: the sections' headings and bodies, laid out by the same column
// layout.Widget the app scrolls — and, at the top of each, a section heading
// rather than the group banner a tab of one group does not need.
func TestGroupTabGoldens(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	for _, page := range []string{pageComponents, pagePatterns, pageMarkdown} {
		for _, tc := range schemeCases {
			t.Run(page+"/"+tc.name, func(t *testing.T) {
				w := renderGroupTab(shaper, tabGroups[page], tc.colors, tokens.DefaultTypography)
				golden.Render(t, page+"-tab-"+tc.name, groupFrameSize, scene(w, tc.bg))
			})
		}
	}
}

// TestGroupTabsFollowScheme is the standing hunt for an inventory surface
// drawn from something other than the set it was handed: the same column in
// the two appearances must not come out the same bytes.
func TestGroupTabsFollowScheme(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	for _, page := range []string{pageComponents, pagePatterns, pageMarkdown} {
		t.Run(page, func(t *testing.T) {
			light := renderGroupTab(shaper, tabGroups[page], tokens.PlatformLight, tokens.DefaultTypography)
			dark := renderGroupTab(shaper, tabGroups[page], tokens.PlatformDark, tokens.DefaultTypography)
			a := golden.Capture(t, groupFrameSize, scene(light, bg))
			b := golden.Capture(t, groupFrameSize, scene(dark, bg))
			if golden.PixelDiff(a, b) == 0 {
				t.Fatalf("%s tab renders identically in light and dark — the column is not following the set it was handed", page)
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
	c := tokens.PlatformLight
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
	c := tokens.PlatformLight
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

// TestNoInventorySectionIsLost checks the arithmetic: every section the
// published inventory builds is on exactly one tab, save the colour section
// the Theme tab tells with the shared board instead. A section added upstream
// lands on a tab or fails here; it does not quietly vanish because sitedocs
// picks its groups by name.
func TestNoInventorySectionIsLost(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	inv := inventory.NewForOS(shaper, "darwin")
	c := tokens.PlatformLight

	// dropped names the sections sitedocs does not show: the inventory's own
	// board of the platform's set, which the Theme tab draws from the shared
	// section instead, so a reader meets those names once.
	dropped := map[string]bool{"foundations-platform": true}

	// shown counts the sections each surface accounts for: the three group
	// tabs by their groups, the Theme tab by the one section the colour
	// board borrows for its type scale.
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
