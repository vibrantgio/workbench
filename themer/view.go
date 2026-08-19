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
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/gallery/inventory"
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
	Radius   unit.Dp = 12 // the picture mat and the candidate cards
	Hairline unit.Dp = 1  // the mat's resting outline
	Ring     unit.Dp = 2  // the chosen candidate's ring, and the drag highlight

	TopBarH   unit.Dp = 72  // thumbnail, file name, the keep button and the scheme switch
	ThumbW    unit.Dp = 108 // the thumbnail's mat
	ThumbPad  unit.Dp = 6   // mat edge to the picture inside it
	RowLabelH unit.Dp = 20  // the label over the candidate row, and over the page
	KeepW     unit.Dp = 150 // the keep button, at a width neither of its two labels squeezes
)

// What the keep affordance says. The second word is a state and not an
// invitation: pressing it again writes the same file, which is why it stays
// pressable rather than going grey.
const (
	KeepLabel = "Keep this theme"
	KeptLabel = "Kept"
)

// dropZone is the one zone the window registers: the whole of it. The
// application has no second drop target, so the index is a constant.
const dropZone = 0

// buildLayers returns the layer-builder the theme window renders.
func buildLayers(modelObs rx.Observable[Model], zones *desktop.ZoneGroup) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th, modelObs),
			ContentLayer(th, modelObs, zones),
		}
	}
}

// themed carries one emission's OS palette and typography. The palette the
// window actually draws in is resolved from this one and the model together,
// in SchemeFor, because the chosen candidate re-seeds it.
type themed struct {
	os  tokens.ColorTokens
	typ Type
}

// BackdropLayer fills the window. It follows the model as well as the OS,
// because the background is one of the surfaces a chosen seed changes.
func BackdropLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
		return t.Color
	})
	return rx.Map(rx.CombineLatest2(colors, modelObs),
		func(n rx.Tuple2[tokens.ColorTokens, Model]) layout.Widget {
			return backdrop.Widget(SchemeFor(n.First, n.Second).Background)
		})
}

// ContentLayer renders the page: the dropped picture and the candidate row
// over the whole inventory drawn in the chosen candidate's theme, or the
// invitation to drop something.
//
// The click handlers and the embedded page's state live at subscription
// scope, OUTSIDE the per-emission Map (llms.txt rule 2). A gesture handler
// reconstructed every emission loses the press it is in the middle of, and
// every selection re-emits; an inventory rebuilt every emission would re-read
// the reading sample on every pick, which is the one thing on this page that
// costs anything. There is one click handler per candidate slot, not per
// candidate, so the handlers outlive a picture being replaced by another.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], zones *desktop.ZoneGroup) rx.Observable[layout.Widget] {
	clicks := make([]gesture.Click, imageseed.DefaultMax)
	keep, scheme := new(gesture.Click), new([2]gesture.Click)
	page, bases := newEmbed(), newBaseSelector()
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Typography),
			func(n rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themed {
				return themed{os: n.First, typ: TypeFrom(n.Second)}
			})
	})
	return rx.Map(rx.CombineLatest2(themes, modelObs),
		func(n rx.Tuple2[themed, Model]) layout.Widget {
			return Page(n.First, n.Second, zones, clicks, keep, scheme, page, bases)
		})
}

// Page lays the window out and registers it, whole, as the drop zone.
//
// The window is three bands: what was dropped, the seeds taken out of it, and
// the whole design system drawn in the one that is chosen. The last of those
// gets the room, because it is the thing being judged — the picture is a
// reference and needs only to be recognisable.
func Page(t themed, m Model, zones *desktop.ZoneGroup, clicks []gesture.Click, keep *gesture.Click, scheme *[2]gesture.Click, page *embed, bases *baseSelector) layout.Widget {
	c := SchemeFor(t.os, m)
	p := PaletteFrom(c)
	dark := m.Dark(t.os)
	// Each candidate's generated primary pair, on the side the window is
	// currently showing, so a swatch promises what choosing it delivers.
	pairs := make([]tokens.ColorTokens, len(m.Candidates))
	for i, cand := range m.Candidates {
		light, darkTokens := tokens.FromSeed(cand.Color)
		pairs[i] = light
		if dark {
			pairs[i] = darkTokens
		}
	}
	var picture paint.ImageOp
	if m.Preview != nil {
		picture = paint.NewImageOp(m.Preview)
	}
	// Built here rather than per frame: the sections are a function of the
	// palette and the chosen syntax base, and both change on an emission,
	// not on a frame.
	items := page.items(t.typ.Shaper, c, m.AppliedBases())
	// The base selector rides in the code specimen's own row rather than
	// standing beside the page: the choice belongs next to its consequence,
	// and nowhere else on the page is it worth a column.
	if row := page.codeRow(); row >= 0 && row < len(items) {
		items[row] = BesideTheCode(p, c, t.typ, m, dark, bases, items[row])
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		// The whole window is the target, recorded once per frame before
		// anything is laid out inside it.
		zones.Update(gtx)
		zones.Record(dropZone, image.Rectangle{Max: size})

		inset := layout.UniformInset(Pad)
		inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{}
			if len(m.Candidates) > 0 {
				children = append(children,
					rigid(TopBar(p, c, t.typ, m, picture, dark, keep, scheme)),
					spacer(Gap),
					rigid(CandidateRow(p, t.typ, m, pairs, clicks)),
					spacer(Gap),
					layout.Flexed(1, Gallery(p, c, t.typ, GalleryHintFor(m, dark), page.st, items)),
				)
			} else {
				children = append(children, layout.Flexed(1, Invitation(p, t.typ, m)))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})

		// The drag highlight rings the window itself, not a panel inside
		// it, because the window itself is what accepts the drop.
		if m.DragOver {
			strokeRRect(gtx, image.Rectangle{Max: size}, gtx.Dp(Radius+Pad/2), gtx.Dp(Ring+1), p.Accent)
		}
		return layout.Dimensions{Size: size}
	}
}

// TopBar is the loaded picture's line: a thumbnail of it, its name with the
// standing offer to replace it, and the switch to the other side of the
// scheme.
//
// The picture is a thumbnail and not a plate. What a seed is judged on is the
// page below, so the picture takes the room a reference needs and no more:
// enough to recognise which image these colours came out of.
func TopBar(p Palette, c tokens.ColorTokens, ty Type, m Model, src paint.ImageOp, dark bool, keep *gesture.Click, scheme *[2]gesture.Click) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		h := gtx.Dp(TopBarH)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			rigid(Thumbnail(p, m, src)),
			spacer(Gap),
			layout.Flexed(1, Caption(p, ty, m)),
			spacer(Gap),
			rigid(KeepButton(c, ty, m, keep)),
			spacer(Gap),
			rigid(SchemeToggle(c, dark, scheme)),
		)
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, h)}
	}
}

// KeepButton is what makes a colour outlast the window: it writes the
// chosen seed where every application that adopts a brand looks for one.
//
// It is the published button component rather than something drawn here,
// because it is the one control in this window that is not furniture — it
// is the thing the window is for once the looking is done — and it should
// be the button of the design system on display beneath it. What it says
// changes with the answer: an offer while the choice on screen is not the
// one on disk, a confirmation the moment it is.
func KeepButton(c tokens.ColorTokens, ty Type, m Model, click *gesture.Click) layout.Widget {
	label, emphasis := KeepLabel, button.Filled
	if m.SeedIsKept() {
		label, emphasis = KeptLabel, button.Tonal
	}
	draw := button.Render(ty.Shaper, label, c, tokens.Spacing, tokens.Radius, ty.Role, tokens.Comfortable,
		button.RenderState{Emphasis: emphasis, Hovered: click.Hovered(), Pressed: click.Pressed()})
	return func(gtx layout.Context) layout.Dimensions {
		// The component fills the width it is offered, and what it is
		// offered inside a row is everything the caption did not take. It
		// is given a fixed width instead, so the button keeps its size
		// whichever of the two words is on it.
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

// Thumbnail draws the dropped image on a mat, scaled to fit and centred, so
// the candidates beside it can be compared against what they came from.
func Thumbnail(p Palette, m Model, src paint.ImageOp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(ThumbW), gtx.Dp(TopBarH))
		r := image.Rectangle{Max: size}
		fill, edge := p.Surface, p.Divider
		if m.DragOver {
			fill, edge = p.Selection, p.Accent
		}
		fillRRect(gtx, r, gtx.Dp(Radius), fill)
		strokeRRect(gtx, r, gtx.Dp(Radius), gtx.Dp(Hairline), edge)
		if m.Preview == nil {
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

// Caption names the loaded picture and says what to do with the window next.
func Caption(p Palette, ty Type, m Model) layout.Widget {
	hint := "Drop another image to replace it"
	tone := p.Muted
	if m.Problem != "" {
		hint, tone = m.Problem, p.Problem
	}
	return func(gtx layout.Context) layout.Dimensions {
		w, h := gtx.Constraints.Max.X, gtx.Dp(TopBarH)
		line := gtx.Dp(20)
		name := image.Rect(0, h/2-line, w, h/2)
		note := image.Rect(0, h/2, w, h/2+line)
		textdraw.FillText(gtx, ty.Shaper, ty.Body, name, 0, 0.5, p.Text, m.Name)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, note, 0, 0.5, tone, hint)
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
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
func SchemeToggle(c tokens.ColorTokens, dark bool, clicks *[2]gesture.Click) layout.Widget {
	segment := func(i int, wantDark bool) layout.FlexChild {
		draw := inventory.SchemeSegment(c, wantDark, dark == wantDark)
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := draw(gtx)
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

// Invitation is the window before anything has been dropped on it: a well
// covering the page, saying what to drop and where. "Where" is the whole
// window, and saying so is the only way a target with no edges of its own
// can be discovered.
func Invitation(p Palette, ty Type, m Model) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		r := image.Rectangle{Max: size}
		fill, edge, width := p.Surface, p.Divider, gtx.Dp(Hairline)
		if m.DragOver {
			fill, edge, width = p.Selection, p.Accent, gtx.Dp(Ring)
		}
		fillRRect(gtx, r, gtx.Dp(Radius), fill)
		strokeRRect(gtx, r, gtx.Dp(Radius), width, edge)

		line := gtx.Dp(28)
		mid := size.Y / 2
		title := image.Rect(0, mid-line*3/2, size.X, mid-line/2)
		sub := image.Rect(0, mid-line/2, size.X, mid+line/2)
		note := image.Rect(0, mid+line/2, size.X, mid+line*3/2)
		textdraw.FillText(gtx, ty.Shaper, ty.Title, title, 0.5, 0.5, p.Text, "Drop an image here")
		textdraw.FillText(gtx, ty.Shaper, ty.Body, sub, 0.5, 0.5, p.Muted, "Anywhere on the window. PNG, JPEG or GIF.")
		if m.Problem != "" {
			textdraw.FillText(gtx, ty.Shaper, ty.Small, note, 0.5, 0.5, p.Problem, m.Problem)
		}
		return layout.Dimensions{Size: size}
	}
}

// rigid wraps a widget as a Flex child that takes the height it asks for.
func rigid(w layout.Widget) layout.FlexChild { return layout.Rigid(w) }

// spacer is a fixed vertical gap between two Flex children.
func spacer(h unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Height: h}.Layout)
}

// fillRRect paints a rounded rectangle.
func fillRRect(gtx layout.Context, r image.Rectangle, radius int, c stdcolor.NRGBA) {
	defer clip.UniformRRect(r, radius).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// strokeRRect outlines a rounded rectangle, inset by half the stroke width
// so the whole line lands inside the rectangle rather than half outside it.
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
