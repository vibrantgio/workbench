package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Palette is the app's view of the platform's colour set: the named places
// this window paints, each resolved from tokens.PlatformColors on every
// theme emission. Because the theme window feeds a live OS theme, an OS
// light/dark switch re-emits the set and restyles the whole app with no
// imperative wiring.
//
// Which platform name each place takes is what the window IS on macOS, not
// this app's invention: the transcript is the content, so it wears
// ControlBackground; the conversation pane is chrome, so it wears the chrome
// material a sidebar carries; a surface that appears and leaves — the
// settings dialog, the model menu, the undo bar — is a floating surface and
// wears the window's own plane under the platform's shadow. Every
// alpha-carrying name is flattened onto the fill it lands on, in encoded
// sRGB, so what Gio is handed is opaque.
type Palette struct {
	Sidebar   color.NRGBA // conversation-pane fill — the chrome material
	Separator color.NRGBA // the pane header's seam, over the chrome
	Heading   color.NRGBA // pane heading and chrome glyphs
	Row       color.NRGBA // chat-row text on the chrome
	RowActive color.NRGBA // text on the selected row's pill
	// RowSelected is the fill under the conversation the window is showing.
	// It is the platform's sidebar pill, which patterns/sidebar draws and
	// this app does not paint itself; the colour is kept here for the rows
	// that only measure against it. A sidebar row does NOT tint under the
	// pointer on this platform, so there is no hovered fill beside it.
	RowSelected color.NRGBA
	Accent      color.NRGBA // the in-flight dot and the waiting indicator
	// Transcript is the transcript's fill: the content plane, which is what
	// the platform gives a window's document area.
	Transcript color.NRGBA
	// TurnText is the foreground a turn's words are set in — the assistant's
	// document on the transcript, and the user's text on the card raised over
	// it. One colour for both, because both are the conversation's own prose;
	// what tells the two apart is the card, not a second grey.
	TurnText color.NRGBA
	// Note is the system note's foreground: a line that carries no status of
	// its own takes the platform's secondary label over the transcript.
	Note color.NRGBA
	// ChipText is the label over the dialog's own template chips, which stand
	// on the dialog's plane.
	ChipText color.NRGBA
	// ModalChip is a chip inside the settings dialog: the fill an ordinary
	// push button wears on this platform, which is what the reference
	// measures a resting chip at. It does not tint under the pointer.
	ModalChip color.NRGBA
	// Toast is a surface that appears and leaves — the undo bar and the
	// settings dialog alike. It is the window's own plane, which is what the
	// platform fills a floating surface with; its shadow is the caller's.
	Toast color.NRGBA
	// Panel is the platform's grouped box: the fill a small inset region
	// takes inside a dialog. It carries no hairline and no shadow.
	Panel color.NRGBA
	// FloatingText and FloatingHeading are the two text strengths over a
	// floating surface, flattened onto the plane it fills with — which is not
	// the chrome Row and Heading are flattened onto.
	FloatingText    color.NRGBA
	FloatingHeading color.NRGBA
	// ListSelected is the fill under the selected row of a content list, and
	// ListSelectedText the foreground on it. A content list's selected row and
	// a sidebar's are two different colours on this platform.
	ListSelected     color.NRGBA
	ListSelectedText color.NRGBA
	Icon             color.NRGBA // assistant avatar glyph
	Error            color.NRGBA // settings fetch-error text
}

// PaletteFrom resolves the window's places from the platform's set.
func PaletteFrom(c tokens.PlatformColors) Palette {
	chrome := c.SidebarMaterial
	content := c.ControlBackground
	floating := c.WindowBackground
	return Palette{
		Sidebar:         chrome,
		Separator:       vgcolor.Flatten(c.Separator, chrome),
		Heading:         vgcolor.Flatten(c.SecondaryLabel, chrome),
		Row:             vgcolor.Flatten(c.Label, chrome),
		RowActive:       vgcolor.Flatten(sidebar.SelectionLabel(c, false), sidebar.SelectionFill(c, false)),
		RowSelected:     sidebar.SelectionFill(c, false),
		Accent:          c.ControlAccent,
		Transcript:      content,
		TurnText:        vgcolor.Flatten(c.Label, content),
		Note:            vgcolor.Flatten(c.SecondaryLabel, content),
		ChipText:        vgcolor.Flatten(c.Label, c.PushButtonFill),
		ModalChip:       c.PushButtonFill,
		Toast:           floating,
		Panel:           c.CardFill,
		FloatingText:    vgcolor.Flatten(c.Label, floating),
		FloatingHeading: vgcolor.Flatten(c.SecondaryLabel, floating),

		ListSelected:     c.SelectedContentBackground,
		ListSelectedText: vgcolor.Flatten(c.AlternateSelectedControlText, c.SelectedContentBackground),

		Icon:  c.ControlAccent,
		Error: c.SystemRed,
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

// isDarkColor reports whether c reads as a dark fill (Rec. 601 luma below
// mid-grey), selecting the dark chroma style for code highlighting.
func isDarkColor(c color.NRGBA) bool {
	luma := 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
	return luma < 128
}

// Static layout dimensions; these do not vary with the colour scheme.
const (
	ChatPaneWidth unit.Dp = 794

	// TurnMeasure is the widest a turn's prose may lay out: the
	// conversation's reading measure. Past it a wider window gives the
	// reader more of the conversation rather than longer lines.
	//
	// The number is measured, not chosen. BodyLarge sets at sixteen, and
	// shaping prose in it measures an average advance of 7.19 dp a
	// character, so 540 dp is 75 characters of this system's body text —
	// the top of the 60 to 75 characters a line that typography has long
	// held a reader can walk back along without losing the next one.
	TurnMeasure unit.Dp = 540

	// UserCardMeasure is the widest the user's card may grow — the card
	// itself, not the words in it: 432 dp of prose plus the card's own S4
	// inset either side, which is 16 dp on the shipped spacing scale. The
	// 432 is 60 characters at the same 7.19 dp advance, the BOTTOM of the
	// band [TurnMeasure] sits at the top of: a prompt is still prose, so
	// its lines may not drop under the band. Its prose stopping a fifth
	// short of the answer's is what makes the two turns tell each other
	// apart by shape as well as by side, before any colour is read.
	UserCardMeasure unit.Dp = 432 + 2*16

	// TurnInset is the air every turn keeps off the transcript's edges and
	// its neighbours.
	TurnInset unit.Dp = 12

	// AvatarGutter is the column the assistant's mark stands in, which the
	// answer beside it is indented past.
	AvatarGutter unit.Dp = 50

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

	// PaneMargin is the sliver of backdrop the pane floats off its
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
	// MenuMaxHeight is the tallest the HEADER's model menu draws before its
	// rows start scrolling inside it. The settings dialog's field takes no
	// number of its own: it is told the room its body leaves and the picker
	// caps the plane to that.
	//
	// It is a share of the window rather than a count of rows: a catalogue
	// of forty models is a menu 1600 dp tall, and what the number has to
	// keep is a menu that fits the window it opens in with its own host
	// still visible around it. 320 dp is under half the 768 dp window this
	// app is drawn for, which leaves the header the menu drops from in view.
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
// the row that recalls it, and neither drops a step as the pane comes and
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
