// The colour board under the Colours tab's two groups. The board itself is
// the shared section in components/gallery/palette, so what is on screen is
// the set it describes and the same board another window shows.
//
// What is kept here is what belongs to this window: the colours and type
// roles the board's own frame is drawn in, and the conversion that hands the
// shown typography over in the shape the board asks for.

package main

import (
	"gioui.org/layout"
	"gioui.org/text"

	"github.com/vibrantgio/components/gallery/palette"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// colourChrome is the board's own frame, drawn in the set the board
// describes. The band a section heading fills is the sidebar's fill, and the
// platform's seam parts it from what is under it.
//
// The two foregrounds are the platform's labels at two strengths, flattened
// onto the fill each one is drawn on: the board's rows stand on the content's
// fill, which is what the shared section fills its bodies with.
func colourChrome(c tokens.PlatformColors) palette.Chrome {
	return palette.Chrome{
		Surface: c.SidebarMaterial,
		Seam:    vgcolor.Flatten(c.Separator, c.SidebarMaterial),
		Text:    vgcolor.Flatten(c.Label, c.ControlBackground),
		Muted:   vgcolor.Flatten(c.SecondaryLabel, c.ControlBackground),
	}
}

// boardType is the shown typography as the shared section takes it: Title 3
// for the board's column heads, the System font for a section's title and a
// row's name, and Subheadline for values and captions, shaped with shaper.
func boardType(shaper *text.Shaper, t tokens.Typography) palette.Type {
	return palette.Type{
		Shaper: shaper,
		Head:   textStyle(t.Title3),
		Label:  textStyle(t.SystemFont),
		Body:   textStyle(t.SystemFont),
		Small:  textStyle(t.Subheadline),
	}
}

// ColourSectionRows is how many rows ColourRows returns: the board's
// heading and its body.
const ColourSectionRows = 2

// ColourRows is the colour board as rows of the Colours tab's column, in the
// set c and the typography t, shaped with shaper.
func ColourRows(c tokens.PlatformColors, t tokens.Typography, shaper *text.Shaper) []layout.Widget {
	return palette.Rows(colourChrome(c), c, boardType(shaper, t))
}
