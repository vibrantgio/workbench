// tree.go is the folder tree at the left: an app-local composition over
// components/list in the shell's sidebar slot — the design system's
// sidebar pattern is deliberately flat, so nesting is this app's own.
// TreeRows flattens the scanned index and the model's fold state into
// the visible rows (folders first, then notes, name order, indent per
// depth, dot-directories hidden); the view renders them with disclosure
// toggles on folder rows, the current note active, and click-to-open on
// note rows.
//
// Above the rows sits the find field: typing filters the tree to the
// notes whose name matches, as a flat list with the folder as the quiet
// annotation. It is a filter over the names the scan already collected —
// it reads no file and searches no prose.
//
// The column claims a fixed rail width. The shell lets its sidebar slot
// size itself, so a tree that answered with the constraint it was handed
// would take the whole window and leave the note nothing.
//
// Keyboard: the list's arrows move the selection; Return activates the
// selected row — a folder toggles its fold, a note navigates.

package main

import (
	"image"
	"image/color"
	"path"
	"sort"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Tree layout constants. treeRowInsetDp is the rail's one horizontal
// inset system: the find field and the row fills share it, so the
// selection pill's edges line up with the field's own.
const (
	treeWidthDp      = 240 // the rail's own width, whatever the slot offers
	treeRowInsetDp   = 8   // shared horizontal inset: field and row fills
	treeRowPadDp     = 8   // breathing room between a fill's edge and its ink
	treeIndentDp     = 14  // additional inset per depth level
	treeChevronDp    = 16  // fixed disclosure column, so names align per level
	treeFieldPadDp   = 8   // breathing room around the find field
	treePillRadiusDp = 8   // corner radius of the selection/active fill
	treePillVPadDp   = 2   // vertical gap between adjacent row fills
	treeHideBoxDp    = 24  // the pane's own hide control: a square hit area
)

// TreeRow is one visible row of the folder tree.
type TreeRow struct {
	Idx    int    // position in the flattened row slice
	Path   string // vault-relative; the folder path or the note path
	Name   string // display name; the note title for note rows
	Detail string // quiet trailing annotation; the folder on a filtered row
	Depth  int    // nesting depth, 0 at the vault root
	IsDir  bool   // a folder row, carrying a disclosure toggle
	Open   bool   // folder rows only: the fold is open
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

// MatchRows is the find field's answer: the notes whose name contains
// the query, case-insensitively, as flat rows in title order — the folder
// carried as each row's quiet annotation, since two vaults' worth of
// notes may share a title. A note whose title does not match still
// matches on its folder path, so "meetings/" narrows to a folder.
//
// It is a filter over the scanned names and nothing else: no file is
// read, no prose is searched, and a blank query answers nothing so the
// caller falls back to the folder tree.
func MatchRows(idx *Index, query string) []TreeRow {
	q := strings.ToLower(strings.TrimSpace(query))
	if idx == nil || q == "" {
		return nil
	}
	var out []TreeRow
	for _, f := range idx.Files {
		segs := strings.Split(f.Path, "/")
		if hasDotSegment(segs) {
			continue
		}
		base := segs[len(segs)-1]
		title := base[:len(base)-len(path.Ext(base))]
		if !strings.Contains(strings.ToLower(title), q) && !strings.Contains(strings.ToLower(f.Path), q) {
			continue
		}
		folder := path.Dir(f.Path)
		if folder == "." {
			folder = ""
		}
		out = append(out, TreeRow{Path: f.Path, Name: title, Detail: folder})
	}
	// Title order, with the vault's own walk order holding two notes of
	// the same name apart — the folder annotation is what tells them
	// apart on screen.
	sortByName(out, func(r TreeRow) string { return r.Name })
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
	hideClick widget.Clickable
	rowClicks []*widget.Clickable
}

// treeSidebar builds the sidebar slot's widget stream: the find field
// above the rows. The field is a components TextField built once at
// subscription scope, so its editor keeps what was typed across
// emissions; each keystroke reaches the model as a SetFilter message.
// The frame closure reads the model and token snapshots at frame time;
// repaints on model change are driven by the routed layer's re-emission.
func treeSidebar(th rx.Observable[theme.Theme], loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	field := input.TextField(th, input.TextFieldProps{
		Placeholder: "Find a note…",
		Description: "filter notes by name",
		OnChange: func(gtx layout.Context, text string) {
			mvu.MessageOp{Message: SetFilter{Text: text}}.Add(gtx.Ops)
		},
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		v := &treeView{list: list.NewState()}
		return rx.Map(field, func(fieldW layout.Widget) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				return v.layout(gtx, loadModel(), loadTok(), fieldW)
			}
		})
	})
}

// layout draws the rail: a header of the find field and the pane's own
// hide control, then the rows the model asks for — the filter's matches
// while it is typed in, the folder tree otherwise. The returned width is
// the rail's own, never the slot's.
func (v *treeView) layout(gtx layout.Context, m Model, tok themeTokens, fieldW layout.Widget) layout.Dimensions {
	railW := gtx.Dp(treeWidthDp)
	if railW > gtx.Constraints.Max.X {
		railW = gtx.Constraints.Max.X
	}
	size := image.Pt(railW, gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(size)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return complayout.Inset(treeFieldPadDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if fieldW == nil {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
						}
						return fieldW(gtx)
					}),
					layout.Rigid(complayout.HSpacer(treeFieldPadDp)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return v.hideControl(gtx, tok)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return v.rows(gtx, m, tok)
		}),
	)
}

// hideControl is the pane's own way to put itself away, at the corner of
// the rail where the pane ends. The toolbar row's toggle is what brings
// it back — a control that travels with the pane cannot be the one that
// recalls it — so the two exist for different halves of the same state
// rather than as duplicates of one control.
//
// The mark is the chevron the note's history buttons already use, so the
// rail's arrow and the document's arrows are the same shape from the same
// shipped face.
func (v *treeView) hideControl(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if v.hideClick.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	return v.hideClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp("Hide the folder rail").Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		box := gtx.Dp(treeHideBoxDp)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(image.Pt(box, box))
		layout.Center.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
			return drawLabel(gtx, tok.shaper, "‹", tok.typ.TitleMedium, tok.col.Ramps.Neutral.Step(700))
		})
		return layout.Dimensions{Size: image.Pt(box, box)}
	})
}

// rows lays out the row region below the find field.
func (v *treeView) rows(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	rows := TreeRows(m.Index, m.Folds)
	filtering := strings.TrimSpace(m.Filter) != ""
	if filtering {
		rows = MatchRows(m.Index, m.Filter)
	}
	if len(rows) == 0 {
		if filtering {
			complayout.Inset(treeRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, "No note by that name.", tok.typ.BodyMedium, tok.col.Ramps.Neutral.Step(700))
			})
		}
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
				// The fill is a rounded pill inset to the rail's shared
				// horizontal inset — the same edges the find field sits
				// on — never a full-bleed bar. Keyboard selection and the
				// active note keep their distinct colours.
				var fill color.NRGBA
				switch {
				case active:
					fill = tok.col.Ramps.Primary.Step(300)
				case selected:
					fill = tok.col.Ramps.Neutral.Step(300)
				}
				if fill.A > 0 {
					ins := gtx.Dp(unit.Dp(treeRowInsetDp))
					vp := gtx.Dp(unit.Dp(treePillVPadDp))
					r := gtx.Dp(unit.Dp(treePillRadiusDp))
					pill := clip.RRect{
						Rect: image.Rect(ins, vp, size.X-ins, size.Y-vp),
						NE:   r, NW: r, SE: r, SW: r,
					}
					paint.FillShape(gtx.Ops, fill, pill.Op(gtx.Ops))
				}
				semantic.LabelOp(row.Name).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				// Disclosure marks drawn from the shipped face: Roboto
				// owns + and −, where the triangle glyphs resolve only
				// through system fallback and have rendered as
				// missing-glyph boxes at runtime.
				chevron := ""
				if row.IsDir {
					chevron = "+"
					if row.Open {
						chevron = "−"
					}
				}
				layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(complayout.HSpacer(treeRowInsetDp+treeRowPadDp+float32(row.Depth)*treeIndentDp)),
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
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if row.Detail == "" {
							return layout.Dimensions{}
						}
						return drawLabel(gtx, tok.shaper, row.Detail, tok.typ.BodySmall, tok.col.Ramps.Neutral.Step(700))
					}),
					layout.Rigid(complayout.HSpacer(treeRowInsetDp+treeRowPadDp)),
				)
				return layout.Dimensions{Size: size}
			})
		})
}

// renderTree is the static counterpart of treeSidebar used by goldens: a
// fresh rail with fresh widget state, laid out once from pre-resolved
// tokens and processing no events. The find field is drawn through the
// component's own static path, so the golden carries the same field the
// live rail wears.
func renderTree(
	shaper *text.Shaper,
	m Model,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	den tokens.Density,
) layout.Widget {
	v := &treeView{list: list.NewState()}
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	fieldW := input.Render(shaper, "Find a note…", colors, sp, rad, typo.BodyLarge, den,
		input.RenderState{Text: m.Filter})
	return func(gtx layout.Context) layout.Dimensions {
		return v.layout(gtx, m, tok, fieldW)
	}
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
