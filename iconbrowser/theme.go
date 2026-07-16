package main

import (
	"image/color"

	"gioui.org/unit"

	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"
)

// Palette is the app's view of the prism colour tokens, derived per theme
// emission so the OS light/dark switch restyles the app live.
type Palette struct {
	Backdrop color.NRGBA // window background
	Text     color.NRGBA // icon captions
	Muted    color.NRGBA // "no icons match" notice
	Icon     color.NRGBA // the glyphs themselves
}

func PaletteFrom(c tokens.ColorTokens) Palette {
	return Palette{
		Backdrop: c.Background,
		Text:     c.OnSurface,
		Muted:    c.OnSurfaceVariant,
		Icon:     c.Primary,
	}
}

// Static layout dimensions; these do not vary with the colour scheme.
const (
	Padding  unit.Dp = 12
	CellW    unit.Dp = 160 // grid cell width: glyph + caption both fit
	CellH    unit.Dp = 84  // 40 dp glyph, gap, one caption line, padding
	IconSize unit.Dp = 40
)

var Caption = style.Caption
