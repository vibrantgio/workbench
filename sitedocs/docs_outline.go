// docs_outline.go is the Docs shell's left tree: the outline of the one
// guide document. Each ## of llms.txt is a top-level row wearing a
// disclosure triangle when it has ### children; disclosed children are
// the ### rows and nothing deeper is shown. Clicking a row scrolls the
// markdown document to that heading's block and marks the row selected;
// clicking a triangle toggles its section's disclosure. Disclosure and
// selection state live in the MVU model (ToggleOutline / SelectHeading) —
// the widget here holds only Gio interaction state (clickables, list
// scroll).

package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Outline layout constants. The tree column is wide because the guide's
// ## titles are full sentences; a one-line row truncates what still does
// not fit.
const (
	docsOutlineWidthDp   = 300 // the tree column's width; the guide's ## titles are sentences
	docsOutlineRowHDp    = 28  // one row's height
	docsOutlineInsetDp   = 8   // shared horizontal inset: row fills sit on it
	docsOutlineMarkDp    = 12  // the disclosure mark's own square
	docsOutlineMarkColDp = 20  // fixed column holding it, so titles align
	docsOutlineIndentDp  = 14  // additional inset for ### children
	docsOutlinePillRDp   = 8   // corner radius of the selection fill
	docsOutlinePillVDp   = 2   // vertical gap between adjacent row fills
	docsOutlineTopPadDp  = 8   // breathing room above the first row
)

// outlineState is the model snapshot the tree renders from: which ##
// sections are disclosed and which heading block is selected (-1 none).
type outlineState struct {
	open     map[int]bool
	selected int
}

// outlineRow is one visible row of the flattened tree.
type outlineRow struct {
	Title       string
	Block       int  // heading block index in the parsed document
	Top         int  // index of the owning ## entry
	Child       bool // a ### row
	HasChildren bool // ## rows only: wears a disclosure triangle
	Open        bool // ## rows only: the fold is open
}

// outlineRows flattens the outline and the disclosure state into the
// tree's visible rows: every ## in document order — childless ones
// included — and, under each open ##, its ### children.
func outlineRows(entries []outlineEntry, open map[int]bool) []outlineRow {
	var out []outlineRow
	for i, e := range entries {
		isOpen := open[i]
		out = append(out, outlineRow{
			Title:       e.Title,
			Block:       e.Block,
			Top:         i,
			HasChildren: len(e.Children) > 0,
			Open:        isOpen && len(e.Children) > 0,
		})
		if !isOpen {
			continue
		}
		for _, c := range e.Children {
			out = append(out, outlineRow{Title: c.Title, Block: c.Block, Top: i, Child: true})
		}
	}
	return out
}

// outlineView holds the tree's Gio interaction state, allocated once at
// subscription scope so clicks and scroll survive re-emissions. Rows come
// and go with disclosure, so clickables are keyed by identity — the
// heading's block index for row activation, the ## entry index for the
// triangles — never by visible position.
type outlineView struct {
	entries    []outlineEntry
	scrollTo   func(int)
	list       layout.List
	rowClicks  map[int]*widget.Clickable
	discClicks map[int]*widget.Clickable
}

func newOutlineView(entries []outlineEntry, scrollTo func(int)) *outlineView {
	return &outlineView{
		entries:    entries,
		scrollTo:   scrollTo,
		list:       layout.List{Axis: layout.Vertical},
		rowClicks:  map[int]*widget.Clickable{},
		discClicks: map[int]*widget.Clickable{},
	}
}

func ensureClick(m map[int]*widget.Clickable, key int) *widget.Clickable {
	c := m[key]
	if c == nil {
		c = &widget.Clickable{}
		m[key] = c
	}
	return c
}

// layout draws the tree column: the rail's own ground the fixed width of the
// rail, the visible rows as a scrolling list under a little top padding.
//
// It stands on the BACKDROP. This column is an outline rail — chrome
// furniture — and furniture is the desk the document lies on rather than a
// level stacked over it, in both schemes. colors.Surface would not do: it is
// a RAMP ALIAS rather than a level (neutral 200, which coincides with the
// light scheme's floor but with the dark scheme's RAISED rung), so one line
// of code would put the rail under the guide on paper and over it on slate.
// The floor is neutral 200 on paper and #0C0C0C on slate, below the #181818
// page it indexes.
func (v *outlineView) layout(gtx layout.Context, st outlineState, tok themeTokens) layout.Dimensions {
	w := gtx.Dp(unit.Dp(docsOutlineWidthDp))
	if w > gtx.Constraints.Max.X {
		w = gtx.Constraints.Max.X
	}
	size := image.Pt(w, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, tok.col.SurfaceAt(tokens.LevelBackdrop), clip.Rect{Max: size}.Op())
	// A hairline on the trailing edge parts the tree from the document: in
	// dark schemes the two grounds are close and would otherwise bleed
	// together.
	hair := max(gtx.Dp(unit.Dp(1)), 1)
	paint.FillShape(gtx.Ops, tok.col.Divider, clip.Rect{
		Min: image.Pt(size.X-hair, 0), Max: size,
	}.Op())

	rows := outlineRows(v.entries, st.open)
	pad := gtx.Dp(unit.Dp(docsOutlineTopPadDp))
	defer op.Offset(image.Pt(0, pad)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(size.X, max(size.Y-pad, 0)))
	v.list.Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return v.row(gtx, rows[i], st, tok)
	})
	return layout.Dimensions{Size: size}
}

// row draws one outline row: the selection pill when this row's heading
// is the selected one, the disclosure triangle for a ## with children,
// and the clickable title. The triangle and the title are separate hit
// areas — the triangle only discloses, the title scrolls the document.
func (v *outlineView) row(gtx layout.Context, row outlineRow, st outlineState, tok themeTokens) layout.Dimensions {
	rowH := gtx.Dp(unit.Dp(docsOutlineRowHDp))
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
	size := gtx.Constraints.Max

	click := ensureClick(v.rowClicks, row.Block)
	if click.Clicked(gtx) {
		if v.scrollTo != nil {
			v.scrollTo(row.Block)
		}
		mvu.MessageOp{Message: SelectHeading{Block: row.Block}}.Add(gtx.Ops)
	}

	if st.selected == row.Block {
		ins := gtx.Dp(unit.Dp(docsOutlineInsetDp))
		vp := gtx.Dp(unit.Dp(docsOutlinePillVDp))
		r := gtx.Dp(unit.Dp(docsOutlinePillRDp))
		pill := clip.RRect{
			Rect: image.Rect(ins, vp, size.X-ins, size.Y-vp),
			NE:   r, NW: r, SE: r, SW: r,
		}
		paint.FillShape(gtx.Ops, tok.col.Ramps.Primary.Step(300), pill.Op(gtx.Ops))
	}

	indent := float32(docsOutlineInsetDp)
	if row.Child {
		indent += docsOutlineIndentDp
	}
	style := tok.typ.BodyMedium
	if row.Child {
		style = tok.typ.BodySmall
	}

	children := []layout.FlexChild{
		layout.Rigid(hSpacer(indent)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// The mark column is held whether or not the row has a
			// triangle, so titles align per level.
			col := gtx.Dp(unit.Dp(docsOutlineMarkColDp))
			mark := gtx.Dp(unit.Dp(docsOutlineMarkDp))
			if !row.Child && row.HasChildren {
				disc := ensureClick(v.discClicks, row.Top)
				if disc.Clicked(gtx) {
					mvu.MessageOp{Message: ToggleOutline{Idx: row.Top}}.Add(gtx.Ops)
				}
				dgtx := gtx
				dgtx.Constraints = layout.Exact(image.Pt(col, mark))
				return disc.Layout(dgtx, func(gtx layout.Context) layout.Dimensions {
					semantic.LabelOp("disclose " + row.Title).Add(gtx.Ops)
					semantic.EnabledOp(true).Add(gtx.Ops)
					pointer.CursorPointer.Add(gtx.Ops)
					drawOutlineDisclosure(gtx, row.Open, unit.Dp(docsOutlineMarkDp), tok.col.Ramps.Neutral.Step(700))
					return layout.Dimensions{Size: image.Pt(col, mark)}
				})
			}
			return layout.Dimensions{Size: image.Pt(col, mark)}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(gtx.Constraints.Max)
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp(row.Title).Add(gtx.Ops)
				semantic.EnabledOp(true).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				return drawOutlineLabel(gtx, tok.shaper, row.Title, style, tok.col.Ramps.Neutral.Step(900))
			})
		}),
		layout.Rigid(hSpacer(docsOutlineInsetDp)),
	}
	layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	return layout.Dimensions{Size: size}
}

// hSpacer is a fixed horizontal blank.
func hSpacer(dp float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Dp(unit.Dp(dp)), 0)}
	}
}

// drawOutlineLabel paints a one-line, middle-aligned row title; what does
// not fit the row truncates.
func drawOutlineLabel(
	gtx layout.Context,
	shaper *text.Shaper,
	label string,
	style tokens.TextStyle,
	fg color.NRGBA,
) layout.Dimensions {
	size := gtx.Constraints.Max
	mColor := op.Record(gtx.Ops)
	paint.ColorOp{Color: fg}.Add(gtx.Ops)
	material := mColor.Stop()

	labelGtx := gtx
	labelGtx.Constraints.Min = image.Point{}

	mLabel := op.Record(gtx.Ops)
	labelDims := typeset.Layout(labelGtx, shaper, typeset.Label(style, 1),
		typeset.Font(style, font.Normal), unit.Sp(style.Size), label, material)
	labelCall := mLabel.Stop()

	offY := (size.Y - labelDims.Size.Y) / 2
	if offY < 0 {
		offY = 0
	}
	stk := op.Offset(image.Pt(0, offY)).Push(gtx.Ops)
	labelCall.Add(gtx.Ops)
	stk.Pop()
	return layout.Dimensions{Size: size}
}

// drawOutlineDisclosure draws the disclosure triangle: the shared icon
// mark pointing along the row when closed, rotated a quarter turn open.
func drawOutlineDisclosure(gtx layout.Context, open bool, sizeDp unit.Dp, c color.NRGBA) layout.Dimensions {
	px := gtx.Dp(sizeDp)
	if open {
		half := float32(px) / 2
		defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), math.Pi/2)).Push(gtx.Ops).Pop()
	}
	if mark := icons.Mark(icons.Disclosure); mark != nil {
		mark(gtx, px, c)
	}
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// docsOutline returns the live outline tree observable. The state stream
// carries the model's disclosure map and selection; combining it with the
// token streams keeps the returned layer re-emitting on both model and
// theme changes, which is what drives theme/window's Invalidate and the
// same-frame repaint after a click.
func docsOutline(
	th rx.Observable[theme.Theme],
	entries []outlineEntry,
	stateObs rx.Observable[outlineState],
	scrollTo func(int),
) rx.Observable[layout.Widget] {
	v := newOutlineView(entries, scrollTo)
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	tokensObs := rx.CombineLatest2(colObs, typObs)
	full := rx.CombineLatest2(stateObs, tokensObs)
	return rx.Map(full, func(t rx.Tuple2[outlineState, rx.Tuple2[tokens.ColorTokens, tokens.Typography]]) layout.Widget {
		st := t.First
		typ := t.Second.Second
		tok := themeTokens{col: t.Second.First, typ: typ, shaper: typ.Shaper()}
		return func(gtx layout.Context) layout.Dimensions {
			return v.layout(gtx, st, tok)
		}
	})
}

// renderDocsTab is the static counterpart of the Docs shell's content row
// used by goldens and review captures: the outline tree in the leading
// column and a fresh top-scrolled document filling the rest, laid out
// once from pre-resolved tokens with no event processing.
func renderDocsTab(
	shaper *text.Shaper,
	source []byte,
	st outlineState,
	colors tokens.ColorTokens,
	typo tokens.Typography,
) layout.Widget {
	blocks := markdown.Parse(source)
	// A selected heading means the reader is there: the static render
	// seats the document at that block (NewDocumentAt), so tree selection
	// and document position agree the way they do after a live click.
	var doc *markdown.Document
	if st.selected >= 0 {
		doc = markdown.NewDocumentAt(blocks, st.selected)
	} else {
		doc = markdown.NewDocument(blocks)
	}
	v := newOutlineView(guideOutline(blocks), doc.ScrollToBlock)
	tok := themeTokens{col: colors, typ: typo, shaper: shaper}
	style := docsMarkdownStyle(colors, typo)
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return v.layout(gtx, st, tok)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return drawGuideDoc(gtx, doc, shaper, style)
			}),
		)
	}
}
