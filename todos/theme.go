package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// Palette is the app's view of the platform's colour set: the fills and
// foregrounds this window draws, resolved fresh on every theme emission.
// Because the theme window feeds a live OS theme, an appearance switch
// re-emits the set and restyles the whole app with no imperative wiring.
type Palette struct {
	Backdrop    color.NRGBA // the window's own plane
	Dialog      color.NRGBA // the modal's own plane
	Field       color.NRGBA // the dialog's text-entry fill
	FieldEdge   color.NRGBA // the hairline a field draws around itself
	Label       color.NRGBA // a todo's text
	Secondary   color.NRGBA // a completed todo's text
	Editing     color.NRGBA // the editor's text, on the field
	Placeholder color.NRGBA // the empty field's prompt
	Selection   color.NRGBA // the fill behind selected text
	Icon        color.NRGBA // the add and delete glyphs
	Cover       color.NRGBA // the dim over the page the modal interrupts
}

// PaletteFrom reads this window's fills and foregrounds off the platform's
// set. The list is what the window exists to show and stands on the window's
// own plane, painting no fill of its own. The only fills over it belong to
// the modal, which is a floating surface: the window's plane again, told
// apart by the dim its scrim lays over everything it interrupts. Its
// text-entry field is the platform's text plane inside the hairline a field
// draws around itself.
//
// Every alpha-carrying name is flattened onto the fill it lands on, so what
// reaches Gio is opaque. Scrim is the exception: it covers whatever the
// window happens to be showing, so there is no one fill to flatten it onto.
func PaletteFrom(p tokens.PlatformColors) Palette {
	return Palette{
		Backdrop:    p.WindowBackground,
		Dialog:      p.WindowBackground,
		Field:       p.TextBackground,
		FieldEdge:   p.FieldEdge,
		Label:       vgcolor.Flatten(p.Label, p.WindowBackground),
		Secondary:   vgcolor.Flatten(p.SecondaryLabel, p.WindowBackground),
		Editing:     vgcolor.Flatten(p.Label, p.TextBackground),
		Placeholder: vgcolor.Flatten(p.PlaceholderText, p.TextBackground),
		Selection:   p.SelectedTextBackground,
		Icon:        p.ControlAccent,
		Cover:       p.Scrim,
	}
}

// Type is the app's view of the theme's Typography: the two roles the app
// draws directly, converted to textdraw styles, plus the theme's cached
// shaper. The app builds no shaper and bundles no font of its own — the
// typeface arrives through the theme.
type Type struct {
	Shaper   *text.Shaper
	Headline textdraw.TextStyle // HeadlineSmall: the dialog's editor text
	Title    textdraw.TextStyle // TitleLarge: list rows and placeholder text
}

func TypeFrom(t tokens.Typography) Type {
	return Type{
		Shaper:   t.Shaper(),
		Headline: textStyle(t.HeadlineSmall),
		Title:    textStyle(t.TitleLarge),
	}
}

// textStyle converts one Typography role to a single-line textdraw style.
func textStyle(ts tokens.TextStyle) textdraw.TextStyle {
	f := font.Font{Typeface: font.Typeface(ts.Typeface)}
	if ts.Weight != 0 {
		f.Weight = tokens.FontWeight(ts.Weight)
	}
	return textdraw.TextStyle{Font: f, Alignment: textdraw.Start, Size: unit.Sp(ts.Size), MaxLines: 1, Truncator: "…"}
}

// Static layout dimensions; these do not vary with the colour scheme.
const (
	ModalWidth   unit.Dp = 650
	ModalHeight  unit.Dp = 200
	BorderRadius unit.Dp = 5
	BorderWidth  unit.Dp = 2
	Padding      unit.Dp = 12
)
