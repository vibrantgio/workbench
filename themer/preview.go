// The preview: the platform's set with the theme colour standing in for the
// accent, shown twice over.
//
// Once as a board, which is the set itself — every name the platform answers
// for, with the value each name has under the appearance on screen. That is
// the whole of what was chosen: a theme colour is not a palette, it stands in
// where the platform uses its accent and nothing else moves.
//
// And once as a window, because a board of names is not what anybody is
// judging. The colour lands in a handful of places — the default button, the
// selection, the sidebar's pill, the focus ring — and seeing them together, on
// the fills the platform actually paints them on, is the shortest honest
// answer to "what would this colour look like".
package main

import (
	"image"
	stdcolor "image/color"
	"reflect"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/gallery/palette"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The preview's own dimensions.
const (
	// SampleW is the sample window's width: wide enough for a rail and a
	// content pane to read as two regions rather than as a rail with a
	// margin beside it.
	SampleW unit.Dp = 360
	// SampleH is its height, the shortest that still holds a title band, a
	// heading, a field, a list and a row of buttons without crowding.
	SampleH unit.Dp = 260
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
	SampleInset unit.Dp = 12
	// SampleFieldH is a text field's own height and SampleButtonH a push
	// button's, both the density scale's.
	SampleFieldH  unit.Dp = unit.Dp(tokens.ComfortableFieldHeight)
	SampleButtonH unit.Dp = unit.Dp(tokens.ComfortableControlHeight)
	// SampleShadow is how far the floating shadow reaches under the window,
	// measured into the platform set's own comment.
	SampleShadow unit.Dp = 12
)

// PreviewLabel heads the preview and PreviewHint says what it is of.
const (
	PreviewLabel = "Preview"
	PreviewHint  = "the platform's colours with the theme colour standing in for the accent"
)

// previewChrome is the board's own frame, drawn in the set the board is
// describing: the board stands inside the preview, and the preview is the
// theme.
func previewChrome(c tokens.PlatformColors) palette.Chrome {
	return palette.Chrome{
		Surface: c.SidebarMaterial,
		Seam:    vgcolor.Flatten(c.Separator, c.SidebarMaterial),
		Text:    vgcolor.Flatten(c.Label, c.WindowBackground),
		Muted:   vgcolor.Flatten(c.SecondaryLabel, c.WindowBackground),
	}
}

// story is the window's typography as the board takes it: the four roles it
// sets text in, with the theme's cached shaper.
func (t Type) story() palette.Type {
	return palette.Type{
		Shaper: t.Shaper,
		Head:   t.Head,
		Label:  t.Label,
		Body:   t.Body,
		Small:  t.Small,
	}
}

// Preview lays the two halves out side by side: the window on the leading
// edge, where a reader looks first, and the board beside it with the room
// left over.
//
// Both stand on the previewed set's own plane, not the window's. For a moment
// after a pick those are the same thing, and that is the point: what is inside
// the panel is the theme, not a picture of it standing on this window's page.
func Preview(p Palette, c tokens.PlatformColors, ty Type, st *list.State) layout.Widget {
	board := Board(c, ty, st)
	sample := SampleWindow(c, ty)
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		labelH := gtx.Dp(RowLabelH)
		head := image.Rect(0, 0, size.X, labelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, p.Text, PreviewLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, head, 1, 0.5, p.Muted, PreviewHint)

		panel := image.Rect(0, labelH, size.X, size.Y)
		if panel.Dy() <= 0 {
			return layout.Dimensions{Size: size}
		}
		fillRRect(gtx, panel, gtx.Dp(Radius), c.WindowBackground)
		strokeRRect(gtx, panel, gtx.Dp(Radius), gtx.Dp(Hairline),
			vgcolor.Flatten(c.Separator, p.Backdrop))

		inner := panel.Inset(gtx.Dp(SampleInset))
		sampleW := min(gtx.Dp(SampleW), inner.Dx())
		at(gtx, inner.Min, func(gtx layout.Context) {
			gtx.Constraints = layout.Exact(image.Pt(sampleW, inner.Dy()))
			sample(gtx)
		})
		if left := inner.Dx() - sampleW - gtx.Dp(Gap); left > 0 {
			at(gtx, image.Pt(inner.Min.X+sampleW+gtx.Dp(Gap), inner.Min.Y), func(gtx layout.Context) {
				gtx.Constraints = layout.Exact(image.Pt(left, inner.Dy()))
				board(gtx)
			})
		}
		return layout.Dimensions{Size: size}
	}
}

// What heads the board.
const (
	BoardLabel = "The colour set previewed"
	BoardHint  = "every name the platform answers for · in the order the set carries them · a coverage is written after the value it is a coverage of"
)

// The board's columns: the name, the swatch, and the value written out.
const (
	BoardNameW   unit.Dp = 210
	BoardSwatchW unit.Dp = 26
	BoardValueW  unit.Dp = 130
	BoardRowH    unit.Dp = 22
	BoardColGap  unit.Dp = 14
)

// Board is the set itself: every name the platform answers for, with the
// value each has in the set being previewed, scrolling in its own column.
//
// One column and not two, because there is one set to show: the theme colour
// stands in for the accent under the appearance on screen, and the other
// appearance is one press of the switch away rather than a column beside this
// one that nothing here previews.
func Board(c tokens.PlatformColors, ty Type, st *list.State) layout.Widget {
	chrome := previewChrome(c)
	items := []layout.Widget{
		palette.Heading(chrome, c, ty.story(), BoardLabel, BoardHint),
		palette.Body(c, boardRows(chrome, c, ty)),
	}
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(gtx.Constraints.Max)
		list.LayoutScrollbar(gtx, st, scrollbar.FromTokens(c, c.WindowBackground), list.Overlay, items,
			func(gtx layout.Context, w layout.Widget) layout.Dimensions { return w(gtx) })
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// boardRows draws the previewed set as a table: the column heads once, then a
// row per field — its platform name at the leading edge, its value as a
// swatch, and the value written out beside it.
//
// The fields are read by reflection rather than listed, so a name added to
// the set appears here without an edit. A name this window forgot to list
// would be a name nobody ever judges, which is the one failure a preview
// cannot afford.
//
// A coverage is composited onto the previewed set's own content plane before
// it is shown as a swatch, and written out beside it coverage and all — which
// is the only place a coverage and an opaque colour can be told apart.
func boardRows(p palette.Chrome, c tokens.PlatformColors, ty Type) func(gtx layout.Context, width int) int {
	t := reflect.TypeOf(tokens.PlatformColors{})
	v := reflect.ValueOf(c)
	return func(gtx layout.Context, width int) int {
		rowH := gtx.Dp(BoardRowH)
		gap := gtx.Dp(BoardColGap)
		nameW, swatchW, valueW := gtx.Dp(BoardNameW), gtx.Dp(BoardSwatchW), gtx.Dp(BoardValueW)
		line := gtx.Dp(Hairline)

		swatchX := nameW + gap
		valueX := swatchX + swatchW + gap
		end := min(width, valueX+valueW)

		write := func(x, y, room int, style textdraw.TextStyle, col stdcolor.NRGBA, s string) {
			box := image.Rect(x, y, x+room, y+rowH)
			textdraw.FillText(gtx, ty.Shaper, style, box, 0, 0.5, col,
				palette.FitLine(gtx, ty.Shaper, style, s, room))
		}

		y := 0
		write(0, y, nameW, ty.Head, p.Muted, palette.NameHead)
		write(valueX, y, valueW, ty.Head, p.Muted, BoardValueHead)
		y += rowH
		fillRect(gtx, image.Rect(0, y, end, y+line), p.Seam)
		y += line

		for i := range t.NumField() {
			val := v.Field(i).Interface().(stdcolor.NRGBA)
			write(0, y, nameW, ty.Body, p.Text, t.Field(i).Name)
			box := image.Rect(swatchX, y+2, swatchX+swatchW, y+rowH-2)
			fillRect(gtx, box, vgcolor.Flatten(val, c.ControlBackground))
			strokeRect(gtx, box, line, p.Seam)
			write(valueX, y, valueW, ty.Small, p.Text, inventory.Hex(val))
			y += rowH
		}
		return y
	}
}

// BoardValueHead names the one value column: the set on screen, not a
// catalogue beside it.
const BoardValueHead = "Value"

// strokeRect outlines a rectangle inside its own bounds.
func strokeRect(gtx layout.Context, r image.Rectangle, width int, c stdcolor.NRGBA) {
	if width <= 0 || r.Dx() <= 0 || r.Dy() <= 0 {
		return
	}
	fillRect(gtx, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+width), c)
	fillRect(gtx, image.Rect(r.Min.X, r.Max.Y-width, r.Max.X, r.Max.Y), c)
	fillRect(gtx, image.Rect(r.Min.X, r.Min.Y, r.Min.X+width, r.Max.Y), c)
	fillRect(gtx, image.Rect(r.Max.X-width, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// The sample window's words. They label the parts of a picture of an
// application, so each says what its part is rather than naming anything this
// window does.
const (
	SampleTitle    = "Sample"
	SampleHeading  = "Colours in place"
	SampleLine     = "Where the platform uses its accent, the theme colour stands."
	SampleField    = "Search"
	SampleDefault  = "Done"
	SampleOrdinary = "Cancel"
)

// sampleRails are the rail's rows; the second is the open one.
var sampleRails = [...]string{"Library", "Recents", "Shared"}

// sampleRows are the content list's rows; the first is the selected one.
var sampleRows = [...]string{"The selected row", "Another row", "One more"}

// SampleWindow draws a picture of an application in the previewed set: a
// window with a title band, a chrome rail, a content pane holding a field, a
// list and two buttons.
//
// Every fill is the platform's name for what that element is — the plane
// windowBackground, the rail the chrome material, the content the content's
// fill, a seam separatorColor over what it lies on — so the only thing the
// theme colour moves is what the platform moves with its accent: the open
// rail entry's pill, the selected row, the default button and the focus ring.
func SampleWindow(c tokens.PlatformColors, ty Type) layout.Widget {
	label := vgcolor.Flatten(c.Label, c.ControlBackground)
	secondary := vgcolor.Flatten(c.SecondaryLabel, c.ControlBackground)
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
			vgcolor.Flatten(c.Label, c.SidebarMaterial), SampleTitle)

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
		line := gtx.Dp(RowLabelH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label,
			image.Rect(pane.Min.X, pane.Min.Y, pane.Max.X, pane.Min.Y+line), 0, 0.5, label, SampleHeading)
		textdraw.FillText(gtx, ty.Shaper, ty.Small,
			image.Rect(pane.Min.X, pane.Min.Y+line, pane.Max.X, pane.Min.Y+2*line), 0, 0.5, secondary, SampleLine)

		// A field carries the platform's own fill inside its hairline, and
		// the ring a focused control wears is the one place the theme colour
		// reaches a control that is not filled with it.
		fieldTop := pane.Min.Y + 2*line + gtx.Dp(8)
		field := image.Rect(pane.Min.X, fieldTop, pane.Max.X, fieldTop+gtx.Dp(SampleFieldH))
		ring := gtx.Dp(3)
		fillRRect(gtx, field.Inset(-ring), gtx.Dp(6)+ring,
			vgcolor.Flatten(c.KeyboardFocusIndicator, c.ControlBackground))
		fillRRect(gtx, field, gtx.Dp(6), c.TextBackground)
		strokeRRect(gtx, field, gtx.Dp(6), seam, c.FieldEdge)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, field.Inset(gtx.Dp(8)), 0, 0.5,
			vgcolor.Flatten(c.PlaceholderText, c.TextBackground), SampleField)

		// The list's selected row is the platform's emphasized selection,
		// which follows the accent, under the label it pairs with.
		listTop := field.Max.Y + gtx.Dp(10)
		listRowH := gtx.Dp(SampleList)
		for i, name := range sampleRows {
			top := listTop + i*listRowH
			row := image.Rect(pane.Min.X, top, pane.Max.X, top+listRowH)
			if row.Max.Y > pane.Max.Y-gtx.Dp(SampleButtonH)-gtx.Dp(10) {
				break
			}
			foreground := label
			if i == 0 {
				fillRRect(gtx, row, gtx.Dp(4), c.SelectedContentBackground)
				foreground = vgcolor.Flatten(c.AlternateSelectedControlText, c.SelectedContentBackground)
			}
			textdraw.FillText(gtx, ty.Shaper, ty.Small, row.Inset(gtx.Dp(6)), 0, 0.5, foreground, name)
		}

		// The default action is the one control the platform fills with the
		// accent; the ordinary button beside it is the platform's own fill.
		buttonH := gtx.Dp(SampleButtonH)
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
