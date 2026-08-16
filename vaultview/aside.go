// aside.go is the backlinks panel in the shell's Aside slot: one row per
// note whose outgoing wikilinks resolve to the current note, click
// navigates there. The rows come from Backlinks over the scanned index
// and are memoised per (index, current note) pair, so the resolver runs
// once per landing rather than once per frame.

package main

import (
	"image"
	"path"

	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
)

// Aside layout constants.
const (
	asideInsetDp     = 16
	asideHeaderGapDp = 8
	asideRowInsetDp  = 8
)

// backlinkRow is one citing note in the panel.
type backlinkRow struct {
	Idx    int    // position in the row slice
	Path   string // vault-relative path Navigate opens
	Title  string // the note's display name
	Folder string // the citing note's folder, "" at the vault root
}

// asideView is the panel's widget state: the list scroll state, per-row
// clickables (pointer-stable across frames), and the memoised rows.
type asideView struct {
	list      *list.State
	rowClicks []*widget.Clickable

	// Memo key and value: Backlinks re-runs only when the index pointer
	// or the current note changes.
	memoIdx *Index
	memoCur string
	memo    []backlinkRow
}

// backlinks returns the citing-note rows for the model, recomputing only
// when the index or the current note moved.
func (v *asideView) backlinks(m Model) []backlinkRow {
	if m.Index != v.memoIdx || m.Current != v.memoCur {
		v.memoIdx = m.Index
		v.memoCur = m.Current
		paths := Backlinks(m.Index, m.Current)
		v.memo = make([]backlinkRow, len(paths))
		for i, p := range paths {
			folder := path.Dir(p)
			if folder == "." {
				folder = ""
			}
			v.memo[i] = backlinkRow{Idx: i, Path: p, Title: noteTitle(p), Folder: folder}
		}
	}
	return v.memo
}

// backlinksAside builds the aside slot's widget stream. The frame closure
// reads the model and token snapshots at frame time; repaints on model
// change are driven by the routed layer's re-emission.
func backlinksAside(loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	return rx.Defer(func() rx.Observable[layout.Widget] {
		v := &asideView{list: list.NewState()}
		return rx.Of[layout.Widget](func(gtx layout.Context) layout.Dimensions {
			return v.layout(gtx, loadModel(), loadTok())
		})
	})
}

func (v *asideView) layout(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	rows := v.backlinks(m)
	size := gtx.Constraints.Max
	complayout.Inset(asideInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, "Backlinks", tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
			}),
			layout.Rigid(complayout.VSpacer(asideHeaderGapDp)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(rows) == 0 {
					return drawText(gtx, tok.shaper, "No notes link here.", tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(700))
				}
				return v.rows(gtx, tok, rows)
			}),
		)
	})
	return layout.Dimensions{Size: size}
}

// rows lays out the citing notes: the note title leading, its folder as
// the quiet trailing annotation when it has one.
func (v *asideView) rows(gtx layout.Context, tok themeTokens, rows []backlinkRow) layout.Dimensions {
	for len(v.rowClicks) < len(rows) {
		v.rowClicks = append(v.rowClicks, &widget.Clickable{})
	}
	rowH := gtx.Dp(list.RowHeight(tok.den))
	return list.LayoutSelectable(gtx, v.list, rows,
		func(gtx layout.Context, row backlinkRow, selected bool) layout.Dimensions {
			click := v.rowClicks[row.Idx]
			if click.Clicked(gtx) {
				v.list.Select(row.Idx)
				mvu.MessageOp{Message: Navigate{Path: row.Path}}.Add(gtx.Ops)
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				if selected {
					paint.FillShape(gtx.Ops, tok.col.Ramps.Neutral.Step(300), clip.Rect{Max: size}.Op())
				}
				semantic.LabelOp(row.Title).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				complayout.Inset(asideRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return drawLabel(gtx, tok.shaper, row.Title, tok.typ.BodyMedium, tok.col.Text)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if row.Folder == "" {
								return layout.Dimensions{}
							}
							return drawLabel(gtx, tok.shaper, row.Folder, tok.typ.BodySmall, tok.col.Ramps.Neutral.Step(700))
						}),
					)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				})
				return layout.Dimensions{Size: size}
			})
		})
}
