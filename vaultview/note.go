// note.go is the vault screen: a patterns/shell ThreeColumn (nil sidebar
// and aside for now) whose main slot renders the current note — a
// breadcrumb row, a collapsible properties panel fed by the frontmatter
// split, and the parsed body as a markdown Document.

package main

import (
	"image"
	"image/color"
	"path"
	"strings"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/markdown/obsidian"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/breadcrumb"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Note-page layout constants.
const (
	noteInsetDp    = 24
	noteGapDp      = 16
	propRowGapDp   = 6
	propPadDp      = 12
	propKeyColDp   = 160
	propRadiusDp   = 8
)

// Chroma styles for the two appearance modes; built once, shared.
var (
	noteHighlightLight = highlight.New("github")
	noteHighlightDark  = highlight.New("github-dark")
)

// noteStyle derives the markdown document style for the current tokens:
// the token-themed defaults plus chroma highlighting matched to the
// appearance. No link hook yet — wikilinks render as the literal text
// they are.
func noteStyle(c tokens.ColorTokens, typ tokens.Typography) markdown.Style {
	st := markdown.FromTokens(c, typ)
	st.Mono = font.Typeface(typ.Code.Typeface)
	st.CodeSize = unit.Sp(typ.Code.Size)
	if isDarkColor(c.Background) {
		st.Highlight = noteHighlightDark
	} else {
		st.Highlight = noteHighlightLight
	}
	return st
}

// isDarkColor reports whether c reads as a dark ground (Rec. 601 luma
// below mid-grey), selecting the dark chroma style.
func isDarkColor(c color.NRGBA) bool {
	luma := 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
	return luma < 128
}

// vaultLayer composes the vault screen: the ThreeColumn shell with a nil
// sidebar and aside, and the note in the main slot. The main slot reads
// the model and token snapshots at frame time; repaints on model change
// are driven by the routed layer's re-emission.
func vaultLayer(th rx.Observable[theme.Theme], loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	// The Document is allocated once per note path and reused on every
	// frame, so scroll and richtext interaction state survive. All state
	// here is touched only on the frame goroutine.
	var (
		docPath   string
		doc       *markdown.Document
		propClick widget.Clickable
	)
	docFor := func(n *Note) *markdown.Document {
		if doc == nil || n.Path != docPath {
			doc = markdown.NewDocument(n.Blocks)
			docPath = n.Path
		}
		return doc
	}
	mainSlot := func(gtx layout.Context) layout.Dimensions {
		return layoutNotePage(gtx, loadModel(), loadTok(), &propClick, docFor)
	}
	return shell.Shell(th, shell.Props{
		Layout: shell.ThreeColumn,
		Navbar: vaultNavbar(loadTok),
		Main:   mainSlot,
	})
}

// vaultNavbar is the shell's top bar: the app name as the brand, no
// links yet.
func vaultNavbar(loadTok func() themeTokens) navbar.Props {
	return navbar.Props{
		Brand: func(gtx layout.Context) layout.Dimensions {
			tok := loadTok()
			return drawLabel(gtx, tok.shaper, "Vault View", tok.typ.TitleMedium, tok.col.Text)
		},
	}
}

// layoutNotePage lays out the main slot: breadcrumb, properties panel,
// document — or the scanning/error/empty message standing in for them.
func layoutNotePage(
	gtx layout.Context,
	m Model,
	tok themeTokens,
	propClick *widget.Clickable,
	docFor func(*Note) *markdown.Document,
) layout.Dimensions {
	inset := complayout.Inset(noteInsetDp)
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bcW := breadcrumb.Render(tok.shaper, breadcrumb.Props{Items: noteBreadcrumb(m)}, tok.col, tok.sp, tok.typ.TitleSmall)

		var body layout.FlexChild
		switch {
		case m.Scanning:
			body = messageChild(tok, "Scanning vault…")
		case m.ScanErr != "":
			body = messageChild(tok, m.ScanErr)
		case m.Note == nil:
			body = messageChild(tok, "No notes in this vault.")
		default:
			note := m.Note
			doc := docFor(note)
			style := noteStyle(tok.col, tok.typ)
			body = layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return doc.Layout(gtx, tok.shaper, style)
			})
		}

		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return bcW(gtx) }),
			layout.Rigid(complayout.VSpacer(noteGapDp)),
		}
		if m.Note != nil && m.Note.FM.Present {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutProperties(gtx, tok, m.Note.FM, m.PropsOpen, propClick)
				}),
				layout.Rigid(complayout.VSpacer(noteGapDp)),
			)
		}
		children = append(children, body)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// messageChild renders a status line in place of the document.
func messageChild(tok themeTokens, msg string) layout.FlexChild {
	return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		drawLabel(gtx, tok.shaper, msg, tok.typ.BodyLarge, tok.col.Ramps.Neutral.Step(700))
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

// noteBreadcrumb builds the trail: vault name, folder segments, note
// title. Labels only for now — navigation grows later.
func noteBreadcrumb(m Model) []breadcrumb.Item {
	items := []breadcrumb.Item{{Label: path.Base(strings.TrimRight(m.Vault, "/"))}}
	if m.Note != nil {
		dir := path.Dir(m.Note.Path)
		if dir != "." {
			for _, seg := range strings.Split(dir, "/") {
				items = append(items, breadcrumb.Item{Label: seg})
			}
		}
		items = append(items, breadcrumb.Item{Label: m.Note.Title})
	}
	return items
}

// layoutProperties renders the collapsible properties panel: a header
// row toggling the fold, then either the key/value pairs the trivial
// split could read, or the raw block in a code style.
func layoutProperties(
	gtx layout.Context,
	tok themeTokens,
	fm obsidian.FrontMatter,
	open bool,
	click *widget.Clickable,
) layout.Dimensions {
	if click.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleProperties{}}.Add(gtx.Ops)
	}
	chev := "▸"
	if open {
		chev = "▾"
	}
	header := func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Properties").Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawLabel(gtx, tok.shaper, chev, tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
				}),
				layout.Rigid(complayout.HSpacer(6)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawLabel(gtx, tok.shaper, "Properties", tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
				}),
			)
		})
	}
	if !open {
		return header(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(header),
		layout.Rigid(complayout.VSpacer(propRowGapDp)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return propertiesBody(gtx, tok, fm)
		}),
	)
}

// propertiesBody is the expanded panel: pairs when the trivial split read
// them, the raw block in code style otherwise. Both sit on a tinted fill
// so the panel reads as one surface above the prose.
func propertiesBody(gtx layout.Context, tok themeTokens, fm obsidian.FrontMatter) layout.Dimensions {
	fill := tok.col.Ramps.Neutral.Step(300)
	radius := gtx.Dp(unit.Dp(propRadiusDp))
	rec := func(gtx layout.Context) layout.Dimensions {
		return complayout.Inset(propPadDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if len(fm.Fields) == 0 {
				raw := strings.TrimRight(fm.Raw, "\n")
				if raw == "" {
					raw = "(empty)"
				}
				return drawText(gtx, tok.shaper, raw, tok.typ.Code, tok.col.Ramps.Neutral.Step(700))
			}
			keyW := gtx.Dp(unit.Dp(propKeyColDp))
			var rows []layout.FlexChild
			for i, f := range fm.Fields {
				f := f
				if i > 0 {
					rows = append(rows, layout.Rigid(complayout.VSpacer(propRowGapDp)))
				}
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							g := gtx
							if g.Constraints.Max.X > keyW {
								g.Constraints.Max.X = keyW
							}
							dims := drawLabel(g, tok.shaper, f.Key, tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
							w := dims.Size.X
							if w < keyW && keyW <= gtx.Constraints.Max.X {
								w = keyW
							}
							return layout.Dimensions{Size: image.Pt(w, dims.Size.Y), Baseline: dims.Baseline}
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return drawText(gtx, tok.shaper, fieldValue(f), tok.typ.BodyMedium, tok.col.Text)
						}),
					)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	}
	// Measure, then paint the rounded fill under the measured content.
	macro := op.Record(gtx.Ops)
	dims := rec(gtx)
	call := macro.Stop()
	rect := clip.RRect{
		Rect: image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)},
		NE:   radius, NW: radius, SE: radius, SW: radius,
	}
	paint.FillShape(gtx.Ops, fill, rect.Op(gtx.Ops))
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, dims.Size.Y)}
}

// fieldValue renders one frontmatter field's value: the scalar as
// written, or a block list joined with commas.
func fieldValue(f obsidian.Field) string {
	if len(f.Items) > 0 {
		return strings.Join(f.Items, ", ")
	}
	if f.Value == "" {
		return "—"
	}
	return f.Value
}
