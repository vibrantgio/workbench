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
	Error       color.NRGBA // settings fetch-error text
	Ok          color.NRGBA // settings key-check success icon
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
		Error:       c.Error,
		// The token set has no green family; Tailwind green 600 is legible
		// on both schemes' surfaces.
		Ok: color.NRGBA{0x16, 0xa3, 0x4a, 0xff},
	}
}

// isDarkColor reports whether c reads as a dark ground (Rec. 601 luma below
// mid-grey), selecting the dark chroma style for code highlighting.
func isDarkColor(c color.NRGBA) bool {
	luma := 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
	return luma < 128
}

// Blend mixes over into base at the given alpha (0–255) — the cadence
// toast tint recipe, reused by the undo bar.
func Blend(base, over color.NRGBA, alpha uint8) color.NRGBA {
	a := float32(alpha) / 255
	return color.NRGBA{
		R: uint8(float32(over.R)*a + float32(base.R)*(1-a)),
		G: uint8(float32(over.G)*a + float32(base.G)*(1-a)),
		B: uint8(float32(over.B)*a + float32(base.B)*(1-a)),
		A: 0xff,
	}
}

// Static layout dimensions; these do not vary with the colour scheme.
const (
	ChatPaneWidth    unit.Dp = 794
	AvatarSize       unit.Dp = 40
	DeleteIconSize   unit.Dp = 16
	AddIconSize      unit.Dp = 18
	SettingsIconSize unit.Dp = 22
	UndoBarRadius    unit.Dp = 6
	UndoBarMargin    unit.Dp = 24

	BrandRowHeight     unit.Dp = 52
	ToggleIconSize     unit.Dp = 20
	FooterIconSize     unit.Dp = 18
	FooterRowHeight    unit.Dp = 46
	RailThresholdWidth unit.Dp = 110
	StreamDotSize      unit.Dp = 7
	StreamDotSlot      unit.Dp = 15
	RenameFieldHeight  unit.Dp = 48
	RenameButtonHeight unit.Dp = 44
	RenameButtonWidth  unit.Dp = 100

	// Settings modal geometry.
	SettingsBodyHeight  unit.Dp = 300
	SettingsListWidth   unit.Dp = 150
	SettingsRowHeight   unit.Dp = 28
	SettingsFieldHeight unit.Dp = 42
	SettingsCaptionRow  unit.Dp = 22
	SettingsIconBtn     unit.Dp = 18
	SettingsPanelInset  unit.Dp = 6
	TemplateRowHeight   unit.Dp = 26
	SelectRowHeight     unit.Dp = 32
	DropChipWidth       unit.Dp = 260
	ModelRowHeight      unit.Dp = 26
	ModelDotSlot        unit.Dp = 16
	ModelDotSize        unit.Dp = 6

	// Chat header (model picker chip) geometry.
	HeaderRowHeight unit.Dp = 44
	ChipHeight      unit.Dp = 28
	ChipWidth       unit.Dp = 230
	ChipRadius      unit.Dp = 14
	MenuWidth       unit.Dp = 260
	MenuMaxHeight   unit.Dp = 320
)
