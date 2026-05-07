# MIGRATION.md — Gio v0.1.0 → v0.9.x API Audit

**Purpose:** Phase −1 prerequisite (DESIGN.md §"Phase −1"). Read-only enumeration of every Gio call site in the workspace that must change during the migration. No code is modified here.

**Baseline:** all `go.mod` files pin `gioui.org v0.1.0`. Target is current Gio (v0.9.x).

---

## Verification

The PLAN.md G−1.1 Measurable grep (`grep -rn … --include="*.go"`) does not work with macOS BSD grep — `--include` is silently ignored when combined with `-r`. Use the portable equivalent below. It must return **53** (Gio-related lines; 4 additional hits from `kiwi/errors.go` and `kiwi/solver.go` for the `InternalSolverError` constant are not Gio API).

```bash
find . -name "*.go" -print0 \
  | xargs -0 grep -En 'Events\(\)|InputOp|InvalidateOp\{\}|Internal' \
  | grep -v 'kiwi/solver\|kiwi/errors' \
  | wc -l
# Expected: 53
```

---

## API Changes Reference

| Category | Old API (v0.1.0) | New API (v0.9.x) |
|---|---|---|
| **A** | `app.Window.Events() <-chan event.Event` | `app.Window.Event() event.Event` (blocking method) |
| **B** | `widget.Editor.Events() []widget.EditorEvent` | `widget.Editor.Update(gtx, ...) widget.EditorEvent` (context required) |
| **B** | `richtext.InteractiveText.Events()` iterator | event-iterator pattern changed; verify against v0.9 `gioui.org/x/richtext` |
| **C** | `pointer.InputOp{Tag, Types}.Add(ops)` + `gtx.Events(tag)` | `event.Op{Tag, Types}.Add(ops)` (inside clip) + `gtx.Source.Event(filter)` |
| **C** | `key.InputOp{Tag}.Add(ops)` + `gtx.Events(tag)` | `key.FocusOp{Tag}.Add(ops)` + `gtx.Source.Event(key.Filter{…})` |
| **D** | `op.InvalidateOp{}.Add(ops)` | `gtx.Execute(op.InvalidateCmd{})` |
| **D** | `op.InvalidateOp{At: t}.Add(ops)` | `gtx.Execute(op.InvalidateCmd{At: t})` |
| **E** | `(*unsafeOps)(unsafe.Pointer(&ops.Internal)).refs` | `gtx.Source.Event(filter)` — first-class event routing |

### Risk levels

| Level | Meaning |
|---|---|
| LOW | Mechanical rewrite; one-to-one substitution. |
| MED | Semantics changed or context now required; test coverage essential. |
| HIGH | Affects architectural wiring (`mvu` runtime); `rx` adapter must be redesigned. |
| CRITICAL | `unsafe` cast; silent data corruption on wrong Gio version; must be eliminated before any version bump. |

---

## Call-Site Table

Paths are relative to workspace root. Every row corresponds to a line returned by the verification grep above.

### A — `app.Window.Events()` channel (25 sites)

Risk shared by all: channel → method; `for range` and `select` patterns must be rewritten as `for { e := window.Event() }`. The `mvu/window.go` row additionally requires redesigning the `rx.Recv()` adapter.

| File:Line | Old API call | New API | Risk |
|---|---|---|---|
| `mvu/window.go:35` | `rx.Recv(w.window.Events())` | wrap `window.Event()` in `rx.Func` producer goroutine | HIGH |
| `traer/gio/gravity/main.go:51` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `traer/gio/arboretum/main.go:54` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `traer/gio/scrolling/main.go:60` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `traer/gio/attraction/main.go:57` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `kiwi/gio/example/quadrilateral/main.go:122` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `svg/driver/gio/example/berries/main.go:91` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `svg/driver/gio/example/circles/main.go:43` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `svg/driver/gio/example/primitive/main.go:72` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/arrow/main.go:33` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/favicon/main.go:56` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/blend/main.go:31` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/info/main.go:45` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/logo/main.go:59` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/icons/main.go:49` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/cowbell/main.go:57` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `ivg/raster/gio/example/gradients/main.go:56` | `for next := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/helloworld/main.go:36` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/rectangle/main.go:37` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/noisywavepatch/main.go:91` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/combinedsolid/main.go:120` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/text/main.go:116` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/noisysphere/main.go:56` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/giftbox/main.go:169` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |
| `seen/context/gio/example/poem/main.go:79` | `for event := range window.Events()` | `for { e := window.Event() }` | LOW |

### B — Widget `Events()` (3 sites)

Risk shared: widget event API now requires the layout context; `Events()` (no context) no longer exists.

| File:Line | Old API call | New API | Risk |
|---|---|---|---|
| `mindchat/view.go:165` | `edit.Events()` (`widget.Editor`) | `edit.Update(gtx, shaper, font, size)` → returns events | MED |
| `todos/upsertdialog.go:93` | `edit.Events()` (`widget.Editor`) | `edit.Update(gtx, shaper, font, size)` → returns events | MED |
| `mvu/example/richtext/main.go:88` | `state.Events()` (`richtext.InteractiveText`) | event-iterator pattern changed; verify `gioui.org/x/richtext` v0.9 API | MED |

### C — `pointer.InputOp` / `key.InputOp` (16 sites)

Risk shared for pointer: registration now requires the area to be inside an active clip; `Types` bitmask filter maps to `event.Filter` types; all paired `gtx.Events(tag)` / `queue.Events(tag)` reads (see §Additional Patterns) must also be replaced. Risk shared for key: key focus model changed; handler must specify accepted key names.

| File:Line | Old API call | New API | Risk |
|---|---|---|---|
| `coinviz/content.go:130` | `pointer.InputOp{Tag: &offset, Types: pointer.Scroll, ScrollBounds: …}.Add(gtx.Ops)` | `pointer.ScrollOp{Tag: &offset, ScrollBounds: …}.Add(gtx.Ops)` or `event.Op{Tag: &offset}.Add(gtx.Ops)` | MED |
| `coinviz/content.go:153` | `pointer.InputOp{Tag: &crosshair, Types: pointer.Press|Drag|Release|Move}.Add(gtx.Ops)` | `event.Op{Tag: &crosshair}.Add(gtx.Ops)` inside clip | MED |
| `mvu/message.go:15` | `pointer.InputOp{Tag: op}.Add(o)` | first-class `MessageOp` event routing via `gtx.Source.Event(filter)` | HIGH |
| `todos/onescapekey.go:12` | `key.InputOp{Tag: esc}.Add(gtx.Ops)` | `key.FocusOp{Tag: esc}.Add(gtx.Ops)` + `key.InputOp{Tag: esc, Keys: key.NameEscape}.Add(gtx.Ops)` | MED |
| `traer/gio/gravity/main.go:88` | `pointer.InputOp{Tag: field, Types: pointer.Press|Release|Drag}.Add(gtx.Ops)` | `event.Op{Tag: field}.Add(gtx.Ops)` inside clip | MED |
| `traer/gio/arboretum/main.go:59` | `pointer.InputOp{Tag: arboretum, Types: pointer.Press}.Add(gtx.Ops)` | `event.Op{Tag: arboretum}.Add(gtx.Ops)` inside clip | MED |
| `traer/gio/scrolling/main.go:68` | `pointer.InputOp{Tag: scroller, Types: pointer.Press|Release|Drag|Scroll}.Add(gtx.Ops)` | `event.Op{Tag: scroller}.Add(gtx.Ops)` inside clip | MED |
| `traer/gio/attraction/main.go:75` | `pointer.InputOp{Tag: tag, Types: pointer.Move}.Add(gtx.Ops)` | `event.Op{Tag: tag}.Add(gtx.Ops)` inside clip | MED |
| `mvu/example/08-iconscroll/main.go:45` | `pointer.InputOp{Tag: &offset, Types: pointer.Scroll, ScrollBounds: sb}.Add(gtx.Ops)` | `event.Op{Tag: &offset}.Add(gtx.Ops)` inside clip | MED |
| `mvu/example/circles/main.go:41` | `pointer.InputOp{Tag: &circles, Types: pointer.Press|Drag|Release}.Add(gtx.Ops)` | `event.Op{Tag: &circles}.Add(gtx.Ops)` inside clip | MED |
| `kiwi/gio/example/quadrilateral/main.go:135` | `pointer.InputOp{Tag: backdrop, Types: pointer.Move|Press|Drag|Release}.Add(ops)` | `event.Op{Tag: backdrop}.Add(ops)` inside clip | MED |
| `seen/context/gio/context.go:78` | `pointer.InputOp{Tag: d, Types: pointer.Press|Drag|Release}.Add(ops)` | `event.Op{Tag: d}.Add(ops)` inside clip | MED |
| `seen/context/gio/context.go:136` | `pointer.InputOp{Tag: z, Types: pointer.Scroll, ScrollBounds: …}.Add(ops)` | `event.Op{Tag: z}.Add(ops)` inside clip | MED |
| `ivg/raster/gio/example/favicon/main.go:61` | `pointer.InputOp{Tag: backend, Types: pointer.Release}.Add(gtx.Ops)` | `event.Op{Tag: backend}.Add(gtx.Ops)` inside clip | MED |
| `ivg/raster/gio/example/icons/main.go:54` | `pointer.InputOp{Tag: backend, Types: pointer.Release}.Add(gtx.Ops)` | `event.Op{Tag: backend}.Add(gtx.Ops)` inside clip | MED |
| `ivg/raster/gio/example/gradients/main.go:61` | `pointer.InputOp{Tag: backend, Types: pointer.Release}.Add(gtx.Ops)` | `event.Op{Tag: backend}.Add(gtx.Ops)` inside clip | MED |

### D — `op.InvalidateOp{}` (8 sites)

Risk shared: `ops.Add()` pattern replaced by context method; no ops buffer required; `op.InvalidateCmd{}` has the same optional `At` field.

| File:Line | Old API call | New API | Risk |
|---|---|---|---|
| `traer/gio/gravity/main.go:99` | `op.InvalidateOp{}.Add(gtx.Ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |
| `traer/gio/arboretum/main.go:92` | `op.InvalidateOp{}.Add(gtx.Ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |
| `traer/gio/scrolling/main.go:102` | `op.InvalidateOp{}.Add(gtx.Ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |
| `traer/gio/attraction/main.go:95` | `op.InvalidateOp{}.Add(gtx.Ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |
| `mvu/example/richtext/main.go:96` | `op.InvalidateOp{}.Add(gtx.Ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |
| `seen/context/gio/context.go:61` | `op.InvalidateOp{}.Add(ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |
| `svg/driver/gio/example/berries/main.go:101` | `op.InvalidateOp{}.Add(ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |
| `svg/driver/gio/example/primitive/main.go:82` | `op.InvalidateOp{}.Add(ops)` | `gtx.Execute(op.InvalidateCmd{})` | LOW |

### E — `ops.Internal` unsafe cast (1 site)

| File:Line | Old API call | New API | Risk |
|---|---|---|---|
| `mvu/window.go:74` | `(*unsafeOps)(unsafe.Pointer(&ops.Internal)).refs` — reinterpret-cast to extract `MessageOp` from ops buffer | `gtx.Source.Event(filter)` — `MessageOp` becomes a first-class event tag read via the router | CRITICAL |

**Context:** `mvu/window.go:69–78` defines a local `unsafeOps` struct mirroring `op.Ops` internals, then casts `&ops.Internal` to extract the `refs []any` slice. An inline comment records that the `version` field changed from `int` to `uint32` between v0.1.0 and v0.8; the current cast is correct only because the workspace is pinned to v0.1.0. Any version bump before this hack is removed will produce silent wrong reads or a crash.

---

## Additional Patterns

These call sites are not matched by the Measurable grep (`Events()` requires no args; `InvalidateOp{}` requires empty braces) but are in the same migration scope.

### A2 — `op.InvalidateOp{At: t}` (timed invalidation)

| File:Line | Old API | Note |
|---|---|---|
| `svg/driver/gio/example/berries/main.go:104` | `op.InvalidateOp{At: time.Now().Add(250ms)}.Add(ops)` | → `gtx.Execute(op.InvalidateCmd{At: t})` |
| `svg/driver/gio/example/primitive/main.go:85` | `op.InvalidateOp{At: time.Now().Add(250ms)}.Add(ops)` | → `gtx.Execute(op.InvalidateCmd{At: t})` |
| `ivg/raster/gio/example/icons/main.go:93` | `op.InvalidateOp{At: at}.Add(ops)` | → `gtx.Execute(op.InvalidateCmd{At: at})` |

### C2 — `gtx.Events(tag)` / `frame.Queue.Events(tag)` (event reads paired with every InputOp site)

Each `pointer.InputOp` registration in §C is paired with an event read using one of these patterns:

| File:Line | Old API | Note |
|---|---|---|
| `coinviz/content.go:136` | `gtx.Events(&offset)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `coinviz/content.go:154` | `gtx.Events(&crosshair)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `todos/onescapekey.go:13` | `gtx.Events(esc)` | → `gtx.Source.Event(key.Filter{…})` |
| `mvu/example/08-iconscroll/main.go:47` | `gtx.Events(&offset)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `mvu/example/circles/main.go:42` | `gtx.Events(&circles)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `traer/gio/gravity/main.go:89` | `frame.Queue.Events(field)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `traer/gio/arboretum/main.go:60` | `frame.Queue.Events(arboretum)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `traer/gio/scrolling/main.go:69` | `frame.Queue.Events(scroller)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `traer/gio/attraction/main.go:76` | `frame.Queue.Events(tag)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `kiwi/gio/example/quadrilateral/main.go:165` | `frame.Queue.Events(backdrop)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `ivg/raster/gio/example/favicon/main.go:62` | `event.Queue.Events(backend)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `ivg/raster/gio/example/icons/main.go:55` | `event.Queue.Events(backend)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `ivg/raster/gio/example/gradients/main.go:62` | `event.Queue.Events(backend)` | → `gtx.Source.Event(pointer.Filter{…})` |
| `seen/context/gio/context.go:85` | `q.Events(d)` (via `event.Queue`) | → `gtx.Source.Event(pointer.Filter{…})` |
| `seen/context/gio/context.go:143` | `q.Events(z)` (via `event.Queue`) | → `gtx.Source.Event(pointer.Filter{…})` |

### C3 — `event.Queue` interface (seen/context)

`seen/context/gio/context.go` accepts `event.Queue` as a parameter type (`Process(ops, queue)`) and stores `[]func(event.Queue)` handlers. The `event.Queue` interface no longer exists in v0.9.x; its role is taken by `input.Source` (accessible via `gtx.Source`). This requires a redesign of the `Context` type's handler signature.

| File:Line | Note |
|---|---|
| `seen/context/gio/context.go:28` | `handlers []func(event.Queue)` — field type must change |
| `seen/context/gio/context.go:49` | `Process(ops *op.Ops, queue event.Queue)` — signature must change |
| `seen/context/gio/context.go:84` | `c.handlers = append(…, func(q event.Queue) {…})` |
| `seen/context/gio/context.go:142` | `c.handlers = append(…, func(q event.Queue) {…})` |

### A3 — `widget.Events(gtx.Queue)` gesture click (appviz)

`appviz/periodpanel.go` uses `gesture.Click.Events(queue event.Queue)` (taking an `event.Queue`), not the no-arg `Events()`:

| File:Line | Old API | Note |
|---|---|---|
| `appviz/periodpanel.go:57` | `older.Events(gtx.Queue)` (`gesture.Click`) | → `gesture.Click.Update(gtx)` or direct pointer event read |
| `appviz/periodpanel.go:73` | `newer.Events(gtx.Queue)` (`gesture.Click`) | → `gesture.Click.Update(gtx)` or direct pointer event read |

### D2 — `system.FrameEvent` / `system.DestroyEvent` type assertions

All 25 `window.Events()` event loops (§A) contain a `system.FrameEvent` type assertion. In v0.9.x the event loop yields `app.FrameEvent` and `app.DestroyEvent` (package moved from `gioui.org/io/system` to `gioui.org/app`). These are mechanical package-path fixes but they appear at every `window.Events()` site listed in §A.

---

## Architectural Surprises

No surprises during the audit. The five architectural patterns documented in DESIGN.md (§"Known Fragilities") are confirmed present at exactly the call sites predicted:

1. **`window.Events()` channel** — 25 sites (§A). All are straightforward `for range` loops except `mvu/window.go:35` where `rx.Recv()` wraps the channel. The `WithLatestFrom2` model in `mvu/window.go` is untouched by this table; the architectural claim survives once the channel source is replaced.

2. **`pointer.InputOp`** — 16 sites (§C). All follow the register-once-per-frame pattern. Migration to `event.Op` preserves the per-frame registration contract.

3. **`op.InvalidateOp{}`** — 8 sites (§D). All are immediate-invalidate; 3 additional timed-invalidate sites (§A2). Uniform `gtx.Execute()` replacement.

4. **`ops.Internal` unsafe cast** — 1 site (§E, `mvu/window.go:74`). Load-bearing prerequisite for stable MVU runtime. Must be eliminated before any Gio version bump.

5. **`event.Queue` interface** — 4 sites in `seen/context/gio/context.go` (§C3). Requires redesign of the `Context` handler signature.

## Risk-Gate: Architectural Pattern Review

*G−1.7 deliverable. Each of the five DESIGN.md architectural patterns assessed against the v0.9.x API audit above. All findings are prospective — the workspace is still at v0.1.0; the verdict reflects whether the pattern survives once the call sites in the audit are rewritten.*

### 1. FRP Application Structure ✅

The one-directional dataflow pipeline (`Data source → rx.Observable → rx.CombineLatest → rx.Map → Window.Render`) operates entirely within the `reactivego/rx` layer and does not call any deprecated Gio API directly. The only Gio-facing boundary in the FRP path is the events source observable that feeds `WithLatestFrom2`; that source changes from `rx.Recv(w.window.Events())` to a goroutine wrapping `window.Event()`, a mechanical one-line substitution in `mvu/window.go`. Theme switching via `theme.AutoLightDark()` and reactive re-renders on upstream changes are unaffected — neither `CombineLatest` nor `rx.Map` nor `Window.Render` touch deprecated surfaces. The pattern holds on v0.9.x.

### 2. MVU Application Structure ✅

The `Init → (Model, Command) → Update(Model, Message) → (Model, Command) → View(Model) → layout.Widget` state machine is implemented at the application layer (`todos`, `mindchat`); its shape is independent of the Gio event API version. The two MED-risk `widget.Editor.Events()` sites (`mindchat/view.go:165`, `todos/upsertdialog.go:93`) are leaf widget interactions that route through the new `Update(gtx, …)` pattern without altering the command-loop contract. The HIGH-risk `mvu/window.go:35` and CRITICAL `mvu/window.go:74` sites affect the runtime's *wiring* — how events are sourced and how MessageOps are dispatched — but not the MVU *shape*. After the wiring is rebuilt on the new API the pattern is preserved; no structural change to the Init/Update/View contract is required.

### 3. `WithLatestFrom2` Frame Synchronisation Model ✅

The audit confirms that `WithLatestFrom2` itself is untouched by the migration: `events` remains the leading observable, the trailing `CombineLatest` of layers still runs on `rx.Goroutine`, and the observer still gates all Gio code to the events drumbeat. The only change is the source: `rx.Recv(w.window.Events())` is replaced by a producer goroutine that calls `window.Event()` in a loop and emits into a subject (or equivalent `rx.Func` producer). The synchronisation guarantee — that heavy upstream work parallelises freely while everything touching Gio code runs sequentially on the events thread — is structurally identical under either source. The MIGRATION.md audit note in §Architectural Surprises ("the architectural claim survives once the channel source is replaced") is confirmed by the audit.

### 4. `rx.Defer` Subscription-State Pattern ✅

`rx.Defer` is a `reactivego/rx` primitive with no Gio API dependency. The subscription-state pattern (`Defer` factory allocates state once, `Map` closure captures it by reference, widget closure reads and mutates it during the frame) is unaffected by the v0.1.0 → v0.9.x bump. The two sites in `coinviz/content.go` that register pointer input inside `Defer`-scoped widgets (lines 130 and 153) change their registration call (`pointer.InputOp` → `event.Op`) and event read (`gtx.Events(tag)` → `gtx.Source.Event(filter)`), but the `Defer` closure structure and state lifetime are identical. The "why this works without locks" invariant — that both the `Map` closure and the widget closure serialise through `WithLatestFrom2` — is preserved because Pattern 3 holds.

### 5. Frame-Driven Physics Loop ✅

The self-scheduling mechanism (`op.InvalidateOp{}.Add(gtx.Ops)`) migrates verbatim to `gtx.Execute(op.InvalidateCmd{})` — a one-to-one substitution that appears at all four `traer/gio` sites (§D in the audit). The semantics are identical: the widget requests its next frame, the window stays idle when no `InvalidateCmd` is issued, and multiple animated widgets each contribute independently. The "idle at rest" and "composable" properties depend only on Gio posting a frame only when invalidation is requested — a scheduler invariant unchanged across the migration range. The three timed-invalidate sites (`op.InvalidateOp{At: t}` → `gtx.Execute(op.InvalidateCmd{At: t})`, §A2) are equally mechanical. The pattern holds intact on v0.9.x.
