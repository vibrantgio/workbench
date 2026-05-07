# Experiment C: Coordination Context

**Goal G00.C** — Validate the general-purpose cross-widget coordination primitive
for drag-drop, modal stacking, and tooltip arbitration.

**Verdict (C1): rx.Subject[DragState] integrates cleanly — with known caveats.**

---

## §C1 — Drag-drop with shared Subject

### Module

`experiments/coordination` — a standalone Gio app with a two-column kanban
board. Cards are draggable between columns. An `rx.Subject[DragState]` is
created at board level and passed explicitly to the drag source (each card) and
all drop targets (each column). Drop targets highlight when an active drag
hovers over them. Cards move to the target column on release.

Run with: `go run ./experiments/coordination/`

---

### (a) Subject creation and propagation pattern

**Creation (board level)**

```go
// age=0 (no replay), size=0, cap=32 (buffer prevents frame-goroutine blocking),
// scap=4 (max concurrent subscribers).
dragObserver, dragObservable := rx.Subject[DragState](0, 0, 32, 4)
```

`dragObserver` is the write side (Observer[DragState]); `dragObservable` is the
read side (Observable[DragState]).

**DragState type**

```go
type DragState struct {
    Active bool
    CardID int       // card being dragged
    SrcCol int       // column the drag started from
    Pos    f32.Point // drag pointer in window-relative coordinates
}
```

**Propagation to drag source**

`dragObserver` is stored in the `*board` struct. The board's widget closure is
regenerated on every DragState emission. During each frame, the card's
`gesture.Drag` widget (stable across frames — hoisted outside the closure)
reads pointer events and calls `b.send(DragState{...}, nil, false)`:

```go
switch ev.Kind {
case pointer.Press:
    b.send(DragState{Active: true, CardID: cardID, SrcCol: colIdx, Pos: winPos}, nil, false)
case pointer.Drag:
    b.send(DragState{Active: true, CardID: cardID, SrcCol: ds.SrcCol, Pos: winPos}, nil, false)
case pointer.Release:
    // resolve drop, then:
    b.send(DragState{Active: false}, nil, false)
}
```

`winPos` is computed by translating the card-local `ev.Position` to window
coordinates using the card's known layout offset:

```go
winPos := f32.Point{
    X: float32(colIdx*b.colW + cardOriginInCol.X) + ev.Position.X,
    Y: float32(cardOriginInCol.Y) + ev.Position.Y,
}
```

**Propagation to drop targets**

`dragObservable` drives a `rx.Map` that emits a new `layout.Widget` closure on
every DragState:

```go
func makeLayer(b *board, dragObs rx.Observable[DragState]) rx.Observable[layout.Widget] {
    return rx.Map(dragObs.StartWith(DragState{}), func(ds DragState) layout.Widget {
        return func(gtx layout.Context) layout.Dimensions {
            return b.layout(gtx, ds)
        }
    })
}
```

This Observable is passed to `mvu.Window.Render`. The mvu layer subscribes on a
goroutine: when a new widget arrives it stores the snapshot atomically and calls
`window.Invalidate()`. On the next frame the stored widget renders with the
updated `ds` — every column sees the current DragState and can compute whether
to highlight itself.

---

### (b) Do async Subject emissions integrate cleanly with Gio's synchronous frame model?

**Answer: Yes, with one structural caveat and two implementation constraints.**

#### Structural caveat — one-frame highlight lag

Subject delivery is asynchronous. The sequence is:

```
Frame N  │ gesture.Drag detects Drag event
         │ b.send(DragState{Active:true, Pos:p})
         │   → Subject observer called (non-blocking if buffer not full)
         ↓
Goroutine│ Subject subscriber delivers to rx.Map
         │ rx.Map emits new layout.Widget closure
         │ mvu.Window stores snapshot atomically
         │ window.Invalidate() called
         ↓
Frame N+1│ mvu event loop wakes on FrameEvent
         │ new widget closure is called with updated ds
         │ drop targets render highlighted / unhighlighted
```

Drop-target highlighting always lags the drag position by exactly one frame
(~16 ms at 60 fps). In practice this is imperceptible to users. However,
within Gio's strict "one synchronous render pass per frame" model, **same-frame
cross-widget communication via Subject is impossible**. Subject is inherently
a next-frame primitive.

#### Implementation constraint 1 — buffer prevents frame-goroutine blocking

`rx.Subject` blocks the observer if the subscriber has not consumed the previous
item and the buffer is full. Calling the observer from Gio's frame goroutine
with an unbuffered or full Subject would stall frame processing. A generous
buffer (`cap=32`) ensures fast rendering never blocks even under burst pointer
events. In production, `cap` should be sized to the maximum burst expected
between subscriber wakeups (~2 × target FPS).

#### Implementation constraint 2 — mutable widget state must be hoisted outside the FRP closure

`gesture.Drag` accumulates state between frames (pressed/dragging flags, pointer
ID). If the `gesture.Drag` instances lived inside the `rx.Map` closure, they
would be recreated on every DragState emission and lose their accumulated state,
breaking gesture recognition.

The fix: hoist all per-card `gesture.Drag` instances into the `*board` struct,
which is created once and closed over by every widget closure. The closures are
regenerated on each Subject emission but always reference the same stable
`board.drags[cardID]` values.

This is the same requirement identified in Experiment A: **per-widget mutable
state must live outside FRP closures**, either in `rx.Defer` (for truly
subscription-scoped state) or in a struct owned outside the Observable pipeline
(for board-global state).

---

### (c) Candidate coordination primitive shape

Based on C1, the `prism.Coordination` package should expose:

```go
// Subject is a typed broadcast channel. The observer side is held by one
// producer; the observable side is subscribed by N consumers.
func Subject[T any](bufferCap int) (rx.Observer[T], rx.Observable[T])

// WithSubject injects a Subject into a widget subtree via Gio's layer mechanism.
// All widgets in the subtree receive the observable and can register as producers
// or consumers without explicit parameter threading.
func WithSubject[T any](obs rx.Observable[T], child layout.Widget) layout.Widget
```

`WithSubject` addresses the main ergonomic issue surfaced by C1: passing
`dragObserver` and `dragObservable` explicitly through every layer of the widget
tree is verbose. A context/layer injection mechanism (analogous to React Context
or Gio's own `op.Defer` escape hatch) would let any widget in the subtree
subscribe without the parent threading the value.

The exact layer mechanism is an open question (C2 milestone: modal stacking and
tooltip arbitration will stress the multi-producer, ordered-delivery case).

---

### Open questions for C2

- Can a single coordination Subject serve multiple concurrent interactions (two
  simultaneous drags, or drag + tooltip)? Subject's flat multicast suggests a
  discriminated union or per-interaction Subject.
- Modal stacking (concern #3 second sub-case): ordered delivery of "who is on
  top" across independent widget subtrees.
- Tooltip arbitration: debouncing and cancellation via Subject — does the async
  delivery model interact badly with hover exit events?
