// picker.go is the vault picker: an in-app folder browser composed from
// the vocabulary — a breadcrumb for the current directory, a
// components/list of child directories (dot-directories hidden), each row
// carrying the folder mark, the directory's name, what kind of directory it
// is and how many notes it holds, and a filled "Open this vault" action on
// the current directory.
//
// It browses the way the platform's own open panel does. A directory is
// named and never spelled: the trail carries the place names from the home
// directory or the startup volume down to the one on show, and going up is
// done by clicking an ancestor in it rather than by a row standing for the
// parent. So there is no ".." row and no literal separator anywhere on
// screen.
//
// This is the FULL-SCREEN picker, which the first launch with no vault
// opens: there is no window for a dialog to stand over then. Switching the
// vault from an open one raises the dialog in vaultpicker.go instead, which
// reuses the browser below as its body.
//
// Keyboard: the list holds focus, arrows move the selection, Return
// descends into the selected folder, and the action button opens.

package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/breadcrumb"
	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/icons"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/system/naming"
	"github.com/vibrantgio/theme/theme"
)

// DirEntry is one row of the folder browser.
type DirEntry struct {
	Idx     int    // position in the row slice
	Name    string // the directory's own name
	Path    string // absolute path the row navigates to
	IsVault bool   // the directory holds a .obsidian marker
	MDCount int    // direct *.md children
}

// ListDir returns the folder browser's rows for a directory: its child
// directories in the order the platform's file browser sorts names
// (theme/system/naming), with dot-directories hidden. The order is the
// browser's own and not the filesystem's: os.ReadDir answers in byte order,
// which puts "Vault 10" before "Vault 2" and every capital before every
// lower case.
//
// The parent is not among them. The trail above the rows is how one goes
// up — that is what the platform's open panel does, and a row standing for
// the parent would be a second way of saying it that carries no name.
func ListDir(dir string) []DirEntry {
	var out []DirEntry
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if !e.IsDir() && !symlinkToDir(e, p) {
				continue
			}
			d := DirEntry{Name: e.Name(), Path: p}
			d.IsVault, d.MDCount = vaultMarks(p)
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return naming.Less(out[i].Name, out[j].Name) })
	for i := range out {
		out[i].Idx = i
	}
	return out
}

// symlinkToDir reports whether a listing entry is a symlink whose
// target is a directory — a linked vault browses like the real one.
func symlinkToDir(e os.DirEntry, path string) bool {
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// vaultMarks probes a directory for what its row says about it: whether it
// holds a .obsidian marker directory, and how many direct *.md children
// it has.
func vaultMarks(dir string) (isVault bool, mdCount int) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false, 0
	}
	for _, e := range ents {
		if e.IsDir() {
			if e.Name() == ".obsidian" {
				isVault = true
			}
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			mdCount++
		}
	}
	return isVault, mdCount
}

// kind is what the row's directory IS, drawn after its name: a directory
// holding a .obsidian marker is a vault and every other one is a folder.
// It is the open panel's Kind column, which names the thing rather than
// counting anything in it.
func (d DirEntry) kind() string {
	if d.IsVault {
		return "Vault"
	}
	return "Folder"
}

// count is how many notes the row's directory holds directly, drawn in the
// row's trailing column. A directory holding none draws no count, the way a
// chrome rail's row without one does: the column does not move for it.
func (d DirEntry) count() string {
	if d.MDCount == 0 {
		return ""
	}
	return strconv.Itoa(d.MDCount)
}

// Picker layout constants.
const (
	pickerInsetDp    = 24
	pickerGapDp      = 16
	pickerRowInsetDp = 8
	pickerMaxWDp     = 720
)

// pickerView is the picker's own view state: the list scroll/selection
// state, per-row clickables (pointer-stable across frames), and the
// one-shot initial focus. The directory trail keeps its own state, in the
// row the theme stream hands this screen every frame.
type pickerView struct {
	list *list.State
	// rowClicks are the rows' pointer targets. A row is a gesture.Click and
	// not a widget.Clickable because a Clickable registers a focus filter of
	// its own, and a list is ONE focus target: with a filter per row Tab
	// walks the rows one by one and never reaches what stands after the
	// list, where the platform hands Tab straight on to the next control.
	// The keys reach a row through the list's own tag, which a click hands
	// them to.
	rowClicks []*gesture.Click
	focused   bool
	// dir is the directory the list was last laid out for, so a move to
	// another one puts the list back at its first row: a listing the reader
	// has never seen must not open part way down because the listing before
	// it was scrolled.
	dir string
	// arrived records that the listing for dir has been laid out at least
	// once with rows in it. The rows arrive a frame or more after the
	// directory does — the listing is read off the filesystem by a command —
	// and a selection set against an empty slice is dropped by the list, so
	// onArrival is applied on the first frame that actually has rows.
	arrived bool
	// onArrival answers the row a listing opens selected on, or -1 for none.
	// A list at the front of a dialog is a focusable and shows a selected
	// row the moment it takes the keyboard; the full-screen picker's list
	// opens with nothing picked, so it leaves this nil.
	onArrival func(m Model) int
}

// pickerLayer builds the picker screen. The frame closure reads the
// model and token snapshots at frame time; repaints on model change are
// driven by the routed layer's re-emission.
func pickerLayer(th rx.Observable[theme.Theme], loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	openBtn := button.Button(th, button.Props{
		Label: "Open this vault",
		OnClick: func(gtx layout.Context) {
			mvu.MessageOp{Message: OpenVault{Path: loadModel().PickerDir}}.Add(gtx.Ops)
		},
	})
	// The trail comes off the theme stream so a palette change redraws it,
	// and its interaction state lives in the stream rather than in this
	// screen: the row it hands out on every emission is the same row, so a
	// click survives the theme changing under the pointer.
	trail := breadcrumb.Trail(th, breadcrumb.TrailProps{Chevron: trailChevronDp})
	return rx.Defer(func() rx.Observable[layout.Widget] {
		v := &pickerView{list: list.NewState()}
		return rx.Map(rx.CombineLatest2(openBtn, trail),
			func(next rx.Tuple2[layout.Widget, breadcrumb.TrailLayout]) layout.Widget {
				btn, row := next.First, next.Second
				return func(gtx layout.Context) layout.Dimensions {
					return v.screen(gtx, loadModel(), loadTok(), btn, row)
				}
			})
	})
}

// screen draws the picker over the whole window: its own surface first,
// then the rows under the native title-bar strip.
//
// The picker is one chrome region taking the window entire, so it paints the
// platform's chrome material rather than letting the backdrop beneath it
// show: nothing stands inset on this screen, so nothing of the window's
// plane is meant to be seen. The strip is left to the platform — this screen claims none of
// it, while the vault screen's chrome row lays itself out inside it — so the
// rows are inset past it and the region's own fill runs behind it.
func (v *pickerView) screen(
	gtx layout.Context,
	m Model,
	tok themeTokens,
	btn layout.Widget,
	trail breadcrumb.TrailLayout,
) layout.Dimensions {
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, chromeSurface(tok.col), clip.Rect{Max: size}.Op())
	desktop.InsetTop(desktop.TopInset, func(gtx layout.Context) layout.Dimensions {
		return v.layout(gtx, m, tok, btn, trail)
	})(gtx)
	return layout.Dimensions{Size: size}
}

func (v *pickerView) layout(
	gtx layout.Context,
	m Model,
	tok themeTokens,
	btn layout.Widget,
	trail breadcrumb.TrailLayout,
) layout.Dimensions {
	if !v.focused {
		gtx.Execute(key.FocusCmd{Tag: v.list.Focus()})
		v.focused = true
	}
	// Return descends into the selected folder. The list consumes the
	// arrows; activation is the caller's semantics, filtered on the
	// list's focus tag.
	for _, name := range []key.Name{key.NameReturn, key.NameEnter} {
		for {
			e, ok := gtx.Event(key.Filter{Focus: v.list.Focus(), Name: name})
			if !ok {
				break
			}
			if ke, ok := e.(key.Event); ok && ke.State == key.Press {
				if sel := v.list.Selected(); sel >= 0 && sel < len(m.PickerEntries) {
					mvu.MessageOp{Message: BrowseTo{Dir: m.PickerEntries[sel].Path}}.Add(gtx.Ops)
				}
			}
		}
	}

	size := gtx.Constraints.Max
	// Centre a column of at most pickerMaxWDp.
	maxW := gtx.Dp(unit.Dp(pickerMaxWDp))
	colW := size.X
	if colW > maxW {
		colW = maxW
	}
	offX := (size.X - colW) / 2
	cgtx := gtx
	cgtx.Constraints = layout.Exact(image.Pt(colW, size.Y))
	defer op.Offset(image.Pt(offX, 0)).Push(gtx.Ops).Pop()

	inset := complayout.Inset(pickerInsetDp)
	inset.Layout(cgtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawLabel(gtx, tok.shaper, "Choose a vault", tok.typ.HeadlineSmall, tok.col.Text)
			}),
			layout.Rigid(complayout.VSpacer(pickerGapDp)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return v.browser(gtx, m, tok, trail, chromeSurface(tok.col), pickerRowInsetDp, 0)
			}),
			layout.Rigid(complayout.VSpacer(pickerGapDp)),
			layout.Rigid(btn),
		)
	})
	return layout.Dimensions{Size: size}
}

// browser lays out the folder browser itself — the trail over the list of
// child directories — with the list scrolling inside the available room
// below the trail. It is the whole of what the vault-switch dialog shows, and the
// middle of what the full-screen picker shows: one composition, so the two
// browse identically.
//
// surface is the opaque fill the browser stands on — the picker screen's
// chrome material, the dialog's window background — which is what an
// unselected row's foreground flattens against and what it repaints with.
//
// rowInset is the leading and trailing air a row spends, ahead of the
// symbol and label columns inside it. A screen that lays the browser out
// itself gives its rows their own, and a dialog gives none: the surface has
// already inset the body, so a row that inset itself again would stand its
// symbols in from the header and the footer beside it.
//
// rowsPx is how tall the list stands: zero gives it every pixel left under
// the trail, which is what a screen taking the window entire wants, and a
// positive value pins it so the browser hugs a stated number of rows, which
// is what a dialog sized to show them wants.
func (v *pickerView) browser(
	gtx layout.Context,
	m Model,
	tok themeTokens,
	trail breadcrumb.TrailLayout,
	surface color.NRGBA,
	rowInset float32,
	rowsPx int,
) layout.Dimensions {
	if v.dir != m.PickerDir {
		v.dir = m.PickerDir
		v.arrived = false
		v.list.Select(-1)
		v.list.Reveal(0)
	}
	if !v.arrived && len(m.PickerEntries) > 0 {
		v.arrived = true
		if v.onArrival != nil {
			if sel := v.onArrival(m); sel >= 0 {
				v.list.Select(sel)
				v.list.Reveal(sel)
			}
		}
	}
	rows := func(gtx layout.Context) layout.Dimensions {
		return v.rows(gtx, tok, m.PickerEntries, surface, rowInset)
	}
	listChild := layout.Flexed(1, rows)
	if rowsPx > 0 {
		listChild = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowsPx))
			return rows(gtx)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return trail(gtx, trailSegments(dirPlaces(m.PickerDir, m.PickerRoot), browseTo))
		}),
		layout.Rigid(complayout.VSpacer(pickerGapDp)),
		listChild,
	)
}

// rows lays out the folder list with keyboard traversal, on the surface
// the browser stands on.
func (v *pickerView) rows(gtx layout.Context, tok themeTokens, entries []DirEntry, standsOn color.NRGBA, rowInset float32) layout.Dimensions {
	for len(v.rowClicks) < len(entries) {
		v.rowClicks = append(v.rowClicks, &gesture.Click{})
	}
	rowH := gtx.Dp(list.RowHeight(tok.den))
	// The browser fills the selected row and nothing else — its rows take
	// no fill under the pointer — so that is the one fill the band's over
	// half lands on besides the surface the list stands on.
	rowFill := func(i int) color.NRGBA {
		if i == v.list.Selected() {
			return tok.col.SelectedContentBackground
		}
		return color.NRGBA{}
	}
	return list.Halo(gtx, v.list, tok.col, standsOn, rowFill, func(gtx layout.Context) layout.Dimensions {
		return v.selectableRows(gtx, tok, entries, standsOn, rowInset, rowH)
	})
}

// selectableRows is the list itself, inside the band [list.Halo] draws on its
// box.
func (v *pickerView) selectableRows(gtx layout.Context, tok themeTokens, entries []DirEntry, standsOn color.NRGBA, rowInset float32, rowH int) layout.Dimensions {
	return list.LayoutSelectable(gtx, v.list, entries,
		func(gtx layout.Context, item DirEntry, selected bool) layout.Dimensions {
			click := v.rowClicks[item.Idx]
			for {
				e, ok := click.Update(gtx.Source)
				if !ok {
					break
				}
				if e.Kind != gesture.KindClick {
					continue
				}
				v.list.Select(item.Idx)
				gtx.Execute(key.FocusCmd{Tag: v.list.Focus()})
				mvu.MessageOp{Message: BrowseTo{Dir: item.Path}}.Add(gtx.Ops)
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			size := gtx.Constraints.Max
			surface := standsOn
			if selected {
				surface = tok.col.SelectedContentBackground
				paint.FillShape(gtx.Ops, surface, clip.Rect{Max: size}.Op())
			}
			// The inset is the row's leading and trailing air ALONE. A row is
			// the list's own height — one control height — and a BodyLarge
			// line box very nearly fills it, so vertical air here would push
			// the label out of the row and the list's clip would cut it in
			// half. What centres each part is the row's own height, which
			// every painter below is handed.
			complayout.InsetXY(rowInset, 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				drawBrowserRow(gtx, tok, item, selected, surface)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
			sidebar.RowTarget(gtx, click, size, item.Name)
			return layout.Dimensions{Size: size}
		})
}

// drawBrowserRow paints one browser row's four parts into the block it is
// handed: the folder symbol, the directory's name, what kind of directory it
// is after the name, and how many notes it holds at the trailing end.
//
// The columns are the sidebar row's measured ones — [sidebar.PaintSymbol]
// puts the mark 17 in, the name begins at [sidebar.LabelInset] 48, and
// [sidebar.PaintCount] lands the count's last covered column
// [sidebar.CountInset] 17 in from the trailing edge. They stand in because no
// stored capture holds the platform's own open panel; the panel's own columns
// replace them when one does (on the capture list).
//
// Every row of this browser is a directory, so every row carries the one
// mark: the symbol names the KIND of entry, and rows of one kind share it.
// The kind is spelled out beside the name because the mark cannot tell a
// vault from a plain folder, and it is set in the secondary label to keep the
// name itself the more pronounced half — where the annotation stood before
// it. The symbol's colour is NOT the sidebar's measured symbol value: that
// value was read off a chrome rail, where the platform draws the symbol 38 of
// 255 stronger than the name beside it, and this browser is content standing
// on a content surface. Nothing measures a content row's symbol apart from
// its name, so it takes the name's own foreground.
func drawBrowserRow(gtx layout.Context, tok themeTokens, item DirEntry, selected bool, standsOn color.NRGBA) {
	size := gtx.Constraints.Max
	nameFG := vgcolor.Flatten(tok.col.Label, standsOn)
	secondaryLabel := vgcolor.Flatten(tok.col.SecondaryLabel, standsOn)
	if selected {
		sel := vgcolor.Flatten(tok.col.AlternateSelectedControlText, standsOn)
		nameFG, secondaryLabel = sel, sel
	}

	sidebar.PaintSymbol(gtx, icons.Mark(icons.Folder), size, nameFG)

	trail := gtx.Dp(sidebar.CountInset)
	if c := item.count(); c != "" {
		trail += sidebar.PaintCount(gtx, tok.shaper, c, tok.typ.BodySmall, size, secondaryLabel)
	}

	lead := gtx.Dp(sidebar.LabelInset)
	room := size.X - lead - trail
	if room <= 0 {
		return
	}
	lGtx := gtx
	lGtx.Constraints = layout.Constraints{Max: image.Pt(room, size.Y)}
	rec := op.Record(gtx.Ops)
	dims := drawLabel(lGtx, tok.shaper, item.Name, tok.typ.BodyLarge, nameFG)
	call := rec.Stop()
	stk := op.Offset(image.Pt(lead, (size.Y-dims.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stk.Pop()

	// The kind follows the name one spacing step along and takes whatever
	// the name left: a row whose name fills the browser says what it is by
	// its own name already. No capture holds the platform's open panel, so
	// the gap is the system's own step rather than a reading.
	gap := gtx.Dp(unit.Dp(tok.sp.S2))
	x := lead + dims.Size.X + gap
	left := size.X - trail - x
	if left <= 0 {
		return
	}
	kGtx := gtx
	kGtx.Constraints = layout.Constraints{Max: image.Pt(left, size.Y)}
	rec = op.Record(gtx.Ops)
	kDims := drawLabel(kGtx, tok.shaper, item.kind(), tok.typ.BodySmall, secondaryLabel)
	call = rec.Stop()
	stk = op.Offset(image.Pt(x, (size.Y-kDims.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stk.Pop()
}

// browseTo is the click an ancestor in the picker's trail carries: the
// directory it names becomes the one on show.
func browseTo(dir string) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		mvu.MessageOp{Message: BrowseTo{Dir: dir}}.Add(gtx.Ops)
	}
}

// dirPlaces splits an absolute directory into the trail's places, each named
// the way the platform names a place: by its own name, never by its path.
// [trailRoot] gives the root the split starts from.
//
// The Finder sidebar is where that reading comes from
// (finder-window-untinted-dark.png): its Locations run carries the home
// directory under the account's own name beside a house — "rene", not
// "/Users/rene" — and the startup volume under the name the filesystem knows
// it by. So the trail's root is a name too, and a literal separator stands
// nowhere on screen.
//
// A root with no label is one nothing on the filesystem names: the trail then
// starts at the first named segment below it, which is what a path with
// nothing to name its root is.
func dirPlaces(dir string, root place) []place {
	dir = filepath.Clean(dir)
	sep := string(filepath.Separator)
	cum := filepath.Clean(root.path)
	if root.path == "" {
		cum = sep
	}
	var segs []place
	if root.label != "" {
		segs = append(segs, place{label: root.label, path: cum})
	}
	rest := strings.TrimPrefix(dir, cum)
	for _, part := range strings.Split(rest, sep) {
		if part == "" {
			continue
		}
		cum = strings.TrimSuffix(cum, sep) + sep + part
		segs = append(segs, place{label: part, path: cum})
	}
	return segs
}

// trailRoot reports the place a browser trail starts at for dir: the home
// directory when dir is inside it, and the startup volume otherwise.
//
// It reads the filesystem, so it is called where the model is reduced and
// never where it is drawn: the trail a stored image carries is the one its
// model states, on whatever machine the image is taken.
func trailRoot(dir string) place {
	sep := string(filepath.Separator)
	dir = filepath.Clean(dir)
	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(home)
		if home != sep && (dir == home || strings.HasPrefix(dir, home+sep)) {
			return place{label: filepath.Base(home), path: home}
		}
	}
	return place{label: volumeName(), path: sep}
}

// volumeName reports what the startup volume is called, or "" when nothing
// on the filesystem says. macOS keeps a symlink to the startup volume in
// /Volumes under the name the platform shows for it, which is the one place
// the name can be read from without asking the window system.
func volumeName() string {
	ents, err := os.ReadDir("/Volumes")
	if err != nil {
		return ""
	}
	for _, e := range ents {
		p := filepath.Join("/Volumes", e.Name())
		if target, err := filepath.EvalSymlinks(p); err == nil && target == string(filepath.Separator) {
			return e.Name()
		}
	}
	return ""
}
