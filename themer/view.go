package main

import (
	"fmt"
	"image"
	stdcolor "image/color"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/gallery/inventory"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Layout dimensions. None of them varies with the colour scheme.
const (
	Pad      unit.Dp = 20 // window margin
	Gap      unit.Dp = 14 // between the page's stacked parts
	Radius   unit.Dp = 12 // the picture's mat, a swatch card, the preview panel
	Hairline unit.Dp = 1  // a resting outline
	Ring     unit.Dp = 2  // the drag highlight

	// TitleAir is what the title row keeps above and below the tallest thing
	// standing in it: the smallest step on the theme's spacing scale, and no
	// more. A title row is not a band of controls, and every point it spends
	// is a point the page under it does not get.
	TitleAir unit.Dp = 4
	// TitleH is the row across the top of the window: the scheme switch,
	// which is the tallest thing in it, with TitleAir either side. It is
	// derived from the switch's own height rather than pinned, so a control
	// that grows cannot end up cropped by a number written down beside it.
	TitleH unit.Dp = inventory.SchemeSwitchH + 2*TitleAir
	// TitleCenter is the line everything in the title row is centred on,
	// measured from the window's top edge: the page's margin plus half the
	// row. The window's own control buttons stand on it too.
	TitleCenter = Pad + TitleH/2

	// HeadH is the source row: where the colour came from, what to call it,
	// the field it can be written into, and the offer to keep it.
	HeadH unit.Dp = 64
	// ThumbW is the picture's mat and ThumbPad the air inside it.
	ThumbW   unit.Dp = 110
	ThumbPad unit.Dp = 6
	// IdentW is the widest the name and its caption may run, so neither can
	// land on the mat beside them.
	IdentW unit.Dp = 300
	// KeepW is the keep affordance's width, fixed so the control keeps its
	// size whichever of its two words is on it; HexW is the colour field's.
	KeepW unit.Dp = 150
	HexW  unit.Dp = 132

	// LineH is one line of running text and RowLabelH a section's label row.
	LineH     unit.Dp = 20
	RowLabelH unit.Dp = 22
)

// What the keep affordance says: an offer while the colour on screen is not
// the one on disk, a confirmation the moment it is.
const (
	KeepLabel = "Keep this theme"
	KeptLabel = "Kept"
)

// HexPlaceholder is what the colour field says when it is empty: the spelling
// it reads, so nobody has to guess at the form.
const HexPlaceholder = "#RRGGBB"

// AppName is what this window is, said once — on the window's own title and
// at the head of its title row, which are the same claim made in two places.
const AppName = "Themer"

// dropZone is the one zone the window registers a drop in: the whole of it.
// The application has no second drop target, so the index is a constant.
const dropZone = 0

// rowSlots is how many press targets the swatch row can want: one per colour
// the extraction can return, and one more for the platform's own.
const rowSlots = imageseed.DefaultMax + 1

// ButtonPlacement is where the window's own control buttons stand: the
// leading edge of the group of three, and the line their centres sit on, both
// in dp from the window's top-leading corner. It is the whole placement — the
// buttons keep their own size and their own spacing, which are the platform's.
type ButtonPlacement struct{ Leading, Center unit.Dp }

// WindowButtons answers where this window puts its control buttons: leading
// at the page's own margin, on the title row's centre line.
func WindowButtons() ButtonPlacement {
	return ButtonPlacement{Leading: Pad, Center: TitleCenter}
}

// windowButtonsEnd reports the trailing edge of the window's control buttons,
// and is the one place the page asks the window about them. The edge is
// measured from the buttons themselves rather than worked out from the
// placement above: their size and spacing are the platform's, and a number
// written down here would drift with the next release of it.
//
// A render with no window behind it is told zero — which is every render in
// the test suite, and also every platform that keeps its decorations, where
// there are no such buttons to clear. It is a variable so that a test can
// state a measurement it has no window to take.
var windowButtonsEnd = desktop.LeadingInset

// TitleLead is where the title row's own content may start: one row gap past
// the window's control buttons where the window has them, and the page's own
// margin where it does not.
func TitleLead() unit.Dp {
	return desktop.BandLeadFrom(windowButtonsEnd(), Gap, Pad)
}

// buildLayers returns the layer-builder the theme window renders.
func buildLayers(modelObs rx.Observable[Model], zones *desktop.ZoneGroup) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs, zones),
		}
	}
}

// themed carries one emission's platform colours and typography — the set the
// window itself wears, which is the platform's live reading of the appearance
// the desktop is on.
type themed struct {
	col tokens.PlatformColors
	typ Type
}

// BackdropLayer fills the window. It follows the platform alone: this window
// wears the theme the desktop is set to, like every other application, and
// what a chosen colour changes is the preview inside it.
func BackdropLayer(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(colors, func(c tokens.PlatformColors) layout.Widget {
		return backdrop.Widget(c.WindowBackground)
	})
}

// ContentLayer renders the page: where the colour came from, the colours on
// offer, and the preview of the platform's set with the chosen one standing in
// for the accent.
//
// The click handlers, the colour field and the board's scroll position live at
// subscription scope, outside the per-emission Map. A gesture handler
// reconstructed every emission loses the press it is in the middle of, and
// every selection re-emits. There is one click handler per swatch slot, not
// per colour, so the handlers outlive a picture being replaced by another.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], zones *desktop.ZoneGroup) rx.Observable[layout.Widget] {
	clicks := make([]gesture.Click, rowSlots)
	bar := new(topClicks)
	board := list.NewState()
	hex := input.TextField(th, input.TextFieldProps{
		Placeholder: HexPlaceholder,
		Description: "Theme colour, written as a hex triplet",
		OnChange: func(gtx layout.Context, txt string) {
			mvu.MessageOp{Message: HexTyped{Text: txt}}.Add(gtx.Ops)
		},
	})
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Platform, t.Typography),
			func(n rx.Tuple2[tokens.PlatformColors, tokens.Typography]) themed {
				return themed{col: n.First, typ: TypeFrom(n.Second)}
			})
	})
	return rx.Map(rx.CombineLatest3(themes, modelObs, hex),
		func(n rx.Tuple3[themed, Model, layout.Widget]) layout.Widget {
			return Page(n.First, n.Second, zones, clicks, bar, board, n.Third)
		})
}

// topClicks are the handlers of the controls along the top of the window: the
// keep affordance and the two halves of the scheme switch. They are one value
// rather than two parameters because they have one lifetime — subscription
// scope, so a press in flight survives an emission.
type topClicks struct {
	keep   gesture.Click
	scheme [2]gesture.Click
}

// Page lays the window out and registers it, whole, as the drop zone.
//
// Four rows down the page: the title row, the source row — where the colour
// came from, what it is, and the two ways of settling it — the colours on
// offer, and under them the preview, which gets the room because it is the
// thing being judged.
func Page(t themed, m Model, zones *desktop.ZoneGroup, clicks []gesture.Click, bar *topClicks, board *list.State, hex layout.Widget) layout.Widget {
	p := PaletteFrom(t.col)
	dark := m.Dark(t.col)
	preview := PreviewSet(t.col, m, dark)
	var picture paint.ImageOp
	if m.Preview != nil {
		picture = paint.NewImageOp(m.Preview)
	}
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		// The whole window is the target, recorded once per frame before
		// anything is laid out inside it.
		zones.Update(gtx)
		zones.Record(dropZone, image.Rectangle{Max: size})

		// One margin all round. The title row draws no fill of its own, so
		// it has nothing to carry the window's top margin on and takes it
		// from the page like every other row. Its own leading edge is the
		// one thing that is not the margin: with the content behind the
		// title bar, the window's control buttons stand in this row, and the
		// row starts past them.
		layout.UniformInset(Pad).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				rigid(reserve(TitleLead()-Pad, TitleRow(p, t.col, t.typ, dark, bar))),
				spacer(Gap),
				rigid(SourceRow(p, t.col, t.typ, m, picture, bar, hex)),
				spacer(Gap),
				rigid(SwatchRow(p, t.typ, m, clicks)),
				spacer(Gap),
				layout.Flexed(1, Preview(p, preview, t.typ, board)),
			)
		})

		// The drag highlight rings the window itself, not a panel inside it,
		// because the window itself is what accepts the drop.
		if m.DragOver {
			strokeRRect(gtx, image.Rectangle{Max: size}, gtx.Dp(Radius+Pad/2), gtx.Dp(Ring+1), p.Accent)
		}
		return layout.Dimensions{Size: size}
	}
}

// TitleRow is the row across the top of the window: what the window is at its
// leading edge, and the switch that shows the other side of the preview at its
// trailing one, on one centre line.
//
// It is a title row and not a bar. Nothing here is drawn on a fill of its own
// and nothing is ruled off from what follows: the row stands on the window's
// own page, and what makes it the top of the window is that it is at the top
// of the window.
//
// The row is also the window's own top strip. Nothing above it belongs to the
// platform, so three of the window's controls stand in it — placed on its
// centre line, with the row starting past them — and the run of it that holds
// nothing is what the window is dragged by, the press that would have gone to
// a title bar having nowhere else to go.
func TitleRow(p Palette, c tokens.PlatformColors, ty Type, dark bool, bar *topClicks) layout.Widget {
	slots := []slot{
		{leading, 0, AppTitle(p, ty)},
		{trailing, 0, SchemeToggle(c, dark, &bar.scheme)},
	}
	return func(gtx layout.Context) layout.Dimensions {
		dims, free := centreRowFree(gtx, gtx.Dp(TitleH), gtx.Dp(Gap), slots...)
		desktop.DragBand(gtx, free)
		return dims
	}
}

// reserve indents a row past chrome that is not the page's: the run between
// the margin where the row would otherwise begin and the point where it may.
// Nothing is drawn in that run and no drag is declared over it — the window's
// control buttons stand there, and a move action over them would fight them
// for the press.
func reserve(d unit.Dp, w layout.Widget) layout.Widget {
	if d <= 0 {
		return w
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: d}.Layout(gtx, w)
	}
}

// AppTitle is what the window is, at the head of its own title row, in the
// theme's heading type at the size a line of running text takes.
func AppTitle(p Palette, ty Type) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		w := min(natural(gtx, ty.Shaper, ty.Head, AppName), gtx.Constraints.Max.X)
		if w <= 0 {
			return layout.Dimensions{}
		}
		size := image.Pt(w, gtx.Dp(LineH))
		textdraw.FillText(gtx, ty.Shaper, ty.Head, image.Rectangle{Max: size}, 0, 0.5, p.Text, AppName)
		return layout.Dimensions{Size: size}
	}
}

// SourceRow is the line under the title row: where the colour came from, what
// it is called, the field it can be written into instead, and the offer to
// make it outlast the window.
//
// The order the slots are named in is the order they keep their size in when
// the window is too narrow for all of them. The mat, the keep affordance and
// the field are fixed objects and are named first; the identity block last,
// because it is the one thing here that can honestly give room, having a
// truncator to give it with.
func SourceRow(p Palette, c tokens.PlatformColors, ty Type, m Model, src paint.ImageOp, bar *topClicks, hex layout.Widget) layout.Widget {
	slots := []slot{
		{leading, 0, Thumbnail(p, ty, m, src)},
		{trailing, 0, KeepButton(c, ty, m, &bar.keep)},
		{trailing, 0, fixed(HexW, hex)},
		{leading, 0, Identity(p, ty, m)},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return centreRow(gtx, gtx.Dp(HeadH), gtx.Dp(Gap), slots...)
	}
}

// fixed gives a component a width of its own inside a row that would
// otherwise hand it everything left over.
func fixed(w unit.Dp, inner layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(w))
		return inner(gtx)
	}
}

// KeepButton is what makes a colour outlast the window: it writes the theme
// colour where every application that adopts a brand looks for one.
//
// It is the published button component rather than something drawn here,
// because it is the one control in this window that is not part of the frame —
// it is the thing the window is for once the looking is done. What it says
// changes with the answer: an offer while the choice on screen is not the one
// on disk, a confirmation the moment it is.
func KeepButton(c tokens.PlatformColors, ty Type, m Model, click *gesture.Click) layout.Widget {
	label, emphasis := KeepLabel, button.Filled
	if m.IsKept() {
		label, emphasis = KeptLabel, button.Tonal
	}
	draw := button.Render(ty.Shaper, label, c, tokens.Spacing, tokens.Radius, ty.Role, tokens.Comfortable,
		button.RenderState{Emphasis: emphasis, Hovered: click.Hovered(), Pressed: click.Pressed(),
			Surface: c.WindowBackground})
	return func(gtx layout.Context) layout.Dimensions {
		// The component fills the width it is offered, and what it is offered
		// inside a row is everything the caption did not take. It is given a
		// fixed width instead, so the button keeps its size whichever of the
		// two words is on it.
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(KeepW))
		dims := draw(gtx)
		area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
		click.Add(gtx.Ops)
		area.Pop()
		for {
			e, ok := click.Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind == gesture.KindClick {
				mvu.MessageOp{Message: KeepSeed{}}.Add(gtx.Ops)
			}
		}
		return dims
	}
}

// Thumbnail draws where the colours came from on a mat, so the swatches
// beside it can be compared against their source: the dropped image scaled to
// fit and centred, or — with nothing dropped — the invitation to drop one,
// which is the only way a target with no edges of its own is discovered.
func Thumbnail(p Palette, ty Type, m Model, src paint.ImageOp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(ThumbW), gtx.Dp(HeadH))
		r := image.Rectangle{Max: size}
		fill, edge, width := p.Surface, p.Edge, gtx.Dp(Hairline)
		if m.DragOver {
			fill, edge, width = p.Selection, p.Accent, gtx.Dp(Ring)
		}
		fillRRect(gtx, r, gtx.Dp(Radius), fill)
		strokeRRect(gtx, r, gtx.Dp(Radius), width, edge)
		if m.Preview == nil {
			foreground := p.CardMuted
			if m.DragOver {
				foreground = p.OnAccent
			}
			textdraw.FillText(gtx, ty.Shaper, ty.Small, r, 0.5, 0.5, foreground, "Drop an image")
			return layout.Dimensions{Size: size}
		}
		defer clip.UniformRRect(r, gtx.Dp(Radius)).Push(gtx.Ops).Pop()
		gtx.Constraints = layout.Exact(size)
		layout.UniformInset(ThumbPad).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return widget.Image{
					Src:      src,
					Fit:      widget.Contain,
					Position: layout.Center,
					Scale:    1 / gtx.Metric.PxPerDp,
				}.Layout(gtx)
			})
		})
		return layout.Dimensions{Size: size}
	}
}

// Identity names the theme colour on screen and, under it, where it came
// from: two lines that are one object, and are laid out as one so the second
// is never orphaned from the first.
//
// The block is as wide as its own text and never wider than IdentW, and it is
// clipped to that width besides. Both of those are here for one reason:
// nothing this block draws may land on the mat to its leading side. Only the
// width is clipped — the height is left open, because a clip tight enough to
// cut a descender is a bug of its own.
func Identity(p Palette, ty Type, m Model) layout.Widget {
	name, hint, tone := IdentityName(m), IdentityHint(m), p.Muted
	if m.Problem != "" {
		hint, tone = m.Problem, p.Problem
	}
	return func(gtx layout.Context) layout.Dimensions {
		line := gtx.Dp(LineH)
		room := min(gtx.Constraints.Max.X, gtx.Dp(IdentW))
		w := min(room, max(natural(gtx, ty.Shaper, ty.Body, name), natural(gtx, ty.Shaper, ty.Small, hint)))
		if w <= 0 {
			return layout.Dimensions{}
		}
		size := image.Pt(w, 2*line)
		guard := clip.Rect(image.Rect(0, -size.Y, size.X, 2*size.Y)).Push(gtx.Ops)
		textdraw.FillText(gtx, ty.Shaper, ty.Body, image.Rect(0, 0, size.X, line), 0, 0.5, p.Text, name)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, image.Rect(0, line, size.X, 2*line), 0, 0.5, tone, hint)
		guard.Pop()
		return layout.Dimensions{Size: size}
	}
}

// IdentityName is the theme colour on screen, written out, and says so in
// words where there is no colour to write.
func IdentityName(m Model) string {
	col, ok := m.Color()
	if !ok {
		return "No theme colour"
	}
	return hexOf(col)
}

// IdentityHint is where that colour came from: the platform's setting, the
// picture it was taken out of, or the field it was written into.
func IdentityHint(m Model) string {
	switch m.From {
	case FromImage:
		if m.Name != "" {
			return "from " + m.Name
		}
		return "from a picture"
	case FromHex:
		return "written in"
	}
	if m.Platform.A == 0 {
		return "this desktop reports no accent colour"
	}
	return "the " + SystemCaption() + ", and it follows it"
}

// side names which end of a row a control is packed against.
type side int

const (
	leading side = iota
	trailing
)

// slot is one control in a centred row: the end it packs against, how much
// room to leave between it and the control already placed at that end, and
// the [layout.Widget] itself. The zero value for the room is the row's own
// gap.
type slot struct {
	at    side
	apart unit.Dp
	w     layout.Widget
}

// centreRow lays a row of controls out on one shared centre line: the leading
// ones packed against the leading edge in the order they are named, the
// trailing ones against the trailing edge in the order they are named, and
// every one of them placed so its own middle lands on the row's middle.
//
// The middle is arithmetic and not an alignment flag. A control dy tall goes
// at h/2 - dy/2 from the top, which makes its own middle — top + dy/2 — the
// row's middle exactly, for every height, odd or even, and even for a control
// taller than the row it is in.
//
// It is arithmetic rather than a Flex for a second reason. A Flex hands its
// children the row's own minimum on the cross axis, and a child that honours a
// minimum it was never meant to fill comes back the full height of the row and
// then draws itself at the top of it.
//
// The order the slots are named in is the order they are measured in, and each
// is offered only what the ones before it left. So it is also the order they
// keep their size in when the row is too narrow for all of them: the first
// slot named is the last to be squeezed, and a control handed nothing takes no
// room and no gap.
func centreRow(gtx layout.Context, h, gap int, slots ...slot) layout.Dimensions {
	dims, _ := centreRowFree(gtx, h, gap, slots...)
	return dims
}

// centreRowFree is centreRow reporting, beside the row it laid out, the run of
// it nothing was laid in: from the trailing edge of the last leading control
// to the leading edge of the first trailing one, the row's full height.
//
// Only the title row asks. It stands in the strip a title bar would otherwise
// own, and that makes its empty middle the window's drag handle — which has to
// be the space between the controls exactly, since a handle overlapping one
// would take the press meant for it.
func centreRowFree(gtx layout.Context, h, gap int, slots ...slot) (layout.Dimensions, image.Rectangle) {
	width := gtx.Constraints.Max.X
	calls := make([]op.CallOp, len(slots))
	sizes := make([]image.Point, len(slots))
	apart := make([]int, len(slots))
	spent, taken := 0, 0 // the controls' own widths, and the space between them
	drawn := [2]int{}
	for i, s := range slots {
		if drawn[s.at] > 0 {
			apart[i] = gtx.Dp(s.apart)
		}
		room := width - spent - taken - apart[i]
		if drawn[leading]+drawn[trailing] > 0 {
			room -= gap
		}
		macro := op.Record(gtx.Ops)
		cgtx := gtx
		cgtx.Constraints = layout.Constraints{Max: image.Pt(max(0, room), h)}
		sizes[i] = s.w(cgtx).Size
		calls[i] = macro.Stop()
		if sizes[i].X <= 0 {
			apart[i] = 0
			continue
		}
		if drawn[leading]+drawn[trailing] > 0 {
			taken += gap
		}
		taken += apart[i]
		spent += sizes[i].X
		drawn[s.at]++
	}
	lead, trail := 0, width
	free := image.Rect(0, 0, width, h)
	for i, s := range slots {
		if sizes[i].X <= 0 {
			continue
		}
		origin := image.Pt(lead+apart[i], h/2-sizes[i].Y/2)
		if s.at == leading {
			lead = origin.X + sizes[i].X + gap
			free.Min.X = origin.X + sizes[i].X
		} else {
			trail -= apart[i] + sizes[i].X
			origin.X = trail
			trail -= gap
			free.Max.X = origin.X
		}
		stack := op.Offset(origin).Push(gtx.Ops)
		calls[i].Add(gtx.Ops)
		stack.Pop()
	}
	// A row too narrow for its controls has them meeting or overlapping, and
	// there is no space between them to hand back.
	free.Max.X = max(free.Max.X, free.Min.X)
	return layout.Dimensions{Size: image.Pt(width, h)}, free
}

// SchemeToggle is the light/dark control: a sun and a moon, the one on screen
// filled. The segments are the ones the inventory's own pages carry, so the
// control that changes scheme looks the same wherever the inventory is shown;
// what is added here is the press.
//
// A target per segment, not one over the pair. Each half names a scheme and
// the message it sends says which — pointing at the moon asks for dark from
// either side, and pointing at the half already filled asks for the scheme
// that is already on, which the update treats as the no-op it is.
func SchemeToggle(c tokens.PlatformColors, dark bool, clicks *[2]gesture.Click) layout.Widget {
	segment := func(i int, wantDark bool) layout.FlexChild {
		draw := inventory.SchemeSegment(c, wantDark, dark == wantDark)
		// The press area is the one the control hands out and not the one it
		// draws. Its track is cut to the scale of the strip it stands in, and
		// the height that came off it is given back as slop above and below.
		press := func(gtx layout.Context, w layout.Widget) layout.Dimensions {
			dims := w(gtx)
			area := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
			clicks[i].Add(gtx.Ops)
			area.Pop()
			for {
				e, ok := clicks[i].Update(gtx.Source)
				if !ok {
					break
				}
				if e.Kind == gesture.KindClick {
					mvu.MessageOp{Message: SetScheme{Dark: wantDark}}.Add(gtx.Ops)
				}
			}
			return dims
		}
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inventory.SchemeTarget(gtx, press, draw)
		})
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			segment(schemeLightSegment, false),
			segment(schemeDarkSegment, true),
		)
	}
}

// The halves of the scheme control, in the order they are laid out.
const (
	schemeLightSegment = iota
	schemeDarkSegment
)

// natural is how wide a string wants to be, unconstrained by the room it is
// about to be given. It is measured against a width nothing reaches rather
// than against the constraints in hand, because the shaper truncates to
// whatever MaxWidth it is handed — so measuring inside the available room
// would report the room back, and never that the string did not fit in it.
func natural(gtx layout.Context, shaper *text.Shaper, style textdraw.TextStyle, str string) int {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	return textdraw.MeasureText(gtx, shaper, style, str).X
}

// rigid wraps a [layout.Widget] as a Flex child that takes the height it asks
// for.
func rigid(w layout.Widget) layout.FlexChild { return layout.Rigid(w) }

// spacer is a fixed vertical gap between two Flex children.
func spacer(h unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Height: h}.Layout)
}

// fillRect paints a rectangle.
func fillRect(gtx layout.Context, r image.Rectangle, c stdcolor.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

// fillRRect paints a rounded rectangle.
func fillRRect(gtx layout.Context, r image.Rectangle, radius int, c stdcolor.NRGBA) {
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// strokeRRect outlines a rounded rectangle, inset by half the stroke width so
// the whole line lands inside the rectangle rather than half outside it.
func strokeRRect(gtx layout.Context, r image.Rectangle, radius, width int, c stdcolor.NRGBA) {
	if width <= 0 {
		return
	}
	half := float32(width) / 2
	inner := image.Rect(r.Min.X+width/2, r.Min.Y+width/2, r.Max.X-width/2, r.Max.Y-width/2)
	path := clip.UniformRRect(inner, max(0, radius-width/2)).Path(gtx.Ops)
	defer clip.Stroke{Path: path, Width: half * 2}.Op().Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// hexOf writes a colour the way a stylesheet would.
func hexOf(c stdcolor.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// at offsets the operations w records to origin, leaving the caller's
// coordinate system untouched.
func at(gtx layout.Context, origin image.Point, w func(gtx layout.Context)) {
	defer op.Offset(origin).Push(gtx.Ops).Pop()
	w(gtx)
}
