package main

import (
	"image/color"

	"gioui.org/unit"

	"github.com/vibrantgio/prism/tokens"
)

// Palette is the app's view of the prism colour tokens: named roles derived
// from tokens.ColorTokens on every theme emission. Because the spectrum
// window feeds a live OS theme, an OS light/dark switch re-emits the tokens
// and restyles the whole app with no imperative wiring.
type Palette struct {
	Sidebar     color.NRGBA // conversation-list surface
	Separator   color.NRGBA // sidebar header underline
	Heading     color.NRGBA // sidebar heading text
	Row         color.NRGBA // chat-row text
	RowActive   color.NRGBA // selected/hovered chat-row text
	RowSelected color.NRGBA // selected chat-row fill
	RowHovered  color.NRGBA // hovered chat-row fill (over Sidebar)
	Accent      color.NRGBA // selected-row accent bar
	UserBubble  color.NRGBA // user message fill
	UserText    color.NRGBA // user message text
	BotBubble   color.NRGBA // assistant message fill
	BotText     color.NRGBA // assistant message text
	Icon        color.NRGBA // assistant avatar glyph
}

func PaletteFrom(c tokens.ColorTokens) Palette {
	// The hover fill is the selected fill at half opacity, painted over the
	// sidebar surface, so it sits between rest and selected in both schemes.
	hover := c.SurfaceVariant
	hover.A = 128
	return Palette{
		Sidebar:     c.Surface,
		Separator:   c.Outline,
		Heading:     c.OnSurfaceVariant,
		Row:         c.OnSurfaceVariant,
		RowActive:   c.OnSurface,
		RowSelected: c.SurfaceVariant,
		RowHovered:  hover,
		Accent:      c.Primary,
		UserBubble:  c.Primary,
		UserText:    c.OnPrimary,
		BotBubble:   c.SurfaceVariant,
		BotText:     c.OnSurface,
		Icon:        c.Primary,
	}
}

// Static layout dimensions; these do not vary with the colour scheme.
const (
	ChatPaneWidth  unit.Dp = 794
	SidebarWidth   unit.Dp = 260
	AvatarSize     unit.Dp = 40
	DeleteIconSize unit.Dp = 16
)
