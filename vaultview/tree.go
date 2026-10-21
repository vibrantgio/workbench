// tree.go is the folder tree at the left: an app-local composition over
// components/list in the shell's sidebar slot — the design system's
// sidebar pattern is flat, so nesting is this app's own. TreeRows
// flattens the scanned index and the model's fold state into the visible
// rows (one run per folder in name order, folders and notes interleaved,
// indent per depth, dot-directories hidden); the view renders them with
// disclosure toggles on folder rows, the current note active, and
// click-to-open on note rows.
//
// Above the rows sits the find field: typing filters the tree to the
// notes whose name matches, as a flat list with the folder as the faint
// annotation. It is a filter over the names the scan already collected —
// it reads no file and searches no prose.
//
// The column fills the pane it stands in: patterns/shell lays it out at
// exactly the width the reader dragged the pane's edge to.
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

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
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
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/system/naming"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// Tree layout constants. treeRowInsetDp is the rail's one horizontal
// inset system: the find field and the row fills share it, so the
// selection pill's edges line up with the field's own.
const (
	treeWidthDp = 240 // the pane's width where nothing was kept
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
	// the whole tree is indented one level: a disclosure drawn before the
	// sidebar's own first column would stand outside the selection pill, and
	// no row of the reference draws anything there.
	treeDiscloseColDp = treeIndentDp
	treeDiscloseDp    = 12
	// treeFieldPadDp is the air around the find field. It is the row pills'
	// own inset, so the field's edges and every pill's stand on one pair of
	// lines down the rail — which is the whole of what makes the field read
	// as part of the column rather than as a pane set into it.
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
	Idx    int    // position in the flattened row slice
	Path   string // vault-relative; the folder path or the note path
	Name   string // display name; the note title for note rows
	Detail string // the folder a found note is in, read after its name
	Depth  int    // nesting depth, 0 at the vault root
	IsDir  bool   // a folder row, carrying a disclosure toggle
	Open   bool   // folder rows only: the fold is open
}

// treeRootPath is the vault's own row in the rail's vault-relative
// addressing: the empty path, which is the path every other row's path is
// relative to and the one RootTree returns the tree to.
const treeRootPath = ""

// rootOpen answers whether the vault's row stands open.
//
// It is the one entry the fold map states the other way round, and it has
// to be: a map of path to open has one default, a folder under the vault is
// closed until something opens it, and the vault's own row is open until
// the reader closes it. A separate field would carry the same one bit while
// costing every place that toggles, remembers, reveals or walks the folds a
// second case to get right — the keys, the click, the reveal and the row
// run all address the vault by its path alone as it is.
func rootOpen(folds map[string]bool) bool { return !folds[treeRootPath] }

// treeNode is the intermediate nested shape TreeRows flattens from.
type treeNode struct {
	dirs  map[string]*treeNode
	notes []TreeRow // Path and Name set; Depth filled at flatten time
}

// TreeRows flattens the scanned index and the fold state into the tree's
// visible rows: the vault itself as the run's first row, and under it, at
// each level, the folders and the notes in one run, in the order the
// platform's file browser sorts names (theme/system/naming), a folder
// sorting under its own name and a note under its title; a closed folder
// hides its whole subtree and any path with a dot-directory segment is
// hidden outright. A folder and a note of one name leave the folder first.
//
// The vault heads the outline as the platform's file browsers head theirs,
// and it is a folder row like any other: it carries the folder symbol, it
// opens and closes, and everything the vault holds is nested one level
// under it. vault is the name it draws — the vault's own folder name — and
// its path is the empty one, which is what the rail's vault-relative
// addressing calls the vault.
//
// The run is not headed. A section parts collections the application keeps
// apart, never one collection sorted by kind (DOMAIN, Sidebar), and a
// folder — the vault's root included — is one collection.
func TreeRows(idx *Index, folds map[string]bool, vault string) []TreeRow {
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
	out := []TreeRow{{Path: treeRootPath, Name: vault, Depth: 0, IsDir: true, Open: rootOpen(folds)}}
	var walk func(n *treeNode, prefix string, depth int)
	walk = func(n *treeNode, prefix string, depth int) {
		// The folders are gathered before the notes, so the stable sort
		// leaves the folder first where a folder and a note share a name.
		entries := make([]TreeRow, 0, len(n.dirs)+len(n.notes))
		for name := range n.dirs {
			dirPath := name
			if prefix != "" {
				dirPath = prefix + "/" + name
			}
			entries = append(entries, TreeRow{Path: dirPath, Name: name, Depth: depth, IsDir: true, Open: folds[dirPath]})
		}
		for _, r := range n.notes {
			r.Depth = depth
			entries = append(entries, r)
		}
		sortByName(entries, func(r TreeRow) string { return r.Name })
		for _, r := range entries {
			out = append(out, r)
			if r.IsDir && r.Open {
				walk(n.dirs[r.Name], r.Path, depth+1)
			}
		}
	}
	if rootOpen(folds) {
		walk(root, "", 1)
	}
	for i := range out {
		out[i].Idx = i
	}
	return out
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

// sortByName sorts stably by the extracted name, in the order the platform's
// file browser sorts: case-insensitive, with a run of digits read as the
// number it spells. The sort is stable because theme/system/naming answers
// zero only for names that are equal, which is what leaves a folder before a
// note of the same name — the caller gathered the folders first.
func sortByName[T any](s []T, name func(T) string) {
	sort.SliceStable(s, func(i, j int) bool {
		return naming.Less(name(s[i]), name(s[j]))
	})
}

// treeView is the tree's own view state: the list scroll/selection state
// and per-row clickables (pointer-stable across frames).
type treeView struct {
	list *list.State
	// The pane's own marks, in the order they stand in its top strip: the
	// two that act on the vault the pane is showing. The toggle that puts
	// the pane away stands after them and is the window's, which the
	// arrangement adds. Their state is the pane's rather than the frame's
	// because the controls stand nowhere else — a control that acts on a
	// pane's collection waits for the pane, so nothing outside this view
	// draws them or reads their presses.
	rescanClick widget.Clickable
	switchClick widget.Clickable
	// rowClicks are the rows' pointer targets. They are gesture.Clicks and
	// not widget.Clickables because the rail is ONE focus target: see
	// sidebar.RowTarget.
	rowClicks []*gesture.Click

	// post is where a row's answer goes — what its keys and its clicks
	// send: the live rail's is the frame's message op, which is what a nil
	// leaves, and a test's is a recorder, so a rail laid out without a
	// window to post to can still be read back.
	post func(gtx layout.Context, msg mvu.Message)
}

// send hands one message on to whatever is collecting this frame's.
func (v *treeView) send(gtx layout.Context, msg mvu.Message) {
	if v.post != nil {
		v.post(gtx, msg)
		return
	}
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
}

// railFocus carries the focus tag of the rail's rows to the note column
// beside them. The rail's list is built when the sidebar's stream is
// subscribed, which happens after the column that reads the tag is built, so
// the tag travels through this holder rather than as a value. A nil holder,
// and one whose tag has not arrived, answer for a composition with no rail:
// there, the rail holds nothing.
type railFocus struct{ tag event.Tag }

// holds reports whether the rail's rows hold the keyboard this frame.
func (f *railFocus) holds(gtx layout.Context) bool {
	return f.focus() != nil && gtx.Focused(f.tag)
}

// focus is the tag the rail's rows take the keyboard on, nil where there is
// no rail and where its list has not been built yet.
func (f *railFocus) focus() event.Tag {
	if f == nil {
		return nil
	}
	return f.tag
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
//
// The rail publishes its rows' focus tag into rail, which is how the note
// column beside it can ask whether the keyboard is in the rail.
func treeSidebar(th rx.Observable[theme.Theme], loadModel func() Model, loadTok func() themeTokens, rail *railFocus) rx.Observable[shell.PaneColumn] {
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
	return rx.Defer(func() rx.Observable[shell.PaneColumn] {
		v := &treeView{list: list.NewState()}
		if rail != nil {
			rail.tag = v.list.Focus()
		}
		return rx.Map(field, func(fieldW layout.Widget) shell.PaneColumn {
			return v.column(loadModel, loadTok, func(gtx layout.Context) layout.Dimensions {
				focusFindField(gtx, fieldTag)
				if fieldW == nil {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
				}
				return fieldW(gtx)
			})
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

// column is what the rail hands the shell's pane: the two marks that act on
// the vault the pane is showing, the find field as the header slot, and the
// rows under it. The strip the marks stand in, the toggle last of them and
// the pane's own width are the arrangement's.
//
// The rows are the column's only stretching slot and nothing stands below
// them, so they run from under the field to the pane's own foot. The
// vault's two actions stand on the pane rather than in the toolbar: a
// control that acts on a pane's collection waits for the pane.
func (v *treeView) column(loadModel func() Model, loadTok func() themeTokens, fieldW layout.Widget) shell.PaneColumn {
	return shell.PaneColumn{
		Marks: []layout.Widget{
			func(gtx layout.Context) layout.Dimensions { return v.rescanControl(gtx, loadTok()) },
			func(gtx layout.Context) layout.Dimensions { return v.switchVaultControl(gtx, loadTok()) },
		},
		// The air around the find field is the row pills' own inset, so the
		// field's edges and every pill's stand on one pair of lines down the
		// rail — which is the whole of what makes the field read as part of
		// the column rather than as a pane set into it.
		Header: func(gtx layout.Context) layout.Dimensions {
			return complayout.Inset(treeFieldPadDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if fieldW == nil {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
				}
				return fieldW(gtx)
			})
		},
		Rows: func(gtx layout.Context) layout.Dimensions { return v.rows(gtx, loadModel(), loadTok()) },
	}
}

// rescanControl re-walks the vault and switchVaultControl leaves it for
// another. Both act on the collection the pane is showing, which is why they
// stand on the pane and travel with it: once the pane is away the window
// offers neither, and expanding it is how they are reached again.
//
// Each carries one symbol and no word, which is what every document action
// in every stored band carries — Finder's view, group, share and tag,
// Mail's compose, reply and archive, Notes' format and table — and their
// spoken titles are the only names they have.
func (v *treeView) rescanControl(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if v.rescanClick.Clicked(gtx) {
		v.send(gtx, Rescan{})
	}
	return shell.PaneMark(gtx, tok.col, &v.rescanClick, icons.Mark(icons.Refresh), "Rescan")
}

func (v *treeView) switchVaultControl(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if v.switchClick.Clicked(gtx) {
		v.send(gtx, SwitchVault{})
	}
	// Title case, which is what the platform's own controls use.
	return shell.PaneMark(gtx, tok.col, &v.switchClick, icons.Mark(icons.Folder), "Switch Vault")
}

// rows lays out the row region below the find field.
func (v *treeView) rows(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	rows := TreeRows(m.Index, m.Folds, vaultName(m))
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
				return typeset.Text(gtx, tok.shaper, "No note by that name.", tok.typ.BodyMedium,
					vgcolor.Flatten(tok.col.SecondaryLabel, chromeSurface(tok.col)), 1)
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
					v.activate(gtx, rows[sel])
				}
			}
		}
	}
	v.foldKeys(gtx, rows)
	for len(v.rowClicks) < len(rows) {
		v.rowClicks = append(v.rowClicks, &gesture.Click{})
	}
	// A tree rail is chrome, so its rows take the sidebar's own row height
	// and the sidebar's own columns: the symbol, the name after it and the
	// trailing end, each where the platform draws it.
	rowH := gtx.Dp(sidebar.RowHeight)
	// Which of the platform's two pills a filled row wears. The emphasized
	// one is the row the keyboard stands on while the rail has the keys;
	// every other filled row, and every row while the keys are elsewhere,
	// wears the grey pill with its title in the accent.
	holdsKeyboard := gtx.Focused(v.list.Focus())
	return list.LayoutSelectable(gtx, v.list, rows,
		func(gtx layout.Context, row TreeRow, selected bool) layout.Dimensions {
			click := v.rowClicks[row.Idx]
			for {
				e, ok := click.Update(gtx.Source)
				if !ok {
					break
				}
				if e.Kind != gesture.KindClick {
					continue
				}
				v.list.Select(row.Idx)
				// A click on a rail row hands the rail the keyboard, as it
				// does in patterns/sidebar and as it does on the platform:
				// the arrows then move down the rail, and the row the
				// reader landed on wears the emphasized pill.
				gtx.Execute(key.FocusCmd{Tag: v.list.Focus()})
				v.activate(gtx, row)
			}
			width := gtx.Constraints.Max.X
			size := image.Pt(width, rowH)
			gtx.Constraints = layout.Exact(size)
			active := !row.IsDir && row.Path == m.Current
			// The pill is the sidebar pattern's — its inset, its corner and
			// its two colours — so a rail in this window is a rail on this
			// platform. The accent pill goes where the platform puts it: on
			// the row the keyboard stands on, while the rail holds the
			// keyboard. The open note keeps its own pill in the grey state,
			// which is what the rail shows while the keys are in the note
			// beside it.
			filled := active || selected
			unemphasized := !(selected && holdsKeyboard)
			surface := chromeSurface(tok.col)
			if filled {
				sidebar.PaintSelection(gtx, size, tok.col, unemphasized)
				surface = sidebar.SelectionFill(tok.col, unemphasized)
			}
			v.drawRow(gtx, row, tok, size, surface, filled, unemphasized, query)
			sidebar.RowTarget(gtx, click, size, row.Name)
			return layout.Dimensions{Size: size}
		})
}

// foldKeys drains the platform's outline keys over the tree's visible rows
// and acts on the fold each one names.
//
// components/list walks the run and takes no part in the folds, so the tree
// answers these two itself, filtered on the list's own focus tag. A fold
// changes only rows BELOW the one the cursor lands on, so the row's place in
// the run is the same before and after and the selection can be set straight
// away.
//
// A flipped fold leaves a run this frame does not have, so one fold a frame
// is all these keys can honestly answer: a second would be answered against
// the run the first replaced.
func (v *treeView) foldKeys(gtx layout.Context, rows []TreeRow) {
	folded := false
	for _, name := range []key.Name{key.NameLeftArrow, key.NameRightArrow} {
		for {
			e, ok := gtx.Event(key.Filter{Focus: v.list.Focus(), Name: name})
			if !ok {
				break
			}
			ke, isKey := e.(key.Event)
			if !isKey || ke.State != key.Press || folded {
				continue
			}
			fold, folds, cursor := treeFoldKey(rows, v.list.Selected(), ke.Name)
			if folds {
				v.send(gtx, ToggleFold{Dir: fold})
				folded = true
			}
			if cursor >= 0 {
				v.list.Select(cursor)
				v.list.Reveal(cursor)
			}
		}
	}
}

// treeFoldKey answers one of the platform's outline keys over the tree's
// visible rows: the folder whose fold it flips, whether it flips one at all,
// and the row the cursor lands on, -1 when the cursor does not move.
//
// Left closes the open folder the cursor stands on and stays there. On
// anything else — a note, a collapsed folder — there is nothing under the
// cursor to close, so it walks to the parent folder's row and a second Left
// closes that; at the root it stays. Right opens a collapsed folder under the
// cursor and steps into an open one.
func treeFoldKey(rows []TreeRow, at int, name key.Name) (fold string, folds bool, cursor int) {
	if at < 0 || at >= len(rows) {
		return "", false, -1
	}
	row := rows[at]
	switch name {
	case key.NameLeftArrow:
		if row.IsDir && row.Open {
			return row.Path, true, at
		}
		parent := treeParentRow(rows, at)
		if parent < 0 {
			return "", false, -1
		}
		return "", false, parent
	case key.NameRightArrow:
		if !row.IsDir {
			return "", false, -1
		}
		if !row.Open {
			return row.Path, true, at
		}
		if at+1 < len(rows) && rows[at+1].Depth == row.Depth+1 {
			return "", false, at + 1
		}
	}
	return "", false, -1
}

// treeParentRow answers where the folder holding the given row stands in the
// run, or -1 when the row stands at the vault's root. The run is the tree
// flattened in reading order, so the parent is the nearest row above it one
// depth shallower.
func treeParentRow(rows []TreeRow, at int) int {
	for i := at - 1; i >= 0; i-- {
		if rows[i].Depth == rows[at].Depth-1 {
			return i
		}
	}
	return -1
}

// treeRowLead is how far a row's own symbol and name stand in from the
// sidebar's measured columns: one indent per depth, the same step at every
// depth, and one more for the disclosure column every row's first column is.
// A row's disclosure stands one indent back from that, at the rail's own
// first column, which is where the reference puts the first thing on a row.
func treeRowLead(depth int) float32 { return float32(depth+1) * treeIndentDp }

// drawRow paints one tree row's parts into the columns the sidebar pattern
// measures, each shifted by one indent per depth: the disclosure in the
// rail's own leading inset where the row is a folder, the symbol at
// SymbolInset and the name at TitleInset.
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
func (v *treeView) drawRow(gtx layout.Context, row TreeRow, tok themeTokens, size image.Point, surface color.NRGBA, filled, unemphasized bool, query string) {
	indent := gtx.Dp(unit.Dp(treeRowLead(row.Depth)))
	// The find mark is the platform's own, and the words it covers keep
	// their colour.
	hl := tok.col.FindHighlight
	secondaryLabel := vgcolor.Flatten(tok.col.SecondaryLabel, surface)
	if filled {
		secondaryLabel = vgcolor.Flatten(sidebar.SelectionLabel(tok.col, unemphasized), surface)
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
		fg = vgcolor.Flatten(sidebar.SelectionLabel(tok.col, unemphasized), surface)
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
	symbolFG := vgcolor.Flatten(sidebar.SymbolForeground(tok.col, filled, unemphasized), surface)
	stk := op.Offset(image.Pt(indent, 0)).Push(gtx.Ops)
	sidebar.PaintSymbol(gtx, icons.Mark(name), image.Pt(size.X-indent, size.Y), symbolFG)
	stk.Pop()

	trail := gtx.Dp(sidebar.CountInset)
	lead := indent + gtx.Dp(sidebar.TitleInset)
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
		return typeset.Text(gtx, shaper, msg, style, fg, 1)
	}
	macro := op.Record(gtx.Ops)
	dims := typeset.Text(gtx, shaper, msg, style, fg, 1)
	call := macro.Stop()
	x0, x1 := runWidth(gtx, shaper, style, msg[:start]), runWidth(gtx, shaper, style, msg[:end])
	// The mark is the role's own line box tall, centred in whatever box the
	// row gave the text — which is the row's full height, and taller. A
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
	dims := typeset.Text(m, shaper, s, style, color.NRGBA{}, 1)
	rec.Stop()
	return dims.Size.X
}

// renderTree is the static counterpart of treeSidebar used by goldens: a
// fresh rail with fresh widget.Clickable state, built from pre-resolved
// tokens and processing no events. The find field is drawn through the
// component's own static path, so the golden carries the same field the live
// rail wears.
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
) shell.PaneColumn {
	v := &treeView{list: list.NewState()}
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	fieldW := input.RenderSearch(shaper, "Search", colors, sp, rad, typo.BodyLarge, den,
		input.RenderState{Text: m.Filter, Surface: treeFieldSurface(colors), Variant: input.Chrome})
	return v.column(func() Model { return m }, func() themeTokens { return tok }, fieldW)
}

// activate performs one row's action: a folder toggles its fold, a note
// navigates. It goes out the same way the keys' answers do, so a rail laid
// out without a window to post to can be read back.
func (v *treeView) activate(gtx layout.Context, row TreeRow) {
	if row.IsDir {
		v.send(gtx, ToggleFold{Dir: row.Path})
		return
	}
	v.send(gtx, Navigate{Path: row.Path})
}
