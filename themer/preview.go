// The preview: the platform's colours with the theme colour standing in for
// the accent, drawn as a picture of an application in the scheme the window
// is on.
//
// A picture and not a table of names. The colour lands in a handful of places
// — the default button, the sidebar's pill, the focus ring — and seeing them
// together, on the fills the platform actually paints them on, is the
// shortest honest answer to "what would this theme look like". The colour set
// itself, every name beside its value, is what the gallery's colour board is
// for, and a second copy of it here only crowded the thing being judged.
//
// One picture and not two. The appearance switch at the top of the window
// moves the whole window, this sample included, so the sample is always the
// scheme in force: what a reader sees under the choices is the theme those
// choices make.
//
// It carries one of everything the choices above can move — a sidebar with
// the selection pill, a toolbar with the platform's window controls and its
// two push buttons, a text field, a heading with a badge beside it, prose and
// a fenced code block in the chosen code face and highlighter style — so no
// choice in this window changes nothing on screen.
//
// The push buttons stand in the toolbar and not at the foot of the pane. The
// window's own default button is the last thing in the page, and a second
// filled button at the foot of the picture directly above it read cold as two
// default buttons in one column: "56px apart, same capsule, same size class
// — nothing tells you which of the three is clickable". The toolbar is where
// this platform puts a window's own actions anyway.
package main

import (
	"image"
	stdcolor "image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/badge"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/patterns/sidebar"
	"github.com/vibrantgio/textdraw"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// The sample window's own dimensions. Everything here is a fixed part of a
// window's chrome — a band, a sidebar, the air inside a pane — and what is
// left over is the pane, so the picture grows with the room the group has
// rather than standing in the middle of it at a size written down here.
const (
	// SampleRadius is the corner a window wears on this platform, and
	// SampleShadow how far its floating shadow reaches. The shadow is also
	// the run left clear round the picture, since it has to fall somewhere.
	SampleRadius unit.Dp = 10
	SampleShadow unit.Dp = 8
	// SampleToolbarH is the sample's toolbar and SampleRailW its sidebar.
	SampleToolbarH unit.Dp = 38
	SampleRailW    unit.Dp = 170
	// SampleRowH is a sidebar row, the sidebar pattern's own height.
	SampleRowH unit.Dp = sidebar.RowHeight
	// SampleInset is the air inside the content pane and SampleStep the air
	// between two things stacked in it.
	SampleInset unit.Dp = 14
	SampleStep  unit.Dp = 10
	// SampleFieldH is a text field's own height and SampleButtonH a push
	// button's, both the density scale's.
	SampleFieldH  unit.Dp = unit.Dp(tokens.ComfortableFieldHeight)
	SampleButtonH unit.Dp = unit.Dp(tokens.ComfortableControlHeight)
	// SampleButtonW is a push button's width and SampleButtonGap the run
	// between the two.
	SampleButtonW   unit.Dp = 78
	SampleButtonGap unit.Dp = 10
	// SampleDot is one of the window's own control buttons and SampleDotGap
	// the run between two of them; SampleDotLead is where the group of three
	// starts, measured from the window's leading edge.
	SampleDot     unit.Dp = 12
	SampleDotGap  unit.Dp = 8
	SampleDotLead unit.Dp = 14
	// SampleHeadH is the row the pane's own heading and its badge stand on.
	SampleHeadH unit.Dp = 22
	// SampleBarPad is the run from the toolbar's trailing edge to the last
	// button on it.
	SampleBarPad unit.Dp = 12
	// SampleFieldR is the corner a field and a push button wear.
	SampleFieldR unit.Dp = 6
	// SampleRing is how far the focus ring reaches outside the field it is on.
	SampleRing unit.Dp = 3
)

// PreviewLabel heads the preview group and PreviewHint says what it is of.
const (
	PreviewLabel = "Preview"
	PreviewHint  = "The platform's colours with the theme colour where the accent goes."
)

// The sample window's words. They label the parts of a picture of an
// application, so each says what its part is rather than naming anything this
// window does.
const (
	SampleTitle    = "Notes"
	SampleHeading  = "Release notes"
	SampleBadge    = "Draft"
	SampleField    = "Search"
	SampleDefault  = "Done"
	SampleOrdinary = "Cancel"
)

// sampleRails are the sidebar's rows; the second is the open one.
var sampleRails = [...]string{"All Notes", "Recents", "Shared", "Tags"}

// sampleSelected is the row of the sidebar the pill is on.
const sampleSelected = 1

// Preview draws the sample in the group's box, filling the room the box has
// with the shadow's own reach left clear round it.
//
// Nothing stands beside it: the window is on one appearance at a time, and
// what the picture shows is that appearance's theme.
func Preview(c tokens.PlatformColors, ty Type, typ tokens.Typography, shaper *text.Shaper, doc *markdown.Document, st markdown.Style) layout.Widget {
	draw := SampleWindow(c, ty, typ, shaper, doc, st)
	return func(gtx layout.Context) layout.Dimensions {
		room := gtx.Constraints.Max
		reach := gtx.Dp(SampleShadow)
		size := image.Pt(room.X-2*reach, room.Y-2*reach)
		if size.X <= 0 || size.Y <= 0 {
			return layout.Dimensions{Size: room}
		}
		at(gtx, image.Pt(reach, reach), func(gtx layout.Context) {
			gtx.Constraints = layout.Exact(size)
			draw(gtx)
		})
		return layout.Dimensions{Size: room}
	}
}

// SampleWindow draws a picture of an application in the previewed set: a
// window with a toolbar carrying the platform's own control buttons, a
// sidebar with the selection pill, and a content pane holding a heading with
// a badge beside it, prose, a fenced code block, a field and two push
// buttons.
//
// Every fill is the platform's name for what that element is — the plane
// windowBackground, the toolbar and sidebar the chrome material, the pane the
// content's fill, a seam separatorColor over what it lies on — so the only
// thing the theme colour moves is what the platform moves with its accent:
// the open sidebar entry's pill, the default button and the focus ring.
//
// The picture carries ONE selection, the sidebar's, and it is the platform's
// sidebar pill: [sidebar.RowHeight] tall, inset [sidebar.SelectionInset] from
// the sidebar's edges, cornered at [sidebar.SelectionRadius], filled
// sidebarSelection. A second selection — a full-width bar in the content pane
// — read as a slab of the theme colour dropped on the page rather than as a
// row a reader had chosen, and the two marks together said the colour lands
// in more places than it does.
func SampleWindow(c tokens.PlatformColors, ty Type, typ tokens.Typography, shaper *text.Shaper, doc *markdown.Document, st markdown.Style) layout.Widget {
	def := button.Render(ty.Shaper, SampleDefault, c, tokens.Spacing, tokens.Radius, ty.Role, tokens.Comfortable,
		button.RenderState{Emphasis: button.Filled, Surface: c.ControlBackground})
	ord := button.Render(ty.Shaper, SampleOrdinary, c, tokens.Spacing, tokens.Radius, ty.Role, tokens.Comfortable,
		button.RenderState{Emphasis: button.Tonal, Surface: c.ControlBackground})
	mark := badge.Render(ty.Shaper, SampleBadge, nil, badge.Warning, c, tokens.Spacing, tokens.Radius,
		badge.Style(typ, tokens.Comfortable), badge.RenderState{Surface: c.ControlBackground})
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		win := image.Rectangle{Max: size}
		if win.Dx() <= 0 || win.Dy() <= 0 {
			return layout.Dimensions{Size: size}
		}
		radius := gtx.Dp(SampleRadius)
		// A floating surface is told by its shadow on this platform, which
		// is the one thing under a window that is not a fill.
		shadow(gtx, win, radius, c.FloatingShadow.Peak, gtx.Dp(SampleShadow))
		fillRRect(gtx, win, radius, c.WindowBackground)

		// The picture is a window and has to read as one. Its own chrome material
		// is the same fill as the box it stands in under the light appearance,
		// so without a line round it the sidebar simply is not there: read
		// cold, "the left 108 points of that sample is not there — Library /
		// Recents / Shared floats in the box". A window's own edge is what the
		// platform would draw.
		edge := vgcolor.Flatten(c.Separator, c.SidebarMaterial)
		seam := vgcolor.Flatten(c.Separator, c.SidebarMaterial)
		hair := gtx.Dp(Hairline)

		defer clip.UniformRRect(win, radius).Push(gtx.Ops).Pop()

		barH := gtx.Dp(SampleToolbarH)
		railW := min(gtx.Dp(SampleRailW), win.Dx()/3)

		// The toolbar and the sidebar are chrome; the pane beside them is
		// the content. The seams are drawn by the regions above and leading.
		fillRect(gtx, image.Rect(0, 0, win.Dx(), barH), c.SidebarMaterial)
		fillRect(gtx, image.Rect(0, barH, railW, win.Dy()), c.SidebarMaterial)
		fillRect(gtx, image.Rect(railW, barH, win.Dx(), win.Dy()), c.ControlBackground)
		fillRect(gtx, image.Rect(0, barH-hair, win.Dx(), barH), seam)
		fillRect(gtx, image.Rect(railW-hair, barH, railW, win.Dy()), seam)

		SampleWindowButtons(gtx, c, barH)
		textdraw.FillText(gtx, ty.Shaper, ty.Body,
			image.Rect(0, 0, win.Dx(), barH), 0.5, 0.5,
			vgcolor.Flatten(c.Label, c.SidebarMaterial), SampleTitle)

		// The window's own actions, at the trailing end of its toolbar: the
		// ordinary one and then the default one, which is the control the
		// platform fills with its accent and so the plainest place the theme
		// colour lands.
		buttonW, buttonH := gtx.Dp(SampleButtonW), gtx.Dp(SampleButtonH)
		for i, draw := range []layout.Widget{ord, def} {
			x := win.Dx() - gtx.Dp(SampleBarPad) - (2-i)*buttonW - (1-i)*gtx.Dp(SampleButtonGap)
			if x < win.Dx()/2 {
				continue
			}
			at(gtx, image.Pt(x, barH/2-buttonH/2), func(gtx layout.Context) {
				gtx.Constraints = layout.Constraints{
					Min: image.Pt(buttonW, 0), Max: image.Pt(buttonW, buttonH),
				}
				draw(gtx)
			})
		}

		// The sidebar's open entry is the platform's pill, which is where the
		// theme colour lands in the chrome.
		rowH := gtx.Dp(SampleRowH)
		for i, name := range sampleRails {
			top := barH + gtx.Dp(SampleStep) + i*rowH
			row := image.Rect(0, top, railW-hair, top+rowH)
			if row.Max.Y > win.Dy() {
				break
			}
			foreground := vgcolor.Flatten(c.Label, c.SidebarMaterial)
			if i == sampleSelected {
				at(gtx, row.Min, func(gtx layout.Context) {
					sidebar.PaintSelection(gtx, image.Pt(row.Dx(), rowH), c, false)
				})
				foreground = vgcolor.Flatten(sidebar.SelectionLabel(c, false), sidebar.SelectionFill(c, false))
			}
			textdraw.FillText(gtx, ty.Shaper, ty.Body,
				row.Inset(gtx.Dp(SampleInset)), 0, 0.5, foreground, name)
		}

		pane := image.Rect(railW, barH, win.Dx(), win.Dy()).Inset(gtx.Dp(SampleInset))
		if pane.Dx() <= 0 || pane.Dy() <= 0 {
			strokeRRect(gtx, win, radius, hair, edge)
			return layout.Dimensions{Size: size}
		}
		step := gtx.Dp(SampleStep)
		label := vgcolor.Flatten(c.Label, c.ControlBackground)

		// The find field leads the pane, which is where this platform opens
		// one, and it wears the focus ring — the one place the theme colour
		// reaches a control that is not filled with it.
		field := image.Rect(pane.Min.X, pane.Min.Y, pane.Max.X, pane.Min.Y+gtx.Dp(SampleFieldH))
		SampleFieldAt(gtx, c, ty, field)

		// The heading row: what the pane is about, and the badge that says
		// what state it is in, at the trailing end.
		head := image.Rect(pane.Min.X, field.Max.Y+step, pane.Max.X, field.Max.Y+step+gtx.Dp(SampleHeadH))
		textdraw.FillText(gtx, ty.Shaper, ty.Label, head, 0, 0.5, label, SampleHeading)
		at(gtx, head.Min, func(gtx layout.Context) {
			gtx.Constraints = layout.Constraints{Max: head.Size()}
			trailingIn(gtx, head.Dx(), mark)
		})

		// And under them the document: a line of prose, the fenced block in
		// the chosen face and style, and a line of prose after it. It is the
		// tallest thing in the pane and takes everything the rows left.
		body := image.Rect(pane.Min.X, head.Max.Y+step, pane.Max.X, pane.Max.Y)
		if body.Dx() > 0 && body.Dy() > 0 {
			SampleDocument(gtx, body, shaper, doc, st)
		}

		strokeRRect(gtx, win, radius, hair, edge)
		return layout.Dimensions{Size: size}
	}
}

// SampleWindowButtons draws the three control buttons the platform puts at
// the leading end of a window's toolbar, on the band's own centre line: close,
// minimise and zoom, in the platform's own red, yellow and green.
func SampleWindowButtons(gtx layout.Context, c tokens.PlatformColors, barH int) {
	dia, gap, lead := gtx.Dp(SampleDot), gtx.Dp(SampleDotGap), gtx.Dp(SampleDotLead)
	top := barH/2 - dia/2
	for i, col := range []stdcolor.NRGBA{c.SystemRed, c.SystemYellow, c.SystemGreen} {
		x := lead + i*(dia+gap)
		fillRRect(gtx, image.Rect(x, top, x+dia, top+dia), dia/2, col)
	}
}

// SampleFieldAt draws the sample's text field in r: the platform's own field
// fill inside its hairline, wearing the focus ring, which is the one place
// the theme colour reaches a control that is not filled with it.
func SampleFieldAt(gtx layout.Context, c tokens.PlatformColors, ty Type, r image.Rectangle) {
	h := min(r.Dy(), gtx.Dp(SampleFieldH))
	field := image.Rect(r.Min.X, r.Min.Y+(r.Dy()-h)/2, r.Max.X, r.Min.Y+(r.Dy()-h)/2+h)
	if field.Dx() <= 0 || field.Dy() <= 0 {
		return
	}
	rad, ring := gtx.Dp(SampleFieldR), gtx.Dp(SampleRing)
	fillRRect(gtx, field.Inset(-ring), rad+ring,
		vgcolor.Flatten(c.KeyboardFocusIndicator, c.ControlBackground))
	fillRRect(gtx, field, rad, c.TextBackground)
	strokeRRect(gtx, field, rad, gtx.Dp(Hairline), c.FieldEdge)
	textdraw.FillText(gtx, ty.Shaper, ty.Body, field.Inset(gtx.Dp(8)), 0, 0.5,
		vgcolor.Flatten(c.PlaceholderText, c.TextBackground), SampleField)
}

// SampleDocument lays the prose and the fence out in r, clipped to it and
// aligned to its top: the document is a fixed source drawn in whatever height
// the pane has, and a fence running past the pane's edge would read as a pane
// that failed to draw.
func SampleDocument(gtx layout.Context, r image.Rectangle, shaper *text.Shaper, doc *markdown.Document, st markdown.Style) {
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	macro := op.Record(gtx.Ops)
	inner := gtx
	inner.Constraints = layout.Constraints{Min: image.Pt(r.Dx(), 0), Max: r.Size()}
	doc.LayoutColumn(inner, shaper, st)
	call := macro.Stop()
	stack := op.Offset(r.Min).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
}

// trailingIn draws w packed against the trailing edge of a run width wide,
// centred on the run's own line.
func trailingIn(gtx layout.Context, width int, w layout.Widget) {
	macro := op.Record(gtx.Ops)
	inner := gtx
	inner.Constraints.Min = image.Point{}
	dims := w(inner)
	call := macro.Stop()
	origin := image.Pt(max(0, width-dims.Size.X), max(0, (gtx.Constraints.Max.Y-dims.Size.Y)/2))
	stack := op.Offset(origin).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
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
