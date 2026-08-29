package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Palette is the app's view of the components colour tokens: named roles derived
// from tokens.ColorTokens on every theme emission. Because the theme
// window feeds a live OS theme, an OS light/dark switch re-emits the tokens
// and restyles the whole app with no imperative wiring.
//
// The storeys the roles resolve to are the window grammar's, not this app's
// invention: the transcript is the window's CONTENT GROUND and fills at
// level 0, the Background pin; the conversation list is CHROME FURNITURE and
// is therefore the window's FLOOR, one step UNDER the paper toward the
// scheme's dark extreme in both schemes; levels 2 and 3 are kept for what
// appears and leaves — the settings dialog, the model menu, the undo bar —
// and for edges. A raised thing walks its storey from the surface it is
// lying on, so a chip on the transcript ground is level 1 while a chip
// inside the level-2 settings dialog is measured from level 2.
//
// Since ADR-022 the ladder runs one way in both schemes: nearer the viewer
// is lighter. This window is darkest at its leading edge and lightest where
// a dialog stands over it, on paper and on slate alike — no mirror. The
// light scheme does not move for the re-founding (its floor is the neutral
// 200 the sidebar already wore); the dark sidebar drops from #222222 to
// #0C0C0C, which is the ruling landing.
type Palette struct {
	Sidebar   color.NRGBA // conversation-list surface — chrome furniture, the floor
	Separator color.NRGBA // sidebar header underline
	Heading   color.NRGBA // sidebar heading text
	Row       color.NRGBA // chat-row text
	RowActive color.NRGBA // selected/hovered chat-row text
	// RowSelected is the fill of the conversation the window is showing, and
	// RowHovered the fill under the pointer. The two are deliberately
	// different kinds of colour, not two steps of one: what is *chosen* is
	// Primary-tinted, what is *transient* is a neutral walk. A list has to be
	// able to say "this is the one you are reading" and "something is
	// happening here" at once, and it cannot if both are neutral steps — which
	// is what they were before, and why the open conversation and a hovered
	// one read as the same thing.
	RowSelected color.NRGBA
	RowHovered  color.NRGBA
	Accent      color.NRGBA // selected-row accent bar
	// Ground is the transcript's resting fill — the header band, the
	// assistant's turns and the space around them. It is the Background pin,
	// level 0: the transcript is the thing the window exists to show, so it
	// is the paper everything else in the pane is measured from, and it is
	// lighter than the furniture beside it in BOTH schemes — the window
	// reads lighter toward its middle on paper and on slate alike.
	Ground     color.NRGBA
	UserBubble color.NRGBA // user message fill — a Primary turn, not a rung
	UserText   color.NRGBA // user message text
	BotText    color.NRGBA // assistant message text — the ink pinned to Ground
	// The header model picker's own fill, hover and rim are no longer here:
	// it is components/chip now, which derives all three from the storey it
	// stands on. What this app still says about it is where it stands —
	// the level-0 paper of the transcript's header band — and the component
	// answers the rest.
	ChipText color.NRGBA // label and chevron over the dialog's own chips
	// ModalChip is a chip inside the settings dialog. Its ground is the
	// dialog's level-2 surface, so it rests flush on it and reveals itself
	// with that surface's own state walk rather than reaching for a rung the
	// transcript's chips use.
	ModalChip        color.NRGBA
	ModalChipHovered color.NRGBA
	// Toast is the base of a surface that appears and leaves — today the undo
	// bar. It is a level-2 fill because that is the rung the ladder keeps for
	// exactly that, and it is its own role rather than a borrowed one: the bar
	// used to tint the selected-row fill, which was fine only while that fill
	// was a neutral step and became a purple-on-purple wash the moment the
	// selection turned into a Primary tint.
	Toast color.NRGBA
	Icon  color.NRGBA // assistant avatar glyph
	Error color.NRGBA // settings fetch-error text
	Ok    color.NRGBA // settings key-check success icon
}

func PaletteFrom(c tokens.ColorTokens) Palette {
	// The hover fill is the sidebar's OWN state walk at half strength, painted
	// over the sidebar surface. It can no longer be derived from the selected
	// fill — that one is a Primary tint now — and it must not be: hover is a
	// transient state, and a transient state is a neutral walk from the ground
	// it happens on. That ground is the sidebar's storey, and since ADR-022
	// the sidebar's storey is the FLOOR — so the walk is taken with StateAt
	// from the floor's own fill rather than from a ramp index. Asking the ramp
	// for the old level-1 step would have kept answering the light scheme
	// right by accident (its floor IS neutral 200) and the dark scheme wrong
	// by a whole storey.
	//
	// Half strength rather than the full step, and that is the re-derivation
	// the tint forced. On paper the neutral hover step lands at luma 212 and
	// the Primary-tinted selection at 215, so a full-strength hover would sit
	// a hair *past* selected and the two would trade places; half of it lands
	// at 221, between the resting surface's 232 and the selection's 215, which
	// is the order a reader expects — none of those three numbers moved with
	// the re-founding. On slate the whole trio dropped with the floor: rest is
	// luma 12, the half-step lifts to 23 and the tinted row sits at 34, so the
	// same soft lift still leaves the hue to do the choosing.
	hover := c.StateAt(tokens.LevelFloor, tokens.StateHover)
	hover.A = 128
	return Palette{
		Sidebar:   c.SurfaceAt(tokens.LevelFloor),
		Separator: c.Divider,
		Heading:   c.Ramps.Neutral.Step(700),
		Row:       c.Ramps.Neutral.Step(700),
		RowActive: c.Ramps.Neutral.Step(900),
		// The open conversation wears the Primary ramp's tinted end — the
		// same step the vault's current note wears, and for the same reason:
		// a tint says "this is the one you are looking at" where a neutral
		// step can only say "something happened here".
		RowSelected: c.Ramps.Primary.Step(300),
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
		ChipText:         c.Ramps.Neutral.Step(900),
		ModalChip:        c.SurfaceAt(tokens.Level2),
		ModalChipHovered: c.StateAt(tokens.Level2, tokens.StateHover),
		Toast:            c.SurfaceAt(tokens.Level2),
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

	BrandRowHeight unit.Dp = 52

	// SidebarGutter is the leading inset the sidebar's rows and labels share.
	SidebarGutter unit.Dp = 16

	// WindowButtonGap is the air between the last window control and whatever
	// the brand row puts beside it. The measurement the window reports is the
	// bare trailing edge of the third circle and carries no breathing room of
	// its own, so the row owes the buttons the same gap it would leave between
	// any two things standing in it.
	WindowButtonGap unit.Dp = 12

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
	// The header band is as deep as the brand row beside it, and both are the
	// depth the platform's unified toolbar was measured at. They are the two
	// halves of one strip across the top of a window that now paints its own,
	// so a reader sees one band rather than a step where the split falls — and
	// the model chip's centre line lands on the window controls' line instead
	// of four dp above it. The band was 44 while the strip above it was the
	// system's and the two never had to agree.
	HeaderRowHeight = BrandRowHeight
	// ChipHeight is the settings dialog's own hand-rolled chips. The header
	// model picker no longer takes a height from here: it is components/chip
	// and draws at the theme density's control height.
	ChipHeight unit.Dp = 28
	// ChipWidth is the widest the header picker may grow, not the width it
	// draws at — the chip is sized to its label and clamped to this.
	ChipWidth     unit.Dp = 230
	MenuWidth     unit.Dp = 260
	MenuMaxHeight unit.Dp = 320
)

// The three macOS window controls stand in the brand row: this window
// paints its own title bar, so the row at the top of the sidebar is the
// band it gives them and nothing native stands above it. desktop.ButtonRunIn
// derives their whole geometry from the band's own height — the buttons are
// centred in whatever band a window has, and their leading inset equals
// their top inset — so BrandRowHeight is the only input; at 52dp that is
// 19dp leading and centred, 14dp across, the same numbers the platform
// reference was measured at.
var windowButtonRun = desktop.ButtonRunIn(BrandRowHeight)

// WindowButtonDiameter, WindowButtonInset and WindowButtonCenter are
// windowButtonRun's fields, named for the call sites that already expect
// them.
var (
	WindowButtonDiameter = windowButtonRun.Diameter
	WindowButtonInset    = windowButtonRun.Leading
	WindowButtonCenter   = windowButtonRun.Center
)
