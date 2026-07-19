// modelmenu.go owns the chat header's model picker: a chip showing the
// model prompts in the current chat use, opening a cadence/popover that
// lists Default plus every provider's cached models. Picking an entry
// reduces SetChatModel — a per-chat override persisted in the chat file.
// Open state is model state (Model.ModelMenu), the mindchat idiom.
//
// popover-canvas coupling (the watchlist recipe): the popover centres its
// anchor in the canvas it is given and measures Content at canvas/2, so
// ChatPane hands it an Exact chip-sized box in the header and the content
// overrides its incoming constraints to self-size.
package main

import (
	"image"
	"image/color"
	"sync/atomic"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/cadence/popover"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/list"
	"github.com/vibrantgio/prism/scrollbar"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"
	"github.com/vibrantgio/textdraw"
)

// menuRow is one popover entry: a provider caption or a pickable model.
type menuRow struct {
	caption  bool
	label    string
	provider string
	model    string
	active   bool
}

// menuThemed is the picker's slice of the theme.
type menuThemed struct {
	palette Palette
	bar     scrollbar.Style
}

// ModelMenu builds the chat header picker stream: the widget it emits is
// laid out by ChatPane in the header's chip box and draws the chip (the
// popover anchor) plus, while open, the model list surface.
func ModelMenu(th rx.Observable[theme.Theme], shaper *text.Shaper, modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	openObs := rx.Map(modelObs, func(m Model) bool { return m.ModelMenu }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))

	palObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[menuThemed] {
		return rx.Map(t.Color, func(c tokens.ColorTokens) menuThemed {
			return menuThemed{palette: PaletteFrom(c), bar: scrollbar.FromTokens(c)}
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
		chipCell.Store(menuChip(shaper, t, m, &chipClick))
		contentCell.Store(menuContent(shaper, t, menuRows(m), rowClicks, rows))
		return 0
	})

	popObs := popover.Popover(th, popover.Props{
		Open:      openObs,
		Anchor:    slot(&chipCell),
		Content:   slot(&contentCell),
		Placement: popover.Bottom,
		OnDismiss: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseModelMenu{}}.Add(gtx.Ops)
		},
	})

	// Fold the data stream onto the popover stream so chip/content updates
	// repaint it.
	return rx.Map(rx.CombineLatest2(popObs, dataObs), func(next rx.Tuple2[layout.Widget, int]) layout.Widget {
		return next.First
	})
}

// menuChip is the header chip: the effective model label plus a chevron.
// Clicking toggles the menu.
func menuChip(shaper *text.Shaper, t menuThemed, m Model, click *widget.Clickable) layout.Widget {
	p := t.palette
	label := "No model configured"
	if provider, id, ok := m.EffectiveModel(); ok {
		label = provider.Name + " · " + id
		if m.CurrentChat.Provider == "" {
			label = "Default · " + label
		}
	}
	open := m.ModelMenu
	return func(gtx layout.Context) layout.Dimensions {
		for click.Clicked(gtx) {
			if open {
				mvu.MessageOp{Message: CloseModelMenu{}}.Add(gtx.Ops)
			} else {
				mvu.MessageOp{Message: OpenModelMenu{}}.Add(gtx.Ops)
			}
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			pointer.CursorPointer.Add(gtx.Ops)
			fill := p.BotBubble
			if click.Hovered() || open {
				fill = p.RowHovered
			}
			FillRect(gtx, image.Rectangle{Max: size}, gtx.Dp(ChipRadius), fill)
			chevW := gtx.Dp(12)
			r := image.Rect(gtx.Dp(12), 0, size.X-chevW-gtx.Dp(12), size.Y)
			textdraw.FillText(gtx, shaper, style.Caption, r, 0.5, 0.5, p.BotText, label)
			ChevronDown(gtx, image.Rect(size.X-chevW-gtx.Dp(8), (size.Y-chevW/2)/2, size.X-gtx.Dp(8), (size.Y+chevW/2)/2), p.BotText)
			return layout.Dimensions{Size: size}
		})
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
func menuContent(shaper *text.Shaper, t menuThemed, entries []menuRow, rowClicks map[string]*widget.Clickable, rows *list.State) layout.Widget {
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
					textdraw.FillText(gtx, shaper, style.Caption, r, 0, 0.5, p.Heading, e.label)
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
					textdraw.FillText(gtx, shaper, style.Subtitle2, r, 0, 0.5, textColor, e.label)
					return layout.Dimensions{Size: size}
				})
			})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// ChevronDown draws a small V glyph in box with clip paths (the cadence
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
