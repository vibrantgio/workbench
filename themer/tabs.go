// The window's tabs: the strip under the title row and what each tab holds.
package main

import (
	stdcolor "image/color"
	"sync/atomic"

	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/tabs"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// The window's tabs, in the order the strip shows them.
const (
	TabColours = iota
	TabCode
	tabCount
)

// tabTitles are the strip's titles, in strip order.
var tabTitles = [tabCount]string{"Colours", "Code"}

// TabGap is the run from the foot of the tab strip to the first row of a
// tab's content. It is the same run the title row keeps above the strip.
const TabGap = Gap

// tabFill is the fill the tab strip and every tab's content are drawn on: the
// window's own plane, which the groups' boxes are drawn on too.
func tabFill(c tokens.PlatformColors) stdcolor.NRGBA { return PaletteFrom(c).Backdrop }

// tabCells hold each tab's content for the strip. The page stores the
// contents when it is composed, and the strip reads the selected tab's cell
// when it is laid out. The strip's tabs are fixed when the strip is made, and
// the contents change with every emission, so the cells join the two.
type tabCells [tabCount]atomic.Value

// store puts each tab's content in its cell.
func (c *tabCells) store(content [tabCount]layout.Widget) {
	for i, w := range content {
		c[i].Store(w)
	}
}

// tabs is the strip's list of tabs. Each tab's content reads its cell at
// frame time and starts TabGap under the strip.
func (c *tabCells) tabs() []tabs.Tab {
	out := make([]tabs.Tab, tabCount)
	for i := range out {
		cell := &c[i]
		out[i] = tabs.Tab{Title: tabTitles[i], Content: func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				layout.Inset{Top: TabGap}.Layout(gtx, w)
			}
			return layout.Dimensions{Size: size}
		}}
	}
	return out
}

// TabStrip is the live tab strip. It is drawn through [WindowTheme], so the
// titles and the selected tab's mark follow the scheme control with the rest
// of the window. A selection sends [SelectTab].
func TabStrip(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], cells *tabCells) rx.Observable[layout.Widget] {
	return tabs.Tabs(WindowTheme(th, modelObs), tabs.Props{
		Tabs:     cells.tabs(),
		Selected: rx.Map(modelObs, func(m Model) int { return m.Tab }),
		Fill:     tabFill,
		OnSelect: func(gtx layout.Context, idx int) {
			mvu.MessageOp{Message: SelectTab{Index: idx}}.Add(gtx.Ops)
		},
	})
}
