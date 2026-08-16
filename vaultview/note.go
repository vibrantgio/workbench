// note.go is the vault screen: a patterns/shell ThreeColumn — the folder
// tree in the sidebar slot, the backlinks panel in the aside — whose main
// slot renders the current note: a header row with back/forward and the
// breadcrumb, a collapsible properties panel fed by the frontmatter split,
// and the parsed body as a markdown Document. Wikilink clicks resolve
// against the index and navigate; web links open the system browser; a
// link matching several notes raises the chooser, and every other refusal
// surfaces as a toast.

package main

import (
	"image"
	"image/color"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"gioui.org/font"
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

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/markdown/obsidian"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/navbar"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/patterns/toast"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Note-page layout constants.
const (
	noteInsetDp  = 24
	noteGapDp    = 16
	propRowGapDp = 6
	propPadDp    = 12
	propKeyGapDp = 16
	propRadiusDp = 12
)

// Chroma styles for the two appearance modes; built once, shared.
var (
	noteHighlightLight = highlight.New("github")
	noteHighlightDark  = highlight.New("github-dark")
)

// noteStyle derives the markdown document style for the current tokens:
// the token-themed defaults plus chroma highlighting matched to the
// appearance. The link hook is attached per frame in layoutNotePage,
// where the model is at hand.
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

// docEntry is one cached document and the Note value it was built from.
type docEntry struct {
	note *Note
	doc  *markdown.Document
}

// vaultLayer composes the vault screen: the ThreeColumn shell with the
// folder tree in the sidebar slot, the backlinks panel in the aside, and
// the note in the main slot. The main slot reads the model and token
// snapshots at frame time; repaints on model change are driven by the
// routed layer's re-emission.
func vaultLayer(th rx.Observable[theme.Theme], loadModel func() Model, loadTok func() themeTokens) rx.Observable[layout.Widget] {
	// Documents are cached per note path and reused on every frame, so
	// each note's scroll position and richtext interaction state survive
	// revisiting. A landing that carries an anchor (NavSeq moved and
	// CurAnchor is set) re-creates the target's document seated at the
	// anchor's block index; every other visit keeps the cached viewport.
	// The cached document also remembers which Note value it was built
	// from: notes are never mutated, so a different pointer at the same
	// path means the file was read again and the document must be rebuilt
	// — without that, a note edited outside the app would reload into a
	// viewport still showing the old blocks. All state here is touched
	// only on the frame goroutine.
	var (
		docs      = map[string]docEntry{}
		docsVault string
		seatedSeq int
		propClick widget.Clickable
		backClick widget.Clickable
		fwdClick  widget.Clickable
		trail     crumbRow
	)
	docFor := func(m Model, n *Note) *markdown.Document {
		if m.Vault != docsVault {
			docs = map[string]docEntry{}
			docsVault = m.Vault
		}
		e, cached := docs[n.Path]
		switch {
		case m.CurAnchor >= 0 && m.NavSeq != seatedSeq:
			seatedSeq = m.NavSeq
			e = docEntry{note: n, doc: markdown.NewDocumentAt(n.Blocks, m.CurAnchor)}
		case !cached || e.note != n:
			e = docEntry{note: n, doc: markdown.NewDocument(n.Blocks)}
		default:
			return e.doc
		}
		docs[n.Path] = e
		return e.doc
	}
	mainSlot := func(gtx layout.Context) layout.Dimensions {
		return layoutNotePage(gtx, loadModel(), loadTok(), &propClick, &backClick, &fwdClick, &trail, docFor)
	}
	return shell.Shell(th, shell.Props{
		Layout:  shell.ThreeColumn,
		Navbar:  vaultNavbar(loadTok),
		Sidebar: treeSidebar(th, loadModel, loadTok),
		Aside:   backlinksAside(loadModel, loadTok),
		Main:    mainSlot,
	})
}

// vaultNavbar is the shell's top bar, laid out as one row on one
// baseline: the app name leads, and the two affordances — re-walk the
// vault, return to the folder browser — trail right-aligned as a group.
// The whole row rides in the navbar's Brand slot: the pattern's Links
// slot centres its content in the bar, and these actions belong at a
// deliberate edge, not afloat near the middle.
func vaultNavbar(loadTok func() themeTokens) navbar.Props {
	var rescanClick, switchClick widget.Clickable
	return navbar.Props{
		Brand: func(gtx layout.Context) layout.Dimensions {
			tok := loadTok()
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawLabel(gtx, tok.shaper, "Vault View", tok.typ.TitleMedium, tok.col.Text)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
				}),
				layout.Rigid(headerAction(&rescanClick, "Rescan", tok, Rescan{})),
				layout.Rigid(complayout.HSpacer(tok.sp.S4)),
				layout.Rigid(headerAction(&switchClick, "Switch vault", tok, SwitchVault{})),
			)
		},
	}
}

// headerAction renders one top-bar affordance: a clickable label that
// emits its message on the frame the click lands. It reports the label's
// own baseline, so a Baseline-aligned row seats it on the same line as
// the brand.
func headerAction(click *widget.Clickable, label string, tok themeTokens, msg mvu.Message) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if click.Clicked(gtx) {
			mvu.MessageOp{Message: msg}.Add(gtx.Ops)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			return drawLabel(gtx, tok.shaper, label, tok.typ.LabelLarge, tok.col.Text)
		})
	}
}

// layoutNotePage lays out the main slot: the header row (back/forward
// and the breadcrumb), properties panel, document — or the
// scanning/error/empty message standing in for them.
func layoutNotePage(
	gtx layout.Context,
	m Model,
	tok themeTokens,
	propClick, backClick, fwdClick *widget.Clickable,
	trail *crumbRow,
	docFor func(Model, *Note) *markdown.Document,
) layout.Dimensions {
	note := m.CurrentNote()
	// The reading column lies on its own paper: the pinned app background,
	// one neutral step lighter than the Surface the window chrome — header
	// band, tree rail, aside — sits on. The panel and code fills below tint
	// down the neutral ramp from this paper, so the note reads as a
	// document resting on darker furniture rather than more chrome.
	paint.FillShape(gtx.Ops, tok.col.Background, clip.Rect{Max: gtx.Constraints.Max}.Op())
	inset := complayout.Inset(noteInsetDp)
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		crumbs := noteCrumbs(m)

		var body layout.FlexChild
		switch {
		case m.Scanning && note == nil:
			body = messageChild(tok, "Scanning vault…")
		case m.ScanErr != "":
			body = messageChild(tok, m.ScanErr)
		case note == nil:
			body = messageChild(tok, "No notes in this vault.")
		default:
			doc := docFor(m, note)
			style := noteStyle(tok.col, tok.typ)
			style.Text.OnLinkClick = func(gtx layout.Context, url string) {
				linkClicked(gtx, m, url)
			}
			body = layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return doc.Layout(gtx, tok.shaper, style)
			})
		}

		header := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return navButton(gtx, tok, backClick, "‹", "Back", m.Cursor > 0, GoBack{})
				}),
				layout.Rigid(complayout.HSpacer(8)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return navButton(gtx, tok, fwdClick, "›", "Forward", m.Cursor+1 < len(m.History), GoForward{})
				}),
				layout.Rigid(complayout.HSpacer(noteGapDp)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return trail.layout(gtx, tok, crumbs)
				}),
			)
		}

		children := []layout.FlexChild{
			layout.Rigid(header),
			layout.Rigid(complayout.VSpacer(noteGapDp)),
		}
		if note != nil && note.FM.Present {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutProperties(gtx, tok, note.FM, m.PropsOpen, propClick)
				}),
				layout.Rigid(complayout.VSpacer(noteGapDp)),
			)
		}
		children = append(children, body)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// renderNotePage is the static counterpart of the vault screen's main
// slot, used by goldens: fresh widget state, a fresh top-scrolled
// document per note, laid out once from pre-resolved tokens and
// processing no events.
func renderNotePage(
	shaper *text.Shaper,
	m Model,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	typo tokens.Typography,
	den tokens.Density,
) layout.Widget {
	tok := themeTokens{col: colors, typ: typo, sp: sp, den: den, shaper: shaper}
	var (
		propClick widget.Clickable
		backClick widget.Clickable
		fwdClick  widget.Clickable
		trail     crumbRow
	)
	docs := map[string]*markdown.Document{}
	docFor := func(_ Model, n *Note) *markdown.Document {
		d := docs[n.Path]
		if d == nil {
			d = markdown.NewDocument(n.Blocks)
			docs[n.Path] = d
		}
		return d
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layoutNotePage(gtx, m, tok, &propClick, &backClick, &fwdClick, &trail, docFor)
	}
}

// navButton renders one history affordance: an arrow glyph that emits its
// message on click while enabled, and renders dimmed and inert at the
// stack's end.
func navButton(
	gtx layout.Context,
	tok themeTokens,
	click *widget.Clickable,
	glyph, label string,
	enabled bool,
	msg mvu.Message,
) layout.Dimensions {
	if click.Clicked(gtx) && enabled {
		mvu.MessageOp{Message: msg}.Add(gtx.Ops)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		c := tok.col.Ramps.Neutral.Step(500)
		if enabled {
			c = tok.col.Text
			pointer.CursorPointer.Add(gtx.Ops)
		}
		return drawLabel(gtx, tok.shaper, glyph, tok.typ.TitleMedium, c)
	})
}

// linkClicked dispatches one link activation: wikilinks (embeds included
// — an embed navigates like an ordinary link) resolve against the index
// and navigate on the same frame; web links open the system browser; a
// link whose file part matches several notes raises the chooser with the
// candidates the resolver refused to pick between, and every other refusal
// surfaces its reason as a toast; anything else is ignored.
func linkClicked(gtx layout.Context, m Model, url string) {
	switch {
	case strings.HasPrefix(url, "wiki:"), strings.HasPrefix(url, "wikiembed:"):
		if m.Index == nil {
			return
		}
		body := strings.TrimPrefix(strings.TrimPrefix(url, "wikiembed:"), "wiki:")
		res, rerr := Resolve(m.Index, m.Current, body)
		if rerr != nil {
			if len(rerr.Candidates) > 0 {
				mvu.MessageOp{Message: OpenChooser{Body: body, Candidates: rerr.Candidates}}.Add(gtx.Ops)
				return
			}
			toast.Notify(gtx, toast.Warning, rerr.Reason)
			return
		}
		mvu.MessageOp{Message: Navigate{Path: res.Path, Headings: res.Headings, BlockID: res.BlockID}}.Add(gtx.Ops)
	case strings.HasPrefix(url, "https://"), strings.HasPrefix(url, "http://"):
		openBrowser(url)
	}
}

// openBrowser opens an absolute web URL in the system browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// messageChild renders a status line in place of the document.
func messageChild(tok themeTokens, msg string) layout.FlexChild {
	return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		drawLabel(gtx, tok.shaper, msg, tok.typ.BodyLarge, tok.col.Ramps.Neutral.Step(700))
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

// noteCrumbs builds the trail: the vault name, one segment per folder on
// the current note's path, then the note title. The vault crumb returns
// the tree to its root state; each folder crumb reveals that folder in the
// tree; the note title is where you already are and stays inert.
func noteCrumbs(m Model) []crumb {
	items := []crumb{{label: path.Base(strings.TrimRight(m.Vault, "/")), msg: RootTree{}}}
	note := m.CurrentNote()
	if note == nil {
		return items
	}
	if dir := path.Dir(note.Path); dir != "." {
		cum := ""
		for _, seg := range strings.Split(dir, "/") {
			if cum == "" {
				cum = seg
			} else {
				cum += "/" + seg
			}
			items = append(items, crumb{label: seg, msg: RevealFolder{Dir: cum}})
		}
	}
	return append(items, crumb{label: note.Title})
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
	// Disclosure marks drawn from the shipped face: Roboto owns + and −,
	// where the triangle glyphs resolve only through system fallback and
	// have rendered as missing-glyph boxes at runtime.
	chev := "+"
	if open {
		chev = "−"
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
// them, the raw block in code style otherwise. Both sit on a rounded
// tinted fill two neutral steps below the paper the note lies on — the
// same tint the document's code fences take — so the panel reads as an
// inset region of the page rather than a stray grey.
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
			// The key column is as wide as the longest key plus a fixed
			// gap: each key is measured into a discarded recording, and
			// the widest ink wins, capped at half the panel so a runaway
			// key cannot squeeze the values out.
			keyW := 0
			mg := gtx
			mg.Constraints.Min = image.Point{}
			for _, f := range fm.Fields {
				macro := op.Record(mg.Ops)
				d := drawLabel(mg, tok.shaper, f.Key, tok.typ.TitleSmall, tok.col.Ramps.Neutral.Step(700))
				macro.Stop()
				if d.Size.X > keyW {
					keyW = d.Size.X
				}
			}
			if maxW := gtx.Constraints.Max.X / 2; keyW > maxW {
				keyW = maxW
			}
			keyGap := gtx.Dp(unit.Dp(propKeyGapDp))
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
							return layout.Dimensions{Size: image.Pt(keyW+keyGap, dims.Size.Y), Baseline: dims.Baseline}
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
