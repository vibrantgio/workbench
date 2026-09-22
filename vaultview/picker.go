// picker.go is the vault picker: an in-app folder browser composed from
// the vocabulary — a breadcrumb for the current directory, a
// components/list of child directories (dot-directories hidden), each row
// carrying the folder mark at the sidebar row's measured column and
// annotated when it holds a .obsidian marker or with its *.md count,
// and a filled "Open this vault" action on the current directory.
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
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

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
	"github.com/vibrantgio/theme/theme"
)

// DirEntry is one row of the folder browser.
type DirEntry struct {
	Idx     int    // position in the row slice
	Name    string // display name; ".." for the parent row
	Path    string // absolute path the row navigates to
	IsVault bool   // the directory holds a .obsidian marker
	MDCount int    // direct *.md children
	Up      bool   // the parent row
}

// ListDir returns the folder browser's rows for a directory: a parent
// row when one exists, then the child directories in name order with
// dot-directories hidden.
func ListDir(dir string) []DirEntry {
	var out []DirEntry
	if parent := filepath.Dir(dir); parent != dir {
		out = append(out, DirEntry{Name: "..", Path: parent, Up: true})
	}
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

// vaultMarks probes a directory for the row annotations: whether it
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

// annotation is the row's trailing text: the vault marker, or the note
// count, or nothing.
func (d DirEntry) annotation() string {
	switch {
	case d.Up:
		return ""
	case d.IsVault:
		return ".obsidian vault"
	case d.MDCount == 1:
		return "1 note"
	case d.MDCount > 1:
		return fmt.Sprintf("%d notes", d.MDCount)
	}
	return ""
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
	list      *list.State
	rowClicks []*widget.Clickable
	focused   bool
	// dir is the directory the list was last laid out for, so a move to
	// another one puts the list back at its first row: a listing the reader
	// has never seen must not open part way down because the listing before
	// it was scrolled.
	dir string
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
// child directories — with the list scrolling inside the room below the
// trail. It is the whole of what the vault-switch dialog shows, and the
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
		v.list.Select(-1)
		v.list.Reveal(0)
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
			return trail(gtx, trailSegments(dirPlaces(m.PickerDir), browseTo))
		}),
		layout.Rigid(complayout.VSpacer(pickerGapDp)),
		listChild,
	)
}

// rows lays out the folder list with keyboard traversal, on the surface
// the browser stands on.
func (v *pickerView) rows(gtx layout.Context, tok themeTokens, entries []DirEntry, standsOn color.NRGBA, rowInset float32) layout.Dimensions {
	for len(v.rowClicks) < len(entries) {
		v.rowClicks = append(v.rowClicks, &widget.Clickable{})
	}
	rowH := gtx.Dp(list.RowHeight(tok.den))
	return list.LayoutSelectable(gtx, v.list, entries,
		func(gtx layout.Context, item DirEntry, selected bool) layout.Dimensions {
			click := v.rowClicks[item.Idx]
			if click.Clicked(gtx) {
				v.list.Select(item.Idx)
				gtx.Execute(key.FocusCmd{Tag: v.list.Focus()})
				mvu.MessageOp{Message: BrowseTo{Dir: item.Path}}.Add(gtx.Ops)
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				surface := standsOn
				if selected {
					surface = tok.col.SelectedContentBackground
					paint.FillShape(gtx.Ops, surface, clip.Rect{Max: size}.Op())
				}
				semantic.LabelOp(item.Name).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				// The inset is the row's leading and trailing air ALONE. A
				// row is the list's own height — one control height — and a
				// BodyLarge line box very nearly fills it, so vertical air
				// here would push the label out of the row and the list's
				// clip would cut it in half. What centres the label is the
				// flex's Middle alignment over the row's full height.
				complayout.InsetXY(rowInset, 0).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return folderSymbol(gtx, tok, selected, surface)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							name := tok.col.Label
							if selected {
								name = tok.col.AlternateSelectedControlText
							}
							return drawLabel(gtx, tok.shaper, item.Name, tok.typ.BodyLarge,
								vgcolor.Flatten(name, surface))
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							a := item.annotation()
							if a == "" {
								return layout.Dimensions{}
							}
							ann := tok.col.SecondaryLabel
							if selected {
								ann = tok.col.AlternateSelectedControlText
							}
							return drawLabel(gtx, tok.shaper, a, tok.typ.BodySmall,
								vgcolor.Flatten(ann, surface))
						}),
					)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				})
				return layout.Dimensions{Size: size}
			})
		})
}

// folderSymbol paints the folder mark at the leading end of a browser row
// and reports the column it stands in, so the name beside it begins where a
// sidebar row's name begins.
//
// Every row of this browser is a directory — the parent row included — so
// every row carries the one mark: the symbol names the KIND of entry, and
// rows of one kind share it.
//
// The column is the sidebar row's measured one, which is the only reading
// the reference holds for a symbol beside a name: a [sidebar.SymbolBox]
// square set [sidebar.SymbolInset] in, with the name beginning at
// [sidebar.LabelInset]. The colour is NOT the sidebar's measured symbol
// value: that value was read off a chrome rail, where the platform draws the
// symbol 38 of 255 stronger than the name beside it, and this browser is
// content standing on a content surface. Nothing measures a content row's
// symbol apart from its name, so it takes the name's own foreground.
func folderSymbol(gtx layout.Context, tok themeTokens, selected bool, standsOn color.NRGBA) layout.Dimensions {
	fg := tok.col.Label
	if selected {
		fg = tok.col.AlternateSelectedControlText
	}
	box := gtx.Dp(sidebar.SymbolBox)
	stk := op.Offset(image.Pt(gtx.Dp(sidebar.SymbolInset), (gtx.Constraints.Max.Y-box)/2)).Push(gtx.Ops)
	drawMark(gtx, icons.Folder, sidebar.SymbolBox, vgcolor.Flatten(fg, standsOn))
	stk.Pop()
	return layout.Dimensions{Size: image.Pt(gtx.Dp(sidebar.LabelInset), gtx.Constraints.Max.Y)}
}

// browseTo is the click an ancestor in the picker's trail carries: the
// directory it names becomes the one on show.
func browseTo(dir string) func(gtx layout.Context) {
	return func(gtx layout.Context) {
		mvu.MessageOp{Message: BrowseTo{Dir: dir}}.Add(gtx.Ops)
	}
}

// dirPlaces splits an absolute directory into the trail's places, the
// filesystem root first.
func dirPlaces(dir string) []place {
	dir = filepath.Clean(dir)
	sep := string(filepath.Separator)
	segs := []place{{label: sep, path: sep}}
	if dir == sep || dir == "." {
		return segs
	}
	cum := ""
	for _, part := range strings.Split(strings.TrimPrefix(dir, sep), sep) {
		if part == "" {
			continue
		}
		cum = cum + sep + part
		segs = append(segs, place{label: part, path: cum})
	}
	return segs
}
