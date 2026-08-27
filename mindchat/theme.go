package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Palette is the app's view of the components colour tokens: named roles derived
// from tokens.ColorTokens on every theme emission. Because the theme
// window feeds a live OS theme, an OS light/dark switch re-emits the tokens
// and restyles the whole app with no imperative wiring.
//
// The rungs the roles resolve to are the window grammar's, not this app's
// invention: the transcript is the window's CONTENT GROUND and fills at
// level 0, the Background pin; the conversation list is CHROME FURNITURE and
// stands one rung up at level 1; levels 2 and 3 are kept for what appears
// and leaves — the settings dialog, the model menu, the undo bar — and for
// edges. A raised thing walks its rung from the surface it is lying on, so a
// chip on the transcript ground is level 1 while a chip inside the level-2
// settings dialog is measured from level 2.
type Palette struct {
	Sidebar     color.NRGBA // conversation-list surface — chrome furniture, level 1
	Separator   color.NRGBA // sidebar header underline
	Heading     color.NRGBA // sidebar heading text
	Row         color.NRGBA // chat-row text
	RowActive   color.NRGBA // selected/hovered chat-row text
	RowSelected color.NRGBA // selected chat-row fill
	RowHovered  color.NRGBA // hovered chat-row fill (over Sidebar)
	Accent      color.NRGBA // selected-row accent bar
	// Ground is the transcript's resting fill — the header band, the
	// assistant's turns and the space around them. It is the Background pin,
	// level 0: the transcript is the thing the window exists to show, so it
	// is the paper everything else in the pane is measured from, and the
	// window reads lightest in the middle in the light scheme and darkest in
	// the middle in the dark one.
	Ground     color.NRGBA
	UserBubble color.NRGBA // user message fill — a Primary turn, not a rung
	UserText   color.NRGBA // user message text
	BotText    color.NRGBA // assistant message text — the ink pinned to Ground
	// Chip is the header model picker's fill: an interactive region on the
	// level-0 ground, which the ladder says to draw as a level-1 surface
	// because the Background pin is off-ramp and has no step to walk from.
	// ChipHovered is that surface's own one-rung state walk.
	Chip        color.NRGBA
	ChipHovered color.NRGBA
	ChipText    color.NRGBA // chip label and chevron over a raised chip
	// ModalChip is a chip inside the settings dialog. Its ground is the
	// dialog's level-2 surface, so it rests flush on it and reveals itself
	// with that surface's own state walk rather than reaching for a rung the
	// transcript's chips use.
	ModalChip        color.NRGBA
	ModalChipHovered color.NRGBA
	Icon             color.NRGBA // assistant avatar glyph
	Error            color.NRGBA // settings fetch-error text
	Ok               color.NRGBA // settings key-check success icon
}

func PaletteFrom(c tokens.ColorTokens) Palette {
	// The hover fill is the selected fill at half opacity, painted over the
	// sidebar surface, so it sits between rest and selected in both schemes.
	hover := c.Ramps.Neutral.Step(300)
	hover.A = 128
	return Palette{
		Sidebar:     c.Surface,
		Separator:   c.Divider,
		Heading:     c.Ramps.Neutral.Step(700),
		Row:         c.Ramps.Neutral.Step(700),
		RowActive:   c.Ramps.Neutral.Step(900),
		RowSelected: c.Ramps.Neutral.Step(300),
		RowHovered:  hover,
		Accent:      c.Primary,
		Ground:      c.SurfaceAt(tokens.Level0),
		UserBubble:  c.Primary,
		UserText:    c.OnPrimary,
		// The ink over the Background pin is the Text pin, not the neutral
		// ramp's far end: a ground that is off-ramp takes the ink pinned to
		// it. The two coincide in the shipped schemes and need not in a
		// brand's.
		BotText:          c.Text,
		Chip:             c.SurfaceAt(tokens.Level1),
		ChipHovered:      c.StateColor(tokens.RoleNeutral, tokens.Elevation.SurfaceStep(tokens.Level1), tokens.StateHover),
		ChipText:         c.Ramps.Neutral.Step(900),
		ModalChip:        c.SurfaceAt(tokens.Level2),
		ModalChipHovered: c.StateColor(tokens.RoleNeutral, tokens.Elevation.SurfaceStep(tokens.Level2), tokens.StateHover),
		Icon:             c.Primary,
		Error:            c.Error,
		// The token set has no green family; Tailwind green 600 is legible
		// on both schemes' surfaces.
		Ok: color.NRGBA{0x16, 0xa3, 0x4a, 0xff},
	}
}

// roleFont converts a theme Typography role into the gio font it shapes
// with: typeface and weight come from the theme (a zero weight means
// unset, per tokens.FontWeight's convention).
func roleFont(role tokens.TextStyle) font.Font {
	return typeset.Font(role, font.Normal)
}

// roleLabel builds the widget.Label for a role with the role's line box
// installed, capped at maxLines. Set Alignment or Truncator on the result,
// then draw it with typeset.Layout — never with widget.Label.Layout, which
// spends the line height on a gap a capped label does not have.
func roleLabel(role tokens.TextStyle, maxLines int) widget.Label {
	return typeset.Label(role, maxLines)
}

// roleText converts a theme Typography role into the textdraw TextStyle the
// app's FillText calls shape with — typeface, weight and size all come from
// the theme; the single-line ellipsis truncation is the app's own
// convention for chrome text.
func roleText(role tokens.TextStyle) textdraw.TextStyle {
	return textdraw.TextStyle{
		Font:      roleFont(role),
		Alignment: textdraw.Start,
		Size:      unit.Sp(role.Size),
		MaxLines:  1,
		Truncator: "…",
	}
}

// isDarkColor reports whether c reads as a dark ground (Rec. 601 luma below
// mid-grey), selecting the dark chroma style for code highlighting.
func isDarkColor(c color.NRGBA) bool {
	luma := 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
	return luma < 128
}

// Blend mixes over into base at the given alpha (0–255) — the patterns
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
	WaitingDotGap      unit.Dp = 6
	WaitingDotCount            = 3
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
