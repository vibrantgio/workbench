// tree.go is the folder tree at the left: an app-local composition over
// components/list in the shell's sidebar slot — the design system's
// sidebar pattern is deliberately flat, so nesting is this app's own.
// TreeRows flattens the scanned index and the model's fold state into
// the visible rows (folders first, then notes, name order, indent per
// depth, dot-directories hidden); the view renders them with disclosure
// toggles on folder rows, the current note active, and click-to-open on
// note rows.
//
// Keyboard: the list's arrows move the selection; Return activates the
// selected row — a folder toggles its fold, a note navigates.

package main

import (
	"image"
	"path"
	"sort"
	"strings"

	"gioui.org/io/key"
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

// Tree layout constants.
const (
	treeRowInsetDp = 8  // leading inset of a depth-0 row
	treeIndentDp   = 14 // additional inset per depth level
	treeChevronDp  = 16 // fixed chevron column, so names align per level
)

// TreeRow is one visible row of the folder tree.
type TreeRow struct {
	Idx   int    // position in the flattened row slice
	Path  string // vault-relative; the folder path or the note path
	Name  string // display name; the note title for note rows
	Depth int    // nesting depth, 0 at the vault root
	IsDir bool   // a folder row, carrying a disclosure toggle
	Open  bool   // folder rows only: the fold is open
}

// treeNode is the intermediate nested shape TreeRows flattens from.
type treeNode struct {
	dirs  map[string]*treeNode
	notes []TreeRow // Path and Name set; Depth filled at flatten time
}

// TreeRows flattens the scanned index and the fold state into the tree's
// visible rows: at each level the folders in name order then the notes in
// title order (both case-insensitive), a closed folder hiding its whole
// subtree, and any path with a dot-directory segment hidden outright.
func TreeRows(idx *Index, folds map[string]bool) []TreeRow {
	if idx == nil {
		return nil
	}
	root := &treeNode{dirs: map[string]*treeNode{}}
	for _, f := range idx.Files {
		segs := strings.Split(f.Path, "/")
		if hasDotSegment(segs) {
			continue
		}
		cur := root
		for _, seg := range segs[:len(segs)-1] {
			child := cur.dirs[seg]
			if child == nil {
				child = &treeNode{dirs: map[string]*treeNode{}}
				cur.dirs[seg] = child
			}
			cur = child
		}
		base := segs[len(segs)-1]
		title := base[:len(base)-len(path.Ext(base))]
		cur.notes = append(cur.notes, TreeRow{Path: f.Path, Name: title})
	}
	var out []TreeRow
	var walk func(n *treeNode, prefix string, depth int)
	walk = func(n *treeNode, prefix string, depth int) {
		names := make([]string, 0, len(n.dirs))
		for name := range n.dirs {
			names = append(names, name)
		}
		sortByName(names, func(s string) string { return s })
		for _, name := range names {
			dirPath := name
			if prefix != "" {
				dirPath = prefix + "/" + name
			}
			open := folds[dirPath]
			out = append(out, TreeRow{Path: dirPath, Name: name, Depth: depth, IsDir: true, Open: open})
			if open {
				walk(n.dirs[name], dirPath, depth+1)
			}
		}
		notes := append([]TreeRow(nil), n.notes...)
		sortByName(notes, func(r TreeRow) string { return r.Name })
		for _, r := range notes {
			r.Depth = depth
			out = append(out, r)
		}
	}
	walk(root, "", 0)
	for i := range out {
		out[i].Idx = i
	}
	return out
}

// hasDotSegment reports whether any path segment starts with a dot. The
// scanner already skips dot-directories; this keeps the tree honest even
// against an index built some other way.
func hasDotSegment(segs []string) bool {
	for _, s := range segs {
		if strings.HasPrefix(s, ".") {
			return true
		}
	}
	return false
}

// sortByName sorts stably and case-insensitively by the extracted name,
// with the exact spelling as the tiebreak.
func sortByName[T any](s []T, name func(T) string) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := name(s[i]), name(s[j])
		la, lb := strings.ToLower(a), strings.ToLower(b)
		if la != lb {
			return la < lb
		}
		return a < b
	})
}

// treeView is the tree's widget state: the list scroll/selection state
// and per-row clickables (pointer-stable across frames).
type treeView struct {
	list      *list.State
	rowClicks []*widget.Clickable
}

// treeSidebar builds the sidebar slot's widget stream. The frame closure
// reads the model and token snapshots at frame time; repaints on model
// change are driven by the routed layer's re-emission.
func treeSidebar(loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	return rx.Defer(func() rx.Observable[layout.Widget] {
		v := &treeView{list: list.NewState()}
		return rx.Of[layout.Widget](func(gtx layout.Context) layout.Dimensions {
			return v.layout(gtx, loadModel(), loadTok())
		})
	})
}

func (v *treeView) layout(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	rows := TreeRows(m.Index, m.Folds)
	if len(rows) == 0 {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	// Return activates the selected row. The list consumes the arrows;
	// activation is the caller's semantics, filtered on the list's focus
	// tag.
	for _, name := range []key.Name{key.NameReturn, key.NameEnter} {
		for {
			e, ok := gtx.Event(key.Filter{Focus: v.list.Focus(), Name: name})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				if sel := v.list.Selected(); sel >= 0 && sel < len(rows) {
					activateTreeRow(gtx, rows[sel])
				}
			}
		}
	}
	for len(v.rowClicks) < len(rows) {
		v.rowClicks = append(v.rowClicks, &widget.Clickable{})
	}
	rowH := gtx.Dp(list.RowHeight(tok.den))
	return list.LayoutSelectable(gtx, v.list, rows,
		func(gtx layout.Context, row TreeRow, selected bool) layout.Dimensions {
			click := v.rowClicks[row.Idx]
			if click.Clicked(gtx) {
				v.list.Select(row.Idx)
				activateTreeRow(gtx, row)
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				active := !row.IsDir && row.Path == m.Current
				switch {
				case active:
					paint.FillShape(gtx.Ops, tok.col.Ramps.Primary.Step(300), clip.Rect{Max: size}.Op())
				case selected:
					paint.FillShape(gtx.Ops, tok.col.Ramps.Neutral.Step(300), clip.Rect{Max: size}.Op())
				}
				semantic.LabelOp(row.Name).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				chevron := ""
				if row.IsDir {
					chevron = "▸"
					if row.Open {
						chevron = "▾"
					}
				}
				layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(complayout.HSpacer(treeRowInsetDp+float32(row.Depth)*treeIndentDp)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dims := drawLabel(gtx, tok.shaper, chevron, tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(700))
						w := gtx.Dp(treeChevronDp)
						if dims.Size.X > w {
							w = dims.Size.X
						}
						return layout.Dimensions{Size: image.Pt(w, dims.Size.Y), Baseline: dims.Baseline}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawLabel(gtx, tok.shaper, row.Name, tok.typ.BodyMedium, tok.col.Text)
					}),
				)
				return layout.Dimensions{Size: size}
			})
		})
}

// activateTreeRow performs one row's action: a folder toggles its fold,
// a note navigates.
func activateTreeRow(gtx layout.Context, row TreeRow) {
	if row.IsDir {
		mvu.MessageOp{Message: ToggleFold{Dir: row.Path}}.Add(gtx.Ops)
		return
	}
	mvu.MessageOp{Message: Navigate{Path: row.Path}}.Add(gtx.Ops)
}
