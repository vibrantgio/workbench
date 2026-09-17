// The preview: the platform's set with the theme colour standing in for the
// accent, drawn as a picture of an application.
//
// A picture and not a table of names. The colour lands in a handful of places
// — the default button, the rail's pill, the focus ring — and seeing them
// together, on the fills the platform actually paints them on, is the shortest
// honest answer to "what would this colour look like". The set itself, every
// name beside its value, is what the gallery's colour board is for, and a
// second copy of it here only crowded the thing being judged.
//
// Twice over, side by side, because a theme colour has to be seen on both
// appearances and a switch showing one at a time makes that two looks and a
// memory instead of one look.
package main

import (
	"image"
	stdcolor "image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The sample window's own dimensions. They are fixed: a picture of a window
// that grew with the room it was given would be a different picture every
// time the window was resized, and the two halves of the preview would stop
// being the same picture in two appearances.
const (
	// SampleW is the sample window's width: wide enough for a rail and a
	// content pane to read as two regions rather than as a rail with a
	// margin beside it.
	SampleW unit.Dp = 420
	// SampleH is its height, the shortest that still holds a title band, a
	// field, two list rows and a row of buttons without crowding.
	SampleH unit.Dp = 154
	// SampleRadius is the corner a window wears on this platform.
	SampleRadius unit.Dp = 10
	// SampleTitleH is its title band, SampleRailW its rail.
	SampleTitleH unit.Dp = 28
	SampleRailW  unit.Dp = 108
	// SampleRowH is a rail row, the sidebar pattern's own height; the
	// content list's rows are the platform's 20.
	SampleRowH unit.Dp = sidebar.RowHeight
	SampleList unit.Dp = 20
	// SampleInset is the air inside the content pane.
	SampleInset unit.Dp = 10
	// SampleStep is the air between two things stacked in that pane.
	SampleStep unit.Dp = 6
	// SampleFieldH is a text field's own height and SampleButtonH a push
	// button's, both the density scale's.
	SampleFieldH  unit.Dp = unit.Dp(tokens.ComfortableFieldHeight)
	SampleButtonH unit.Dp = unit.Dp(tokens.ComfortableControlHeight)
	// SampleShadow is how far the floating shadow reaches under the window,
	// measured into the platform set's own comment.
	SampleShadow unit.Dp = 12
)

// PreviewLabel heads the preview group and PreviewHint says what it is of.
const (
	PreviewLabel = "Preview"
	PreviewHint  = "The platform's colours with the theme colour where the accent goes, in both appearances."
)

// Preview draws the two samples side by side in the group's box, light
// leading, at their own fixed size and centred in whatever room the box has.
//
// Neither stands on the window's own plane: what is inside a sample is the
// theme, not a picture of it standing on this window's page.
func Preview(light, dark tokens.PlatformColors, ty Type) layout.Widget {
	sun, moon := SampleWindow(light, ty, false), SampleWindow(dark, ty, true)
	return func(gtx layout.Context) layout.Dimensions {
		room := gtx.Constraints.Max
		w, h, gap := gtx.Dp(SampleW), gtx.Dp(SampleH), gtx.Dp(Gap)
		both := 2*w + gap
		x := max(0, (room.X-both)/2)
		y := max(0, (room.Y-h)/2)
		for i, draw := range []layout.Widget{sun, moon} {
			at(gtx, image.Pt(x+i*(w+gap), y), func(gtx layout.Context) {
				gtx.Constraints = layout.Exact(image.Pt(w, h))
				draw(gtx)
			})
		}
		return layout.Dimensions{Size: room}
	}
}

// The sample window's words. They label the parts of a picture of an
// application, so each says what its part is rather than naming anything this
// window does.
const (
	SampleLight    = "Light appearance"
	SampleDark     = "Dark appearance"
	SampleField    = "Search"
	SampleDefault  = "Done"
	SampleOrdinary = "Cancel"
)

// SampleTitleFor is what a sample's title band says: which of the two
// appearances the picture under it is drawn in. Both are on screen at once
// and neither is the appearance the window itself is wearing, so each says
// which it is — the platform labels every thumbnail in its own appearance row
// the same way.
func SampleTitleFor(dark bool) string {
	if dark {
		return SampleDark
	}
	return SampleLight
}

// sampleRails are the rail's rows; the second is the open one.
var sampleRails = [...]string{"Library", "Recents", "Shared"}

// sampleRows are the content list's rows.
var sampleRows = [...]string{"A row", "Another row", "One more"}

// SampleWindow draws a picture of an application in the previewed set: a
// window with a title band, a chrome rail, a content pane holding a field, a
// list and two buttons.
//
// Every fill is the platform's name for what that element is — the plane
// windowBackground, the rail the chrome material, the content the content's
// fill, a seam separatorColor over what it lies on — so the only thing the
// theme colour moves is what the platform moves with its accent: the open
// rail entry's pill, the default button and the focus ring.
//
// The picture carries ONE selection, the rail's, and it is the platform's
// sidebar pill: [sidebar.RowHeight] tall, inset [sidebar.SelectionInset] from
// the rail's edges, cornered at [sidebar.SelectionRadius], filled
// sidebarSelection. A second selection — a full-width bar across the content
// list — read as a slab of the theme colour dropped on the page rather than as
// a row a reader had chosen, and the two marks together said the colour lands
// in more places than it does.
func SampleWindow(c tokens.PlatformColors, ty Type, dark bool) layout.Widget {
	label := vgcolor.Flatten(c.Label, c.ControlBackground)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		h := min(gtx.Dp(SampleH), size.Y)
		win := image.Rect(0, 0, min(gtx.Dp(SampleW), size.X), h)
		if win.Dx() <= 0 || win.Dy() <= 0 {
			return layout.Dimensions{Size: size}
		}
		radius := gtx.Dp(SampleRadius)
		// A floating surface is told by its shadow on this platform, which
		// is the one thing under a window that is not a fill.
		shadow(gtx, win, radius, c.FloatingShadow, gtx.Dp(SampleShadow))
		fillRRect(gtx, win, radius, c.WindowBackground)

		// The picture is a window and has to read as one in both
		// appearances. Its own chrome material is the same fill as the box it
		// stands in under the light appearance, so without a line round it
		// the rail simply is not there: read cold, "the left 108 points of
		// that sample is not there — Library / Recents / Shared floats in the
		// box". A window's own edge is what the platform would draw.
		edge := vgcolor.Flatten(c.Separator, c.SidebarMaterial)

		defer clip.UniformRRect(win, radius).Push(gtx.Ops).Pop()

		titleH := gtx.Dp(SampleTitleH)
		railW := min(gtx.Dp(SampleRailW), win.Dx()/2)
		seam := gtx.Dp(Hairline)

		// The title band and the rail are chrome; the pane beside them is
		// the content. The seams are drawn by the regions above and leading.
		fillRect(gtx, image.Rect(0, 0, win.Dx(), titleH), c.SidebarMaterial)
		fillRect(gtx, image.Rect(0, titleH, railW, win.Dy()), c.SidebarMaterial)
		fillRect(gtx, image.Rect(railW, titleH, win.Dx(), win.Dy()), c.ControlBackground)
		fillRect(gtx, image.Rect(0, titleH-seam, win.Dx(), titleH),
			vgcolor.Flatten(c.Separator, c.SidebarMaterial))
		fillRect(gtx, image.Rect(railW-seam, titleH, railW, win.Dy()),
			vgcolor.Flatten(c.Separator, c.SidebarMaterial))

		textdraw.FillText(gtx, ty.Shaper, ty.Small,
			image.Rect(0, 0, win.Dx(), titleH), 0.5, 0.5,
			vgcolor.Flatten(c.Label, c.SidebarMaterial), SampleTitleFor(dark))

		// The rail's open entry is the platform's sidebar pill, which is
		// where the theme colour lands in the chrome.
		rowH := gtx.Dp(SampleRowH)
		for i, name := range sampleRails {
			top := titleH + i*rowH
			row := image.Rect(0, top, railW-seam, top+rowH)
			foreground := vgcolor.Flatten(c.Label, c.SidebarMaterial)
			if i == 1 {
				at(gtx, row.Min, func(gtx layout.Context) {
					sidebar.PaintSelection(gtx, image.Pt(row.Dx(), rowH), c, false)
				})
				foreground = vgcolor.Flatten(sidebar.SelectionLabel(c, false), sidebar.SelectionFill(c, false))
			}
			textdraw.FillText(gtx, ty.Shaper, ty.Small,
				row.Inset(gtx.Dp(SampleInset)), 0, 0.5, foreground, name)
		}

		pane := image.Rect(railW, titleH, win.Dx(), win.Dy()).Inset(gtx.Dp(SampleInset))
		step := gtx.Dp(SampleStep)

		// A field carries the platform's own fill inside its hairline, and
		// the ring a focused control wears is the one place the theme colour
		// reaches a control that is not filled with it.
		field := image.Rect(pane.Min.X, pane.Min.Y, pane.Max.X, pane.Min.Y+gtx.Dp(SampleFieldH))
		ring := gtx.Dp(3)
		fillRRect(gtx, field.Inset(-ring), gtx.Dp(6)+ring,
			vgcolor.Flatten(c.KeyboardFocusIndicator, c.ControlBackground))
		fillRRect(gtx, field, gtx.Dp(6), c.TextBackground)
		strokeRRect(gtx, field, gtx.Dp(6), seam, c.FieldEdge)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, field.Inset(gtx.Dp(8)), 0, 0.5,
			vgcolor.Flatten(c.PlaceholderText, c.TextBackground), SampleField)

		// Plain rows under it: the list is here to show the content's own
		// fill under the platform's label, and the one selection this picture
		// carries is the rail's.
		listTop := field.Max.Y + step
		listRowH := gtx.Dp(SampleList)
		buttonH := gtx.Dp(SampleButtonH)
		for i, name := range sampleRows {
			top := listTop + i*listRowH
			row := image.Rect(pane.Min.X, top, pane.Max.X, top+listRowH)
			if row.Max.Y > pane.Max.Y-buttonH-step {
				break
			}
			textdraw.FillText(gtx, ty.Shaper, ty.Small, row.Inset(gtx.Dp(6)), 0, 0.5, label, name)
		}

		// The default action is the one control the platform fills with the
		// accent; the ordinary button beside it is the platform's own fill.
		buttonW := min(gtx.Dp(72), pane.Dx()/2-gtx.Dp(4))
		top := pane.Max.Y - buttonH
		def := image.Rect(pane.Max.X-buttonW, top, pane.Max.X, pane.Max.Y)
		ord := image.Rect(def.Min.X-gtx.Dp(8)-buttonW, top, def.Min.X-gtx.Dp(8), pane.Max.Y)
		fillRRect(gtx, def, gtx.Dp(6), c.ControlAccent)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, def, 0.5, 0.5,
			vgcolor.Flatten(c.AlternateSelectedControlText, c.ControlAccent), SampleDefault)
		fillRRect(gtx, ord, gtx.Dp(6), c.PushButtonFill)
		strokeRRect(gtx, ord, gtx.Dp(6), seam, vgcolor.Flatten(c.Separator, c.PushButtonFill))
		textdraw.FillText(gtx, ty.Shaper, ty.Small, ord, 0.5, 0.5,
			vgcolor.Flatten(c.ControlText, c.PushButtonFill), SampleOrdinary)

		strokeRRect(gtx, win, radius, seam, edge)

		return layout.Dimensions{Size: size}
	}
}

// shadow lays the platform's floating shadow under a rounded rectangle: the
// measured peak coverage at the edge, falling to nothing over reach.
func shadow(gtx layout.Context, r image.Rectangle, radius int, col stdcolor.NRGBA, reach int) {
	if reach <= 0 || col.A == 0 {
		return
	}
	for i := reach; i > 0; i-- {
		step := col
		step.A = uint8(int(col.A) * (reach - i + 1) / (reach * reach))
		if step.A == 0 {
			continue
		}
		fillRRect(gtx, r.Inset(-i), radius+i, step)
	}
}
