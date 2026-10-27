// The Components, Patterns and Markdown tabs: one group of the published
// inventory each, as one scrolling column of live controls. The catalogue is
// components/gallery/inventory. The rows are the real components (buttons
// press, fields take text, toggles flip), and this file adds no second
// inventory: it asks the published one for its rows and puts a viewport in
// front of them.
//
// One group per tab, which is the cut the inventory publishes as TabItems:
// the group's sections without the banner that repeats the tab's title,
// closing line included. The Foundations group has no tab of its own: its
// colour section is the board on the Colours tab, and its type scale is the
// Typography tab.
//
// Each tab's inventory is made on the tab's first frame and kept: a change of
// the theme being judged gives new row values over the same parsed documents
// and scroll positions. Each tab has its own inventory, so each tab keeps its
// own scroll positions and its own control states.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// The inventory's own group names, as Groups() spells them. They are the
// lookup key rather than an index, so a group reordered upstream still
// lands on the tab that names it. TabItems returns nothing for a name no
// group has, and TestEveryTabNamesALiveGroup turns that into a failure.
const (
	groupComponents  = "Components"
	groupPatterns    = "Patterns"
	groupMarkdown    = "Markdown"
	groupFoundations = "Foundations"
)

// tabGroups maps each inventory tab to the group it shows. The Colours,
// Typography and Code tabs are not in it: they are not cut from the
// inventory's groups.
var tabGroups = map[int]string{
	TabComponents: groupComponents,
	TabPatterns:   groupPatterns,
	TabMarkdown:   groupMarkdown,
}

// judged is the theme that themer shows, which the moved tabs are drawn in:
// the platform's set for the chosen scheme with the theme colour as its
// accent, the theme's typography with the chosen code face, and the chosen
// highlighter styles. The Code tab's preview is drawn in the same theme.
type judged struct {
	col    tokens.PlatformColors
	typ    tokens.Typography
	shaper *text.Shaper
	styles highlight.StylePair
	// bars is the reader's "Show scroll bars" setting, read once when the
	// window is composed.
	bars desktop.ScrollBarSetting
	// pinned makes each inventory draw the macOS control marks, so a still
	// render gives the same bytes on every machine.
	pinned bool
}

// prepare makes the inventory in *inv draw in the judged theme. It makes the
// inventory on the first call and keeps its parsed documents after that.
func (j judged) prepare(inv **inventory.Inventory) *inventory.Inventory {
	switch {
	case *inv == nil && j.pinned:
		*inv = inventory.NewForOS(j.shaper, "darwin")
	case *inv == nil:
		*inv = inventory.New(j.shaper)
	default:
		// A code face change needs the matching font collection. The
		// parsed documents stay.
		(*inv).SetShaper(j.shaper)
	}
	(*inv).SetTypography(j.typ)
	(*inv).SetCodeStyles(j.styles)
	// The bar inside a fence or under a wide table follows the column's bar.
	(*inv).SetScrollbar(columnBar(j.col, j.bars))
	return *inv
}

// groupPage is what one inventory tab keeps across emissions: its inventory
// and its column's scroll position.
type groupPage struct {
	inv *inventory.Inventory
	st  *list.State
}

// GroupTab is one inventory tab's content: the column of the group's
// sections, in the judged theme.
func GroupTab(j judged, page *groupPage, group string) layout.Widget {
	inv := j.prepare(&page.inv)
	return scrollingColumn(page.st, j.col, j.bars, inv.TabItems(j.col, group))
}

// columnBar is the bar of every scrolling column on the moved tabs. It is
// made from the set the column's content is drawn in, on the content's fill
// that the sections paint, and it follows the reader's "Show scroll bars"
// setting.
func columnBar(c tokens.PlatformColors, bars desktop.ScrollBarSetting) scrollbar.Style {
	return scrollBarStyle(scrollbar.FromTokens(c, inventory.SectionSurface(c)), bars)
}

// scrollingColumn is the scrolling column of every moved tab: the rows in a
// virtual list, which lays out only the rows that show, with the column's
// bar. The anchor is what [list.AnchorFor] returns: a standing bar takes a
// lane of its own and the rows stop short of it, and a fading bar lies over
// the rows' trailing edge and takes no lane.
func scrollingColumn(st *list.State, c tokens.PlatformColors, bars desktop.ScrollBarSetting, items []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		gtx.Constraints = layout.Exact(size)
		bar := columnBar(c, bars)
		list.LayoutScrollbar(gtx, st, bar, list.AnchorFor(bar), items,
			func(gtx layout.Context, w layout.Widget) layout.Dimensions {
				return w(gtx)
			})
		return layout.Dimensions{Size: size}
	}
}
