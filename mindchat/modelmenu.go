// modelmenu.go owns the chat header's model picker: components/picker's
// chrome-variant toolbar trigger, naming the model the prompts in the current
// chat use, standing under the picker menu that lists Default plus every
// provider's cached models. Picking an entry reduces SetChatModel — a per-chat override
// persisted in the chat file. Open state is model state (Model.ModelMenu), the
// mindchat idiom.
//
// A chrome-variant menu is a floating surface placed against the window and
// patterns/popover places it, so the trigger and the menu meet here rather
// than inside the component: the trigger is the popover's anchor slot and the
// menu its content slot.
//
// The canvas the popover gets is the whole chrome row inside the content
// area's insets (frame.go), because that canvas is the room the open menu may
// use and the popover keeps its surface inside it. Two things follow. The
// trigger is stood at the canvas's trailing edge by the popover, so the
// control lands on the content column's edge whatever the label says; and
// because the room is the row rather than the control, the two things that
// are NOT entitled to all of it cap themselves — the trigger at [ToolbarWidth],
// the surface at [MenuWidth] — over the constraints the popover offers, which
// for content is half the canvas.
package main

import (
	"image"
	"strconv"
	"strings"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/components/picker"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/popover"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// menuEntry is one pickable row: what it says and what picking it sets. An
// empty provider and model is the Default entry, which clears the override.
type menuEntry struct {
	label    string
	provider string
	model    string
}

// ModelMenu builds the chat header picker stream: the widget it emits is
// laid out by ChatPane in the header's picker box and draws the anchor (the
// popover's own anchor) plus, while open, the model list surface.
func ModelMenu(th rx.Observable[theme.Theme], modelObs rx.Observable[Model], popArb *popover.Arbiter) rx.Observable[layout.Widget] {
	// Whether the menu stands, kept where the anchor's click handler can read
	// it: the anchor's mark does not flip, so the open state is not part of
	// what the anchor is a function of and must not rebuild it.
	var menuOpen atomic.Bool

	openObs := rx.Map(modelObs, func(m Model) bool {
		menuOpen.Store(m.ModelMenu)
		return m.ModelMenu
	}).Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))

	// Interaction state at construction scope: the anchor's clickable, held
	// outside the switch below so hover, press and focus survive a rebuild.
	var anchorClick widget.Clickable

	// The anchor and content are static popover props; the live widgets
	// reach them through cells (the observable-over-static-slot hand-off).
	var anchorCell, contentCell atomic.Value
	slot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	// Both halves are live components rather than widgets this file draws, so
	// each is a stream of its own: components/picker takes its props once per
	// subscription, so a control whose value is data needs a new subscription
	// when that data changes. That is why each key is deduplicated first —
	// Model emits on every streamed token, and a subscription per token is not
	// a rate a component was built for.
	anchorObs := rx.Map(rx.SwitchMap(
		rx.Map(modelObs, anchorKeyOf).Pipe(rx.DistinctUntilChanged(func(a, b anchorKey) bool { return a == b })),
		func(k anchorKey) rx.Observable[layout.Widget] {
			return picker.Toolbar(th, picker.ToolbarProps{
				Value:       k.label,
				Description: "Model for this chat",
				// The header band is the transcript's own level-0 paper, so
				// the anchor fills one storey over it — the zero value.
				Ground: tokens.Level0,
				// The anchor reports the shape it drew and nothing wider: the
				// popover aims its tail at that rect, and a control that
				// widened its report to reach the trailing edge would be
				// pointed at from the middle of a box nothing was drawn in.
				// Reaching the edge is the popover's alignment below.
				Clickable: &anchorClick,
				OnClick: func(gtx layout.Context) {
					if menuOpen.Load() {
						mvu.MessageOp{Message: CloseModelMenu{}}.Add(gtx.Ops)
					} else {
						mvu.MessageOp{Message: OpenModelMenu{}}.Add(gtx.Ops)
					}
				},
			})
		}), func(w layout.Widget) int {
		anchorCell.Store(w)
		return 0
	})

	contentObs := rx.Map(rx.SwitchMap(
		rx.Map(modelObs, menuKeyOf).Pipe(rx.DistinctUntilChanged(func(a, b menuKey) bool { return a.id == b.id })),
		func(k menuKey) rx.Observable[layout.Widget] {
			entries := k.entries
			return rx.Map(picker.Menu(th, picker.MenuProps{
				Options:  labelsOf(entries),
				Selected: k.selected,
				// The header's catalogue is the same forty to sixty rows as
				// the settings dialog's, and the popover that places this
				// menu makes room for whatever it is handed: uncapped, a
				// full catalogue is taller than the window it floats in.
				MaxHeight: MenuMaxHeight,
				OnSelect: func(gtx layout.Context, i int) {
					e := entries[i]
					mvu.MessageOp{Message: SetChatModel{Provider: e.provider, Model: e.model}}.Add(gtx.Ops)
				},
			}), func(w layout.Widget) layout.Widget { return menuSurface(w, MenuWidth) })
		}), func(w layout.Widget) int {
		contentCell.Store(w)
		return 0
	})

	popObs := popover.Popover(th, popover.Props{
		Open:      openObs,
		Anchor:    anchorBox(slot(&anchorCell), ToolbarWidth),
		Content:   slot(&contentCell),
		Placement: popover.Bottom,
		Align:     popover.AlignTrailing,
		Arbiter:   popArb,
		OnDismiss: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseModelMenu{}}.Add(gtx.Ops)
		},
	})

	// Fold the anchor and content streams onto the popover stream so their
	// updates repaint it.
	return rx.Map(rx.CombineLatest3(popObs, anchorObs, contentObs), func(next rx.Tuple3[layout.Widget, int, int]) layout.Widget {
		return next.First
	})
}

// anchorBox caps how far the header control may run before its label is
// truncated. The popover offers the anchor the whole room it has, and the
// name of a model is not entitled to the width of the chrome row.
func anchorBox(anchor layout.Widget, width unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(width))
		return anchor(gtx)
	}
}

// menuSurface gives a picker menu the width this app's floating surfaces are
// drawn at and room to stack every row it has. Both halves are needed: the
// popover measures its content at half the canvas it was handed, and half a
// chrome row is neither this width nor tall enough for a catalogue.
func menuSurface(menu layout.Widget, width unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(width), 1<<20)}
		return menu(gtx)
	}
}

// anchorKey is everything the header anchor is a function of, which is the
// label alone: the mark is the component's and does not move, so the open
// state is not in here and opening the menu does not rebuild the control.
type anchorKey struct {
	label string
}

// anchorKeyOf reads the effective model out of the Model and names it:
// provider then model, nothing else. The anchor answers one question — which
// model is this chat talking to — and the answer is the same string whether it
// came from the chat's own override or from the global default. Where it came
// from is a different question, it is asked rarely, and the menu answers it in
// full: the Default row is there, selected exactly when no override is set.
func anchorKeyOf(m Model) anchorKey {
	label := "No model configured"
	if provider, id, ok := m.EffectiveModel(); ok {
		label = provider.Name + " · " + id
	}
	return anchorKey{label: label}
}

// menuKey is the option list and the row standing on the inverse plane, with
// the identity the deduplication compares on — a rebuild of a menu whose rows
// and selection are unchanged would be a rebuild for an identical frame.
type menuKey struct {
	id       string
	entries  []menuEntry
	selected int
}

// menuKeyOf flattens the pickable entries: Default first, then every
// provider's cached models. Each model row carries its provider's name,
// because the surface is a flat option list and which provider a model belongs
// to is part of what the row has to say.
func menuKeyOf(m Model) menuKey {
	defaultLabel := "Default"
	if p, ok := m.ProviderNamed(m.DefaultProvider); ok {
		defaultLabel = "Default (" + p.Name + " · " + m.DefaultModel + ")"
	}
	entries := []menuEntry{{label: defaultLabel}}
	selected := 0
	if m.CurrentChat.Provider != "" {
		selected = -1
	}
	for _, p := range m.Providers {
		for _, id := range p.Models {
			if m.CurrentChat.Provider == p.Name && m.CurrentChat.Model == id {
				selected = len(entries)
			}
			entries = append(entries, menuEntry{
				label:    p.Name + " · " + id,
				provider: p.Name,
				model:    id,
			})
		}
	}
	return menuKey{id: identityOf(entries, selected), entries: entries, selected: selected}
}

// labelsOf is the option list a picker menu takes: the entries' labels in the
// order they are drawn.
func labelsOf(entries []menuEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.label
	}
	return out
}

// identityOf names an option list and its selection in one comparable string,
// so a stream of Models can be deduplicated down to the emissions that
// actually change what the menu offers.
func identityOf(entries []menuEntry, selected int) string {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.label)
		b.WriteByte(0)
	}
	b.WriteString(strconv.Itoa(selected))
	return b.String()
}
