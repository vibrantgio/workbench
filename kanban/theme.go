package main

import (
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// themed is one theme emission with what the view reads off it. Each
// LiveTheme emission is a static snapshot, so every field resolves at once.
type themed struct {
	platform tokens.PlatformColors
	typ      tokens.Typography
	shaper   *text.Shaper
	spacing  tokens.SpacingScale
	radius   tokens.RadiusScale
	// scrollBars is the platform's "Show scroll bars" setting, read once
	// when the window is composed, so that no frame asks the platform.
	scrollBars desktop.ScrollBarSetting
}

// bar is the scroll bar a region on the content fill draws: the fading
// overlay bar, standing without a fade where the reader asked for scroll
// bars always.
func (t themed) bar() scrollbar.Style {
	s := scrollbar.FromTokens(t.platform, t.platform.ControlBackground)
	if t.scrollBars == desktop.ScrollBarsAlways {
		return s.Always()
	}
	return s
}

// columnWidth is the width of every column. DERIVED: the platform has no
// board and gives no column width. The current kanban apps give a column
// a fixed width near 270 dp.
const columnWidth unit.Dp = 272

// The board's gaps, from the spacing scale. The platform gives no gap
// between a board's columns or cards.
var (
	// columnGap is the space between two columns. DERIVED: the spacing
	// scale's S3.
	columnGap = unit.Dp(tokens.Spacing.S3)
	// boardInset is the space between the board's columns and the window's
	// edges. DERIVED: the spacing scale's S4, the card's own inset.
	boardInset = unit.Dp(tokens.Spacing.S4)
	// cardGap is the space between two cards in a column. DERIVED: the
	// spacing scale's S2, which keeps each card apart and the cards of one
	// column together as one run.
	cardGap = unit.Dp(tokens.Spacing.S2)
)

// themes resolves each theme emission into the snapshot the view reads.
func themes(th rx.Observable[theme.Theme], bars desktop.ScrollBarSetting) rx.Observable[themed] {
	return rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest4(t.Platform, t.Typography, t.Spacing, t.Radius),
			func(n rx.Tuple4[tokens.PlatformColors, tokens.Typography, tokens.SpacingScale, tokens.RadiusScale]) themed {
				return themed{platform: n.First, typ: n.Second, shaper: n.Second.Shaper(),
					spacing: n.Third, radius: n.Fourth, scrollBars: bars}
			})
	})
}

// BackdropLayer fills the window with the content fill. It is the bottom
// layer and re-emits whenever the OS appearance changes.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(colors, func(c tokens.PlatformColors) layout.Widget {
		return backdrop.Widget(c.ControlBackground)
	})
}
