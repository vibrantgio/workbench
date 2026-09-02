// tree.go is the folder tree at the left: an app-local composition over
// components/list in the shell's sidebar slot — the design system's
// sidebar pattern is flat, so nesting is this app's own. TreeRows
// flattens the scanned index and the model's fold state into the visible
// rows (folders first, then notes, name order, indent per depth,
// dot-directories hidden); the view renders them with disclosure toggles
// on folder rows, the current note active, and click-to-open on note rows.
//
// Above the rows sits the find field: typing filters the tree to the
// notes whose name matches, as a flat list with the folder as the quiet
// annotation. It is a filter over the names the scan already collected —
// it reads no file and searches no prose.
//
// At the foot of the pane stand the two actions that are the vault's and
// not the document's: rescanning it, and leaving it for another. A
// control that acts on the whole vault belongs to the vault's own column,
// so with the pane put away they go with it; the chrome row's toggle
// brings the pane and them back together.
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
	"gioui.org/op"
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
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Tree layout constants. treeRowInsetDp is the rail's one horizontal
// inset system: the find field and the row fills share it, so the
// selection pill's edges line up with the field's own.
const (
	treeWidthDp       = 240         // the rail's own width, whatever the slot offers
	treeRowInsetDp    = 8           // shared horizontal inset: field and row fills
	treeRowPadDp      = 8           // breathing room between a fill's edge and its ink
	treeIndentDp      = 14          // additional inset per depth level
	treeDiscloseDp    = markSmallDp // the disclosure mark's own square
	treeDiscloseColDp = 20          // fixed column holding it, so names align per level
	treeFieldPadDp    = 8           // breathing room around the find field
	treePillRadiusDp  = 8           // corner radius of the selection/active fill
	treePillVPadDp    = 2           // vertical gap between adjacent row fills
	treeHideBoxDp     = 24          // the pane's own hide control: a square hit area
	treeFootPadDp     = 10          // breathing room above and below the foot's actions
	treeFootGapDp     = 4           // gap between the foot's two hit areas
	treeFootHPadDp    = 8           // an action's hit area either side of its label
	treeFootVPadDp    = 4           // an action's hit area above and below its label
)

// treeFieldLevel is the level of the surface the find field stands on: the
// rail pane, which is the window's furniture and therefore stands at the
// CHROME level. The live rail and the goldens' static rail both name it
// here so they cannot drift apart.
const treeFieldLevel = tokens.LevelChrome

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
	list        *list.State
	hideClick   widget.Clickable
	rescanClick widget.Clickable
	switchClick widget.Clickable
	rowClicks   []*widget.Clickable

	// leading pins the window buttons' trailing edge instead of measuring
	// it: the measurement is a live window's, and a stored image may not
	// depend on one. The live rail leaves this nil and measures.
	leading func() unit.Dp

	// geom is what the last layout arranged, kept so the pane's own
	// stacking can be measured after the fact rather than recomputed from
	// the constants that placed it.
	geom paneGeom
}

// paneGeom is the pane's internal stacking as one layout arranged it: the
// band the scrolling rows occupy, and the foot's band under them. The two
// meet and do not overlap — that is what keeps the rows' scroll the rows'
// own, with the foot standing outside it rather than riding down with it.
type paneGeom struct {
	rows image.Rectangle
	foot image.Rectangle
}

// buttonEdge is where the pane's own top strip may start drawing: past
// the window buttons that now stand inside it.
func (v *treeView) buttonEdge() unit.Dp {
	if v.leading != nil {
		return v.leading()
	}
	return toolbarLeading()
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
		// The field stands on the rail's rounded pane, not on the window
		// surface behind it — the pane covers that. The pane is the
		// window's FURNITURE and so stands at the chrome level; naming that
		// level here is what makes the field's fill, its resting edge and
		// its focus ring all derive against the thing they are actually
		// drawn on. A text field is raised one step off whatever it lies
		// on, so the field fills at the content's own level — lighter than
		// the rail in both schemes,
		// which is the direction the platform draws a search field on a
		// sidebar (the Settings search field sits above its sidebar, not
		// under it).
		Level: treeFieldLevel,
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

// layout draws the rail: its own top strip, the find field under that,
// the rows the model asks for — the filter's matches while it is typed
// in, the folder tree otherwise — and the vault's own actions at the
// foot. The returned width is the rail's own, never the slot's.
//
// The rows are the flex's only flexed child, so the foot takes its own
// height off the pane before the rows are given what is left. That is
// what keeps the two apart: the rows scroll inside a band that stops
// where the foot begins, and no length of vault can push an action off
// the bottom of the window.
//
// The strip is reserved by the flex and drawn afterwards, which is a
// statement about the keyboard and not about paint. Focus follows the
// order the ops are written in, and the reading order of this pane is the
// field, the rows, then the vault's actions: a reader who tabs out of the
// find field means to reach the notes they just filtered for. Drawn last,
// the pane's own control comes after everything the pane is for —
// including the foot, which acts on the vault the pane shows rather than
// on the pane itself.
func (v *treeView) layout(gtx layout.Context, m Model, tok themeTokens, fieldW layout.Widget) layout.Dimensions {
	railW := gtx.Dp(treeWidthDp)
	if railW > gtx.Constraints.Max.X {
		railW = gtx.Constraints.Max.X
	}
	size := image.Pt(railW, gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(size)
	// The strip is the pane's own, not the chrome row's: it is cut to
	// clear the window's control buttons where the window keeps them,
	// which is deeper than the content area's row spends, and the two owe
	// each other nothing — the chrome budget is the content column's, and
	// this band is inside the pane.
	stripH := gtx.Dp(unit.Dp(paneStripDp))
	if stripH > size.Y {
		stripH = size.Y
	}
	var fieldH, rowsH, footH int
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(size.X, stripH)}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := complayout.Inset(treeFieldPadDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if fieldW == nil {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
				}
				return fieldW(gtx)
			})
			fieldH = dims.Size.Y
			return dims
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			dims := v.rows(gtx, m, tok)
			rowsH = dims.Size.Y
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := v.foot(gtx, tok)
			footH = dims.Size.Y
			return dims
		}),
	)
	rowsTop := min(stripH+fieldH, size.Y)
	v.geom = paneGeom{
		rows: image.Rect(0, rowsTop, size.X, min(rowsTop+rowsH, size.Y)),
		foot: image.Rect(0, max(size.Y-footH, 0), size.X, size.Y),
	}
	sgtx := gtx
	sgtx.Constraints = layout.Exact(image.Pt(size.X, stripH))
	v.topStrip(sgtx, tok)
	return layout.Dimensions{Size: size}
}

// foot is the pane's bottom band: a hairline off the rows, and under it
// the two actions that belong to the vault rather than to the note —
// rescan it, or leave it for another. They stand on the same ink margin
// the rows' names do, so the pane reads as one column and not as a bar
// bolted under one.
//
// The rule above them is a hairline: with the foot on the pane's own
// surface there are no two grounds to part, only a seam saying the
// scrolling stops here.
func (v *treeView) foot(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if v.rescanClick.Clicked(gtx) {
		mvu.MessageOp{Message: Rescan{}}.Add(gtx.Ops)
	}
	if v.switchClick.Clicked(gtx) {
		mvu.MessageOp{Message: SwitchVault{}}.Add(gtx.Ops)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			h := max(gtx.Dp(unit.Dp(1)), 1)
			w := gtx.Constraints.Min.X
			paint.FillShape(gtx.Ops, tok.col.Divider, clip.Rect{Max: image.Pt(w, h)}.Op())
			return layout.Dimensions{Size: image.Pt(w, h)}
		}),
		layout.Rigid(complayout.VSpacer(treeFootPadDp)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				// The hit areas start on the row pills' own edge, which
				// puts their labels on the row names' own ink margin.
				layout.Rigid(complayout.HSpacer(treeRowInsetDp)),
				layout.Rigid(footAction(&v.rescanClick, "Rescan", tok)),
				layout.Rigid(complayout.HSpacer(treeFootGapDp)),
				// Title case, which is what the platform's own controls use.
				layout.Rigid(footAction(&v.switchClick, "Switch Vault", tok)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
				}),
			)
		}),
		layout.Rigid(complayout.VSpacer(treeFootPadDp)),
	)
}

// footAction renders one of the foot's affordances: a pressable label,
// named for the screen reader and drawn at full text contrast, since a
// bare label at the low-contrast neutral step reads as a disabled control
// rather than a live one.
//
// The label sits in a hit area of its own, and that area fills under the
// pointer and darkens while it is held: a bare label says nothing about
// being pressable until something answers the pointer. The fill is the
// rows' own pill in the rows' own neutral steps, so the foot answers the
// way everything above it does rather than inventing a button.
func footAction(click *widget.Clickable, label string, tok themeTokens) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			hp, vp := gtx.Dp(treeFootHPadDp), gtx.Dp(treeFootVPadDp)
			macro := op.Record(gtx.Ops)
			igtx := gtx
			igtx.Constraints.Min = image.Point{}
			dims := drawLabel(igtx, tok.shaper, label, tok.typ.LabelLarge, tok.col.Text)
			call := macro.Stop()
			size := image.Pt(dims.Size.X+2*hp, dims.Size.Y+2*vp)
			var fill color.NRGBA
			switch {
			case click.Pressed():
				fill = tok.col.StateAt(tokens.LevelChrome, tokens.StatePressed)
			case click.Hovered():
				fill = tok.col.StateAt(tokens.LevelChrome, tokens.StateHover)
			}
			if fill.A > 0 {
				r := gtx.Dp(unit.Dp(treePillRadiusDp))
				pill := clip.RRect{Rect: image.Rectangle{Max: size}, NE: r, NW: r, SE: r, SW: r}
				paint.FillShape(gtx.Ops, fill, pill.Op(gtx.Ops))
			}
			defer op.Offset(image.Pt(hp, vp)).Push(gtx.Ops).Pop()
			call.Add(gtx.Ops)
			return layout.Dimensions{Size: size, Baseline: dims.Baseline + vp}
		})
	}
}

// topStrip is the band the pane keeps clear under the window's control
// buttons now that the top of the window is the pane's: their space at
// the leading end, left untouched, the pane's own toggle at the trailing
// corner, and between the two a stretch that moves the window. Under the
// full-size-content treatment the native title bar hands over no drag, so
// a pane that owns the top of the window owes the reader one.
func (v *treeView) topStrip(gtx layout.Context, tok themeTokens) layout.Dimensions {
	// The band is the pattern's: the buttons' span skipped at the leading
	// end (a move action over them would fight them for the press), the
	// window's drag across the middle, and this pane's one control at the
	// trailing corner. What the rail supplies is the measurement of where
	// the buttons end — a window fact — and the control itself.
	return pane.Strip(gtx, v.buttonEdge(), func(gtx layout.Context) layout.Dimensions {
		return v.hideControl(gtx, tok)
	})
}

// hideControl is the pane's own way to put itself away, at the pane's
// top-right corner — where the platform's own sidebars keep it. The
// chrome row's toggle is what brings the pane back once it is gone: a
// control that travels with the pane cannot be the one that recalls it,
// so the two are the two halves of one switch rather than duplicates of
// one control, and they wear one figure to say so.
func (v *treeView) hideControl(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if v.hideClick.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	return v.hideClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		box := gtx.Dp(treeHideBoxDp)
		cgtx := gtx
		cgtx.Constraints = layout.Exact(image.Pt(box, max(gtx.Constraints.Max.Y, box)))
		return layout.Center.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
			return railToggleMark(gtx, tok, "Hide the folder rail")
		})
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
					fill = tok.col.StateAt(tokens.LevelChrome, tokens.StateHover)
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
				layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(complayout.HSpacer(treeRowInsetDp+treeRowPadDp+float32(row.Depth)*treeIndentDp)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// The column is held whether or not the row has a
						// mark to put in it, so names line up per level.
						// It is wider than the mark because the mark turns:
						// closed, the mark is narrow and its square has
						// room to spare either side; open, it lies across
						// the full width of that square and would otherwise
						// end where the name begins.
						mark := gtx.Dp(treeDiscloseDp)
						if row.IsDir {
							drawDisclosure(gtx, row.Open, treeDiscloseDp, tok.col.Ramps.Neutral.Step(700))
						}
						return layout.Dimensions{Size: image.Pt(gtx.Dp(treeDiscloseColDp), mark)}
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
// live rail wears. The window buttons' trailing edge is a parameter here
// and a measurement in the live pane, since a stored image may not depend
// on a live window's measurement.
//
// The surface the field is rendered on is named here as well as in
// [treeSidebar], and it must be the same one: a golden that pictures a
// field standing on a surface the rail does not have cannot catch a
// regression in what the window draws.
func renderTree(
	shaper *text.Shaper,
	m Model,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	den tokens.Density,
	leading unit.Dp,
) layout.Widget {
	v := &treeView{list: list.NewState(), leading: func() unit.Dp { return leading }}
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	fieldW := input.Render(shaper, "Find a note…", colors, sp, rad, typo.BodyLarge, den,
		input.RenderState{Text: m.Filter, Level: treeFieldLevel})
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
