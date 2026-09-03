package main

import (
	"fmt"
	"image"
	"image/color"
	"sync/atomic"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	raster "github.com/vibrantgio/ivg/raster/gio"

	"github.com/vibrantgio/components/button"
	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/modal"
	"github.com/vibrantgio/patterns/tabs"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// tabScreens maps tab indexes to the Model.Screen names, in strip order.
var tabScreens = []string{"monitor", "presets", "device"}

// themed pairs one theme emission with the palette, typography and stat
// icons derived from it.
type themed struct {
	palette Palette
	typ     Type
	ic      statIcons
}

// statIcons are the dashboard's glyphs, rasterised once per theme emission
// in the ink they are drawn with.
type statIcons struct {
	Bolt     layout.Widget // input voltage
	BoltOn   layout.Widget // the amp row's output bolt, lit
	BoltOff  layout.Widget // the amp row's output bolt, disabled ink
	Flame    layout.Widget // internal temperature
	FlameHot layout.Widget // internal temperature, running hot
	Battery  layout.Widget // accumulated charge
	Flare    layout.Widget // accumulated energy
	Clock    layout.Widget // output-on time
	PowerOn  layout.Widget // the header's toggle glyph while the output is on
	PowerOff layout.Widget // the header's toggle glyph while it is off
}

// statIconDp is the drawn glyph size of a dashboard stat.
const statIconDp = 20

func iconsFrom(p Palette) statIcons {
	mk := func(data []byte, col color.NRGBA, sz unit.Dp) layout.Widget {
		w, err := raster.Widget(data, sz, sz, raster.WithColors(col))
		if err != nil {
			panic(err)
		}
		return w
	}
	return statIcons{
		Bolt:     mk(icons.ImageFlashOn, p.Label, statIconDp),
		BoltOn:   mk(icons.ImageFlashOn, p.Volt, boltDp),
		BoltOff:  mk(icons.ImageFlashOn, tokens.Disabled(p.Label), boltDp),
		Flame:    mk(icons.SocialWhatsHot, p.Label, statIconDp),
		FlameHot: mk(icons.SocialWhatsHot, p.Danger, statIconDp),
		Battery:  mk(icons.DeviceBatteryChargingFull, p.Label, statIconDp),
		Flare:    mk(icons.ImageFlare, p.Label, statIconDp),
		Clock:    mk(icons.DeviceAccessTime, p.Label, statIconDp),
		PowerOn:  mk(icons.ActionPowerSettingsNew, p.Volt, 28),
		PowerOff: mk(icons.ActionPowerSettingsNew, p.Label, 28),
	}
}

// widgetSet is the page's component slots by name. Every component is
// constructed ONCE at subscription scope (llms.txt rule 2 — a TextField
// rebuilt per model emission would drop the user's typing on every poll),
// collected by one variadic CombineLatest, and addressed here by key.
type widgetSet struct {
	list  []layout.Widget
	index map[string]int
}

func (ws widgetSet) get(key string) layout.Widget {
	if i, ok := ws.index[key]; ok && i < len(ws.list) {
		return ws.list[i]
	}
	return func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }
}

// registry collects the slot observables during construction.
type registry struct {
	obs   []rx.Observable[layout.Widget]
	index map[string]int
}

func (r *registry) add(key string, o rx.Observable[layout.Widget]) {
	r.index[key] = len(r.obs)
	r.obs = append(r.obs, o)
}

// pageState is the frame-time snapshot the static tab Content closures read.
// patterns/tabs captures Content widgets at construction (a static slice),
// so the closures cannot receive the model in-band; they read this cell,
// which the combined map below stores synchronously BEFORE the emitted
// widget can be laid out — a frame never renders a stale snapshot (the
// feeds/detail.go recipe).
type pageState struct {
	t  themed
	m  Model
	ws widgetSet
	tp *tips
}

// buildLayers returns the layer-builder the theme window renders: a backdrop
// layer and the content layer.
func buildLayers(modelObs rx.Observable[Model]) func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
	return func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
		return []rx.Observable[layout.Widget]{
			BackdropLayer(th),
			ContentLayer(th, modelObs),
		}
	}
}

// ContentLayer renders the page. modelObs is subscribed exactly three times
// — the CombineLatest4 below, the tab strip's Selected, and the override
// modal's Open — which is what modelObsConsumers in main.go counts.
func ContentLayer(th rx.Observable[theme.Theme], modelObs rx.Observable[Model]) rx.Observable[layout.Widget] {
	// The frame-time snapshot (stored by the final combine below); declared
	// up here because the hand-rolled controls read it.
	var stateCell atomic.Value
	loadState := func() (pageState, bool) {
		st, ok := stateCell.Load().(pageState)
		return st, ok
	}

	// One text cell per field: OnChange stores the latest text, the button
	// that acts on it reads the cell. The field owns the editor state; the
	// model never echoes it back.
	cells := map[string]*atomic.Value{}
	cell := func(key string) *atomic.Value {
		if c, ok := cells[key]; ok {
			return c
		}
		var c atomic.Value
		c.Store("")
		cells[key] = &c
		return &c
	}
	text := func(key string) string {
		s, _ := cell(key).Load().(string)
		return s
	}

	tp := newTips()
	reg := &registry{index: map[string]int{}}
	// Return in a field submits it through onSubmit (the field's own Set,
	// or the editor's Save); the component clears the editor afterwards,
	// so the cell is cleared with it.
	// editors are the fields' editors by key, for clearing a whole form
	// at once; the component hands them out through FocusTag.
	editors := map[string]*widget.Editor{}
	field := func(key, placeholder string, onSubmit func(gtx layout.Context, s string)) {
		c := cell(key)
		reg.add(key+".field", input.TextField(th, input.TextFieldProps{
			Placeholder: placeholder,
			Description: placeholder,
			FocusTag: func(tag event.Tag) {
				if e, ok := tag.(*widget.Editor); ok {
					editors[key] = e
				}
			},
			OnChange: func(_ layout.Context, s string) {
				c.Store(s)
			},
			Submit: true,
			OnSubmit: func(gtx layout.Context, s string) {
				c.Store(s)
				onSubmit(gtx, s)
				c.Store("")
			},
		}))
	}
	setButton := func(key string, mk func(string) any) {
		reg.add(key+".set", button.Button(th, button.Props{
			Label:    "Set",
			Emphasis: button.Tonal,
			OnClick: func(gtx layout.Context) {
				mvu.MessageOp{Message: mk(text(key))}.Add(gtx.Ops)
			},
		}))
	}

	// The Monitor's Set button: opens the active preset in the editor.
	reg.add("set.active", button.Button(th, button.Props{
		Label: "Set", Emphasis: button.Tonal, Message: EditActivePreset{},
	}))
	// Device fields.
	for _, f := range deviceFields {
		f := f
		field(f.spec().Key, f.spec().Unit, func(gtx layout.Context, s string) {
			mvu.MessageOp{Message: ApplyDevice{F: f, Text: s}}.Add(gtx.Ops)
		})
		setButton(f.spec().Key, func(s string) any { return ApplyDevice{F: f, Text: s} })
	}
	// Preset editor fields: one Save reads them all — from the button or
	// from Return in any of them, so every typed field lands together.
	savePreset := func(gtx layout.Context, stay bool) {
		st, ok := loadState()
		if !ok {
			return
		}
		texts := map[Field]string{}
		for _, f := range presetFields {
			texts[f] = text("p." + f.spec().Key)
		}
		mvu.MessageOp{Message: SavePresetEdit{N: st.m.EditPreset, Texts: texts, Stay: stay}}.Add(gtx.Ops)
	}
	for _, f := range presetFields {
		field("p."+f.spec().Key, f.spec().Unit, func(gtx layout.Context, _ string) { savePreset(gtx, true) })
	}
	// clearPresetFields empties every editor field — on the frame after
	// the model counts a save accepted or the editor closed, so a rejected
	// save keeps what was typed.
	clearsSeen := 0
	clearPresetFields := func(m Model) {
		if m.EditClears == clearsSeen {
			return
		}
		clearsSeen = m.EditClears
		for _, f := range presetFields {
			key := "p." + f.spec().Key
			if e, ok := editors[key]; ok {
				e.SetText("")
			}
			cell(key).Store("")
		}
	}
	reg.add("p.save", button.Button(th, button.Props{
		Label: "Save", OnClick: func(gtx layout.Context) { savePreset(gtx, false) },
	}))
	for i := 0; i <= 9; i++ {
		reg.add(fmt.Sprintf("edit.%d", i), button.Button(th, button.Props{
			Label: "Edit", Emphasis: button.Ghost, Message: EditPreset{N: i},
		}))
		reg.add(fmt.Sprintf("recall.%d", i), button.Button(th, button.Props{
			Label: "Recall", Emphasis: button.Tonal, Message: RecallPreset{N: i},
		}))
	}

	// Hand-rolled controls: the header's power button and the toggle
	// switches. Each owns one clickable at subscription scope and reads
	// state and colours from the snapshot.
	var powerClick widget.Clickable
	reg.add("power", rx.Of(powerButton(loadState, &powerClick)))
	switchClicks := make([]widget.Clickable, switchCount)
	for s := Switch(0); s < switchCount; s++ {
		s := s
		reg.add(switchSpecs[s].Key, rx.Of(switchWidget(loadState, &switchClicks[s], s)))
	}

	reg.add("clear", button.Button(th, button.Props{
		Label: "Clear", Emphasis: button.Tonal, Message: ClearProtection{},
	}))
	reg.add("demo", button.Button(th, button.Props{
		Label: "Try demo mode", Emphasis: button.Tonal, Message: EnterDemo{},
	}))

	// The override dialog's footer actions. Caller-owned clickables put
	// them in the modal's Tab cycle; both stand on the dialog's level-2
	// surface.
	var lvpCancelClick, lvpConfirmClick widget.Clickable
	reg.add("lvp.cancel", button.Button(th, button.Props{
		Label: "Cancel", Emphasis: button.Ghost, Level: tokens.Level2,
		Clickable: &lvpCancelClick, Message: DismissLVP{},
	}))
	reg.add("lvp.confirm", button.Button(th, button.Props{
		Label: "Set anyway", Level: tokens.Level2,
		Clickable: &lvpConfirmClick, Message: ConfirmLVP{},
	}))
	widgets := rx.CombineLatest(reg.obs...)
	index := reg.index

	hov := &hoverState{}

	// slotW hands a slot to a static component slot (the modal's Actions),
	// read from the frame-time snapshot.
	slotW := func(key string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			st, ok := loadState()
			if !ok {
				return layout.Dimensions{}
			}
			return st.ws.get(key)(gtx)
		}
	}

	// The LVP override decision (llms.txt dialog grammar): setting the
	// input cutoff above the live input trips protection and cuts the
	// output the moment it lands, so the write waits for this. Destructive
	// puts Return on Cancel; Escape cancels; the backdrop is inert.
	lvpBody := func(gtx layout.Context) layout.Dimensions {
		st, ok := loadState()
		if !ok {
			return layout.Dimensions{}
		}
		m, t := st.m, st.t
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(textLine(t.typ, t.typ.Body, t.palette.Text,
				fmt.Sprintf("An input cutoff of %.2f V is above the live input (%.2f V).",
					m.LVPPending, m.LVPPendingVIn))),
			vgap(4),
			layout.Rigid(textLine(t.typ, t.typ.Body, t.palette.Danger,
				"Writing it trips LVP protection as soon as the output runs, cutting it off.")),
		)
	}
	lvpModal := modal.Modal(th, modal.Props{
		Open:  rx.Map(modelObs, func(m Model) bool { return m.LVPConfirmOpen }),
		Title: "Input cutoff above live input",
		Body:  lvpBody,
		Actions: []layout.Widget{
			fixed(100, slotW("lvp.cancel")),
			fixed(130, slotW("lvp.confirm")),
		},
		ActionFocusTags: []event.Tag{&lvpCancelClick, &lvpConfirmClick},
		Decision: &modal.Decision{
			Destructive: true,
			Confirm: func(gtx layout.Context) {
				mvu.MessageOp{Message: ConfirmLVP{}}.Add(gtx.Ops)
			},
			Cancel: func(gtx layout.Context) {
				mvu.MessageOp{Message: DismissLVP{}}.Add(gtx.Ops)
			},
		},
	})

	// Scroll state for the list-shaped tabs, one per tab.
	presetsList := &layout.List{Axis: layout.Vertical}
	deviceList := &layout.List{Axis: layout.Vertical}

	tabsObs := tabs.Tabs(th, tabs.Props{
		Tabs: []tabs.Tab{
			{Label: "Monitor", Content: monitorContent(loadState, hov)},
			{Label: "Presets", Content: listContent(loadState, presetsList, func(t themed, m Model, ws widgetSet) []layout.Widget {
				clearPresetFields(m)
				return presetsRows(t, m, ws, tp)
			})},
			{Label: "Device", Content: deviceContent(loadState, deviceList)},
		},
		Selected: rx.Map(modelObs, func(m Model) int {
			for i, s := range tabScreens {
				if m.Screen == s {
					return i
				}
			}
			return 0
		}),
		OnSelect: func(gtx layout.Context, idx int) {
			if idx < 0 || idx >= len(tabScreens) {
				return
			}
			screen := tabScreens[idx]
			// Clicking Presets again while the editor is open returns to
			// the list, discarding what was typed — the tab is the editor's
			// cancel.
			if st, ok := loadState(); ok && screen == "presets" && st.m.Screen == "presets" &&
				st.m.EditPreset != noEdit {
				mvu.MessageOp{Message: EditPreset{N: noEdit}}.Add(gtx.Ops)
				return
			}
			mvu.MessageOp{Message: SetScreen{Screen: screen}}.Add(gtx.Ops)
		},
	})

	themes := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[themed] {
		return rx.Map(rx.CombineLatest2(t.Color, t.Typography),
			func(n rx.Tuple2[tokens.ColorTokens, tokens.Typography]) themed {
				p := PaletteFrom(n.First)
				return themed{palette: p, typ: TypeFrom(n.Second), ic: iconsFrom(p)}
			})
	})

	// The two overlay widgets: the tab strip (content) and the modal above
	// it. Same element type, so one variadic combine carries both.
	overlays := rx.CombineLatest(tabsObs, lvpModal)

	return rx.Map(rx.CombineLatest4(themes, modelObs, widgets, overlays),
		func(n rx.Tuple4[themed, Model, []layout.Widget, []layout.Widget]) layout.Widget {
			ws := widgetSet{list: n.Third, index: index}
			stateCell.Store(pageState{t: n.First, m: n.Second, ws: ws, tp: tp})
			content := desktop.CapTop(desktop.TopInset, Page(n.First, n.Second, ws, n.Fourth[0]))
			dialog := n.Fourth[1]
			return func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Max
				tp.winW = size.X
				content(gtx)
				// The dialog and its scrim lie over the whole window,
				// strip included — a scrim with a hole isolates nothing.
				dialog(gtx)
				return layout.Dimensions{Size: size}
			}
		})
}

// Page lays the whole window: the header band, the tab strip with its
// content panel taking the remaining height, and the notice line under it.
func Page(t themed, m Model, ws widgetSet, tabsW layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(Padding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			rows := []layout.FlexChild{
				layout.Rigid(headerRow(t, m, ws)),
				vgap(12),
				layout.Flexed(1, tabsW),
			}
			if m.Notice != "" {
				rows = append(rows, vgap(8),
					layout.Rigid(textLine(t.typ, t.typ.Small, t.palette.Label, m.Notice)))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	}
}

// monitorContent is the Monitor tab: the readout block, centered in the
// panel on the stat line's width, with the history charts spanning the
// full width underneath.
func monitorContent(load func() (pageState, bool), hov *hoverState) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		st, ok := load()
		if !ok {
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}
		t, m, ws := st.t, st.m, st.ws
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if !m.HaveR {
				rows := append([]layout.FlexChild{vgap(10)}, monitorFallback(t, m, ws)...)
				return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = 0
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				vgap(10),
				layout.Rigid(monitorBlock(t, m, ws)),
				vgap(16),
				layout.Flexed(1, chartPanels(t, m, hov)),
			)
		})
	}
}

// listContent adapts a row builder into a scrolling tab Content widget:
// the rows flow through a layout.List whose scroll state the caller keeps
// at subscription scope.
func listContent(load func() (pageState, bool), list *layout.List, build func(themed, Model, widgetSet) []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		st, ok := load()
		if !ok {
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}
		rows := append([]layout.Widget{vspace(10)}, build(st.t, st.m, st.ws)...)
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return list.Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
				return rows[i](gtx)
			})
		})
	}
}

// powerButton is the header's output toggle: the power glyph, lit in the
// accent ink while the output is on, emitting ToggleOutput on click. The
// clickable lives at subscription scope; state and colours arrive through
// the frame-time snapshot. Affordance is the standard recipe: a pointer
// cursor over the target, a circular hover/press wash one step off the
// ground, and padding that grows the hit area past the glyph.
func powerButton(load func() (pageState, bool), click *widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		st, ok := load()
		if !ok {
			return layout.Dimensions{}
		}
		if click.Clicked(gtx) {
			mvu.MessageOp{Message: ToggleOutput{}}.Add(gtx.Ops)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			sz := gtx.Dp(28)
			pad := gtx.Dp(6)
			total := sz + 2*pad
			box := image.Rectangle{Max: image.Pt(total, total)}
			if click.Hovered() || click.Pressed() {
				paint.FillShape(gtx.Ops, st.t.palette.Hairline,
					clip.UniformRRect(box, total/2).Op(gtx.Ops))
			}
			// The content is replayed inside the Clickable's own clip
			// area, so a bare Add binds the cursor to exactly that area —
			// the same mechanism the library buttons use. A second nested
			// input area here would occlude the clickable underneath it.
			pointer.CursorPointer.Add(gtx.Ops)
			tr := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
			icon := st.t.ic.PowerOff
			if st.m.R.On {
				icon = st.t.ic.PowerOn
			}
			igtx := gtx
			igtx.Constraints = layout.Exact(image.Pt(sz, sz))
			icon(igtx)
			tr.Pop()
			return layout.Dimensions{Size: box.Max}
		})
	}
}

// switchWidget is a toggle switch for one boolean setting: a pill track
// with a knob, the accent ink when on and the hairline when off, emitting
// SetSwitch with the flipped value on click. Same affordance recipe as the
// power button.
func switchWidget(load func() (pageState, bool), click *widget.Clickable, s Switch) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		st, ok := load()
		if !ok {
			return layout.Dimensions{}
		}
		on := s.state(st.m)
		if click.Clicked(gtx) {
			mvu.MessageOp{Message: SetSwitch{S: s, On: !on}}.Add(gtx.Ops)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			p := st.t.palette
			w, h := gtx.Dp(40), gtx.Dp(22)
			pad := gtx.Dp(4)
			box := image.Rectangle{Max: image.Pt(w+2*pad, h+2*pad)}
			if click.Hovered() || click.Pressed() {
				paint.FillShape(gtx.Ops, p.Hairline, clip.UniformRRect(box, box.Max.Y/2).Op(gtx.Ops))
			}
			pointer.CursorPointer.Add(gtx.Ops)
			track := image.Rect(pad, pad, pad+w, pad+h)
			trackCol, knobCol := p.Hairline, p.Label
			knobX := track.Min.X + gtx.Dp(3)
			if on {
				trackCol, knobCol = p.Volt, p.Backdrop
				knobX = track.Max.X - gtx.Dp(3) - (h - 2*gtx.Dp(3))
			}
			paint.FillShape(gtx.Ops, trackCol, clip.UniformRRect(track, h/2).Op(gtx.Ops))
			kd := h - 2*gtx.Dp(3)
			knob := image.Rect(knobX, track.Min.Y+gtx.Dp(3), knobX+kd, track.Min.Y+gtx.Dp(3)+kd)
			paint.FillShape(gtx.Ops, knobCol, clip.UniformRRect(knob, kd/2).Op(gtx.Ops))
			return layout.Dimensions{Size: box.Max}
		})
	}
}

// headerRow is the band both tabs share: the title with the connection
// status right under it on the left, the output cluster — the mini
// bolt+ON/OFF state block and the power toggle — on the right. While a
// protection is tripped, the trip badge overlays the row centered on the
// WINDOW (not the leftover space between the side blocks), and the way out
// stands right under it, centered the same way: the recovery hint for LVP —
// which heals itself and takes no Clear — or the Clear button for the trips
// that latch.
func headerRow(t themed, m Model, ws widgetSet) layout.Widget {
	statusCol := t.palette.Label
	if !m.Online {
		statusCol = t.palette.Danger
	}
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(textLine(t.typ, t.typ.Title, t.palette.Text, "XY-SK150")),
					layout.Rigid(textLine(t.typ, t.typ.Small, statusCol, m.Status)),
				)
			}),
			layout.Flexed(1, spacer),
		}
		if m.HaveR {
			children = append(children,
				layout.Rigid(outputCluster(t, m.R)),
				hgap(10),
				layout.Rigid(ws.get("power")),
			)
		}
		row := func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				}),
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, tripBadge(t, m.R))
				}),
			)
		}
		if !m.HaveR || m.R.Protect == 0 {
			return row(gtx)
		}
		hint := textLine(t.typ, t.typ.Small, t.palette.Label,
			fmt.Sprintf("lower the input cutoff below the %.2f V input to recover", m.R.VIn))
		if m.R.Protect != protectLVP {
			hint = func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(textLine(t.typ, t.typ.Small, t.palette.Label,
						"the trip holds the output off until cleared")),
					hgap(12),
					layout.Rigid(fixed(90, ws.get("clear"))),
				)
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(row),
			vgap(6),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Center.Layout(gtx, hint)
			}),
		)
	}
}

// monitorFallback is the Monitor tab before the first reading: the demo
// offer while no device is reachable, or a waiting line.
func monitorFallback(t themed, m Model, ws widgetSet) []layout.FlexChild {
	p, typ := t.palette, t.typ
	if !m.Online && !m.Demo {
		return []layout.FlexChild{
			layout.Rigid(textLine(typ, typ.Title, p.Text, "No SK150 connected")),
			vgap(6),
			layout.Rigid(textLine(typ, typ.Body, p.Label, m.Status)),
			vgap(14),
			layout.Rigid(fixed(160, ws.get("demo"))),
			vgap(6),
			layout.Rigid(textLine(typ, typ.Small, p.Label,
				"Explore the app with a simulated SK150. Restart the app to use real hardware again.")),
		}
	}
	return []layout.FlexChild{
		layout.Rigid(textLine(typ, typ.Body, p.Label, "waiting for the first reading…")),
	}
}

// monitorBlock is the live readout block: one column as wide as the stat
// line (input, temperature, charge, energy, on-time), centered in the
// panel. The active-preset badge and the Set button share the top line at
// the column's edges, the three readouts are centered at their natural
// width beneath it, and the stat line closes the block.
func monitorBlock(t themed, m Model, ws widgetSet) layout.Widget {
	p, typ := t.palette, t.typ
	r := m.R

	tempIcon, tempCol := t.ic.Flame, p.Text
	if r.TempIn >= 60 {
		tempIcon, tempCol = t.ic.FlameHot, p.Danger
	}
	charge := fmt.Sprintf("%d mAh", r.MilliAh)
	if r.MilliAh >= 1000 {
		charge = fmt.Sprintf("%.2f Ah", float64(r.MilliAh)/1000)
	}
	energy := fmt.Sprintf("%d mWh", r.MilliWh)
	if r.MilliWh >= 1000 {
		energy = fmt.Sprintf("%.1f Wh", float64(r.MilliWh)/1000)
	}
	statLine := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(statItem(typ, t.ic.Bolt, fmt.Sprintf("IN %.2f V", r.VIn), p.Text, 0)),
			hgap(12),
			layout.Rigid(statItem(typ, tempIcon, fmt.Sprintf("%.1f °C", r.TempIn), tempCol, 4)),
			hgap(12),
			layout.Rigid(statItem(typ, t.ic.Battery, charge, p.Text, 4)),
			hgap(12),
			layout.Rigid(statItem(typ, t.ic.Flare, energy, p.Text, 4)),
			hgap(12),
			layout.Rigid(statItem(typ, t.ic.Clock,
				fmt.Sprintf("%02d:%02d:%02d", r.Hours, r.Mins, r.Secs), p.Text, 4)),
		)
	}
	active := activeGroup(m)

	return func(gtx layout.Context) layout.Dimensions {
		// The stat line at its natural width sets the column; it is
		// recorded once here and replayed in place below.
		macro := op.Record(gtx.Ops)
		sgtx := gtx
		sgtx.Constraints.Min.X = 0
		statDims := statLine(sgtx)
		statCall := macro.Stop()
		statW := func(gtx layout.Context) layout.Dimensions {
			statCall.Add(gtx.Ops)
			return statDims
		}
		col := min(statDims.Size.X, gtx.Constraints.Max.X)

		readW := min(readoutWidth(gtx, t, r), col)
		centered := func(w layout.Widget) layout.Widget {
			return func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = readW
					gtx.Constraints.Max.X = readW
					return w(gtx)
				})
			}
		}
		rows := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(presetBadge(t, active)),
					layout.Flexed(1, spacer),
					layout.Rigid(fixed(72, ws.get("set.active"))),
				)
			}),
			vgap(8),
			layout.Rigid(centered(voltRow(t, r))),
			vgap(2),
			layout.Rigid(centered(ampRow(t, r))),
			vgap(2),
			layout.Rigid(centered(wattRow(t, r))),
			vgap(14),
			layout.Rigid(statW),
		}
		// Centered in the panel at the column's width.
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = col
			gtx.Constraints.Max.X = col
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	}
}

// presetsRows is the memory screen: the nine stored profiles with Edit and
// Recall, or — while one is open — its editor with every field.
func presetsRows(t themed, m Model, ws widgetSet, tp *tips) []layout.Widget {
	p, typ := t.palette, t.typ
	if !m.HavePresets {
		return []layout.Widget{textLine(typ, typ.Body, p.Label, "reading memory slots…")}
	}
	if n := m.EditPreset; isGroup(n) {
		pr := m.Presets[n]
		rows := []layout.Widget{
			presetBadge(t, n),
			vspace(6),
			textLine(typ, typ.Small, p.Label, "Blank keeps the stored value. Return writes whole group, Presets tab cancels, Save writes and returns."),
			vspace(10),
		}
		// Two columns filled row by row — Tab follows layout order, so it
		// walks the fields in reading order — with the input cutoff and
		// the power-on switch on lines of their own: the setpoints, then
		// LVP, then the output protections and budgets, then POn.
		cell := func(col int, f Field) layout.Widget {
			key := f.spec().Key
			return tp.wrap(t, "p."+key, hints[key], col, presetColWidth,
				compactField(t, f, pr.Value(f), ws.get("p."+key+".field")))
		}
		pon := switchSpecs[SwPresetPowerOn]
		blank := vspace(0)
		cells := []layout.Widget{
			cell(0, FVSet), cell(1, FISet),
			cell(0, FLVP), blank,
			cell(0, FOVP), cell(1, FOCP),
			cell(0, FOPP), cell(1, FOTP),
			cell(0, FOHPH), cell(1, FOHPM),
			cell(0, FOAH), cell(1, FOWH),
			tp.wrap(t, pon.Key, hints[pon.Key], 0, presetColWidth,
				compactSwitch(t, textLine(typ, typ.Body, p.Text, pon.Short), pr.PowerOn, ws.get(pon.Key))), blank,
		}
		left, right := []layout.Widget{}, []layout.Widget{}
		for i, c := range cells {
			if i%2 == 0 {
				left = append(left, c)
			} else {
				right = append(right, c)
			}
		}
		rows = append(rows, gridRows(presetColWidth, left, right)...)
		rows = append(rows,
			vspace(10),
			// Save is right-aligned to the grid, not the window.
			fixed(2*presetColWidth+compactGap, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, spacer),
					layout.Rigid(fixed(90, ws.get("p.save"))),
				)
			}),
			vspace(10),
		)
		return rows
	}
	rows := []layout.Widget{
		textLine(typ, typ.Title, p.Text, "Memory slots"),
		vspace(4),
		textLine(typ, typ.Small, p.Label,
			"Recall makes a slot the active profile and switches the output off until you turn it on."),
		vspace(8),
		textLine(typ, typ.Table, p.Label, presetTableHeader),
		vspace(3),
	}
	for n := 0; n <= 9; n++ {
		txt := presetTableRow(n, m.Presets[n])
		col := p.Text
		if n == activeGroup(m) {
			col = p.Volt // the active profile
		}
		edit, recall := ws.get(fmt.Sprintf("edit.%d", n)), ws.get(fmt.Sprintf("recall.%d", n))
		rows = append(rows,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(textLine(typ, typ.Table, col, txt)),
					layout.Flexed(1, spacer),
					layout.Rigid(fixed(58, edit)),
					hgap(4),
					layout.Rigid(fixed(76, recall)),
				)
			},
			vspace(2),
		)
	}
	return rows
}

// presetTableHeader and presetTableRow lay one memory group out as a
// table row: every field, fixed columns, the compact mono face.
const presetTableHeader = "M    V      A   LVP   OVP   OCP   OPP  °C  h:mm    Ah     Wh  PWR"

func presetTableRow(n int, p Preset) string {
	pwr := "off"
	if p.PowerOn {
		pwr = "on"
	}
	return fmt.Sprintf("M%d %5.2f %5.3f %5.2f %5.2f %5.3f %5.1f %3.0f %02d:%02d %6.2f %6.1f  %s",
		n, p.VSet, p.ISet, p.LVP, p.OVP, p.OCP, p.OPP, p.OTP, p.OHPH, p.OHPM,
		p.Value(FOAH), p.Value(FOWH), pwr)
}

// deviceRows is the Device tab: the device-wide settings as a two-column
// grid in the panel's own names — input and charging on the left, panel
// behaviour on the right — each numeric cell with its own Set.
func deviceRows(t themed, m Model, ws widgetSet, tp *tips) []layout.Widget {
	p, typ := t.palette, t.typ
	if !m.HaveD {
		return []layout.Widget{textLine(typ, typ.Body, p.Label, "reading the device settings…")}
	}
	sw := func(col int, s Switch, on bool) layout.Widget {
		spec := switchSpecs[s]
		return tp.wrap(t, spec.Key, hints[spec.Key], col, deviceColWidth,
			compactSwitch(t, textLine(typ, typ.Body, p.Text, spec.Short), on, ws.get(spec.Key)))
	}
	fr := func(col int, f Field) layout.Widget {
		spec := f.spec()
		return tp.wrap(t, spec.Key, hints[spec.Key], col, deviceColWidth,
			compactCell(t, textLine(typ, typ.Body, p.Text, spec.Short),
				fmt.Sprintf(spec.Format, m.D.Value(f)), ws.get(spec.Key+".field"), ws.get(spec.Key+".set")))
	}
	d := m.D
	left := []layout.Widget{sw(0, SwMPPT, d.MPPTOn), fr(0, FMPPTPct), fr(0, FBatCutoff), sw(0, SwCP, d.CPOn), fr(0, FCPWatts)}
	right := []layout.Widget{sw(1, SwBeeper, d.Beeper), sw(1, SwFahrenheit, d.Fahrenheit), sw(1, SwLock, d.KeyLock), fr(1, FBacklight), fr(1, FSleep)}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(fixed(deviceColWidth, textLine(typ, typ.Title, p.Text, "Input and charging"))),
				hgap(compactGap),
				layout.Rigid(fixed(deviceColWidth, textLine(typ, typ.Title, p.Text, "Panel"))),
			)
		},
		vspace(8),
	}
	rows = append(rows, gridRows(deviceColWidth, left, right)...)
	return append(rows, vspace(8))
}

// hints are the explanations behind every settings cell, keyed by the
// control's slot key — what the panel's terse name stands for and what
// the value does.
var hints = map[string]string{
	// Memory group fields.
	"vset":    "Voltage setpoint — the output voltage held in CV mode",
	"iset":    "Current limit — the output current held in CC mode",
	"lvp":     "Low-voltage protection — output off when the input falls below this",
	"ovp":     "Over-voltage protection — output off if the output exceeds this",
	"ocp":     "Over-current protection — output off if the output current exceeds this",
	"opp":     "Over-power protection — output off if the output power exceeds this",
	"otp":     "Over-temperature protection — output off above this internal temperature",
	"ohph":    "Output time limit, hours — with the minutes, output off after this long on; 0:00 = no limit. Clear resets the clock",
	"ohpm":    "Output time limit, minutes — with the hours, output off after this long on; 0:00 = no limit. Clear resets the clock",
	"oah":     "Charge limit — output off once this many Ah have been delivered; 0 = off",
	"owh":     "Energy limit — output off once this many Wh have been delivered; 0 = off",
	"sw.pini": "Power-on state — whether the output comes on by itself at power-up while this preset is active",
	"sw.ini":  "Power-on state — whether the output comes on by itself at power-up",
	// Device-wide settings.
	"sw.mppt": "MPPT — solar tracking: holds the input at a share of the panel's open-circuit voltage",
	"mppt":    "MPPT set point — the share of open-circuit voltage to hold the input at",
	"batcut":  "Battery-full cutoff — output off once charge current drops below this; 0 = off",
	"sw.cp":   "Constant power — caps the output power, folding the current back instead of tripping",
	"cpw":     "Constant power set point — the wattage the output is held to",
	"sw.beep": "Beeper — key clicks and the alarm on a protection trip",
	"sw.fahr": "Show temperatures in °F instead of °C",
	"sw.lock": "Key lock — the panel buttons ignore presses",
	"bled":    "Backlight level, 0–5",
	"sleep":   "Screen sleep — blank the display after this many minutes; 0 = never",
}

// deviceFooter is the Device tab's status line, pinned at the bottom: the
// link settings this app leaves alone.
func deviceFooter(t themed, m Model) layout.Widget {
	if !m.HaveD {
		return vspace(0)
	}
	return textLine(t.typ, t.typ.Small, t.palette.Label,
		fmt.Sprintf("Link: %d baud, Modbus address %d — changing either needs a power cycle, so this app leaves them alone.",
			baudRate, slaveAddr))
}

// deviceContent is the Device tab: the scrolling settings grid with the
// status footer pinned beneath it.
func deviceContent(load func() (pageState, bool), list *layout.List) layout.Widget {
	grid := listContent(load, list, func(t themed, m Model, ws widgetSet) []layout.Widget {
		st, _ := load()
		return deviceRows(t, m, ws, st.tp)
	})
	return func(gtx layout.Context) layout.Dimensions {
		st, ok := load()
		if !ok {
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, grid),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx,
					deviceFooter(st.t, st.m))
			}),
		)
	}
}

// gridRows pairs two equal-length columns of cells into rows of the
// compact grid.
func gridRows(colW unit.Dp, left, right []layout.Widget) []layout.Widget {
	rows := []layout.Widget{}
	for i := range left {
		l, r := left[i], right[i]
		rows = append(rows,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(fixed(colW, l)),
					hgap(compactGap),
					layout.Rigid(fixed(colW, r)),
				)
			},
			vspace(4))
	}
	return rows
}

// The compact grid's cell geometry: label, field (with its Set on the
// Device tab), and a fixed value column, so the grid's right edge is the
// values' right edge and the Save button can align to it.
const (
	compactLabelWidth unit.Dp = 64
	compactValueWidth unit.Dp = 80
	compactGap        unit.Dp = 16
	// The preset editor's column: label + 120 field + 10 + value.
	presetColWidth = compactLabelWidth + 120 + 10 + compactValueWidth
	// The Device tab's column: label + 104 field + 6 + 56 Set + 10 + value.
	deviceColWidth = compactLabelWidth + 104 + 6 + 56 + 10 + compactValueWidth
)

// compactField is one cell of the preset editor grid: the device's own
// name for the field, the text field, and the stored value beside it.
func compactField(t themed, f Field, current float64, field layout.Widget) layout.Widget {
	spec := f.spec()
	return compactCell(t, textLine(t.typ, t.typ.Body, t.palette.Text, spec.Short), fmt.Sprintf(spec.Format, current), field, nil)
}

// compactCell is one grid cell: short label, text field, an optional Set
// that applies it, and the current value.
func compactCell(t themed, label layout.Widget, current string, field, apply layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(fixed(compactLabelWidth, label)),
		}
		if apply == nil {
			children = append(children, layout.Rigid(fixed(120, field)))
		} else {
			children = append(children,
				layout.Rigid(fixed(104, field)),
				hgap(6),
				layout.Rigid(fixed(56, apply)))
		}
		children = append(children, hgap(10),
			layout.Rigid(fixed(compactValueWidth, textLine(t.typ, t.typ.Body, t.palette.Label, current))))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// compactSwitch is the grid's switch cell, same columns as compactField.
func compactSwitch(t themed, label layout.Widget, on bool, sw layout.Widget) layout.Widget {
	state := "off"
	if on {
		state = "on"
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(fixed(compactLabelWidth, label)),
			layout.Rigid(sw),
			hgap(10),
			layout.Rigid(textLine(t.typ, t.typ.Body, t.palette.Label, state)),
		)
	}
}

// fieldRow is one labelled control line: caption, text field, an optional
// Set button, and the device's current value.
func fieldRow(t themed, label, current string, field, apply layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(fixed(LabelWidth, textLine(t.typ, t.typ.Body, t.palette.Text, label))),
			layout.Rigid(fixed(FieldWidth, field)),
		}
		if apply != nil {
			children = append(children, hgap(8), layout.Rigid(fixed(72, apply)))
		}
		children = append(children, hgap(12),
			layout.Rigid(textLine(t.typ, t.typ.Body, t.palette.Label, current)))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// switchRow is one labelled toggle line: caption, the switch, its state.
func switchRow(t themed, label string, on bool, sw layout.Widget) layout.Widget {
	state := "off"
	if on {
		state = "on"
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(fixed(LabelWidth, textLine(t.typ, t.typ.Body, t.palette.Text, label))),
			layout.Rigid(sw),
			hgap(10),
			layout.Rigid(textLine(t.typ, t.typ.Body, t.palette.Label, state)),
		)
	}
}

// statItem is one dashboard stat: a glyph beside its value — the icon
// carries the meaning, no caption.
func statItem(typ Type, icon layout.Widget, value string, valCol color.NRGBA, gap unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(icon),
			hgap(gap),
			layout.Rigid(textLine(typ, typ.Small, valCol, value)),
		)
	}
}

// textLine draws one line of text at its natural size.
func textLine(typ Type, style textdraw.TextStyle, col color.NRGBA, txt string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		sz := textdraw.MeasureText(gtx, typ.Shaper, style, txt)
		sz.X = min(sz.X, gtx.Constraints.Max.X)
		rect := image.Rectangle{Max: sz}
		textdraw.FillText(gtx, typ.Shaper, style, rect, 0, 0.5, col, txt)
		return layout.Dimensions{Size: rect.Max}
	}
}

// fixed pins a child to a width in dp (clamped to the space available), so
// fill-width components sit in columns.
func fixed(dp unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		width := min(gtx.Dp(dp), gtx.Constraints.Max.X)
		gtx.Constraints.Max.X = width
		gtx.Constraints.Min.X = width
		dims := w(gtx)
		dims.Size.X = width
		return dims
	}
}

func spacer(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
}

// vspace is a vertical gap as a plain widget, for list rows.
func vspace(dp unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(0, gtx.Dp(dp))}
	}
}

func vgap(dp unit.Dp) layout.FlexChild {
	return layout.Rigid(vspace(dp))
}

func hgap(dp unit.Dp) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Dp(dp), 0)}
	})
}
