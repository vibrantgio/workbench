// tree.go is the folder tree at the left: an app-local composition over
// components/list in the shell's sidebar slot — the design system's
// sidebar pattern is flat, so nesting is this app's own. TreeRows
// flattens the scanned index and the model's fold state into the visible
// rows (folders first, then notes, name order, indent per depth,
// dot-directories hidden); the view renders them with disclosure toggles
// on folder rows, the current note active, and click-to-open on note rows.
//
// Above the rows sits the find field: typing filters the tree to the
// notes whose name matches, as a flat list with the folder as the faint
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

	"gioui.org/io/event"
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

	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/input"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Tree layout constants. treeRowInsetDp is the rail's one horizontal
// inset system: the find field and the row fills share it, so the
// selection pill's edges line up with the field's own.
const (
	treeWidthDp = 240 // the rail's own width where the slot states none
	// treeRowInsetDp is the shared horizontal inset: the find field and the
	// row pills sit on one pair of edges. It is the sidebar pattern's own
	// measured inset, so a pill drawn here is the pill the platform draws.
	treeRowInsetDp = float32(sidebar.SelectionInset)
	treeRowPadDp   = 8  // breathing room between a fill's edge and its text
	treeIndentDp   = 14 // additional inset per depth level
	// treeDiscloseDp is the disclosure mark's square and treeDiscloseColDp
	// the column it stands in. A tree row carries a part the platform's plain
	// sidebar row does not, so the row's first column holds the disclosure
	// and the symbol and the name stand one column further in — which is why
	// the whole tree is indented one level: a disclosure drawn in the room
	// before the sidebar's own first column would stand outside the selection
	// pill, and no row of the reference draws anything there.
	treeDiscloseColDp = treeIndentDp
	treeDiscloseDp    = 12
	// treeFieldPadDp is the air around the find field. It is the row pills'
	// own inset, so the field's edges and every pill's stand on one pair of
	// lines down the rail — which is the whole of what makes the field read
	// as part of the column rather than as a panel set into it.
	treeFieldPadDp = treeRowInsetDp
)

// treeFieldSurface is the fill the find field stands on: the rail pane,
// which is chrome and wears the platform's chrome material. The live rail
// and the goldens' static rail both name it here so they cannot drift apart.
//
// The field takes the chrome variant, so the surface is not what fills its
// interior — that is the platform's recess — but it is still what the focus
// ring composites onto.
func treeFieldSurface(c tokens.PlatformColors) color.NRGBA { return chromeSurface(c) }

// TreeRow is one visible row of the folder tree.
type TreeRow struct {
	Idx     int    // position in the flattened row slice
	Path    string // vault-relative; the folder path or the note path
	Name    string // display name; the note title for note rows
	Detail  string // the folder a found note is in, read after its name
	Section string // the heading this row begins a run under, where it begins one
	Depth   int    // nesting depth, 0 at the vault root
	IsDir   bool   // a folder row, carrying a disclosure toggle
	Open    bool   // folder rows only: the fold is open
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
//
// The vault's own two runs are headed. At the root the walk emits the
// folders and then the notes that sit loose beside them, so those are the
// two runs the rail shows, and the first row of each carries the heading
// the sidebar draws above it. A vault holding only one of the two is left
// unheaded: a heading over the whole list names nothing the reader cannot
// already see. The vault's top-level folders are NOT the sections — a
// folder is a row, with a disclosure that opens it, and a heading is
// neither.
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
	headRuns(out)
	for i := range out {
		out[i].Idx = i
	}
	return out
}

// headRuns heads the vault's two runs where it has both: the first top-level
// folder and the first note standing loose beside them. A folder's own
// subtree is emitted between its row and the next top-level folder's, so the
// run is not contiguous in the flattened slice; what the heading marks is
// where each run starts, which is what the reader sees.
func headRuns(rows []TreeRow) {
	dir, note := -1, -1
	for i := range rows {
		if rows[i].Depth != 0 {
			continue
		}
		if rows[i].IsDir {
			if dir < 0 {
				dir = i
			}
			continue
		}
		if note < 0 {
			note = i
		}
	}
	if dir < 0 || note < 0 {
		return
	}
	rows[dir].Section, rows[note].Section = "Folders", "Notes"
}

// MatchRows is the find field's answer: the notes whose name contains
// the query, case-insensitively, as flat rows in title order — the folder
// carried after each row's name, since two vaults' worth of notes may
// share a title. A note whose title does not match still
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

// treeView is the tree's own view state: the list scroll/selection state
// and per-row clickables (pointer-stable across frames).
type treeView struct {
	list      *list.State
	hideClick widget.Clickable
	rowClicks []*widget.Clickable

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
// band the scrolling rows occupy. The rows run from under the rail's own
// find field to the pane's foot, with nothing standing below them — the
// vault's two actions stand in the window's toolbar band.
type paneGeom struct {
	rows image.Rectangle
}

// buttonEdge is where the pane's own top strip may start drawing: past the
// window buttons that now stand inside it, and past the air the platform
// leaves after them. It is the chrome row's own lead, so the two halves of
// the window's sidebar switch stand in one column.
func (v *treeView) buttonEdge() unit.Dp {
	if v.leading != nil {
		return v.leading()
	}
	return toolbarLeading()
}

// treeSidebar builds the sidebar slot's layout.Widget stream: the find field
// above the rows. The field is a components SearchField built once at
// subscription scope, so its editor keeps what was typed across
// emissions; each keystroke reaches the model as a SetFilter message, and
// the field's own clear mark reaches it as the same message carrying the
// empty query — which is what takes the rail back to the folder tree and
// the match marks off the rows with it.
// The frame closure reads the model and token snapshots at frame time;
// repaints on model change are driven by the routed layer's re-emission.
func treeSidebar(th rx.Observable[theme.Theme], loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	// The field's own focus tag, so the shortcut this rail answers can put
	// the keyboard in it.
	var fieldTag event.Tag
	field := input.SearchField(th, input.SearchFieldProps{
		Placeholder: "Search",
		Description: "filter notes by name",
		FocusTag:    func(tag event.Tag) { fieldTag = tag },
		// A search field standing on chrome is the platform's flat recess
		// there: no edge, ends fully rounded, a fill of its own a shade off
		// the chrome material — measured off the field at the top of System
		// Settings' sidebar. The rail is chrome, so this field is that one.
		Variant: input.Chrome,
		Surface: treeFieldSurface,
		OnChange: func(gtx layout.Context, text string) {
			mvu.MessageOp{Message: SetFilter{Text: text}}.Add(gtx.Ops)
		},
	})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		v := &treeView{list: list.NewState()}
		return rx.Map(field, func(fieldW layout.Widget) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				focusFindField(gtx, fieldTag)
				return v.layout(gtx, loadModel(), loadTok(), fieldW)
			}
		})
	})
}

// focusFindField puts the keyboard in the rail's find field on the shortcut
// the rail answers with it: the platform's find shortcut with Shift held,
// where the page's own find takes that shortcut plain. Two finds, the wider
// of them one modifier further.
//
// It is bound here rather than on the field, because a shortcut has to be
// live when the reader is nowhere near the control it reaches.
func focusFindField(gtx layout.Context, tag event.Tag) {
	if tag == nil {
		return
	}
	for {
		e, ok := gtx.Event(key.Filter{Name: findKey, Required: key.ModShortcut | key.ModShift})
		if !ok {
			return
		}
		if ke, ok := e.(key.Event); ok && ke.State == key.Press {
			gtx.Execute(key.FocusCmd{Tag: tag})
		}
	}
}

// layout draws the rail: its own top strip, the find field under that, and
// the rows the model asks for — the filter's matches while it is typed in,
// the folder tree otherwise. The returned width is the width the slot
// states, and the rail's own where the slot states none — never an open
// constraint's whole extent.
//
// The rows are the flex's only flexed child and nothing stands below them,
// so they run from under the field to the pane's own foot. The vault's two
// actions stand in the window's toolbar band: the toolbar is the strip
// holding the controls that act on the document, and a control drawn there
// is the platform's bordered toolbar control.
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
	// The rail is as wide as the slot says where the slot says: the pane
	// lays its column out at exactly the width the reader dragged the
	// pane's edge to, and the column standing in it is that width or the
	// pane holds a 240 dp strip with a gap beside it. Where the slot
	// leaves the width open — a rail measured on its own, or drawn for a
	// stored image — the rail states its own.
	railW := gtx.Constraints.Min.X
	if railW <= 0 {
		railW = min(gtx.Dp(treeWidthDp), gtx.Constraints.Max.X)
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
	var fieldH, rowsH int
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
	)
	rowsTop := min(stripH+fieldH, size.Y)
	v.geom = paneGeom{rows: image.Rect(0, rowsTop, size.X, min(rowsTop+rowsH, size.Y))}
	sgtx := gtx
	sgtx.Constraints = layout.Exact(image.Pt(size.X, stripH))
	v.topStrip(sgtx, tok)
	return layout.Dimensions{Size: size}
}

// topStrip is the band the panel keeps clear under the window's control
// buttons, which stand inside it: their space at the leading end, left
// untouched, a stretch that moves the window across the middle, and this
// panel's own toggle bare at its top trailing corner. Under the
// full-size-content treatment the native title bar hands over no drag, so
// a pane that owns the top of the window owes the reader one.
func (v *treeView) topStrip(gtx layout.Context, tok themeTokens) layout.Dimensions {
	// The band is the pattern's: the leading run skipped (a move action over
	// the buttons would fight them for the press), the window's drag across
	// the middle, and this panel's one control at its top trailing corner
	// where the platform keeps it. What the rail supplies is the measurement
	// of where that run ends — a window fact — and the control itself.
	return pane.Strip(gtx, v.buttonEdge(), func(gtx layout.Context) layout.Dimensions {
		return v.hideControl(gtx, tok)
	})
}

// hideControl is the panel's own way to put itself away, standing bare in
// the panel's top trailing corner — where
// voicememos-multi-folder-2026-09-18.png keeps its sidebar toggle. The
// chrome row's toggle is what brings the panel back: a control that travels
// with the panel cannot be the one that recalls it, so the two are the two
// halves of one switch rather than duplicates of one control, and they wear
// one figure to say so. The band's half wears the platform's bordered
// control and this one wears nothing, because that is a property of what
// each stands on.
func (v *treeView) hideControl(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if v.hideClick.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	return paneMark(gtx, tok, &v.hideClick, icons.Sidebar, "Hide the folder rail")
}

// rows lays out the row region below the find field.
func (v *treeView) rows(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	rows := TreeRows(m.Index, m.Folds)
	// The query is empty exactly while the rail shows the folder tree, and
	// then nothing on a row is marked: a highlight lives as long as the
	// search that caused it and no longer.
	query := strings.TrimSpace(m.Filter)
	filtering := query != ""
	if filtering {
		rows = MatchRows(m.Index, m.Filter)
	}
	if len(rows) == 0 {
		if filtering {
			complayout.Inset(treeRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, "No note by that name.", tok.typ.BodyMedium,
					vgcolor.Flatten(tok.col.SecondaryLabel, chromeSurface(tok.col)))
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
	// A tree rail is chrome, so its rows take the sidebar's own row height
	// and the sidebar's own columns: the symbol, the name after it and the
	// trailing end, each where the platform draws it.
	rowH := gtx.Dp(sidebar.RowHeight)
	section := sidebar.SectionStyle(tok.typ)
	return list.LayoutSelectable(gtx, v.list, rows,
		func(gtx layout.Context, row TreeRow, selected bool) layout.Dimensions {
			click := v.rowClicks[row.Idx]
			if click.Clicked(gtx) {
				v.list.Select(row.Idx)
				activateTreeRow(gtx, row)
			}
			width := gtx.Constraints.Max.X
			// A row that begins a run stands under its heading, in a block
			// the sidebar pattern measures. The heading is not a row: it
			// takes no click and the keyboard steps over it.
			head := 0
			if row.Section != "" {
				head = gtx.Dp(sidebar.SectionHeight)
				hGtx := gtx
				hGtx.Constraints = layout.Exact(image.Pt(width, head))
				sidebar.PaintSection(hGtx, tok.shaper, row.Section, section, image.Pt(width, head),
					vgcolor.Flatten(sidebar.SectionForeground(tok.col), chromeSurface(tok.col)))
			}
			defer op.Offset(image.Pt(0, head)).Push(gtx.Ops).Pop()
			gtx.Constraints = layout.Exact(image.Pt(width, rowH))
			click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				active := !row.IsDir && row.Path == m.Current
				// The pill is the sidebar pattern's — its inset, its corner
				// and its two colours — so a rail in this window is a rail
				// on this platform. The open note wears the emphasized fill
				// and a row merely arrowed onto the unemphasized one.
				filled := active || selected
				surface := chromeSurface(tok.col)
				if filled {
					sidebar.PaintSelection(gtx, size, tok.col, !active)
					surface = sidebar.SelectionFill(tok.col, !active)
				}
				semantic.LabelOp(row.Name).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				v.drawRow(gtx, row, tok, size, surface, filled, active, query)
				return layout.Dimensions{Size: size}
			})
			return layout.Dimensions{Size: image.Pt(width, rowH+head)}
		})
}

// treeRowLead is how far a row's own symbol and name stand in from the
// sidebar's measured columns: one indent per depth, the same step at every
// depth, and one more for the disclosure column every row's first column is.
// A row's disclosure stands one indent back from that, at the rail's own
// first column — where the section headings begin, which is where the
// reference puts the first thing on a row.
func treeRowLead(depth int) float32 { return float32(depth+1) * treeIndentDp }

// drawRow paints one tree row's parts into the columns the sidebar pattern
// measures, each shifted by one indent per depth: the disclosure in the
// rail's own leading inset where the row is a folder, the symbol at
// SymbolInset and the name at LabelInset.
//
// A found row's folder stands in the name's own run, after the name: the
// column at the trailing end is the count's, and what the Language puts
// there is how many things the entry holds. The folder is not that — it says
// where the note is, which is part of naming it, so it is read as part of
// the name and set in the secondary label to keep the name itself the more
// pronounced half.
//
// The indent moves the whole row and nothing inside it, so a name at depth 2
// stands exactly two indents right of a name at depth 0 and a row keeps its
// columns when the find flattens the tree.
func (v *treeView) drawRow(gtx layout.Context, row TreeRow, tok themeTokens, size image.Point, surface color.NRGBA, filled, active bool, query string) {
	indent := gtx.Dp(unit.Dp(treeRowLead(row.Depth)))
	// The find mark is the platform's own, and the words it covers keep
	// their colour.
	hl := tok.col.FindHighlight
	secondaryLabel := vgcolor.Flatten(tok.col.SecondaryLabel, surface)
	if filled {
		secondaryLabel = vgcolor.Flatten(sidebar.SelectionLabel(tok.col, !active), surface)
	}

	if row.IsDir {
		col := gtx.Dp(unit.Dp(treeDiscloseColDp))
		mark := gtx.Dp(unit.Dp(treeDiscloseDp))
		back := indent - col + gtx.Dp(sidebar.SymbolInset)
		stk := op.Offset(image.Pt(back+(col-mark)/2, (size.Y-mark)/2)).Push(gtx.Ops)
		drawDisclosure(gtx, row.Open, treeDiscloseDp, secondaryLabel)
		stk.Pop()
	}

	// A folder row draws the folder, a note row the document: the mark names
	// the thing the row stands for.
	name := icons.Document
	if row.IsDir {
		name = icons.Folder
	}
	fg := vgcolor.Flatten(tok.col.Label, surface)
	if filled {
		fg = vgcolor.Flatten(sidebar.SelectionLabel(tok.col, !active), surface)
	}
	// The symbol is drawn the strength the platform draws one, which is
	// stronger than the name beside it: the sidebar's own measured value.
	// The rail's own painter puts it in the rail's own column and fills the
	// square with the mark, so it comes out at the platform's own weight:
	// the set's square keyline is 19 of 24 against the folder symbol's
	// measured 20 across, and its one measured band of 1.4 stands inside
	// that symbol's own 1.37 to 1.50. The offset arithmetic is stated once,
	// in patterns/sidebar; the indent is pushed under it because the indent
	// moves the whole row and nothing inside it.
	symbolFG := vgcolor.Flatten(sidebar.SymbolForeground(tok.col, filled, !active), surface)
	stk := op.Offset(image.Pt(indent, 0)).Push(gtx.Ops)
	sidebar.PaintSymbol(gtx, icons.Mark(name), image.Pt(size.X-indent, size.Y), symbolFG)
	stk.Pop()

	trail := gtx.Dp(sidebar.CountInset)
	lead := indent + gtx.Dp(sidebar.LabelInset)
	room := size.X - lead - trail
	if room <= 0 {
		return
	}
	lGtx := gtx
	lGtx.Constraints = layout.Constraints{Max: image.Pt(room, size.Y)}
	rec := op.Record(gtx.Ops)
	dims := drawFound(lGtx, tok.shaper, row.Name, tok.typ.BodyMedium, fg, hl, query)
	call := rec.Stop()
	stk = op.Offset(image.Pt(lead, (size.Y-dims.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stk.Pop()

	if row.Detail == "" {
		return
	}
	// The folder follows the name in the same run, a gap after it, and takes
	// whatever the name left: a found row whose name fills the rail says
	// where it is by its own name already.
	gap := gtx.Dp(unit.Dp(treeRowPadDp))
	x := lead + dims.Size.X + gap
	left := size.X - trail - x
	if left <= 0 {
		return
	}
	dGtx := gtx
	dGtx.Constraints = layout.Constraints{Max: image.Pt(left, size.Y)}
	rec = op.Record(gtx.Ops)
	dDims := drawFound(dGtx, tok.shaper, row.Detail, tok.typ.BodySmall, secondaryLabel, hl, query)
	call = rec.Stop()
	stk = op.Offset(image.Pt(x, (size.Y-dDims.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stk.Pop()
}

// drawFound draws one of a row's words with the run the query matched marked
// behind it, and draws them plainly when the query matched somewhere else or
// is not there at all.
//
// The mark is a fill behind the characters rather than a colour on them: a
// highlight is applied to content, and recolouring the words would make the
// row's own text say something it does not. What the words are set in is
// unchanged, which the highlighter's own derivation guarantees is still
// readable over it.
//
// A query whose lowercase form is a different length than the text's is left
// unmarked rather than marked in the wrong place: the offsets the search is
// found at are the lowercase text's, and only a text that folds character for
// character carries them back unchanged.
func drawFound(gtx layout.Context, shaper *text.Shaper, msg string, style tokens.TextStyle, fg, hl color.NRGBA, query string) layout.Dimensions {
	start, end, ok := foundRun(msg, query)
	if !ok {
		return drawLabel(gtx, shaper, msg, style, fg)
	}
	macro := op.Record(gtx.Ops)
	dims := drawLabel(gtx, shaper, msg, style, fg)
	call := macro.Stop()
	x0, x1 := runWidth(gtx, shaper, style, msg[:start]), runWidth(gtx, shaper, style, msg[:end])
	// The mark is the role's own line box tall, centred in whatever box the
	// row gave the label — which is the row's full height, and taller. A
	// mark cut to that box would run edge to edge and the marks on two
	// neighbouring rows would meet in one unbroken stripe.
	h := gtx.Sp(unit.Sp(style.LineHeight))
	if h <= 0 || h > dims.Size.Y {
		h = dims.Size.Y
	}
	y0 := (dims.Size.Y - h) / 2
	if x1 > x0 {
		paint.FillShape(gtx.Ops, hl, clip.Rect{
			Min: image.Pt(x0, y0),
			Max: image.Pt(x1, y0+h),
		}.Op())
	}
	call.Add(gtx.Ops)
	return dims
}

// foundRun reports the run of msg the query matched, in bytes, the way
// MatchRows matches: the first case-insensitive occurrence.
func foundRun(msg, query string) (start, end int, ok bool) {
	if msg == "" || query == "" {
		return 0, 0, false
	}
	lower := strings.ToLower(msg)
	if len(lower) != len(msg) {
		return 0, 0, false
	}
	i := strings.Index(lower, strings.ToLower(query))
	if i < 0 {
		return 0, 0, false
	}
	return i, i + len(query), true
}

// runWidth measures how wide a run of text is in the style the row draws it
// in, by laying it out into ops that are thrown away. The measurement drops
// the caller's constraints so a run wider than the rail still reports its
// true width, and it is a hit in the shaper's cache: the same run is laid out
// for real on the next line.
func runWidth(gtx layout.Context, shaper *text.Shaper, style tokens.TextStyle, s string) int {
	if s == "" {
		return 0
	}
	m := gtx
	m.Constraints = layout.Constraints{Max: image.Pt(1<<20, 1<<20)}
	rec := op.Record(m.Ops)
	dims := drawLabel(m, shaper, s, style, color.NRGBA{})
	rec.Stop()
	return dims.Size.X
}

// renderTree is the static counterpart of treeSidebar used by goldens: a
// fresh rail with fresh widget.Clickable state, laid out once from
// pre-resolved tokens and processing no events. The find field is drawn
// through the component's own static path, so the golden carries the same
// field the live rail wears. The window buttons' trailing edge is a parameter
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
	colors tokens.PlatformColors,
	sp tokens.SpacingScale,
	rad tokens.RadiusScale,
	typo tokens.Typography,
	den tokens.Density,
	leading unit.Dp,
) layout.Widget {
	v := &treeView{list: list.NewState(), leading: func() unit.Dp { return leading }}
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	fieldW := input.RenderSearch(shaper, "Search", colors, sp, rad, typo.BodyLarge, den,
		input.RenderState{Text: m.Filter, Surface: treeFieldSurface(colors), Variant: input.Chrome})
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
