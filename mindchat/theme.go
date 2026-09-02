package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Palette is the app's view of the components colour tokens: named roles derived
// from tokens.ColorTokens on every theme emission. Because the theme
// window feeds a live OS theme, an OS light/dark switch re-emits the tokens
// and restyles the whole app with no imperative wiring.
//
// The levels the roles resolve to are the window grammar's, not this app's
// invention: the transcript is the window's CONTENT GROUND and fills at
// level 0, the Background pin; the conversation list is CHROME FURNITURE and
// is therefore the window's FLOOR, one step UNDER the paper toward the
// scheme's dark extreme in both schemes; levels 2 and 3 are kept for what
// appears and leaves — the settings dialog, the model menu, the undo bar —
// and for edges. A raised thing walks its level from the surface it is
// lying on, so a chip on the transcript ground is level 1 while a chip
// inside the level-2 settings dialog is measured from level 2.
//
// Since ADR-022 elevation runs one way in both schemes: nearer the viewer
// is lighter. This window is darkest at its leading edge and lightest where
// a dialog stands over it, in both schemes alike — no mirror.
type Palette struct {
	Sidebar   color.NRGBA // conversation-list surface — the window's furniture, at the chrome level
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
	// The header picker's own fill, hover and rim are not here: it is
	// components/picker, which derives all three from the level it stands
	// on. What this app still says about it is where it stands — the level-0
	// paper of the transcript's header band — and the component answers the
	// rest.
	ChipText color.NRGBA // label over the dialog's own template chips
	// ModalChip is a chip inside the settings dialog. Its ground is the
	// dialog's level-2 surface, so it rests flush on it and reveals itself
	// with that surface's own state walk rather than reaching for a rung the
	// transcript's chips use.
	ModalChip        color.NRGBA
	ModalChipHovered color.NRGBA
	// Toast is the base of a surface that appears and leaves — today the undo
	// bar. It is a level-2 fill because that is the level elevation keeps for
	// exactly that, and it is its own role rather than a borrowed one: the bar
	// used to tint the selected-row fill, which was fine only while that fill
	// was a neutral step and became a purple-on-purple wash the moment the
	// selection turned into a Primary tint.
	Toast color.NRGBA
	Icon  color.NRGBA // assistant avatar glyph
	Error color.NRGBA // settings fetch-error text
}

func PaletteFrom(c tokens.ColorTokens) Palette {
	// The hover fill is the sidebar's OWN state walk at half strength, painted
	// over the sidebar surface. It can no longer be derived from the selected
	// fill — that one is a Primary tint now — and it must not be: hover is a
	// transient state, and a transient state is a neutral walk from the ground
	// it happens on. That ground is the sidebar's level, and since ADR-022
	// the sidebar's level is the FLOOR — so the walk is taken with StateAt
	// from the floor's own fill rather than from a ramp index. Asking the ramp
	// for the old level-1 step would have kept answering the light scheme
	// right by accident (its floor IS neutral 200) and the dark scheme wrong
	// by a whole level.
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
	hover := c.StateAt(tokens.LevelChrome, tokens.StateHover)
	hover.A = 128
	return Palette{
		Sidebar:   c.SurfaceAt(tokens.LevelChrome),
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

	// SidebarWidth is what the conversation pane takes while it stands. It
	// is a width and not a ratio: the pane is an object floating in the
	// window rather than one half of a split, so it does not grow with the
	// window and the transcript takes everything it does not.
	SidebarWidth unit.Dp = 240

	// PaneMargin is the sliver of window ground the pane floats off its
	// leading, top and bottom edges, and the air the chrome row and the
	// input bar keep off the window's edges so the content area answers the
	// same margin the pane does. The number is the pattern's.
	PaneMargin = pane.MarginDp

	ToggleIconSize     unit.Dp = 20
	FooterIconSize     unit.Dp = 18
	FooterRowHeight    unit.Dp = 46
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
	// SelectRowHeight is the DEFAULT MODEL row, and it is the height the
	// picker field standing in it draws at: one BodyLarge line box over the
	// comfortable density's vertical padding. The caption and the closed
	// control share the line, so the row cannot be shorter than the control.
	SelectRowHeight unit.Dp = 40
	// DefaultPickerWidth is the width of the settings dialog's default-model
	// field, and so of the menu it drops: a field's menu is as wide as the
	// field.
	DefaultPickerWidth unit.Dp = 260
	// ToolbarWidth is the widest the header picker may grow, not the width it
	// draws at — the trigger is sized to its label and clamped to this.
	ToolbarWidth unit.Dp = 230
	// MenuWidth is the width of the header picker's floating surface.
	MenuWidth unit.Dp = 260
	// MenuMaxHeight is the tallest either model menu draws before its rows
	// start scrolling inside it. Both take it, because both list the same
	// catalogue and a window shows one of them at a time.
	//
	// It is a share of the window rather than a count of rows: a catalogue
	// of forty models is a menu 1600 dp tall, and what the number has to
	// keep is a menu that fits the window it opens in with its own host
	// still visible around it. 320 dp is under half the 768 dp window this
	// app is drawn for, which leaves the header or the dialog footer the
	// menu drops from in view either way.
	MenuMaxHeight unit.Dp = 320
)

// The three macOS window controls are measured from the window's own glass
// and from nothing drawn beneath them: this window paints its own title
// bar, and the pane that floats under the controls while it stands is a
// thing the reader can send away — a control that belongs to the window
// cannot shift because a pane the reader dismissed used to be behind it.
// The whole run is the floating pane pattern's, derived from the inset the
// platform's own sidebar apps draw at: 19 in from both edges, 14 across,
// 23 between centres.
var windowButtonRun = pane.Buttons

// ChromeRowHeight is the depth of the window's chrome row — the title row
// across the top of the content area, beside the pane while it stands and
// across the whole window once it is gone.
//
// It is twice the window buttons' centre line and nothing else, which is
// the whole of how the two pane states hold one line. The buttons never
// move; the pane's strip is cut deep enough to hold them and centres its
// controls on their line by the pattern's own arithmetic; a row twice that
// depth centres ITS controls on the same line. So the toggle and the
// new-chat mark stand at one height whether they ride the pane or stand in
// the row that recalls it, and neither drops a rung as the pane comes and
// goes. That jump — the control just clicked leaving from under the
// pointer — is the defect this composition exists to kill.
var ChromeRowHeight = 2 * windowButtonRun.Center

// WindowButtonDiameter, WindowButtonInset and WindowButtonCenter are
// windowButtonRun's fields, named for the call sites that already expect
// them.
var (
	WindowButtonDiameter = windowButtonRun.Diameter
	WindowButtonInset    = windowButtonRun.Leading
	WindowButtonCenter   = windowButtonRun.Center
)
