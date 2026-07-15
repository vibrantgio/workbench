package main

import (
	"image/color"

	"gioui.org/unit"

	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"
)

// Palette is the app's view of the prism colour tokens: a handful of named
// roles derived from tokens.ColorTokens on every theme emission. Because the
// spectrum window feeds a live OS theme, an OS light/dark switch re-emits the
// tokens and restyles the whole app with no imperative wiring.
type Palette struct {
	Backdrop color.NRGBA // window background
	Pane     color.NRGBA // list and dialog surfaces
	Edit     color.NRGBA // text-entry field fill
	Select   color.NRGBA // placeholder text and editor selection
	Text     color.NRGBA // primary text
	Icon     color.NRGBA // accent glyphs and dialog border
	Cover    color.NRGBA // modal scrim over the disabled page
}

func PaletteFrom(c tokens.ColorTokens) Palette {
	return Palette{
		Backdrop: c.Background,
		Pane:     c.Surface,
		Edit:     c.SurfaceVariant,
		Select:   c.OnSurfaceVariant,
		Text:     c.OnSurface,
		Icon:     c.Primary,
		// A scrim darkens regardless of scheme, so it is black-based
		// rather than token-based.
		Cover: color.NRGBA{A: 153},
	}
}

// Static layout dimensions and type styles; these do not vary with the
// colour scheme.
const (
	ModalWidth   unit.Dp = 650
	ModalHeight  unit.Dp = 200
	BorderRadius unit.Dp = 5
	BorderWidth  unit.Dp = 2
	Padding      unit.Dp = 12
)

var (
	H5 = style.H5
	H6 = style.H6
)
