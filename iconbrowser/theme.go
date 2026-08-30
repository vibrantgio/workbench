package main

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/tokens"
)

// Palette is the app's view of the theme colour tokens, derived per theme
// emission so the OS light/dark switch restyles the app live.
type Palette struct {
	Backdrop color.NRGBA // window background
	Text     color.NRGBA // icon captions
	Muted    color.NRGBA // "no icons match" notice
	Icon     color.NRGBA // the glyphs themselves
}

// PaletteFrom resolves the palette: the pinned Background and Text, the pinned
// Primary for the glyphs, and the Neutral ramp's low-contrast text step 700
// for the muted notice.
func PaletteFrom(c tokens.ColorTokens) Palette {
	return Palette{
		Backdrop: c.Background,
		Text:     c.Text,
		Muted:    c.Ramps.Neutral.Step(700),
		Icon:     c.Primary,
	}
}

// Type is the app's view of the theme's Typography: the roles the app draws
// directly, converted to textdraw styles, plus the theme's cached shaper. The
// app builds no shaper and bundles no font of its own — the typeface arrives
// through the theme.
type Type struct {
	Shaper  *text.Shaper
	Caption textdraw.TextStyle // BodySmall: icon captions
	Section textdraw.TextStyle // TitleSmall: the labels over the two sets
	Notice  textdraw.TextStyle // TitleLarge: the "no icons match" notice
}

func TypeFrom(t tokens.Typography) Type {
	return Type{
		Shaper:  t.Shaper(),
		Caption: textStyle(t.BodySmall),
		Section: textStyle(t.TitleSmall),
		Notice:  textStyle(t.TitleLarge),
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
	Padding  unit.Dp = 12
	CellW    unit.Dp = 160 // grid cell width: glyph + caption both fit
	CellH    unit.Dp = 84  // 40 dp glyph, gap, one caption line, padding
	IconSize unit.Dp = 40

	// A mark is shown at the sizes a control draws it at, so its cell is
	// shorter than a Material cell rather than blowing the marks up to
	// match: the band is the largest of those sizes, and every size in it
	// is centred on the band's own line.
	MarkBand  unit.Dp = 24 // glyph band: the largest size a mark is shown at
	MarkCellH unit.Dp = 60 // band, gap, one caption line, padding
	MarkGap   unit.Dp = 12 // between the sizes one mark is shown at
	HeadingH  unit.Dp = 28 // one section label on its own line
)
