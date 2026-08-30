// modelmenu.go owns the chat header's model picker: a chip showing the
// model prompts in the current chat use, opening a patterns/popover that
// lists Default plus every provider's cached models. Picking an entry
// reduces SetChatModel — a per-chat override persisted in the chat file.
// Open state is model state (Model.ModelMenu), the mindchat idiom.
//
// popover-canvas coupling: the popover centres its anchor in the canvas it
// is given and measures Content at canvas/2, so ChatPane hands it a
// chip-sized box in the header and the content overrides its incoming
// constraints to self-size. The anchor is components/chip, which is sized to
// its own content and refuses to stretch, so the box is a CAP rather than a
// shape: the chip fills it while the label is long and leaves slack in it
// when the label is short. Which end that slack falls on is the chip's
// Pin — PinTrailing here, so the pill's trailing edge is the box's, which
// is the content column's, whatever the label says.
package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/chip"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// menuRow is one popover entry: a provider caption or a pickable model.
type menuRow struct {
	caption  bool
	label    string
	provider string
	model    string
	active   bool
}

// menuThemed is the picker's slice of the theme: the palette plus the
// theme's Typography and its cached shaper for the chip and row text.
type menuThemed struct {
	palette Palette
	bar     scrollbar.Style
	typ     tokens.Typography
	shaper  *text.Shaper
}

// ModelMenu builds the chat header picker stream: the widget it emits is
// laid out by ChatPane in the header's chip box and draws the chip (the
// popover anchor) plus, while open, the model list surface.
func ModelMenu(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], popArb *popover.Arbiter) rx.Observable[layout.Widget] {
	openObs := rx.Map(modelObs, func(m Model) bool { return m.ModelMenu }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))

	palObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[menuThemed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Typography), func(ct rx.Tuple2[tokens.ColorTokens, tokens.Typography]) menuThemed {
			c, typ := ct.First, ct.Second
			return menuThemed{palette: PaletteFrom(c), bar: scrollbar.FromTokens(c), typ: typ, shaper: typ.Shaper()}
		})
	})

	// Interaction state at construction scope: the chip's clickable, the
	// rows' clickables, and the list's scroll position.
	var chipClick widget.Clickable
	rowClicks := map[string]*widget.Clickable{}
	rows := list.NewState()

	// The anchor and content are static popover props; the live widgets
	// reach them through cells (the observable-over-static-slot hand-off).
	var chipCell, contentCell atomic.Value
	slot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	dataObs := rx.Map(rx.CombineLatest2(palObs, modelObs), func(next rx.Tuple2[menuThemed, Model]) int {
		t, m := next.First, next.Second
		contentCell.Store(menuContent(t, menuRows(m), rowClicks, rows))
		return 0
	})

	// The chip is a live component rather than a widget this file draws, so
	// it is a stream of its own: components/chip takes its props once per
	// subscription, so a chip whose label is data needs a new subscription
	// when the label changes. That is why the key is deduplicated first —
	// Model emits on every streamed token, and a subscription per token is
	// not a rate a component was built for. The clickable is ours, so hover,
	// press and focus live outside the switch and survive it.
	chipObs := rx.Map(rx.SwitchMap(
		rx.Map(modelObs, chipKeyOf).Pipe(rx.DistinctUntilChanged(func(a, b chipKey) bool { return a == b })),
		func(k chipKey) rx.Observable[layout.Widget] {
			return chip.Chip(th, chip.Props{
				Label: k.label,
				// The chip's own mark, drawn by this app: the chevron points
				// the way the surface will go, which is the disclosure the
				// component itself has no state for (it knows rest, hover,
				// press and focus, and an anchor is none of those while its
				// popover stands open).
				Icon:        chevronGlyph(k.open),
				Description: "Model for this chat",
				// The header band is the transcript's own level-0 paper, so
				// the chip fills one storey over it — the zero value.
				Ground: tokens.Level0,
				// The picker's box is a CAP at the trailing edge of the
				// chrome row and the pill has to land ON that edge, over
				// the last ink of the message column and the composer
				// under it. The popover standing between this file and
				// the chip centres whatever it is handed, so the ask goes
				// to the chip — the one place both widths are known.
				Pin:       chip.PinTrailing,
				Clickable: &chipClick,
				OnClick: func(gtx layout.Context) {
					if k.open {
						mvu.MessageOp{Message: CloseModelMenu{}}.Add(gtx.Ops)
					} else {
						mvu.MessageOp{Message: OpenModelMenu{}}.Add(gtx.Ops)
					}
				},
			})
		}), func(w layout.Widget) int {
		chipCell.Store(w)
		return 0
	})

	popObs := popover.Popover(th, popover.Props{
		Open:      openObs,
		Anchor:    slot(&chipCell),
		Content:   slot(&contentCell),
		Placement: popover.Bottom,
		Arbiter:   popArb,
		OnDismiss: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseModelMenu{}}.Add(gtx.Ops)
		},
	})

	// Fold the data and chip streams onto the popover stream so chip/content
	// updates repaint it.
	return rx.Map(rx.CombineLatest3(popObs, dataObs, chipObs), func(next rx.Tuple3[layout.Widget, int, int]) layout.Widget {
		return next.First
	})
}

// chipKey is everything the header chip is a function of: the label it
// carries and which way its chevron points. Two Models with the same key
// build the same chip, which is what lets the chip's subscription outlive a
// streamed answer.
type chipKey struct {
	label string
	open  bool
}

// chipKeyOf reads the effective model out of the Model and names it: provider
// then model, nothing else. The chip answers one question — which model is
// this chat talking to — and the answer is the same string whether it came
// from the chat's own override or from the global default. Where it came from
// is a different question, it is asked rarely, and the menu answers it in
// full: the Default row is there, marked active exactly when no override is
// set. Saying "Default ·" on the anchor too spent a third of a header-wide
// pill restating what the pill already showed, which is what made a picker
// read at a search field's width.
//
// Dropping it also makes the key coarser on purpose: picking the model that
// happens to BE the default now yields the same key, so the chip keeps its
// subscription instead of rebuilding for an identical label.
func chipKeyOf(m Model) chipKey {
	label := "No model configured"
	if provider, id, ok := m.EffectiveModel(); ok {
		label = provider.Name + " · " + id
	}
	return chipKey{label: label, open: m.ModelMenu}
}

// chevronGlyph adapts this app's chevron to the mark signature
// components/chip draws with: a painter filling a size×size box at the
// current origin. The V is inscribed in that box at three fifths of its
// width, so the mark reads at the weight the label does rather than filling
// the whole line box.
func chevronGlyph(open bool) chip.Glyph {
	return func(gtx layout.Context, size int, col color.NRGBA) {
		w := size * 3 / 5
		h := w / 2
		box := image.Rect((size-w)/2, (size-h)/2, (size+w)/2, (size+h)/2)
		if open {
			ChevronUp(gtx, box, col)
			return
		}
		ChevronDown(gtx, box, col)
	}
}

// menuRows flattens the pickable entries: Default first, then each
// provider's cached models under a caption.
func menuRows(m Model) []menuRow {
	var out []menuRow
	defaultLabel := "Default"
	if p, ok := m.ProviderNamed(m.DefaultProvider); ok {
		defaultLabel = "Default (" + p.Name + " · " + m.DefaultModel + ")"
	}
	out = append(out, menuRow{label: defaultLabel, active: m.CurrentChat.Provider == ""})
	for _, p := range m.Providers {
		if len(p.Models) == 0 {
			continue
		}
		out = append(out, menuRow{caption: true, label: p.Name})
		for _, id := range p.Models {
			out = append(out, menuRow{
				label:    id,
				provider: p.Name,
				model:    id,
				active:   m.CurrentChat.Provider == p.Name && m.CurrentChat.Model == id,
			})
		}
	}
	return out
}

// menuContent lays the popover surface: a scroll-capped list of menuRows.
// It overrides the incoming canvas/2 constraints (see the file comment).
func menuContent(t menuThemed, entries []menuRow, rowClicks map[string]*widget.Clickable, rows *list.State) layout.Widget {
	p := t.palette
	for _, e := range entries {
		if e.caption {
			continue
		}
		key := e.provider + "\x00" + e.model
		if _, ok := rowClicks[key]; !ok {
			rowClicks[key] = new(widget.Clickable)
		}
	}
	return func(gtx layout.Context) layout.Dimensions {
		rowH := gtx.Dp(ModelRowHeight)
		w := gtx.Dp(MenuWidth)
		h := min(len(entries)*rowH, gtx.Dp(MenuMaxHeight))
		gtx.Constraints = layout.Exact(image.Pt(w, h))
		list.LayoutScrollbar(gtx, rows, t.bar, list.Occupy, entries,
			func(gtx layout.Context, e menuRow) layout.Dimensions {
				size := image.Pt(gtx.Constraints.Max.X, rowH)
				if e.caption {
					r := image.Rect(gtx.Dp(8), 0, size.X, size.Y)
					textdraw.FillText(gtx, t.shaper, roleText(t.typ.LabelSmall), r, 0, 0.5, p.Heading, e.label)
					return layout.Dimensions{Size: size}
				}
				click := rowClicks[e.provider+"\x00"+e.model]
				for click.Clicked(gtx) {
					mvu.MessageOp{Message: SetChatModel{Provider: e.provider, Model: e.model}}.Add(gtx.Ops)
				}
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					textColor := p.Row
					if e.active {
						textColor = p.RowActive
					}
					if click.Hovered() {
						FillRect(gtx, image.Rectangle{Max: size}, 0, p.RowHovered)
					}
					if e.active {
						d := gtx.Dp(ModelDotSize)
						dot := image.Rect(0, (size.Y-d)/2, d, (size.Y+d)/2).Add(image.Pt(gtx.Dp(6), 0))
						FillRect(gtx, dot, d/2, p.Accent)
					}
					r := image.Rect(gtx.Dp(ModelDotSlot+4), 0, size.X-gtx.Dp(4), size.Y)
					textdraw.FillText(gtx, t.shaper, roleText(t.typ.BodyMedium), r, 0, 0.5, textColor, e.label)
					return layout.Dimensions{Size: size}
				})
			})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// ChevronDown draws a small V glyph in box with clip paths (the patterns
// convention for chrome glyphs).
func ChevronDown(gtx layout.Context, box image.Rectangle, col color.NRGBA) {
	stroke := float32(gtx.Dp(unit.Dp(1.5)))
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(box.Min.X), float32(box.Min.Y)))
	path.LineTo(f32.Pt(float32(box.Min.X+box.Max.X)/2, float32(box.Max.Y)))
	path.LineTo(f32.Pt(float32(box.Max.X), float32(box.Min.Y)))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: path.End(), Width: stroke}.Op())
}

// ChevronUp is ChevronDown's mirror: the same V, pointing the other way, for
// a disclosure whose surface is already standing.
func ChevronUp(gtx layout.Context, box image.Rectangle, col color.NRGBA) {
	stroke := float32(gtx.Dp(unit.Dp(1.5)))
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(box.Min.X), float32(box.Max.Y)))
	path.LineTo(f32.Pt(float32(box.Min.X+box.Max.X)/2, float32(box.Min.Y)))
	path.LineTo(f32.Pt(float32(box.Max.X), float32(box.Max.Y)))
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: path.End(), Width: stroke}.Op())
}
