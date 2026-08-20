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

	// NavH is the bar along the top of both screens, and it is the whole top
	// of the window rather than a band inset into it: the bar carries the
	// window's top margin itself, inside its own height, so its controls are
	// centred between the top edge and the rule under them. Left inside the
	// page's uniform inset the same controls would sit a margin's worth low —
	// a bar with a margin's air above its contents and a few points below
	// them, which reads as a row that has slipped rather than as a bar.
	NavH      unit.Dp = 64
	HeadH     unit.Dp = 56  // the identity strip under it: swatch, name, offer, keep
	ThumbW    unit.Dp = 108 // the thumbnail's mat
	ThumbPad  unit.Dp = 6   // mat edge to the picture inside it
	RowLabelH unit.Dp = 20  // the label over the candidate row, and over the page
	LineH     unit.Dp = 20  // one line of the identity block, and of the standing offer
	KeepW     unit.Dp = 150 // the keep button, at a width neither of its two labels squeezes
	BackW     unit.Dp = 140 // the way back to the first screen
	// IdentW is the widest the name and its caption ever get. Past it the
	// slack goes to the space before the standing offer instead, so a window
	// dragged wide opens a gap between the identity and the controls rather
	// than stretching a two-line block across half a screen.
	IdentW unit.Dp = 420
)

// What the keep affordance says. The second word is a state and not an
// invitation: pressing it again writes the same file, which is why it stays
// pressable rather than going grey.
const (
	KeepLabel = "Keep this theme"
	KeptLabel = "Kept"
)

// BackLabel is on the control that undoes a choice by returning to the screen
// it was made on. It names where it goes rather than what it undoes: "cancel"
// on a window with nothing pending is a question, and the grid is a place.
const BackLabel = "Back to styles"

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
	bar := new(topClicks)
	page, bases, grid := newEmbed(), newBaseSelector(), newStyleGrid()
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Typography),
			func(n rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themed {
				return themed{os: n.First, typ: TypeFrom(n.Second)}
			})
	})
	return rx.Map(rx.CombineLatest2(themes, modelObs),
		func(n rx.Tuple2[themed, Model]) layout.Widget {
			return Page(n.First, n.Second, zones, clicks, bar, page, bases, grid)
		})
}

// topClicks are the handlers of the controls along the top of the window:
// the way back to the first screen, the keep affordance, and the two halves of
// the scheme switch. They are one value rather than three parameters because
// they have one lifetime — subscription scope, so a press in flight survives an
// emission — and because the scheme switch is on the first screen too, where
// the other two are not.
type topClicks struct {
	back   gesture.Click
	keep   gesture.Click
	scheme [2]gesture.Click
}

// Page lays the window out and registers it, whole, as the drop zone.
//
// Both screens open with the same bar: the way back at its leading edge, the
// scheme switch at its trailing one. It is the same bar and not two, which is
// what makes the switch stay put across the click that changes screens — a
// control that moved across the window on the first press would be a second
// control as far as anybody reading is concerned.
//
// Under the bar, with a theme on screen, the window is three bands: where the
// colours came from, the seeds taken out of it, and the whole design system
// drawn in the one that is chosen. The last of those gets the room, because it
// is the thing being judged — the source is a reference and needs only to be
// recognisable.
//
// Before anything has been chosen it is the two doors instead: the drop well,
// which is the primary invitation and takes the top of the page, and under it
// the grid of styles for somebody who would rather start from a palette than
// find one. The well is a band rather than the whole page there, because a
// grid nobody can see without scrolling is a grid nobody knows about.
func Page(t themed, m Model, zones *desktop.ZoneGroup, clicks []gesture.Click, bar *topClicks, page *embed, bases *baseSelector, grid *styleGrid) layout.Widget {
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

		// No top margin: the bar is the first child and owns the window's
		// top edge, margin included. Everything under it is inset as before.
		inset := layout.Inset{Left: Pad, Right: Pad, Bottom: Pad}
		inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				rigid(NavBar(p, c, t.typ, m, dark, bar)),
				spacer(Gap),
			}
			if len(m.Candidates) > 0 {
				children = append(children,
					rigid(IdentityRow(p, c, t.typ, m, picture, bar)),
					spacer(Gap),
					rigid(CandidateRow(p, t.typ, m, pairs, clicks)),
					spacer(Gap),
					layout.Flexed(1, Gallery(p, c, t.typ, GalleryHintFor(m, dark), page.st, items)),
				)
			} else {
				children = append(children,
					rigid(DropWell(p, t.typ, m)),
					spacer(Gap),
					layout.Flexed(1, StyleGrid(p, c, t.typ, m, dark, grid)),
				)
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

// NavBar is the strip along the top of both screens: the way back at its
// leading edge, the switch to the other side of the scheme at its trailing one,
// and a rule under the pair of them.
//
// Both ends are fixed places rather than positions in a row, which is the whole
// point of a bar. Where a control that scrolls with the page has to be looked
// for, one pinned to an edge is somewhere a reader already knows to look — and
// the two controls here are precisely the two that are not about the theme on
// screen: one leaves it, the other changes which half of it is showing.
//
// The leading slot is empty before anything has been chosen, because there is
// nowhere to go back to and the window puts no name of its own on the page. An
// empty slot is what an empty slot should look like; the bar is still a bar,
// and the switch is still in the corner it stands in on the other screen.
//
// The rule is what makes the strip read as a bar rather than as two controls
// that happen to be high up. It is the page's own divider weight, the one the
// drop well is outlined in, so the top of the window is ruled off in a line the
// rest of the window already uses.
func NavBar(p Palette, c tokens.ColorTokens, ty Type, m Model, dark bool, bar *topClicks) layout.Widget {
	var slots []slot
	if len(m.Candidates) > 0 {
		slots = append(slots, slot{leading, 0, BackButton(p, c, ty, &bar.back)})
	}
	slots = append(slots, slot{trailing, 0, SchemeToggle(c, dark, &bar.scheme)})
	return func(gtx layout.Context) layout.Dimensions {
		h, line := gtx.Dp(NavH), gtx.Dp(Hairline)
		dims := centreRow(gtx, h, gtx.Dp(Gap), slots...)
		paint.FillShape(gtx.Ops, p.Divider,
			clip.Rect(image.Rect(0, h-line, dims.Size.X, h)).Op())
		return dims
	}
}

// IdentityRow is the line under the bar on the second screen: where the theme's
// colours came from, what it is called, what it is dressing code in, the
// standing offer to replace the lot with a picture, and the offer to make it
// outlast the window.
//
// The picture is a thumbnail and not a plate. What a seed is judged on is the
// page below, so the source takes the room a reference needs and no more:
// enough to recognise which image or which style these colours came out of.
//
// The order the slots are named in is the order they keep their size in when
// the window is too narrow for all of them — see centreRow. The swatch and the
// keep affordance are fixed objects and are named first; then the standing
// offer, which is an instruction and either fits or stands down; and the
// identity block last, because it is the one thing here that can honestly give
// ground, having a truncator to give it with.
func IdentityRow(p Palette, c tokens.ColorTokens, ty Type, m Model, src paint.ImageOp, bar *topClicks) layout.Widget {
	slots := []slot{
		{leading, 0, Thumbnail(p, m, src)},
		{trailing, 0, KeepButton(c, ty, m, &bar.keep)},
		{trailing, Gap, ReplaceHint(p, ty, m)},
		{leading, 0, Identity(p, ty, m)},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return centreRow(gtx, gtx.Dp(HeadH), gtx.Dp(Gap), slots...)
	}
}

// BackButton returns the window to its first screen, where the drop well and
// the whole grid of styles are.
//
// It wears the middle register with an edge drawn round it. A ghost was the
// wrong reading of the same argument: going back is a way out of a choice
// rather than the thing the window is for, so it must not shout — but a label
// with no ground at rest, standing a few points from a hint in the same line,
// was not quiet, it was indistinguishable from the hint.
//
// The tinted ground alone did not settle it either. A tonal fill is a tint by
// design, and on this window it lands a hair off the page in both schemes —
// far enough to see once it is pointed out, nowhere near far enough to make an
// object out of. What a reader sees is a label floating in a wash. The outline
// is what makes it a thing with a boundary, and it is drawn here rather than
// asked of the button because the register is a colour property and a border
// belongs to the surface. Its colour is Palette.Outline, chosen against the
// page by measurement so the boundary carries the same weight under the sun as
// under the moon.
//
// It is painted under the button and one point wider all round, rather than
// stroked along the button's own edge. A one-point stroke sits centred on the
// boundary it names, so it lands as two rows of half-strength antialiasing —
// half of it outside the control's own box, and none of it at the colour it was
// asked for. The colour is the whole point of choosing it by measurement, so
// the outline is a fill with the button laid over it: what shows is one whole
// point of the chosen colour, and it shows inside the box this returns, which
// is what keeps the control's leading edge on the same margin as everything
// else down the page.
func BackButton(p Palette, c tokens.ColorTokens, ty Type, click *gesture.Click) layout.Widget {
	draw := button.Render(ty.Shaper, BackLabel, c, tokens.Spacing, tokens.Radius, ty.Role, tokens.Comfortable,
		button.RenderState{Emphasis: button.Tonal, Hovered: click.Hovered(), Pressed: click.Pressed()})
	return func(gtx layout.Context) layout.Dimensions {
		line := gtx.Dp(Hairline)
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = max(0, min(gtx.Constraints.Max.X, gtx.Dp(BackW))-2*line)
		body := op.Record(gtx.Ops)
		inner := draw(gtx).Size
		drawn := body.Stop()

		size := inner.Add(image.Pt(2*line, 2*line))
		fillRRect(gtx, image.Rectangle{Max: size}, gtx.Dp(unit.Dp(tokens.Radius.Md))+line, p.Outline)
		at(gtx, image.Pt(line, line), func(gtx layout.Context) { drawn.Add(gtx.Ops) })

		area := clip.Rect{Max: size}.Push(gtx.Ops)
		click.Add(gtx.Ops)
		area.Pop()
		for {
			e, ok := click.Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind == gesture.KindClick {
				mvu.MessageOp{Message: ShowStyles{}}.Add(gtx.Ops)
			}
		}
		return layout.Dimensions{Size: size}
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

// Thumbnail draws where the colours came from on a mat, so the candidates
// beside it can be compared against their source: the dropped image scaled to
// fit and centred, or — when the theme was adopted from a style — that style's
// own inks in the same strip its card carried them in, which is the closest
// thing to a picture a palette has.
func Thumbnail(p Palette, m Model, src paint.ImageOp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(ThumbW), gtx.Dp(HeadH))
		r := image.Rectangle{Max: size}
		fill, edge := p.Surface, p.Divider
		if m.DragOver {
			fill, edge = p.Selection, p.Accent
		}
		fillRRect(gtx, r, gtx.Dp(Radius), fill)
		strokeRRect(gtx, r, gtx.Dp(Radius), gtx.Dp(Hairline), edge)
		if m.Preview == nil {
			if m.Style != "" {
				SwatchBands(gtx, r.Inset(gtx.Dp(ThumbPad)), gtx.Dp(InnerR), m.Candidates, p.Edge)
			}
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

// Identity names where the theme on screen came from — a file, or a style —
// and under it the syntax pair it is wearing: two lines that are one object,
// and are laid out as one so the pair is never orphaned from the name it
// belongs to.
//
// Both members of the pair, always, and not the one being drawn. The pair is
// one choice, and half of it is on the other side of the scheme switch: a click
// on a style card sets two names at once, and a line naming one of them would
// make a change to two things look like a change to one. The line under the
// embedded page still names the member in force, because that line answers a
// different question — what the page beneath it is coloured with right now.
//
// The block is as wide as its own text and never wider than IdentW, and it is
// clipped to that width besides. Both of those are here for one reason: nothing
// this block draws may land on the swatch to its left. The width is what keeps
// the text out of the swatch's column, since the block is only ever handed the
// room left over once the swatch has taken its own; the clip is what makes that
// a property of the drawing rather than of the arithmetic, so a name of any
// length, in any face, at any window width, cannot put a pixel there. Only the
// width is clipped — the height is left open, because a clip tight enough to
// cut a descender is a bug of its own.
func Identity(p Palette, ty Type, m Model) layout.Widget {
	hint, tone := CaptionHintFor(m), p.Muted
	if m.Problem != "" {
		hint, tone = m.Problem, p.Problem
	}
	return func(gtx layout.Context) layout.Dimensions {
		line := gtx.Dp(LineH)
		room := min(gtx.Constraints.Max.X, gtx.Dp(IdentW))
		w := min(room, max(natural(gtx, ty.Shaper, ty.Body, m.Name), natural(gtx, ty.Shaper, ty.Small, hint)))
		if w <= 0 {
			return layout.Dimensions{}
		}
		size := image.Pt(w, 2*line)
		guard := clip.Rect(image.Rect(0, -size.Y, size.X, 2*size.Y)).Push(gtx.Ops)
		textdraw.FillText(gtx, ty.Shaper, ty.Body, image.Rect(0, 0, size.X, line), 0, 0.5, p.Text, m.Name)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, image.Rect(0, line, size.X, 2*line), 0, 0.5, tone, hint)
		guard.Pop()
		return layout.Dimensions{Size: size}
	}
}

// ReplaceHint is the standing offer to hand the window another picture. It is
// an instruction rather than a state, which is why it stands on the controls'
// side of the line and not under the name: a state and an instruction stacked
// one above the other read as two states.
//
// It fits whole or it is not drawn at all. A truncated instruction is worse
// than an absent one — "drop an image to repl…" is a sentence nobody can act
// on, and the target it describes is the whole window, which is still there
// whether or not there is room to say so.
func ReplaceHint(p Palette, ty Type, m Model) layout.Widget {
	offer := ReplaceHintFor(m)
	return func(gtx layout.Context) layout.Dimensions {
		w := natural(gtx, ty.Shaper, ty.Small, offer)
		if w > gtx.Constraints.Max.X {
			return layout.Dimensions{}
		}
		size := image.Pt(w, gtx.Dp(LineH))
		textdraw.FillText(gtx, ty.Shaper, ty.Small, image.Rectangle{Max: size}, 0, 0.5, p.Muted, offer)
		return layout.Dimensions{Size: size}
	}
}

// natural is how wide a string wants to be, unconstrained by the room it is
// about to be given. It is measured against a width nothing reaches rather than
// against the constraints in hand, because the shaper truncates to whatever
// MaxWidth it is handed — so measuring inside the available room would report
// the room back, and never that the string did not fit in it.
func natural(gtx layout.Context, shaper *text.Shaper, style textdraw.TextStyle, str string) int {
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	return textdraw.MeasureText(gtx, shaper, style, str).X
}

// CaptionHintFor is the line under the source's name: which syntax base the
// code wears under each appearance.
//
// It says which is which rather than joining the two names with a sign. A pair
// written "perldoc + dracula" leaves a reader looking at a light window with no
// way to tell whether the palette in front of them is the first name or the
// second — and on this window it is very often neither of the names they
// clicked, since one member is completed by measurement. Naming the appearances
// costs four words and answers it.
//
// A style that is its own counterpart says so in one clause. Four of the
// embedded styles are fitted to no ground and stand under both, and writing
// such a name out twice reads as a mistake rather than as the fact it is.
func CaptionHintFor(m Model) string {
	pair := m.AppliedBases()
	if pair.Light == pair.Dark {
		return "syntax base " + pair.Light + ", day and night"
	}
	return "syntax base " + pair.Light + " by day, " + pair.Dark + " by night"
}

// ReplaceHintFor is the standing offer at the caption's trailing edge. A window
// showing a picture's colours can be handed another picture; one showing a
// style's has not been handed one yet.
func ReplaceHintFor(m Model) string {
	if m.Preview != nil {
		return "drop another image to replace it"
	}
	return "drop an image to replace it"
}

// side names which end of a row a control is packed against.
type side int

const (
	leading side = iota
	trailing
)

// slot is one control in a centred row: the end it packs against, how much
// room to leave between it and the control already placed at that end, and the
// widget itself.
//
// The extra room is for the one case a uniform gap gets wrong. A button carries
// its own inner padding, and where the row's gap is the narrower of the two, a
// run of text beside the button sits closer to the label than the label sits to
// its own edge — so the text reads as the first half of the button's label
// rather than as something standing on its own. Nothing else in either row
// needs it, and the zero value is the row's own gap.
type slot struct {
	at    side
	apart unit.Dp
	w     layout.Widget
}

// centreRow lays a row of controls out on one shared centre line: the leading
// ones packed against the left edge in the order they are named, the trailing
// ones against the right edge in the order they are named, and every one of
// them placed so its own middle lands on the row's middle.
//
// The middle is arithmetic and not an alignment flag. A control dy tall goes
// at h/2 - dy/2 from the top, which makes its own middle — top + dy/2 — the
// row's middle exactly, for every height, odd or even, and even for a control
// taller than the row it is in. Written the other way round, as half of what is
// left over, the same line is a point out on odd heights and a point out the
// other way on a control that overflows. That exactness is the point: this row
// is the first thing anybody sees, and four objects a point or two out of line
// with one another is precisely the kind of thing an eye reads as sloppiness
// without being able to name.
//
// It is arithmetic rather than a Flex for a second reason. A Flex hands its
// children the row's own minimum on the cross axis, and a child that honours a
// minimum it was never meant to fill comes back the full height of the row and
// then draws itself at the top of it — which is how a scheme switch ends up
// sitting a dozen points above the buttons it is supposed to be level with.
// Here every control is measured with no minimum at all, so what comes back is
// the height the control actually wanted.
//
// The order the slots are named in is the order they are measured in, and each
// is offered only what the ones before it left. So it is also the order they
// keep their size in when the row is too narrow for all of them: the first slot
// named is the last to be squeezed, and a control handed nothing takes no room
// and no gap.
func centreRow(gtx layout.Context, h, gap int, slots ...slot) layout.Dimensions {
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
	for i, s := range slots {
		if sizes[i].X <= 0 {
			continue
		}
		origin := image.Pt(lead+apart[i], h/2-sizes[i].Y/2)
		if s.at == leading {
			lead = origin.X + sizes[i].X + gap
		} else {
			trail -= apart[i] + sizes[i].X
			origin.X = trail
			trail -= gap
		}
		stack := op.Offset(origin).Push(gtx.Ops)
		calls[i].Add(gtx.Ops)
		stack.Pop()
	}
	return layout.Dimensions{Size: image.Pt(width, h)}
}

// DropWell is the invitation at the height it takes on the first screen: a band
// across the top rather than the whole page, so the styles under it are on
// screen without anybody having to scroll to find out they exist. It is still
// the first thing and the biggest single object there, because dropping a
// picture is still what the window is for.
func DropWell(p Palette, ty Type, m Model) layout.Widget {
	well := Invitation(p, ty, m)
	return func(gtx layout.Context) layout.Dimensions {
		h := min(gtx.Dp(DropH), gtx.Constraints.Max.Y)
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
		return well(gtx)
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
