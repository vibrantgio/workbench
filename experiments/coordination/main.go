// Package main is Experiments C1+C2: drag-drop, modal stacking, and tooltip
// arbitration via independent rx.Subject instances.
//
// C1 established drag-drop coordination. C2 adds two coordination patterns:
//
//   - Modal stacking: "Open Modal" opens a modal; "Open Nested" inside opens a
//     second; Escape pops the stack one level at a time.
//     Coordination signal: rx.Subject[ModalState]
//
//   - Tooltip arbitration: hovering a kanban card shows exactly one tooltip;
//     pointer exit suppresses it automatically.
//     Coordination signal: rx.Subject[TooltipState]
//
// Run with: go run ./experiments/coordination/
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/reactivego/rx"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/coordination"
)

// DragKind classifies each DragState Subject emission for logging.
type DragKind uint8

const (
	KindIdle   DragKind = iota // no drag active
	KindPress                  // initial press
	KindDrag                   // pointer moving while pressed
	KindDrop                   // pointer released
	KindCancel                 // gesture cancelled
)

func (k DragKind) String() string {
	return [...]string{"idle", "press", "drag", "drop", "cancel"}[k]
}

// DragState is the coordination signal broadcast via Subject during drag-drop (C1).
type DragState struct {
	Active    bool
	CardID    int
	SrcCol    int
	Pos       f32.Point
	Kind      DragKind
	SeqN      int64
	EmitFrame int64
}

// ModalState is the coordination signal for the modal stack (C2).
// Depth 0 = no modal; 1 = one modal open; 2 = nested modal.
type ModalState struct {
	Depth int
}

// TooltipState is the coordination signal for tooltip arbitration (C2).
// When Active, exactly the card identified by CardID displays a tooltip.
type TooltipState struct {
	Active     bool
	CardID     int
	CardBounds image.Rectangle // window-relative bounds of the hovered card
}

const numCards = 4

var cardLabels = [numCards]string{"Task A", "Task B", "Task C", "Task D"}

const (
	colHeaderH = 48
	cardH      = 52
	cardGap    = 8
	cardMargin = 10
	tooltipH   = 28
	tooltipW   = 170

	modalPanelW = 380
	modalPanelH = 230
)

var (
	colBgNormal  = color.NRGBA{R: 225, G: 228, B: 240, A: 255}
	colBgHover   = color.NRGBA{R: 180, G: 205, B: 255, A: 255}
	colHeaderCol = color.NRGBA{R: 50, G: 60, B: 130, A: 255}
	cardBg       = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	cardGhostBg  = color.NRGBA{R: 200, G: 215, B: 255, A: 200}
	white        = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	darkText     = color.NRGBA{R: 30, G: 30, B: 60, A: 255}

	tooltipBg = color.NRGBA{R: 30, G: 30, B: 30, A: 220}
	tooltipFg = color.NRGBA{R: 240, G: 240, B: 240, A: 255}

	backdropCol   = color.NRGBA{R: 0, G: 0, B: 0, A: 150}
	modalPanelCol = color.NRGBA{R: 250, G: 250, B: 252, A: 255}
	modalTitleCol = color.NRGBA{R: 20, G: 20, B: 70, A: 255}
	modalBodyCol  = color.NRGBA{R: 60, G: 60, B: 80, A: 255}
	modalHintCol  = color.NRGBA{R: 130, G: 130, B: 150, A: 255}
)

var colNames = [2]string{"To Do", "Done"}

// instr holds drag-subject instrumentation counters.
// All fields are read/written on the frame goroutine only.
type instr struct {
	frameN         int64
	emitN          int64
	emitsThisFrame int64
	prevSeqN       int64
}

// board holds all mutable widget state that must survive across frame renders.
// Widget closures emitted by each layer Observable close over the same *board.
type board struct {
	// C1: drag-drop
	drags  [numCards]gesture.Drag
	cols   [2][]int
	colW   int
	dragObs rx.Observer[DragState]
	th     *material.Theme
	instr  instr

	// C2: tooltip arbitration
	hovers     [numCards]gesture.Hover
	prevHover  int // -1 = no card hovered
	cardBounds [numCards]image.Rectangle
	tooltipObs rx.Observer[TooltipState]

	// C2: modal stacking
	openBtn     widget.Clickable // "Open Modal" button
	nestedBtn   widget.Clickable // "Open Nested" button inside first modal
	escTagModal bool             // focus anchor for Escape key routing
	modalObs    rx.Observer[ModalState]
}

func newBoard(
	dragObs rx.Observer[DragState],
	modalObs rx.Observer[ModalState],
	tooltipObs rx.Observer[TooltipState],
) *board {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Regular()))
	return &board{
		cols:       [2][]int{{0, 1}, {2, 3}},
		dragObs:    dragObs,
		th:         th,
		prevHover:  -1,
		modalObs:   modalObs,
		tooltipObs: tooltipObs,
	}
}

// send is the instrumented wrapper for DragState Subject emissions.
func (b *board) send(ds DragState) {
	b.instr.emitN++
	b.instr.emitsThisFrame++
	ds.SeqN = b.instr.emitN
	ds.EmitFrame = b.instr.frameN
	if b.instr.emitsThisFrame > 1 {
		log.Printf("[frame %4d] ⚠  buffer pressure: %d emissions this frame",
			b.instr.frameN, b.instr.emitsThisFrame)
	}
	log.Printf("[frame %4d] emit #%3d  %-6s  pos=(%.0f,%.0f)",
		b.instr.frameN, ds.SeqN, ds.Kind, ds.Pos.X, ds.Pos.Y)
	b.dragObs(ds, nil, false)
}

// makeKanbanLayer returns an Observable[layout.Widget] driven by DragState.
func makeKanbanLayer(b *board, dragObs rx.Observable[DragState]) rx.Observable[layout.Widget] {
	return rx.Map(dragObs.StartWith(DragState{}), func(ds DragState) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return b.kanbanLayout(gtx, ds)
		}
	})
}

// makeTooltipLayer returns an Observable[layout.Widget] driven by TooltipState.
func makeTooltipLayer(b *board, tooltipObs rx.Observable[TooltipState]) rx.Observable[layout.Widget] {
	return rx.Map(tooltipObs.StartWith(TooltipState{}), func(ts TooltipState) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return b.tooltipLayout(gtx, ts)
		}
	})
}

// makeModalLayer returns an Observable[layout.Widget] driven by ModalState.
func makeModalLayer(b *board, modalObs rx.Observable[ModalState]) rx.Observable[layout.Widget] {
	return rx.Map(modalObs.StartWith(ModalState{}), func(ms ModalState) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return b.modalLayout(gtx, ms)
		}
	})
}

func (b *board) kanbanLayout(gtx layout.Context, ds DragState) layout.Dimensions {
	b.instr.frameN++
	b.instr.emitsThisFrame = 0

	if ds.SeqN > 0 && ds.SeqN != b.instr.prevSeqN {
		lag := b.instr.frameN - ds.EmitFrame
		log.Printf("[frame %4d] render emission #%3d  lag=%d frame(s)  %-6s  pos=(%.0f,%.0f)",
			b.instr.frameN, ds.SeqN, lag, ds.Kind, ds.Pos.X, ds.Pos.Y)
		b.instr.prevSeqN = ds.SeqN
	}

	b.colW = gtx.Constraints.Max.X / 2
	if b.colW == 0 {
		b.colW = 1
	}

	layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.drawColumn(gtx, 0, ds)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.drawColumn(gtx, 1, ds)
		}),
	)

	// Ghost overlay for the dragged card.
	if ds.Active {
		cardW := b.colW - 2*cardMargin
		ghostOrigin := image.Point{
			X: int(ds.Pos.X) - cardW/2,
			Y: int(ds.Pos.Y) - cardH/2,
		}
		rec := op.Record(gtx.Ops)
		st := op.Offset(ghostOrigin).Push(gtx.Ops)
		drawRect(gtx.Ops, image.Point{X: cardW, Y: cardH}, cardGhostBg)
		lbl := material.Label(b.th, unit.Sp(13), cardLabels[ds.CardID])
		lbl.Color = darkText
		layout.Inset{Top: unit.Dp(6), Left: unit.Dp(8)}.Layout(gtx, lbl.Layout)
		st.Pop()
		op.Defer(gtx.Ops, rec.Stop())
	}

	// Tooltip arbitration: sample all hover states; emit to tooltipObs on change.
	hoveredCard := -1
	for i := 0; i < numCards; i++ {
		if b.hovers[i].Update(gtx.Source) {
			hoveredCard = i
		}
	}
	if hoveredCard != b.prevHover {
		b.prevHover = hoveredCard
		if hoveredCard >= 0 {
			b.tooltipObs(TooltipState{
				Active:     true,
				CardID:     hoveredCard,
				CardBounds: b.cardBounds[hoveredCard],
			}, nil, false)
		} else {
			b.tooltipObs(TooltipState{Active: false}, nil, false)
		}
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (b *board) drawColumn(gtx layout.Context, colIdx int, ds DragState) layout.Dimensions {
	colW := gtx.Constraints.Max.X
	sz := gtx.Constraints.Max

	isTarget := ds.Active && b.isOverCol(ds.Pos, colIdx)
	bg := colBgNormal
	if isTarget {
		bg = colBgHover
	}
	drawRect(gtx.Ops, sz, bg)

	// Column header.
	drawRect(gtx.Ops, image.Point{X: colW, Y: colHeaderH}, colHeaderCol)
	{
		hdrGtx := gtx
		hdrGtx.Constraints.Max.Y = colHeaderH
		hdrGtx.Constraints.Min = image.Point{}
		layout.Inset{Top: unit.Dp(14), Left: unit.Dp(12)}.Layout(hdrGtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(b.th, unit.Sp(15), colNames[colIdx])
			lbl.Color = white
			return lbl.Layout(gtx)
		})
	}

	for rowIdx, cardID := range b.cols[colIdx] {
		cardOriginInCol := image.Point{
			X: cardMargin,
			Y: colHeaderH + cardMargin + rowIdx*(cardH+cardGap),
		}

		// Track window-relative card bounds for tooltip positioning.
		winOrigin := image.Point{
			X: colIdx*b.colW + cardOriginInCol.X,
			Y: cardOriginInCol.Y,
		}
		b.cardBounds[cardID] = image.Rectangle{
			Min: winOrigin,
			Max: winOrigin.Add(image.Point{X: b.colW - 2*cardMargin, Y: cardH}),
		}

		// Register drag and hover handlers (take effect next frame).
		{
			st := op.Offset(cardOriginInCol).Push(gtx.Ops)
			cardArea := clip.Rect{Max: image.Point{X: colW - 2*cardMargin, Y: cardH}}.Push(gtx.Ops)
			b.drags[cardID].Add(gtx.Ops)
			b.hovers[cardID].Add(gtx.Ops)
			cardArea.Pop()
			st.Pop()
		}

		// Read drag events (from previous frame's routing).
		for {
			ev, ok := b.drags[cardID].Update(gtx.Metric, gtx.Source, gesture.Both)
			if !ok {
				break
			}
			winPos := f32.Point{
				X: float32(colIdx*b.colW+cardOriginInCol.X) + ev.Position.X,
				Y: float32(cardOriginInCol.Y) + ev.Position.Y,
			}
			switch ev.Kind {
			case pointer.Press:
				b.send(DragState{Active: true, CardID: cardID, SrcCol: colIdx, Pos: winPos, Kind: KindPress})
			case pointer.Drag:
				srcCol := colIdx
				if ds.Active && ds.CardID == cardID {
					srcCol = ds.SrcCol
				}
				b.send(DragState{Active: true, CardID: cardID, SrcCol: srcCol, Pos: winPos, Kind: KindDrag})
			case pointer.Release:
				if ds.Active && ds.CardID == cardID {
					tgtCol := b.colUnder(winPos)
					if tgtCol != ds.SrcCol {
						b.moveCard(cardID, ds.SrcCol, tgtCol)
					}
				}
				b.send(DragState{Kind: KindDrop})
			case pointer.Cancel:
				b.send(DragState{Kind: KindCancel})
			}
		}

		// Draw card (hidden while being dragged — ghost takes its place).
		if !(ds.Active && ds.CardID == cardID) {
			st := op.Offset(cardOriginInCol).Push(gtx.Ops)
			cardW := colW - 2*cardMargin
			drawRect(gtx.Ops, image.Point{X: cardW, Y: cardH}, cardBg)
			cardGtx := gtx
			cardGtx.Constraints = layout.Exact(image.Point{X: cardW, Y: cardH})
			layout.Inset{Top: unit.Dp(6), Left: unit.Dp(8)}.Layout(cardGtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(b.th, unit.Sp(13), cardLabels[cardID])
				lbl.Color = darkText
				return lbl.Layout(gtx)
			})
			st.Pop()
		}
	}

	return layout.Dimensions{Size: sz}
}

func (b *board) tooltipLayout(gtx layout.Context, ts TooltipState) layout.Dimensions {
	if !ts.Active {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}

	label := cardLabels[ts.CardID] + " — tooltip"

	// Position just above the card; clamp to window bounds.
	origin := image.Point{
		X: ts.CardBounds.Min.X,
		Y: ts.CardBounds.Min.Y - tooltipH - 4,
	}
	if origin.Y < 0 {
		origin.Y = ts.CardBounds.Max.Y + 4
	}
	maxX := gtx.Constraints.Max.X - tooltipW
	if origin.X > maxX {
		origin.X = maxX
	}
	if origin.X < 0 {
		origin.X = 0
	}

	st := op.Offset(origin).Push(gtx.Ops)
	drawRect(gtx.Ops, image.Point{X: tooltipW, Y: tooltipH}, tooltipBg)
	ttGtx := gtx
	ttGtx.Constraints = layout.Exact(image.Point{X: tooltipW, Y: tooltipH})
	layout.Inset{Top: unit.Dp(6), Left: unit.Dp(8)}.Layout(ttGtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(b.th, unit.Sp(12), label)
		lbl.Color = tooltipFg
		return lbl.Layout(gtx)
	})
	st.Pop()

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (b *board) modalLayout(gtx layout.Context, ms ModalState) layout.Dimensions {
	sz := gtx.Constraints.Max

	if ms.Depth == 0 {
		// No modal: show "Open Modal" trigger in the top-right corner.
		btnW := gtx.Dp(130)
		btnH := gtx.Dp(36)
		btnOrigin := image.Point{X: sz.X - btnW - 12, Y: 12}
		st := op.Offset(btnOrigin).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Point{X: btnW, Y: btnH})
		openClicked := b.openBtn.Clicked(btnGtx)
		material.Button(b.th, &b.openBtn, "Open Modal").Layout(btnGtx)
		if openClicked {
			log.Printf("[modal] opening depth 1")
			b.modalObs(ModalState{Depth: 1}, nil, false)
		}
		st.Pop()
		return layout.Dimensions{Size: sz}
	}

	// Register the Escape focus anchor inside a full-screen clip area.
	{
		area := clip.Rect{Max: sz}.Push(gtx.Ops)
		event.Op(gtx.Ops, &b.escTagModal)
		area.Pop()
	}
	// Keep keyboard focus on the anchor every frame the modal is visible.
	gtx.Execute(key.FocusCmd{Tag: &b.escTagModal})

	// Read Escape key events.
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: &b.escTagModal},
			key.Filter{Focus: &b.escTagModal, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		if ke, ok := e.(key.Event); ok && ke.Name == key.NameEscape && ke.State == key.Press {
			log.Printf("[modal] Escape: depth %d → %d", ms.Depth, ms.Depth-1)
			b.modalObs(ModalState{Depth: ms.Depth - 1}, nil, false)
		}
	}

	// Semi-transparent backdrop.
	drawRect(gtx.Ops, sz, backdropCol)

	// Centred modal panel.
	panelOrigin := image.Point{
		X: (sz.X - modalPanelW) / 2,
		Y: (sz.Y - modalPanelH) / 2,
	}
	st := op.Offset(panelOrigin).Push(gtx.Ops)
	drawRect(gtx.Ops, image.Point{X: modalPanelW, Y: modalPanelH}, modalPanelCol)

	panelGtx := gtx
	panelGtx.Constraints = layout.Exact(image.Point{X: modalPanelW, Y: modalPanelH})

	layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(24), Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(panelGtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := fmt.Sprintf("Modal Level %d", ms.Depth)
				if ms.Depth == 2 {
					title = "Modal Level 2 (Nested)"
				}
				lbl := material.Label(b.th, unit.Sp(17), title)
				lbl.Color = modalTitleCol
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				body := "Opened via rx.Subject[ModalState]. Escape pops the stack."
				if ms.Depth == 2 {
					body = "Nested modal. Escape pops this level first, then level 1."
				}
				lbl := material.Label(b.th, unit.Sp(13), body)
				lbl.Color = modalBodyCol
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if ms.Depth != 1 {
					return layout.Dimensions{}
				}
				nestedClicked := b.nestedBtn.Clicked(gtx)
				dims := material.Button(b.th, &b.nestedBtn, "Open Nested Modal").Layout(gtx)
				if nestedClicked {
					log.Printf("[modal] opening depth 2")
					b.modalObs(ModalState{Depth: 2}, nil, false)
				}
				return dims
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(b.th, unit.Sp(11), "Press Esc to close")
				lbl.Color = modalHintCol
				return lbl.Layout(gtx)
			}),
		)
	})
	st.Pop()

	return layout.Dimensions{Size: sz}
}

func (b *board) isOverCol(pos f32.Point, colIdx int) bool {
	if colIdx == 0 {
		return pos.X < float32(b.colW)
	}
	return pos.X >= float32(b.colW)
}

func (b *board) colUnder(pos f32.Point) int {
	if pos.X < float32(b.colW) {
		return 0
	}
	return 1
}

func (b *board) moveCard(cardID, fromCol, toCol int) {
	newFrom := make([]int, 0, len(b.cols[fromCol]))
	for _, id := range b.cols[fromCol] {
		if id != cardID {
			newFrom = append(newFrom, id)
		}
	}
	b.cols[fromCol] = newFrom
	b.cols[toCol] = append(b.cols[toCol], cardID)
}

func drawRect(ops *op.Ops, size image.Point, c color.NRGBA) {
	area := clip.Rect{Max: size}.Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	area.Pop()
}

func main() {
	go func() {
		// Three independent Subjects — one per coordination concern.
		// Using coordination.Subject from prism/coordination (BufCapPointer=128
		// for pointer events; BufCapSignal=8 for infrequent modal/tooltip signals).
		dragObserver, dragObservable := coordination.Subject[DragState](coordination.BufCapPointer)
		modalObserver, modalObservable := coordination.Subject[ModalState](coordination.BufCapSignal)
		tooltipObserver, tooltipObservable := coordination.Subject[TooltipState](coordination.BufCapSignal)

		b := newBoard(dragObserver, modalObserver, tooltipObserver)
		w := mvu.NewWindow(
			app.Title("Kanban — G00.C2: drag-drop + modal stacking + tooltip arbitration"),
			app.Size(unit.Dp(700), unit.Dp(500)),
		)

		if err := w.Render(
			makeKanbanLayer(b, dragObservable),
			makeTooltipLayer(b, tooltipObservable),
			makeModalLayer(b, modalObservable),
		).Wait(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	app.Main()
}
