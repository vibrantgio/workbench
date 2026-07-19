// settings.go owns the settings modal the sidebar gear opens: the provider
// catalogue editor (list with +/− , Name/BaseURL/APIKey fields, where the
// key is auto-checked by a debounced /models fetch and its verdict shown
// beside the field) and the GLOBAL default-model row spanning the modal's
// bottom, whose dropdown groups every provider's models. It follows the
// rename-modal recipe (cadence/modal + epoch-rebuilt uncontrolled prism
// fields + cell hand-offs into the modal's static slots); all edits reduce
// into Model.Settings.Draft per keystroke and apply on Save.
package main

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/popover"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/input"
	"github.com/vibrantgio/prism/list"
	"github.com/vibrantgio/prism/scrollbar"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
	"github.com/vibrantgio/style"
	"github.com/vibrantgio/textdraw"
	"sync/atomic"
)

// settingsThemed pairs one theme emission's palette with the icon widgets
// the modal body draws (prebuilt per emission, like view.go's themed).
type settingsThemed struct {
	palette Palette
	bar     scrollbar.Style
	add     layout.Widget
	remove  layout.Widget
	refresh layout.Widget
	keyOK   layout.Widget
	keyBad  layout.Widget
	boxOn   layout.Widget
	boxOff  layout.Widget
}

// settingsTarget keys the rebuild of the three uncontrolled provider
// fields: a new epoch (open, add/remove, selection change) means fresh
// fields seeded from the now-selected draft provider. Comparison is on
// epoch ONLY — keystrokes change the seeds via the draft but must never
// rebuild the field being typed in.
type settingsTarget struct {
	epoch          int
	name, url, key string
	hasProvider    bool
}

// settingsFields carries the three live field widgets of one epoch.
type settingsFields struct {
	name, url, key layout.Widget
}

// SettingsModal builds the settings modal stream. Open state, the draft
// being edited, and fetch errors all live in Model.Settings; the modal is
// pure view over them.
func SettingsModal(th rx.Observable[theme.Theme], shaper *text.Shaper, modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	openObs := rx.Map(modelObs, func(m Model) bool { return m.Settings.Open }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))

	targetObs := rx.Map(modelObs, func(m Model) settingsTarget {
		t := settingsTarget{epoch: m.Settings.Epoch}
		if p, ok := m.Settings.SelectedProvider(); ok {
			t.name, t.url, t.key, t.hasProvider = p.Name, p.BaseURL, p.APIKey, true
		}
		return t
	}).Pipe(rx.DistinctUntilChanged(func(a, b settingsTarget) bool { return a.epoch == b.epoch }))

	// The fields' focus tags, in Tab order, for the modal's focus cycle.
	var nameTag, urlTag, keyTag atomic.Value

	fieldsObs := rx.SwitchMap(targetObs, func(t settingsTarget) rx.Observable[settingsFields] {
		field := func(seed, placeholder, description string, mask rune, f ProviderField, tag *atomic.Value) rx.Observable[layout.Widget] {
			return input.TextField(th, input.TextFieldProps{
				Placeholder: placeholder,
				Description: description,
				Seed:        seed,
				Mask:        mask,
				FocusTag:    func(t event.Tag) { tag.Store(t) },
				Shaper:      shaper,
				OnChange: func(gtx layout.Context, text string) {
					mvu.MessageOp{Message: EditProvider{Field: f, Text: text}}.Add(gtx.Ops)
				},
			})
		}
		return rx.Map(rx.CombineLatest3(
			field(t.name, "Name", "provider name", 0, FieldName, &nameTag),
			field(t.url, "https://api.openai.com/v1", "provider base URL", 0, FieldBaseURL, &urlTag),
			field(t.key, "API key", "provider API key", '•', FieldAPIKey, &keyTag),
		), func(next rx.Tuple3[layout.Widget, layout.Widget, layout.Widget]) settingsFields {
			return settingsFields{name: next.First, url: next.Second, key: next.Third}
		})
	})

	themedObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[settingsThemed] {
		return rx.Map(t.Color, func(c tokens.ColorTokens) settingsThemed {
			p := PaletteFrom(c)
			mk := func(data []byte, col color.NRGBA) layout.Widget {
				w, err := raster.Widget(data, SettingsIconBtn, SettingsIconBtn, raster.WithColors(col))
				if err != nil {
					panic(err)
				}
				return w
			}
			return settingsThemed{
				palette: p,
				bar:     scrollbar.FromTokens(c),
				add:     mk(icons.ContentAdd, p.Heading),
				remove:  mk(icons.ContentRemove, p.Heading),
				refresh: mk(icons.NavigationRefresh, p.Heading),
				keyOK:   mk(icons.ActionCheckCircle, p.Ok),
				keyBad:  mk(icons.AlertError, p.Error),
				boxOn:   mk(icons.ToggleCheckBox, p.Accent),
				boxOff:  mk(icons.ToggleCheckBoxOutlineBlank, p.Heading),
			}
		})
	})

	// Interaction state at construction scope (subscribed once): the
	// provider rows', template chips', model rows', and chrome buttons'
	// clickables, and the two lists' scroll positions.
	provClicks := map[int]*widget.Clickable{}
	modelClicks := map[string]*widget.Clickable{}
	tplClicks := make([]*widget.Clickable, len(ProviderTemplates))
	for i := range tplClicks {
		tplClicks[i] = new(widget.Clickable)
	}
	var addClick, removeClick, refreshClick, webClick, cancelClick, saveClick, dropClick widget.Clickable
	provList := list.NewState()
	modelList := list.NewState()

	cellSlot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}

	// The default-model dropdown is a popover anchored on the chip in the
	// global default-model row at the body's bottom (the same popover-canvas
	// coupling ChatPane's picker uses); it opens UPWARD because the modal's
	// action row is drawn after the body and would paint over a downward
	// surface.
	dropOpenObs := rx.Map(modelObs, func(m Model) bool { return m.Settings.Open && m.Settings.Dropdown }).
		Pipe(rx.DistinctUntilChanged(func(a, b bool) bool { return a == b }))
	var dropChipCell, dropContentCell atomic.Value
	dropObs := popover.Popover(th, popover.Props{
		Open:      dropOpenObs,
		Anchor:    cellSlot(&dropChipCell),
		Content:   cellSlot(&dropContentCell),
		Placement: popover.Top,
		OnDismiss: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseDefaultModelMenu{}}.Add(gtx.Ops)
		},
	})

	var fieldCells struct{ name, url, key atomic.Value }
	bodyObs := rx.Map(rx.CombineLatest3(themedObs, modelObs, dropObs), func(next rx.Tuple3[settingsThemed, Model, layout.Widget]) layout.Widget {
		t, s, drop := next.First, next.Second.Settings, next.Third
		dropChipCell.Store(dropChip(shaper, t, s, &dropClick))
		dropContentCell.Store(dropContent(shaper, t, s, modelClicks, modelList))
		return settingsBody(shaper, t, s, drop,
			provClicks, tplClicks, &addClick, &removeClick, &refreshClick, &webClick,
			provList, &fieldCells.name, &fieldCells.url, &fieldCells.key)
	})

	cancelObs := button.Button(th, button.Props{
		Label:     "Cancel",
		Clickable: &cancelClick,
		Shaper:    shaper,
		OnClick: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseSettings{}}.Add(gtx.Ops)
		},
	})
	saveObs := button.Button(th, button.Props{
		Label:     "Save",
		Clickable: &saveClick,
		Shaper:    shaper,
		OnClick: func(gtx layout.Context) {
			mvu.MessageOp{Message: SaveSettings{}}.Add(gtx.Ops)
		},
	})

	// The modal body and actions are static slots; the live widgets reach
	// them through cells (the observable-over-static-slot hand-off).
	var bodyCell, cancelCell, saveCell atomic.Value
	slot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				return w(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}
	body := func(gtx layout.Context) layout.Dimensions {
		cg := gtx
		cg.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(SettingsBodyHeight)))
		slot(&bodyCell)(cg)
		return layout.Dimensions{Size: cg.Constraints.Max}
	}
	action := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(RenameButtonWidth), gtx.Dp(RenameButtonHeight)))
			return slot(cell)(gtx)
		}
	}

	modalObs := modal.Modal(th, modal.Props{
		Open:    openObs,
		Title:   "Settings",
		Body:    body,
		Actions: []layout.Widget{action(&cancelCell), action(&saveCell)},
		// The provider fields lead the Tab cycle; their tags are dynamic
		// (each epoch rebuilds the fields — new editors, new tags).
		DynamicFocusTags: func() []event.Tag {
			var tags []event.Tag
			for _, cell := range []*atomic.Value{&nameTag, &urlTag, &keyTag} {
				if tag, ok := cell.Load().(event.Tag); ok && tag != nil {
					tags = append(tags, tag)
				}
			}
			return tags
		},
		ActionFocusTags: []event.Tag{&cancelClick, &saveClick},
		HideClose:       true,
		Shaper:          shaper,
		OnClose: func(gtx layout.Context) {
			mvu.MessageOp{Message: CloseSettings{}}.Add(gtx.Ops)
		},
	})

	// Fold the live body/field/button streams onto the modal stream so
	// their emissions repaint it.
	return rx.Map(rx.CombineLatest5(modalObs, fieldsObs, bodyObs, cancelObs, saveObs),
		func(next rx.Tuple5[layout.Widget, settingsFields, layout.Widget, layout.Widget, layout.Widget]) layout.Widget {
			fieldCells.name.Store(next.Second.name)
			fieldCells.url.Store(next.Second.url)
			fieldCells.key.Store(next.Second.key)
			bodyCell.Store(next.Third)
			cancelCell.Store(next.Fourth)
			saveCell.Store(next.Fifth)
			return next.First
		})
}

// settingsBody lays the modal body for one (theme, settings) emission: the
// provider column (its own shaded panel: list + add/remove) beside the
// selected provider's pane — the template bar, the Name/BaseURL fields,
// the API-key row (field + check verdict + refresh) and a status line
// spelling the check out — with the GLOBAL default-model row spanning the
// bottom under both. The dropdown popover widget (chip anchor + upward
// surface) is drawn LAST, over the body, at the chip's rect in that row.
func settingsBody(shaper *text.Shaper, t settingsThemed, s SettingsState, drop layout.Widget,
	provClicks map[int]*widget.Clickable, tplClicks []*widget.Clickable,
	addClick, removeClick, refreshClick, webClick *widget.Clickable,
	provList *list.State,
	nameCell, urlCell, keyCell *atomic.Value,
) layout.Widget {
	p := t.palette
	selected, hasProvider := s.SelectedProvider()
	for i := range s.Draft {
		if _, ok := provClicks[i]; !ok {
			provClicks[i] = new(widget.Clickable)
		}
	}

	fieldSlot := func(cell *atomic.Value) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			cg := gtx
			cg.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(SettingsFieldHeight)))
			if w, ok := cell.Load().(layout.Widget); ok && w != nil {
				w(cg)
			}
			return layout.Dimensions{Size: cg.Constraints.Max}
		}
	}

	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		gtx.Constraints = layout.Exact(size)
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(SettingsListWidth), gtx.Constraints.Max.Y))
						return providerColumn(gtx, shaper, t, s, provClicks, addClick, removeClick, provList)
					}),
					layout.Rigid(layout.Spacer{Width: 12}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if !hasProvider {
							textdraw.FillText(gtx, shaper, style.Subtitle2,
								image.Rectangle{Max: gtx.Constraints.Max}, 0.5, 0.5, p.Row,
								"No providers — add one with +")
							return layout.Dimensions{Size: gtx.Constraints.Max}
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return templateBar(gtx, shaper, t, tplClicks)
							}),
							layout.Rigid(layout.Spacer{Height: 8}.Layout),
							layout.Rigid(fieldSlot(nameCell)),
							layout.Rigid(layout.Spacer{Height: 8}.Layout),
							layout.Rigid(fieldSlot(urlCell)),
							layout.Rigid(layout.Spacer{Height: 8}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return keyRow(gtx, t, s, selected, fieldSlot(keyCell), refreshClick)
							}),
							layout.Rigid(layout.Spacer{Height: 4}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return webSearchRow(gtx, shaper, t, selected.WebSearch, webClick)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: gtx.Constraints.Max}
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return statusLine(gtx, shaper, t, s, selected)
							}),
						)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return defaultModelRow(gtx, shaper, t)
			}),
		)

		// The dropdown popover gets an Exact chip-sized box at the default
		// row's right edge; it centres its anchor (the chip) there and
		// hangs the surface above it.
		chipH := gtx.Dp(ChipHeight)
		chipW := gtx.Dp(DropChipWidth)
		if max := size.X - gtx.Dp(130); chipW > max {
			chipW = max
		}
		chipX := size.X - chipW
		chipY := size.Y - gtx.Dp(SelectRowHeight) + (gtx.Dp(SelectRowHeight)-chipH)/2
		defer op.Offset(image.Pt(chipX, chipY)).Push(gtx.Ops).Pop()
		dg := gtx
		dg.Constraints = layout.Exact(image.Pt(chipW, chipH))
		drop(dg)
		return layout.Dimensions{Size: size}
	}
}

// templateBar renders one chip per ProviderTemplates entry; clicking a
// chip prefills the selected provider's Name and BaseURL.
func templateBar(gtx layout.Context, shaper *text.Shaper, t settingsThemed, tplClicks []*widget.Clickable) layout.Dimensions {
	p := t.palette
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(TemplateRowHeight))
	gtx.Constraints = layout.Exact(size)
	children := make([]layout.FlexChild, 0, 2*len(ProviderTemplates))
	for i := range ProviderTemplates {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 6}.Layout))
		}
		index := i
		click := tplClicks[i]
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			for click.Clicked(gtx) {
				mvu.MessageOp{Message: ApplyTemplate{Index: index}}.Add(gtx.Ops)
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				sz := image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
				pointer.CursorPointer.Add(gtx.Ops)
				fill := p.BotBubble
				if click.Hovered() {
					fill = p.RowHovered
				}
				FillRect(gtx, image.Rectangle{Max: sz}, sz.Y/2, fill)
				textdraw.FillText(gtx, shaper, style.Caption, image.Rectangle{Max: sz}, 0.5, 0.5, p.BotText, ProviderTemplates[index].Name)
				return layout.Dimensions{Size: sz}
			})
		}))
	}
	layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
	return layout.Dimensions{Size: size}
}

// keyRow is the API-key line: the key field, the key-check verdict icon
// (green check = the last /models fetch succeeded, red error = it failed,
// empty while unchecked), and the manual re-check affordance.
func keyRow(gtx layout.Context, t settingsThemed, s SettingsState, prov Provider, field layout.Widget, refreshClick *widget.Clickable) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(SettingsFieldHeight))
	gtx.Constraints = layout.Exact(size)
	verdict := func(gtx layout.Context) layout.Dimensions {
		sz := gtx.Dp(SettingsIconBtn)
		var icon layout.Widget
		switch s.KeyStatus(prov) {
		case KeyOK:
			icon = t.keyOK
		case KeyBad:
			icon = t.keyBad
		}
		if icon != nil {
			ig := gtx
			ig.Constraints = layout.Exact(image.Pt(sz, sz))
			icon(ig)
		}
		return layout.Dimensions{Size: image.Pt(sz, sz)}
	}
	for refreshClick.Clicked(gtx) {
		mvu.MessageOp{Message: RefreshModels{}}.Add(gtx.Ops)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, field),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(verdict),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return IconButton(gtx, refreshClick, SettingsIconBtn, func(gtx layout.Context, sz int) {
				t.refresh(gtx)
			})
		}),
	)
}

// webSearchRow is the provider's server-side search opt-in. With the box
// checked every request attaches the web_search tool, which xAI and
// OpenAI execute on their servers (citations come back as annotations);
// providers that reject unknown tools should leave it off.
func webSearchRow(gtx layout.Context, shaper *text.Shaper, t settingsThemed, on bool, click *widget.Clickable) layout.Dimensions {
	for click.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleWebSearch{}}.Add(gtx.Ops)
	}
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(SettingsCaptionRow))
	gtx.Constraints = layout.Exact(size)
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		pointer.CursorPointer.Add(gtx.Ops)
		sz := gtx.Dp(SettingsIconBtn)
		func() {
			defer op.Offset(image.Pt(0, (size.Y-sz)/2)).Push(gtx.Ops).Pop()
			ig := gtx
			ig.Constraints = layout.Exact(image.Pt(sz, sz))
			if on {
				t.boxOn(ig)
			} else {
				t.boxOff(ig)
			}
		}()
		r := image.Rect(sz+gtx.Dp(8), 0, size.X, size.Y)
		textdraw.FillText(gtx, shaper, style.Caption, r, 0, 0.5, t.palette.Row, "Web search tool (server-side; xAI and OpenAI)")
		return layout.Dimensions{Size: size}
	})
}

// defaultModelRow is the modal-wide DEFAULT MODEL row under both panes:
// the caption of the GLOBAL default picker whose dropdown chip settingsBody
// overlays at the row's right edge.
func defaultModelRow(gtx layout.Context, shaper *text.Shaper, t settingsThemed) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(SelectRowHeight))
	textdraw.FillText(gtx, shaper, style.Caption, image.Rectangle{Max: size}, 0, 0.5, t.palette.Heading, "DEFAULT MODEL")
	return layout.Dimensions{Size: size}
}

// dropChip is the dropdown's anchor: the global default pair, or a
// placeholder while none is picked. Clicking toggles the menu; with no
// models cached anywhere there is nothing to list and the chip stays inert.
func dropChip(shaper *text.Shaper, t settingsThemed, s SettingsState, click *widget.Clickable) layout.Widget {
	p := t.palette
	canOpen := slices.ContainsFunc(s.Draft, func(p Provider) bool { return len(p.Models) > 0 })
	label := "No models"
	switch {
	case s.DefaultProvider != "" && s.DefaultModel != "":
		label = s.DefaultProvider + " · " + s.DefaultModel
	case canOpen:
		label = "Choose model…"
	}
	open := s.Dropdown
	return func(gtx layout.Context) layout.Dimensions {
		for click.Clicked(gtx) {
			if open {
				mvu.MessageOp{Message: CloseDefaultModelMenu{}}.Add(gtx.Ops)
			} else if canOpen {
				mvu.MessageOp{Message: OpenDefaultModelMenu{}}.Add(gtx.Ops)
			}
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			pointer.CursorPointer.Add(gtx.Ops)
			fill := p.BotBubble
			if click.Hovered() || open {
				fill = p.RowHovered
			}
			FillRect(gtx, image.Rectangle{Max: size}, gtx.Dp(6), fill)
			chevW := gtx.Dp(12)
			r := image.Rect(gtx.Dp(10), 0, size.X-chevW-gtx.Dp(10), size.Y)
			textdraw.FillText(gtx, shaper, style.Caption, r, 0, 0.5, p.BotText, label)
			ChevronDown(gtx, image.Rect(size.X-chevW-gtx.Dp(6), (size.Y-chevW/2)/2, size.X-gtx.Dp(6), (size.Y+chevW/2)/2), p.BotText)
			return layout.Dimensions{Size: size}
		})
	}
}

// dropContent lays the dropdown surface: every draft provider's cached
// models under a provider caption (the chat header picker's grouping),
// scroll-capped; clicking one sets the draft's global default pair. It
// overrides the incoming canvas/2 constraints (popover-canvas coupling).
func dropContent(shaper *text.Shaper, t settingsThemed, s SettingsState, modelClicks map[string]*widget.Clickable, rows *list.State) layout.Widget {
	p := t.palette
	var entries []menuRow
	for _, prov := range s.Draft {
		if len(prov.Models) == 0 {
			continue
		}
		entries = append(entries, menuRow{caption: true, label: prov.Name})
		for _, id := range prov.Models {
			entries = append(entries, menuRow{
				label:    id,
				provider: prov.Name,
				model:    id,
				active:   s.DefaultProvider == prov.Name && s.DefaultModel == id,
			})
		}
	}
	for _, e := range entries {
		if e.caption {
			continue
		}
		if _, present := modelClicks[e.provider+"\x00"+e.model]; !present {
			modelClicks[e.provider+"\x00"+e.model] = new(widget.Clickable)
		}
	}
	return func(gtx layout.Context) layout.Dimensions {
		rowH := gtx.Dp(ModelRowHeight)
		w := gtx.Dp(DropChipWidth)
		h := min(len(entries)*rowH, gtx.Dp(180))
		gtx.Constraints = layout.Exact(image.Pt(w, h))
		list.LayoutScrollbar(gtx, rows, t.bar, list.Occupy, entries,
			func(gtx layout.Context, e menuRow) layout.Dimensions {
				size := image.Pt(gtx.Constraints.Max.X, rowH)
				if e.caption {
					r := image.Rect(gtx.Dp(8), 0, size.X, size.Y)
					textdraw.FillText(gtx, shaper, style.Caption, r, 0, 0.5, p.Heading, e.label)
					return layout.Dimensions{Size: size}
				}
				click := modelClicks[e.provider+"\x00"+e.model]
				for click.Clicked(gtx) {
					mvu.MessageOp{Message: SetDefaultModel{Provider: e.provider, Model: e.model}}.Add(gtx.Ops)
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
						dot := image.Rect(0, (size.Y-d)/2, d, (size.Y+d)/2).Add(image.Pt(gtx.Dp(4), 0))
						FillRect(gtx, dot, d/2, p.Accent)
					}
					r := image.Rect(gtx.Dp(ModelDotSlot), 0, size.X-gtx.Dp(4), size.Y)
					textdraw.FillText(gtx, shaper, style.Subtitle2, r, 0, 0.5, textColor, e.label)
					return layout.Dimensions{Size: size}
				})
			})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
}

// providerColumn is the left pane: the PROVIDERS caption, the selectable
// provider rows, and the add/remove buttons underneath — on its own
// shaded panel so the catalogue reads as a distinct surface from the
// editing pane beside it.
func providerColumn(gtx layout.Context, shaper *text.Shaper, t settingsThemed, s SettingsState,
	provClicks map[int]*widget.Clickable, addClick, removeClick *widget.Clickable, rows *list.State,
) layout.Dimensions {
	p := t.palette
	for addClick.Clicked(gtx) {
		mvu.MessageOp{Message: AddProvider{}}.Add(gtx.Ops)
	}
	for removeClick.Clicked(gtx) {
		mvu.MessageOp{Message: RemoveProvider{}}.Add(gtx.Ops)
	}
	size := gtx.Constraints.Max
	FillRect(gtx, image.Rectangle{Max: size}, gtx.Dp(SettingsPanelInset), p.RowHovered)
	defer op.Offset(image.Pt(gtx.Dp(SettingsPanelInset), gtx.Dp(SettingsPanelInset))).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(size.X-2*gtx.Dp(SettingsPanelInset), size.Y-2*gtx.Dp(SettingsPanelInset)))
	indices := make([]int, len(s.Draft))
	for i := range indices {
		indices[i] = i
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			r := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Dp(SettingsCaptionRow))
			textdraw.FillText(gtx, shaper, style.Caption, r, 0, 0.5, p.Heading, "PROVIDERS")
			return layout.Dimensions{Size: r.Max}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return list.LayoutScrollbar(gtx, rows, t.bar, list.Overlay, indices,
				func(gtx layout.Context, i int) layout.Dimensions {
					return providerRow(gtx, shaper, t, s.Draft[i], i == s.Selected, i, provClicks[i])
				})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := func(click *widget.Clickable, icon layout.Widget) layout.Widget {
				return func(gtx layout.Context) layout.Dimensions {
					return IconButton(gtx, click, SettingsIconBtn, func(gtx layout.Context, sz int) {
						icon := icon
						ig := gtx
						ig.Constraints = layout.Exact(image.Pt(sz, sz))
						icon(ig)
					})
				}
			}
			return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(btn(addClick, t.add)),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(btn(removeClick, t.remove)),
				)
			})
		}),
	)
	return layout.Dimensions{Size: size}
}

// providerRow is one selectable provider entry, in the sidebar row idiom
// (hover fill, selected fill + accent bar).
func providerRow(gtx layout.Context, shaper *text.Shaper, t settingsThemed, prov Provider, selected bool, index int, click *widget.Clickable) layout.Dimensions {
	p := t.palette
	for click.Clicked(gtx) {
		mvu.MessageOp{Message: SelectProvider{Index: index}}.Add(gtx.Ops)
	}
	name := prov.Name
	if strings.TrimSpace(name) == "" {
		name = "(unnamed)"
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(SettingsRowHeight))
		textColor := p.Row
		switch {
		case selected:
			FillRect(gtx, image.Rectangle{Max: size}, 0, p.RowSelected)
			FillRect(gtx, image.Rectangle{Max: image.Pt(gtx.Dp(3), size.Y)}, 0, p.Accent)
			textColor = p.RowActive
		case click.Hovered():
			FillRect(gtx, image.Rectangle{Max: size}, 0, p.RowHovered)
			textColor = p.RowActive
		}
		r := image.Rect(gtx.Dp(10), 0, size.X-gtx.Dp(4), size.Y)
		textdraw.FillText(gtx, shaper, style.Subtitle2, r, 0, 0.5, textColor, name)
		return layout.Dimensions{Size: size}
	})
}

// statusLine spells out the selected provider's key check: the fetch
// error, a hint while the key is missing or being checked, or the size of
// the listed catalogue.
func statusLine(gtx layout.Context, shaper *text.Shaper, t settingsThemed, s SettingsState, prov Provider) layout.Dimensions {
	p := t.palette
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(SettingsCaptionRow))
	text, col := "", p.Row
	switch s.KeyStatus(prov) {
	case KeyBad:
		text, col = s.Errors[prov.Name], p.Error
	case KeyMissing:
		text = "Add an API key to list models"
	case KeyChecking:
		text = "Checking API key…"
	case KeyOK:
		text = fmt.Sprintf("%d chat models listed", len(prov.Models))
		if len(prov.Models) == 0 {
			text = "Key OK — no chat models listed"
		}
	}
	textdraw.FillText(gtx, shaper, style.Caption, image.Rectangle{Max: size}, 0, 0.5, col, text)
	return layout.Dimensions{Size: size}
}
