// The window's scroll bar.

package main

import (
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/tokens"
)

// scrollBarStyle is every bar in this window: the fading overlay bar the
// colours alone give, standing without fading where the reader asked for
// scroll bars always. The setting is read once where the window is
// composed, so no frame asks the platform and no still render depends on
// the machine it runs on.
func scrollBarStyle(s scrollbar.Style, bars desktop.ScrollBarSetting) scrollbar.Style {
	if bars == desktop.ScrollBarsAlways {
		return s.Always()
	}
	return s
}

// columnBar is the bar of the page's document column: made from the
// colour set the column's content is drawn in, on the content's fill, and
// following the setting the window read when it was composed.
func columnBar(c tokens.PlatformColors, bars desktop.ScrollBarSetting) scrollbar.Style {
	return scrollBarStyle(scrollbar.FromTokens(c, c.ControlBackground), bars)
}
