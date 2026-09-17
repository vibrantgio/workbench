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
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/imagecolor"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Layout dimensions. None of them varies with the colour scheme.
const (
	Pad      unit.Dp = 20 // window margin
	Gap      unit.Dp = 14 // between the title row and the first group
	Radius   unit.Dp = 12 // the picture's mat, a swatch card, the code plate
	Hairline unit.Dp = 1  // a resting outline
	Ring     unit.Dp = 2  // the drag highlight

	// TitleH is the row across the top of the window: the plain title bar
	// band, 32, which is the band ADR-019 measured TextEdit's window
	// buttons centred in.
	TitleH unit.Dp = 32
	// TitleCenter is the line everything in the title row is centred on,
	// measured from the window's top edge: the page's margin plus half the
	// row. The window's own control buttons stand on it too.
	TitleCenter = Pad + TitleH/2
)

// The choices column's rhythm, measured at 1x off System Settings' Appearance
// pane — `reference/macos/system-settings-grouped-box-{light,dark}.png`, the
// whole 723×720 window — with column scans for the edges between the pane's
// plane and the boxes standing on it.
//
// A box's bottom edge to the next box's top edge is 56 (the plane runs
// 225–280 and 521–576), the group's title standing in that run: its cap band
// sits 33 under the box above it and 14 over its own. GroupAbove, GroupTitleH
// and GroupBelow add back to that 56, with the title's cap centred in the row.
const (
	GroupAbove  unit.Dp = 18
	GroupTitleH unit.Dp = 28
	GroupBelow  unit.Dp = 10

	// BoxRadius is the grouped box's corner, fitted to the contour of the
	// box at y 577 in the light capture: the fill reaches the box's own
	// leading edge 9 rows down, a continuous corner a hair wider than the
	// circle of that radius. BoxInset is the run from the box's edge to
	// what stands in it — the seam between two rows spans x 253–692 inside
	// a box spanning 243–702, and a row's label starts on the same line.
	BoxRadius unit.Dp = 10
	BoxInset  unit.Dp = 10
	// BoxPad is the air above and below a box's content. The platform's own
	// rows carry theirs inside a 38 row pitch; ours hold objects taller than
	// a line of text, so the box states the air and the content states its
	// height.
	BoxPad unit.Dp = 10
)

// The boxes' heights, and the footer's. The last row's two boxes — the style
// list and the preview beside it — take what is left, which is what makes the
// column close at whatever height the window opens at.
const (
	PictureBoxH unit.Dp = 84
	ColourBoxH  unit.Dp = 48
	FaceBoxH    unit.Dp = 46
	FooterH     unit.Dp = 28
)

// PictureLabel and ColourLabel head the first two groups; the other three
// are headed by the sections that draw them.
const (
	PictureLabel = "Picture"
	ColourLabel  = "Theme colour"
)

const (
	// ThumbW is the picture's mat, square, and ThumbPad the air inside it.
	ThumbW   unit.Dp = 64
	ThumbPad unit.Dp = 4
	// KeepW is the keep affordance's width, fixed so the control keeps its
	// size whichever of its two words is on it; HexW is the colour field's.
	KeepW unit.Dp = 150
	HexW  unit.Dp = 132
	// ChipW is the colour in force, drawn beside the field that can replace
	// it, and ChipH its height.
	ChipW unit.Dp = 44
	ChipH unit.Dp = 20
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
const rowSlots = imagecolor.DefaultMax + 1

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
			BackdropLayer(th, modelObs),
			ContentLayer(th, modelObs, zones),
		}
	}
}

// themed carries one emission's platform colours and typography — the
// platform's live reading of the appearance the desktop is on. What the
// window actually draws in is [WindowSet] of it, which is this set while the
// switch agrees with the desktop and the other side's when it does not.
type themed struct {
	col tokens.PlatformColors
	typ Type
	// pinned says the faces are the test suite's rather than the machine's.
	pinned bool
}

// codeType is the typography the code sample draws through: the chosen code
// face applied to the theme's roles. The live path appends emoji the way a
// kept brand's own typography does, so a fence cannot flash tofu before the
// stream emits. Tests pin the faces so a render here cannot depend on the
// machine's font set, and they stay off the colour-emoji face, which no
// pinned render parses.
func (t themed) codeType(m Model) (tokens.Typography, *text.Shaper) {
	applied := tokens.CodeFace(m.AppliedMono())
	if t.pinned {
		return applied, applied.DeterministicShaper()
	}
	applied = applied.WithEmoji()
	return applied, applied.Shaper()
}

// BackdropLayer fills the window, in the appearance the switch at the top of
// the window is on.
//
// It follows the desktop until that switch is pressed and the window's own
// answer from then on, which is the whole of what the switch does: the theme
// being chosen has two appearances, and a person settling one has to see both
// without waiting for the desktop to change its mind.
func BackdropLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	return rx.Map(rx.CombineLatest2(colors, modelObs),
		func(n rx.Tuple2[tokens.PlatformColors, Model]) layout.Widget {
			return backdrop.Widget(WindowSet(n.First, n.Second).WindowBackground)
		})
}

// WindowTheme is the theme a published component in this window draws
// through: the live theme with its platform set replaced by [WindowSet] of
// it, so a component that reads its own colours off the theme wears the
// appearance the switch at the top of the window is on rather than the
// desktop's. Everything else the theme carries — typography, density, the
// scales — is the live theme's unchanged.
//
// A component flattens the theme, so it re-subscribes what the set is
// combined with every time the theme re-emits. The model stream this reads
// is [mvu.Loop]'s, which carries the current model to a subscriber whenever
// it attaches; a stream without that replay would leave the component with
// no colours until the next message.
func WindowTheme(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[theme.Theme] {
	return rx.Map(th, func(t theme.Theme) theme.Theme {
		live := t.Platform
		t.Platform = rx.Map(rx.CombineLatest2(live, modelObs),
			func(n rx.Tuple2[tokens.PlatformColors, Model]) tokens.PlatformColors {
				return WindowSet(n.First, n.Second)
			})
		return t
	})
}

// HexField is the field the theme colour is written into: the published live
// text field, built on [WindowTheme] so its own fill and foreground follow
// the appearance switch along with the page around it.
//
// It stands in the theme colour group's box, so the box's fill is what it
// says it stands on: the platform draws a field as a hairline around the
// surface beneath it rather than as a box of its own, which the save
// dialog's field measures in both appearances
// (reference/macos/save-dialog-light.png and -dark.png: the field's interior
// is the sheet's own fill). Left unsaid the field would fill with
// TextBackground, the window's content plane, which is not the plane this
// one stands on.
func HexField(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	return input.TextField(WindowTheme(th, modelObs), input.TextFieldProps{
		Placeholder: HexPlaceholder,
		Description: "Theme colour, written as a hex triplet",
		Surface:     func(c tokens.PlatformColors) stdcolor.NRGBA { return PaletteFrom(c).Surface },
		OnChange: func(gtx layout.Context, txt string) {
			mvu.MessageOp{Message: HexTyped{Text: txt}}.Add(gtx.Ops)
		},
	})
}

// ContentLayer renders the page: the titled groups in reading order, and
// under them the list of highlighter styles beside the preview of the
// platform's set with the chosen colour standing in for the accent.
//
// The click handlers, the colour field and the style column's scroll position
// live at subscription scope, outside the per-emission Map. A gesture handler
// reconstructed every emission loses the press it is in the middle of, and
// every selection re-emits. There is one click handler per swatch slot, not
// per colour, so the handlers outlive a picture being replaced by another.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], zones *desktop.ZoneGroup) rx.Observable[layout.Widget] {
	clicks := make([]gesture.Click, rowSlots)
	bar := new(topClicks)
	code := newCodeState()
	hex := HexField(th, modelObs)
	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Platform, t.Typography),
			func(n rx.Tuple2[tokens.PlatformColors, tokens.Typography]) themed {
				return themed{col: n.First, typ: TypeFrom(n.Second)}
			})
	})
	return rx.Map(rx.CombineLatest3(themes, modelObs, hex),
		func(n rx.Tuple3[themed, Model, layout.Widget]) layout.Widget {
			return Page(n.First, n.Second, zones, clicks, bar, code, n.Third)
		})
}

// topClicks are the handlers of the controls the page keeps outside its
// boxes: the keep affordance in the footer and the two halves of the
// appearance switch in the title row. They are one value rather than
// two parameters because they have one lifetime — subscription scope, so a
// press in flight survives an emission.
type topClicks struct {
	keep   gesture.Click
	scheme [2]gesture.Click
}

// Page lays the window out and registers it, whole, as the drop zone.
//
// One column, top to bottom: the title row, carrying the window's name and
// the appearance switch that moves the whole window; three titled groups —
// the picture and the colours it gave, the theme colour in force, the code
// face; then the row those choices are judged in, the list of highlighter
// styles beside the preview, which take the room left over because they are
// what is being judged; and a footer holding the one default button this
// window has.
//
// The style list and the preview stand side by side because the list is a
// column of names and the preview is a picture of a window: a full-width list
// would be a column of names in a field of nothing, and a preview under a
// keyhole of five names is what this page had before. Both take the whole run
// from the last choice to the footer, so the list is as long as the window
// allows and the picture as large.
func Page(t themed, m Model, zones *desktop.ZoneGroup, clicks []gesture.Click, bar *topClicks, code *codeState, hex layout.Widget) layout.Widget {
	dark := m.Dark(t.col)
	win := WindowSet(t.col, m)
	p := PaletteFrom(win)
	shown := PreviewSet(t.col, m, dark)
	typo, shaper := t.codeType(m)
	docStyle := CodeStyle(shown, typo, shown.ControlBackground, m.AppliedStyles())
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
				rigid(reserve(TitleLead()-Pad, TitleRow(p, t.typ, SchemeToggle(win, dark, &bar.scheme)))),
				spacer(Gap),

				rigid(fixedH(GroupTitleH, GroupTitle(p, t.typ, PictureLabel, RowHintFor(m), nil))),
				spacer(GroupBelow),
				rigid(fixedH(PictureBoxH, GroupBox(p, PictureRow(p, t.typ, m, picture, clicks)))),
				spacer(GroupAbove),

				rigid(fixedH(GroupTitleH, GroupTitle(p, t.typ, ColourLabel, ColourHint, nil))),
				spacer(GroupBelow),
				rigid(fixedH(ColourBoxH, GroupBox(p, ColourRow(p, t.typ, m, hex)))),
				spacer(GroupAbove),

				rigid(fixedH(GroupTitleH, GroupTitle(p, t.typ, FaceLabel, FaceHint, nil))),
				spacer(GroupBelow),
				rigid(fixedH(FaceBoxH, GroupBox(p, FaceChoices(p, t.typ, m, code.faces)))),
				spacer(GroupAbove),

				layout.Flexed(1, JudgingRow(
					TitledBox(p, t.typ, StyleLabel, StyleHint, StylePanel(p, win, t.typ, m, dark, code.styles)),
					TitledBox(p, t.typ, PreviewLabel, PreviewHint,
						Preview(shown, t.typ, typo, shaper, code.doc, docStyle)))),
				spacer(GroupAbove),

				rigid(fixedH(FooterH, Footer(p, win, t.typ, m, &bar.keep))),
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

// JudgingRow is the run under the choices: the style list at its leading edge
// at the list's own width, and the preview taking everything else. Both are
// whole titled groups and both are as tall as the row.
func JudgingRow(list, preview layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			rigid(fixedW(StyleW, list)),
			layout.Rigid(layout.Spacer{Width: Gap}.Layout),
			layout.Flexed(1, preview),
		)
	}
}

// TitledBox is one titled group taking the whole height it is given: the
// title row, the run under it, and the box with everything left over.
func TitledBox(p Palette, ty Type, title, hint string, inner layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			rigid(fixedH(GroupTitleH, GroupTitle(p, ty, title, hint, nil))),
			spacer(GroupBelow),
			layout.Flexed(1, GroupBox(p, inner)),
		)
	}
}

// ColourHint and FaceHint say what the group under them settles, beside its
// title. The picture's and the syntax style's are answers about the state on
// screen and are worked out per emission. Every one of them is a sentence:
// read cold, the lowercase fragments this row used to carry were called "a
// note to the implementer, not to a user".
const (
	ColourHint     = "Where the platform paints its accent, this colour stands."
	ColourRowLabel = "Colour"
	FaceHint       = "The typeface a code block is set in."
)

// GroupTitle is one group's title row: what the group is at its leading edge,
// what there is to say about it after that, and — where a group has one — the
// control that says which of two things the group is being set for, at the
// trailing end.
//
// The hint stands beside the title and not at the trailing edge. Read cold,
// a caption pushed out to the window's far margin — a thousand points from
// the words it belongs to — made every section a zigzag: title far leading,
// caption far trailing, the value back at the leading edge inside the box,
// the control at the trailing edge again. The platform never separates the
// two: its own captions stand inside the box under the label they explain.
func GroupTitle(p Palette, ty Type, title, hint string, control layout.Widget) layout.Widget {
	slots := []slot{{leading, 0, Line(ty, ty.Label, p.Text, title)}}
	if control != nil {
		slots = append(slots, slot{at: trailing, w: control})
	}
	if hint != "" {
		slots = append(slots, slot{leading, 0, Line(ty, ty.Small, p.Muted, hint)})
	}
	return func(gtx layout.Context) layout.Dimensions {
		return centreRow(gtx, gtx.Dp(GroupTitleH), gtx.Dp(Gap), slots...)
	}
}

// GroupBox is the platform's grouped box: a rounded rectangle of the box's
// own fill with what stands in it inset from its edges.
//
// It carries no hairline and no shadow. The captures say so — a column
// crossing a box's top edge in `system-settings-grouped-box-light.png` steps
// from the pane's plane straight to the fill over two or three rows of
// antialiasing, with no darker line anywhere in the run.
func GroupBox(p Palette, inner layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		fillRRect(gtx, image.Rectangle{Max: size}, gtx.Dp(BoxRadius), p.Surface)
		return layout.Inset{
			Top: BoxPad, Bottom: BoxPad, Left: BoxInset, Right: BoxInset,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			inner(gtx)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
	}
}

// TitleRow is the row across the top of the window: what the window is, at
// its leading edge, and the appearance switch at its trailing one, on one
// centre line.
//
// The switch stands here because what it moves is the window. Every choice
// below it is made for one appearance at a time — the theme colour is judged
// on the plane it lands on, and a highlighter style is fitted to a background
// — so the window shows one appearance, and the control that says which is
// above everything it governs rather than beside one of the groups.
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
func TitleRow(p Palette, ty Type, control layout.Widget) layout.Widget {
	slots := []slot{{leading, 0, AppTitle(p, ty)}}
	if control != nil {
		slots = append(slots, slot{at: trailing, w: control})
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
	return Line(ty, ty.Head, p.Text, AppName)
}

// Line is one line of text as wide as it wants to be and no wider than the
// room it is offered, centred on the row it is laid in. It is what every slot
// in this window that carries a word rather than a control is made of.
//
// It takes the row's full height rather than a line box of its own, so a role
// set larger than the running text is centred in the row instead of clipped
// by a height written down beside it.
func Line(ty Type, style textdraw.TextStyle, col stdcolor.NRGBA, s string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		w := min(natural(gtx, ty.Shaper, style, s), gtx.Constraints.Max.X)
		if w <= 0 || s == "" {
			return layout.Dimensions{}
		}
		size := image.Pt(w, gtx.Constraints.Max.Y)
		textdraw.FillText(gtx, ty.Shaper, style, image.Rectangle{Max: size}, 0, 0.5, col, s)
		return layout.Dimensions{Size: size}
	}
}

// PictureRow is what stands in the picture group's box: the mat the dropped
// picture is shown on, and beside it the colours that picture gave, as cards.
//
// The mat leads because it is what the cards are read against — a colour
// pulled out of a picture is judged by looking from the swatch back to the
// thing it came from.
func PictureRow(p Palette, ty Type, m Model, src paint.ImageOp, clicks []gesture.Click) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		mat := gtx.Dp(ThumbW)
		at(gtx, image.Point{}, func(gtx layout.Context) {
			gtx.Constraints = layout.Exact(image.Pt(mat, size.Y))
			Thumbnail(p, ty, m, src)(gtx)
		})
		lead := mat + gtx.Dp(BoxInset)
		if lead >= size.X {
			return layout.Dimensions{Size: size}
		}
		at(gtx, image.Pt(lead, 0), func(gtx layout.Context) {
			gtx.Constraints = layout.Exact(image.Pt(size.X-lead, size.Y))
			SwatchRow(p, ty, m, clicks)(gtx)
		})
		return layout.Dimensions{Size: size}
	}
}

// ColourRow is what stands in the theme colour group's box: the row's own
// label, then the colour in force and where it came from, and at the trailing
// end the two things that can replace it — the colour itself, and the field
// it can be written into.
//
// The label leads because that is where the platform puts one. Read cold
// without it, the row went value, then provenance, then control — "the
// platform's order with the label deleted".
func ColourRow(p Palette, ty Type, m Model, hex layout.Widget) layout.Widget {
	name, hint, hintColor := IdentityName(m), IdentityHint(m), p.CardMuted
	if m.Problem != "" {
		hint, hintColor = m.Problem, p.Problem
	}
	slots := []slot{
		{leading, 0, Line(ty, ty.Body, p.CardText, ColourRowLabel)},
		{trailing, 0, fixed(HexW, hex)},
		{trailing, 0, Chip(p, m)},
		{leading, 0, Line(ty, ty.Body, p.CardText, name)},
		{leading, 0, Line(ty, ty.Small, hintColor, hint)},
	}
	return func(gtx layout.Context) layout.Dimensions {
		return centreRow(gtx, gtx.Constraints.Max.Y, gtx.Dp(Gap), slots...)
	}
}

// Chip is the colour in force, drawn beside the field that can replace it. It
// carries a hairline of its own for the same reason a swatch does: a picture's
// palest colour is a legal choice, and a near-white chip on a near-white box
// with no boundary reads as a box that failed to draw.
func Chip(p Palette, m Model) layout.Widget {
	col, ok := m.Color()
	return func(gtx layout.Context) layout.Dimensions {
		if !ok {
			return layout.Dimensions{}
		}
		size := image.Pt(gtx.Dp(ChipW), gtx.Dp(ChipH))
		r := image.Rectangle{Max: size}
		fillRRect(gtx, r, gtx.Dp(InnerR), col)
		strokeRRect(gtx, r, gtx.Dp(InnerR), gtx.Dp(Hairline), p.Edge)
		return layout.Dimensions{Size: size}
	}
}

// Footer is the run under the preview: what went wrong at its leading edge,
// and the window's one default button at its trailing one.
func Footer(p Palette, c tokens.PlatformColors, ty Type, m Model, click *gesture.Click) layout.Widget {
	slots := []slot{{trailing, 0, KeepButton(c, ty, m, click)}}
	if m.Problem != "" {
		slots = append(slots, slot{leading, 0, Line(ty, ty.Small, p.Problem, m.Problem)})
	}
	return func(gtx layout.Context) layout.Dimensions {
		return centreRow(gtx, gtx.Dp(FooterH), gtx.Dp(Gap), slots...)
	}
}

// fixedH gives a row a height of its own inside a column that would otherwise
// hand it everything left over.
func fixedH(h unit.Dp, inner layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(h))
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return inner(gtx)
	}
}

// fixedW gives a row's child a width of its own and asks it to fill that
// width, which is what a titled group standing beside another one needs: a
// child left to ask for its own width would report the widest word in it.
func fixedW(w unit.Dp, inner layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(w))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return inner(gtx)
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
				mvu.MessageOp{Message: KeepColor{}}.Add(gtx.Ops)
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
		size := gtx.Constraints.Max
		r := image.Rectangle{Max: size}
		fill, edge, width := p.Backdrop, p.Edge, gtx.Dp(Hairline)
		if m.DragOver {
			fill, edge, width = p.Selection, p.Accent, gtx.Dp(Ring)
		}
		fillRRect(gtx, r, gtx.Dp(InnerR), fill)
		strokeRRect(gtx, r, gtx.Dp(InnerR), width, edge)
		if m.Preview == nil {
			foreground := p.CardMuted
			if m.DragOver {
				foreground = p.AccentForeground
			}
			textdraw.FillText(gtx, ty.Shaper, ty.Small, r, 0.5, 0.5, foreground, "Drop")
			return layout.Dimensions{Size: size}
		}
		defer clip.UniformRRect(r, gtx.Dp(InnerR)).Push(gtx.Ops).Pop()
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
// control that changes appearance looks the same wherever the inventory is
// shown; what is added here is the press.
//
// One switch, at the top of the window, moving everything under it: the
// window's own plane and every group on it, the list of highlighter styles,
// and the picture of an application in the preview. A theme has two
// appearances and this window shows one of them at a time, which is what
// makes the preview a look rather than two looks and a memory.
//
// A target per segment, not one over the pair. Each half names an appearance
// and the message it sends says which — pointing at the moon asks for dark
// from either side, and pointing at the half already filled asks for the
// appearance that is already on, which the update treats as the no-op it
// is.
func SchemeToggle(c tokens.PlatformColors, dark bool, clicks *[2]gesture.Click) layout.Widget {
	segment := func(i int, wantDark bool) layout.FlexChild {
		draw := inventory.SchemeSegment(c, wantDark, dark == wantDark)
		// The press area is the track the segment draws: this control's
		// pointer target is the control.
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
			return press(gtx, draw)
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
