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

**Answer: Yes, with one structural caveat and three implementation constraints.**

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

**Empirically confirmed.** Instrumentation embedded in `DragState` records
`EmitFrame` (the frame counter when the Subject observer was called) and
`SeqN` (the emission sequence number). On every render, `board.layout` logs
`lag = renderFrame − EmitFrame`. Across 37 emissions in two full drags:

```
[frame    3] emit #  1  press   pos=(255,71)
[frame    3] emit #  2  drag    pos=(255,72)   ← buffer pressure (see below)
[frame    4] render emission #  2  lag=1 frame(s)
[frame    5] emit #  3  drag    pos=(258,72)
[frame    6] render emission #  3  lag=1 frame(s)
...
[frame   23] emit # 20  drag    pos=(498,227)
[frame   23] emit # 21  drop    pos=(0,0)      ← buffer pressure
[frame   24] render emission # 21  lag=1 frame(s)
```

`lag=1` on every emission without exception.

#### Implementation constraint 1 — buffer prevents frame-goroutine blocking

`rx.Subject` blocks the observer if the subscriber has not consumed the previous
item and the buffer is full. Calling the observer from Gio's frame goroutine
with an unbuffered or full Subject would stall frame processing. A generous
buffer (`cap=32`) ensures fast rendering never blocks even under burst pointer
events. In production, `cap` should be sized to the maximum burst expected
between subscriber wakeups (~2 × target FPS).

#### Implementation constraint 2 — intermediate emissions are silently dropped under burst

Gio coalesces certain event pairs into a single frame delivery: press+first-drag
and last-drag+release both arrive in the same frame. When two emissions happen
in one frame, the `mvu.Window` atomic snapshot is overwritten before the next
frame fires, so the earlier emission is never rendered:

```
[frame   23] emit # 20  drag    ← emitted, never rendered
[frame   23] emit # 21  drop    ← overwrites snapshot
[frame   24] render emission # 21  lag=1 frame(s)
```

This is not a Subject bug — the Subject delivered both values to the subscriber.
The drop happens in `mvu.Window.Render`'s atomic snapshot: only the most recent
widget closure survives to the next frame. **For drag-drop this is correct
behaviour** (the intermediate position doesn't matter once the pointer is
released), but for use cases where every emission is load-bearing (e.g. an
event log or an undo stack), a Subject-driven layer is not the right tool.

#### Implementation constraint 3 — mutable widget state must be hoisted outside the FRP closure

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

## §C2 — Modal stacking + tooltip arbitration

### Module

Same module as C1 (`experiments/coordination`). C2 extends the kanban prototype
with two additional coordination signals via independent `rx.Subject` instances.

Run with: `go run ./experiments/coordination/`

---

### (a) Do multiple independent Subjects compose cleanly?

**Answer: Yes. Three Subjects running concurrently with three mvu layers
produce no interference.**

C2 adds two Subjects alongside the C1 drag Subject:

```go
dragObserver,    dragObservable    := rx.Subject[DragState](0, 0, 32, 4)
modalObserver,   modalObservable   := rx.Subject[ModalState](0, 0, 8, 4)
tooltipObserver, tooltipObservable := rx.Subject[TooltipState](0, 0, 8, 4)
```

Each Subject drives a dedicated `rx.Observable[layout.Widget]` passed as a
separate layer to `mvu.Window.Render`:

```go
w.Render(
    makeKanbanLayer(b, dragObservable),
    makeTooltipLayer(b, tooltipObservable),
    makeModalLayer(b, modalObservable),
)
```

`mvu.Window.Render` already uses `rx.CombineLatest` internally; the three-layer
call is identical in cost and complexity to the single-layer C1 call. Layer
order determines z-order: kanban at bottom, tooltip in the middle, modal on top.

**No discriminated union is needed.** C1's open question ("can a single Subject
serve multiple concerns via a union type?") resolves as: use separate Subjects,
one per concern. This is simpler to type-check and avoids the coupling that a
shared union would introduce.

---

### (b) Does the one-frame lag generalise to all Subject-driven layers?

**Answer: Yes. The 1-frame lag is an invariant of the Subject → mvu.Window
pipeline, regardless of which concern the Subject carries.**

Modal state change sequence (identical in structure to C1's drag-state sequence):

```
Frame N  │ openBtn.Clicked returns true
         │ b.modalObs(ModalState{Depth: 1}, nil, false) called
         │   → Subject observer called (non-blocking, buffer cap=8)
         ↓
Goroutine│ Subject subscriber delivers to rx.Map
         │ rx.Map emits new layout.Widget closure capturing Depth=1
         │ mvu.Window stores snapshot atomically
         │ window.Invalidate() called
         ↓
Frame N+1│ modal layer widget is called with Depth=1
         │ dim backdrop and modal panel are rendered
```

The same holds for tooltip suppression: hover exit in frame N, tooltip hidden in
frame N+1. In practice this is imperceptible (~16 ms at 60 fps).

---

### (c) Does the async delivery model interact badly with hover exit events?

**Answer: No adverse interaction observed, with one note on debouncing.**

Tooltip arbitration uses `gesture.Hover.Update(gtx.Source)` to sample hover
state every frame. The arbitration logic:

```go
hoveredCard := -1
for i := 0; i < numCards; i++ {
    if b.hovers[i].Update(gtx.Source) {
        hoveredCard = i
    }
}
if hoveredCard != b.prevHover {
    b.prevHover = hoveredCard
    b.tooltipObs(TooltipState{Active: hoveredCard >= 0, CardID: hoveredCard, ...}, nil, false)
}
```

Only one tooltip Subject emission fires per frame (the change-guard prevents
redundant emissions). When the pointer moves directly between two cards,
`Hover.Update` returns false for the exiting card and true for the entering card
in the same frame, so `prevHover` changes from A to B in one step — no
intermediate "no tooltip" frame.

**Debouncing**: hover-enter events can fire on the frame the pointer first enters
the card area. No observable bouncing was seen in testing. If debouncing is
needed (e.g. delay tooltip by 300 ms), it belongs in a `rx.Debounce` operator
applied to `tooltipObservable`, not in the hover detection loop.

---

### (d) Pointer event isolation — an unsolved coordination concern

The modal backdrop (semi-transparent overlay) does NOT automatically block
pointer events from reaching the kanban layer beneath it. Rendering a coloured
rectangle does not register a pointer handler; the kanban's `gesture.Drag`
handlers remain active under the modal.

Blocking pointer events requires registering an absorber in the ops tree.
Options:
- Add `b.backdropClick widget.Clickable` and call `b.backdropClick.Layout(gtx,
  ...)` wrapping the entire modal overlay — its pointer registration absorbs
  all events in that z-layer.
- Use `pointer.InputOp` with all `Kind` bits set on a full-screen clip area.

This is a **Phase 1 follow-up**, not a coordination-primitive concern. The
`rx.Subject` pattern for modal stacking is validated regardless. Noted for the
Phase 1 `prism.Coordination` package to include a documented "modal backdrop
absorber" recipe.


## §Decision — Phase 1 `prism.Coordination` package shape

### Decision

**Commit to explicit Subject threading as the Phase 1 coordination pattern.
`WithSubject` (context-injection) is deferred to Phase 2.**

C2 validated three independent `rx.Subject` instances used concurrently without
friction. Explicit threading — passing `rx.Observer[T]` and `rx.Observable[T]`
as fields in the owning widget struct — was sufficient and clear throughout the
experiment. No case arose where implicit injection would have simplified the
code.

---

### Package: `prism.Coordination`

**Phase 1 surface (what the package exports):**

```go
package coordination

import "github.com/reactivego/rx"

// Subject creates a typed broadcast channel for cross-widget coordination.
// The Observer side is held by one producer; the Observable side may be
// subscribed by N consumers.
//
// bufCap is the producer-side buffer depth. Size to ~2×target-FPS for
// widgets that emit on every pointer event (drag, hover). For infrequent
// signals (modal depth, focus owner) 8–16 suffices.
func Subject[T any](bufCap int) (rx.Observer[T], rx.Observable[T]) {
    return rx.Subject[T](0, 0, bufCap, 8)
}
```

The `rx.Subject` parameters `age=0, size=0` are fixed for all coordination
use-cases (no replay, no value cache). `scap=8` is a safe default for up to
eight concurrent subscribers. Both are unexported implementation details.

---

### Fields (injection pattern)

The owning widget struct holds the write side; each layer factory receives the
read side:

```go
type MyWidget struct {
    dragObs    rx.Observer[DragState]    // write side — held by producer
    modalObs   rx.Observer[ModalState]
    tooltipObs rx.Observer[TooltipState]
    // ...per-concern mutable gesture state hoisted here...
}

// At construction time:
dragObs,    dragObservable    := coordination.Subject[DragState](32)
modalObs,   modalObservable   := coordination.Subject[ModalState](8)
tooltipObs, tooltipObservable := coordination.Subject[TooltipState](8)

w := newWidget(dragObs, modalObs, tooltipObs)

window.Render(
    makeDragLayer(w, dragObservable),
    makeTooltipLayer(w, tooltipObservable),
    makeModalLayer(w, modalObservable),
)
```

---

### Invariants (from C1 + C2)

All of these must be documented in the Phase 1 package:

1. **One-frame lag.** Subject delivery is asynchronous. Cross-widget state
   changes are visible on the frame AFTER the emitting frame. This is correct
   for all three validated concerns (drag, modal, tooltip) and is imperceptible
   to users at ≥30 fps.

2. **Mutable per-widget state must be hoisted outside FRP closures.** Gesture
   accumulators (`gesture.Drag`, `gesture.Hover`, `widget.Clickable`) must live
   in the owning struct, not inside the `rx.Map` closure. Closures are
   regenerated on every Subject emission.

3. **Buffer capacity must exceed maximum burst.** For pointer-event emitters,
   `bufCap ≥ 2×FPS` prevents frame-goroutine blocking. For infrequent emitters
   (modals, tooltips), `bufCap = 8` is sufficient.

4. **Intermediate emissions are silently dropped under burst.** The
   `mvu.Window` atomic snapshot retains only the most recent widget closure
   before the next frame. Coordination signals where every value is
   load-bearing (undo stacks, event logs) require a different mechanism.

---

### What is NOT in Phase 1

- **`WithSubject` (context injection)**: Not validated. Explicit threading
  handled C1–C2 without ergonomic friction. Revisit in Phase 2 once a
  deeper widget tree exercises the pattern at scale.
- **Modal backdrop input-blocking**: Pointer event isolation for modal overlays
  requires an absorber widget (`widget.Clickable` over the full backdrop area).
  This is a Phase 1 follow-up recipe, not a coordination-primitive concern.
- **Debouncing / throttling combinators**: `rx.Debounce`, `rx.Throttle` can be
  applied to any `rx.Observable[T]` before passing to a layer factory. Not
  needed for C1–C2 but documented as a usage pattern.


