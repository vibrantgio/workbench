package main

import (
	"fmt"
	"image"
	stdcolor "image/color"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
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
	"github.com/vibrantgio/components/icons"
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

	// TitleAir is what the title row keeps above and below the tallest thing
	// standing in it: the smallest step on the theme's spacing scale, and no
	// more. A title row is not a band of controls, and every point it spends
	// is a point the page under it does not get.
	TitleAir unit.Dp = 4
	// TitleH is the row across the top of both screens: the scheme switch,
	// which is the tallest thing in it, with TitleAir either side. It is
	// derived from the switch's own height rather than pinned, so a control
	// that grows cannot end up cropped by a number written down beside it.
	//
	// The row stands inside the page's margin like every other row down the
	// window, having no ground of its own to carry a margin on. It costs the
	// page exactly what the band it replaced cost: that band swallowed the top
	// margin and stood its controls in the points below it, so margin plus row
	// comes to the same height and nothing under the row has moved.
	TitleH unit.Dp = inventory.SchemeSwitchH + 2*TitleAir
	// TitleCenter is the line everything in the title row is centred on,
	// measured from the window's own top edge: the page's top margin, which the
	// row stands in, plus half the row.
	//
	// It is the window's line and not just the page's. With the content behind
	// the title bar there is no native strip above this row, so the window's own
	// control buttons stand in it — and they are placed on this line rather than
	// left on the one the platform would default them to, because a row with one
	// centre line through the name, the way back and the switch does not get to
	// have its fourth object on a line of its own.
	//
	// It is derived from the row's height rather than written down beside it, so
	// a control in the row changing size takes the buttons with it instead of
	// leaving them behind.
	TitleCenter unit.Dp = Pad + TitleH/2
	HeadH       unit.Dp = 56  // the identity strip under it: swatch, name, offer, keep
	ThumbW      unit.Dp = 108 // the thumbnail's mat
	ThumbPad    unit.Dp = 6   // mat edge to the picture inside it
	RowLabelH   unit.Dp = 20  // the label over the candidate row, and over the page
	LineH       unit.Dp = 20  // one line of the identity block, and of the standing offer
	KeepW       unit.Dp = 150 // the keep button, at a width neither of its two labels squeezes
	// BackMark is the way back's chevron, at the size a mark takes beside a
	// line of text, and BackGap is what stands between the two: the smallest
	// step, because a mark and the word it belongs to are one control and a
	// wider gap would make them two.
	BackMark unit.Dp = 16
	BackGap  unit.Dp = TitleAir
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

// AppName is what this window is, said once — on the window's own title and
// at the head of its title row, which are the same claim made in two places.
const AppName = "Themer"

// dropZone is the one zone the window registers: the whole of it. The
// application has no second drop target, so the index is a constant.
const dropZone = 0

// ButtonPlacement is where the window's own control buttons stand: the leading
// edge of the group of three, and the line their centres sit on, both in dp
// from the window's top-leading corner. It is the whole placement — the buttons
// keep their own size and their own spacing, which are the platform's.
type ButtonPlacement struct{ Leading, Center unit.Dp }

// WindowButtons answers where this window puts its control buttons.
//
// They lead at the page's own margin, so the three of them start on the same
// edge as the title beneath them and everything else down the window, rather
// than on an edge of the platform's choosing a few dp to one side of it. And
// they sit on the title row's centre line, because that row is the strip they
// are in: one row across the top of the window, one line through it.
func WindowButtons() ButtonPlacement {
	return ButtonPlacement{Leading: Pad, Center: TitleCenter}
}

// windowButtonsEnd reports the trailing edge of the window's control buttons,
// and is the one place the page asks the window about them. The edge is
// measured from the buttons themselves rather than worked out from the
// placement above: their size and spacing are the platform's, and a number
// written down here would drift with the next release of it.
//
// A render with no window behind it is told zero — which is every render in the
// test suite, and also every platform that keeps its decorations, where there
// are no such buttons to clear. It is a variable so that a test can state a
// measurement it has no window to take.
var windowButtonsEnd = desktop.LeadingInset

// TitleLead is where the title row's own content may start: one row gap past
// the window's control buttons where the window has them, and the page's own
// margin where it does not. The gap is added because what is measured is the
// bare glass the buttons end at, and the row owes the last of them the same air
// it puts between any two things standing in it.
func TitleLead() unit.Dp {
	if end := windowButtonsEnd(); end > 0 {
		return end + Gap
	}
	return Pad
}

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
// emission — and because the scheme switch is in the title row on the first
// screen too, where the other two are not.
type topClicks struct {
	back   gesture.Click
	keep   gesture.Click
	scheme [2]gesture.Click
}

// Page lays the window out and registers it, whole, as the drop zone.
//
// Both screens open with the same title row: what the window is at its leading
// edge, the way back beside it once there is somewhere to go back to, and the
// scheme switch at its trailing one. It is the same row and not two, which is
// what makes the switch stay put across the click that changes screens — a
// control that moved across the window on the first press would be a second
// control as far as anybody reading is concerned.
//
// Under the row, with a theme on screen, the window is three bands: where the
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

		// One margin all round. The title row draws no ground of its own, so
		// it has nothing to carry the window's top margin on and takes it from
		// the page like every other row. Its own leading edge is the one thing
		// that is not the margin: with the content behind the title bar, the
		// window's control buttons stand in this row, and the row starts past
		// them.
		inset := layout.UniformInset(Pad)
		inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				rigid(reserve(TitleLead()-Pad, TitleRow(p, c, t.typ, m, dark, bar))),
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

// TitleRow is the row across the top of both screens: what the window is at
// its leading edge, the way back beside it, and the switch to the other side
// of the scheme at its trailing one — all on one centre line.
//
// It is a title row and not a bar. Nothing here is drawn on a ground of its
// own and nothing is ruled off from what follows: the row stands on the
// window's own page, and what makes it the top of the window is that it is at
// the top of the window. A band and a rule are what a row needs when the thing
// under it is a different kind of surface, and here it is the same page all the
// way down.
//
// The name is what earns the leading edge. A window that says what it is can
// let everything else in the row be quiet — which is why the way back stands
// beside the name in the register of chrome rather than dressed as a button:
// with a title to read under, a label at the muted step is a control that knows
// its place, where the same label alone on an unnamed row was just a label. It
// is absent altogether before anything has been chosen, because there is
// nowhere to go back to, and the row is a row of two rather than a row with a
// hole in it.
//
// The switch stays in its corner across that change. Both ends are fixed
// places, which is what lets a reader look for the scheme control once.
//
// The row is also the window's own top strip, which is a thing a row on a page
// is not. Nothing above it belongs to the platform, so three of the window's
// controls stand in it — placed on its centre line, with the row starting past
// them — and the run of it that holds nothing is what the window is dragged by,
// the press that would have gone to a title bar having nowhere else to go.
//
// The order the slots are named in is the order they keep their size in when
// the window is too narrow for all of them — see centreRow. The name goes
// first because it is the one thing here that identifies the window; then the
// switch, which is a control with a fixed size and no way to give ground; and
// the way back last, having a truncator to give ground with.
func TitleRow(p Palette, c tokens.ColorTokens, ty Type, m Model, dark bool, bar *topClicks) layout.Widget {
	slots := []slot{{leading, 0, AppTitle(p, ty)}, {trailing, 0, SchemeToggle(c, dark, &bar.scheme)}}
	if len(m.Candidates) > 0 {
		slots = append(slots, slot{leading, 0, WayBack(p, ty, &bar.back)})
	}
	return func(gtx layout.Context) layout.Dimensions {
		dims, free := centreRowFree(gtx, gtx.Dp(TitleH), gtx.Dp(Gap), slots...)
		windowDrag(gtx, free)
		return dims
	}
}

// reserve indents a row past furniture that is not the page's: the run between
// the margin where the row would otherwise begin and the point where it may.
// Nothing is drawn in that run and no drag is declared over it — the window's
// control buttons stand there, and a move action over them would fight them for
// the press.
//
// Where there is nothing to clear the row is handed back untouched, so away from
// the treatment, and in every render with no window behind it, this is not so
// much as a wrapper.
func reserve(d unit.Dp, w layout.Widget) layout.Widget {
	if d <= 0 {
		return w
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: d}.Layout(gtx, w)
	}
}

// windowDrag declares a run of the title row as the handle the window is moved
// by.
//
// A window is normally dragged by the strip its title bar stands in. This row
// has that strip, and the press that would have reached the title bar reaches
// the application instead — so the window's top edge is a handle only where the
// row says it is, and a window that said nowhere could not be moved at all. It
// says so over its empty middle alone, since a move action swallows the press
// before any control under it sees one.
func windowDrag(gtx layout.Context, r image.Rectangle) {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return
	}
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
}

// AppTitle is what the window is, at the head of its own title row, in the
// theme's title register at the size a line of running text takes.
//
// It is not pressable and it is not a crumb. The window has one screen behind
// its other one and a named control that goes there; a title that also went
// somewhere would be a second way to do the same thing, promising a place of
// its own that this window does not have.
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

// WayBack returns the window to its first screen, where the drop well and the
// whole grid of styles are: a chevron and the name of the place it goes, on the
// page itself with no ground and no boundary of its own.
//
// The dressing is the row's doing. A tonal fill with an outline round it was
// the right answer to a label standing alone on an unnamed bar a few points
// from a hint: with nothing to read it against, the label needed a box to be an
// object at all. Beside a title it does not. What it needs is to read as chrome
// under the name and still be legible, and that is a matter of ink: the muted
// step measures 6.19:1 against the light page and 11.06:1 against the dark one,
// both well over the 4.5:1 a line of text has to reach, while the title's own
// ink stands at 17.19:1 and 15.30:1 — so the two are plainly a name and
// something quieter beside it rather than two things of equal weight. Under the
// pointer the label takes the title's ink, which is the whole of the hover
// state: a control with no ground has nothing else to change.
//
// The chevron is what says "control" before the words are read. It is the mark
// the design system carries for going back, drawn rather than typeset, at the
// size a mark takes beside a line of text — and it is bound to its label by the
// smallest step on the scale, so the two are one object and not a symbol
// standing near a word.
func WayBack(p Palette, ty Type, click *gesture.Click) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		mark, gap := gtx.Dp(BackMark), gtx.Dp(BackGap)
		room := gtx.Constraints.Max.X - mark - gap
		if room <= 0 {
			return layout.Dimensions{}
		}
		w := min(natural(gtx, ty.Shaper, ty.Body, BackLabel), room)
		if w <= 0 {
			return layout.Dimensions{}
		}
		ink := p.Muted
		if click.Hovered() || click.Pressed() {
			ink = p.Text
		}
		h := max(gtx.Dp(LineH), mark)
		size := image.Pt(mark+gap+w, h)
		if chevron := icons.Mark(icons.HistoryBack); chevron != nil {
			at(gtx, image.Pt(0, (h-mark)/2), func(gtx layout.Context) { chevron(gtx, mark, ink) })
		}
		textdraw.FillText(gtx, ty.Shaper, ty.Body,
			image.Rect(mark+gap, 0, size.X, h), 0, 0.5, ink, BackLabel)

		area := clip.Rect{Max: size}.Push(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
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

// IdentityRow is the line under the title row on the second screen: where the theme's
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
	dims, _ := centreRowFree(gtx, h, gap, slots...)
	return dims
}

// centreRowFree is centreRow reporting, beside the row it laid out, the run of
// it nothing was laid in: from the trailing edge of the last leading control to
// the leading edge of the first trailing one, the row's full height.
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
