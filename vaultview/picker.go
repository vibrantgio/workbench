// picker.go is the vault picker: an in-app folder browser composed from
// the vocabulary — a breadcrumb for the current directory, a
// components/list of child directories (dot-directories hidden), each
// row annotated when it holds a .obsidian marker or with its *.md count,
// and a filled "Open this vault" action on the current directory.
//
// Keyboard: the list holds focus, arrows move the selection, Return
// descends into the selected folder, and the action button opens.

package main

import (
	"fmt"
	"image"
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

	"github.com/vibrantgio/components/button"
	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/breadcrumb"
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

// pickerView is the picker's widget state: the list scroll/selection
// state, per-row clickables (pointer-stable across frames), and the
// one-shot initial focus. The directory trail keeps its own state, in the
// row the theme stream hands this screen every frame.
type pickerView struct {
	list      *list.State
	rowClicks []*widget.Clickable
	focused   bool
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
					return v.layout(gtx, loadModel(), loadTok(), btn, row)
				}
			})
	})
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
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return trail(gtx, trailSegments(dirPlaces(m.PickerDir), browseTo))
			}),
			layout.Rigid(complayout.VSpacer(pickerGapDp)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return v.rows(gtx, tok, m.PickerEntries)
			}),
			layout.Rigid(complayout.VSpacer(pickerGapDp)),
			layout.Rigid(btn),
		)
	})
	return layout.Dimensions{Size: size}
}

// rows lays out the folder list with keyboard traversal.
func (v *pickerView) rows(gtx layout.Context, tok themeTokens, entries []DirEntry) layout.Dimensions {
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
				if selected {
					paint.FillShape(gtx.Ops, tok.col.Ramps.Neutral.Step(300), clip.Rect{Max: size}.Op())
				}
				semantic.LabelOp(item.Name).Add(gtx.Ops)
				pointer.CursorPointer.Add(gtx.Ops)
				complayout.Inset(pickerRowInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return drawLabel(gtx, tok.shaper, item.Name, tok.typ.BodyLarge, tok.col.Text)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							a := item.annotation()
							if a == "" {
								return layout.Dimensions{}
							}
							return drawLabel(gtx, tok.shaper, a, tok.typ.BodySmall, tok.col.Ramps.Neutral.Step(700))
						}),
					)
					return layout.Dimensions{Size: gtx.Constraints.Max}
				})
				return layout.Dimensions{Size: size}
			})
		})
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
