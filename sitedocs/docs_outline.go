// docs_outline.go is the Docs shell's left tree: the outline of the one
// guide document. Each ## of llms.txt is a top-level row wearing a
// disclosure triangle when it has ### children; disclosed children are
// the ### rows and nothing deeper is shown. Clicking a row scrolls the
// markdown document to that heading's block and marks the row selected;
// clicking a triangle toggles its section's disclosure. Disclosure and
// selection state live in the MVU model (ToggleOutline / SelectHeading) —
// the state here is only Gio's interaction state (clickables, list
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
	"github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
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

// layout draws the tree column: the rail's own fill the fixed width of the
// rail, the visible rows as a scrolling list under a little top padding.
//
// The column is an outline rail, which on this platform is a sidebar, so it
// wears the chrome material — the fill the platform gives a sidebar, a
// toolbar and an inspector. In the light appearance that material is the
// content's own white exactly, so the seam along the rail's trailing edge is
// the whole of what parts the rail from the document there.
func (v *outlineView) layout(gtx layout.Context, st outlineState, tok themeTokens) layout.Dimensions {
	w := gtx.Dp(unit.Dp(docsOutlineWidthDp))
	if w > gtx.Constraints.Max.X {
		w = gtx.Constraints.Max.X
	}
	size := image.Pt(w, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, tok.col.SidebarMaterial, clip.Rect{Max: size}.Op())
	// Two flush regions, so the one drawn first says where it ends: the
	// platform's seam, laid on the material it is drawn inside.
	hair := max(gtx.Dp(unit.Dp(1)), 1)
	paint.FillShape(gtx.Ops, vgcolor.Flatten(tok.col.Separator, tok.col.SidebarMaterial), clip.Rect{
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

	// The pill is the sidebar pattern's — its inset, its corner and its two
	// colours — so a rail in this window is a rail on this platform. The
	// window drawing it is the frontmost one, so the selected row wears the
	// emphasized fill.
	surface := tok.col.SidebarMaterial
	if st.selected == row.Block {
		sidebar.PaintSelection(gtx, size, tok.col, false)
		surface = sidebar.SelectionFill(tok.col, false)
	}

	indent := float32(docsOutlineInsetDp)
	if row.Child {
		indent += docsOutlineIndentDp
	}
	style := tok.typ.BodyMedium
	if row.Child {
		style = tok.typ.BodySmall
	}

	// A row's words and its mark are the platform's label at two strengths,
	// each composited onto the fill it actually lands on — the material, or
	// the pill where this row is the selected one.
	title := tok.col.Label
	if st.selected == row.Block {
		title = sidebar.SelectionLabel(tok.col, false)
	}
	title = vgcolor.Flatten(title, surface)
	discloseMark := vgcolor.Flatten(tok.col.SecondaryLabel, surface)

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
					drawOutlineDisclosure(gtx, row.Open, unit.Dp(docsOutlineMarkDp), discloseMark)
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
				return drawOutlineLabel(gtx, tok.shaper, row.Title, style, title)
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
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	tokensObs := rx.CombineLatest2(colObs, typObs)
	full := rx.CombineLatest2(stateObs, tokensObs)
	return rx.Map(full, func(t rx.Tuple2[outlineState, rx.Tuple2[tokens.PlatformColors, tokens.Typography]]) layout.Widget {
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
	colors tokens.PlatformColors,
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
