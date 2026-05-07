# Experiment A: Keyed Identity

**Goal G00.A** — Validate whether a `KeyedDefer` abstraction can give FRP
list items stable per-row state that survives reorder, insertion, and deletion.

**Verdict: Adopt (revised API name — see §Decision).**

---

## (a) The API Tried

### Module

`experiments/keyed` — a standalone throwaway module (~90 lines code, ~230 lines
test). No Gio dependency; state management is pure-Go and scheduler-agnostic.

### Type

```go
// Deferred[K, V] maintains stable per-key values for use inside an rx.Defer closure.
type Deferred[K comparable, V any] struct { ... }

func New[K comparable, V any](factory func(K) V) *Deferred[K, V]
func (d *Deferred[K, V]) For(key K) V
func (d *Deferred[K, V]) Sweep(activeKeys []K)
func (d *Deferred[K, V]) Len() int
```

### Intended use-site (inside `rx.Defer`)

```go
rx.Defer(func() rx.Observable[layout.Widget] {
    editors := keyed.New(func(_ ItemID) *widget.Editor { return &widget.Editor{} })

    return rx.Map(items, func(list []Item) layout.Widget {
        rows := make([]layout.Widget, len(list))
        for i, item := range list {
            ed := editors.For(item.ID)   // same *widget.Editor for same ID
            rows[i] = editorRow(ed, item)
        }
        return renderList(rows)
    })
})
```

`V` is almost always a pointer (`*widget.Editor`, `*RowState`) so that mutations
made inside a Gio frame callback are visible on the next `Map` emission without
extra indirection. A value type would silently copy on each `For()` call.

### Cleanup semantics: sticky vs swept

Two policies were considered and both implemented:

| Policy | Behaviour on key re-add after deletion |
|--------|----------------------------------------|
| **Sticky** (default) | Returns the old value — editor content preserved. |
| **Swept** (after `Sweep`) | Old value freed; `factory(k)` called again on re-add. |

Sticky is the default because accidental deletions (undo, swipe-back) should
restore prior state. `Sweep` is the escape hatch for callers that want a clean
slate.

### Test coverage

All tests drive the FRP cycle: a `rx.Subject[[]Item]` emits list permutations;
a `rx.Map` calls `d.For()` for each item; results are collected via a buffered
channel. State mutations happen between emissions, simulating Gio frame
interaction.

| Test | What it verifies |
|------|-----------------|
| `TestReorderPreservesState` | State follows key, not position, across a permutation. |
| `TestInsertionPreservesExistingState` | Inserting a new key leaves existing state untouched; new key gets fresh state. |
| `TestDeletionAndSweep` | (a) Sticky: re-added key recovers old state before `Sweep`. (b) Swept: re-added key gets fresh state after `Sweep`. |
| `TestConcurrentFor` | Internal map is race-free under `-race` when goroutines call `For` with distinct keys. |

All pass (`go test -race`).

---

## (b) Did It Feel Natural?

**Yes, with one friction.**

The core idiom — allocate a `keyed.New(factory)` inside `rx.Defer`, call
`.For(key)` in the `Map` closure — reads almost identically to the existing
`rx.Defer` subscription-state pattern (DESIGN §4). No new concepts for a reader
who already understands that pattern.

The `V`-must-be-a-pointer convention is a mild trap. Nothing in the type system
prevents `V = widget.Editor` (value type); the code compiles and runs, but
mutations are silently lost because `For` returns a copy. The doc comment warns,
but a dedicated `Ptr` constructor — `keyed.NewPtr(factory func(K) *V)` — would
make the pointer requirement structural. Left as a future revision.

`Sweep` was not difficult to reason about, but the call site is awkward: the
caller must pass the active key slice explicitly, re-deriving something the
caller already holds. An `AutoSweep(Observable[[]K])` variant that subscribes
to the upstream keys observable and sweeps automatically would be cleaner. Also
left as a future revision.

The name `Deferred` feels slightly off next to `rx.Defer`. The milestone called
it `rx.KeyedDefer[K, V]`, which names the concept precisely. If this moves into
the `rx`-adjacent layer (e.g., `prism/keyed`), a top-level function
`keyed.Defer(factory)` returning `*keyed.Deferred` would match the convention.

---

## (c) Decision

**Adopt** the `keyed.Deferred[K, V]` pattern. Specific revisions before
integration into a production package:

1. **Rename**: expose as `prism/keyed` with a constructor `keyed.Defer(factory)`
   to match the `rx.Defer` naming family (milestone's original `rx.KeyedDefer`
   is the right spirit, even if the package path differs).

2. **V-must-be-pointer guard**: consider a `keyed.Ptr` variant or a vet check.

3. **AutoSweep**: provide an `AutoSweep(Observable[[]K])` convenience that ties
   key-set lifetime to an upstream observable, eliminating the manual call.

4. **Phase-gate**: items 2 and 3 are Phase 1 work. The core `For` + `Sweep`
   primitive is sufficient for Phase 00 experiments.

---

## Scope note

The "reorderable todo list" in this experiment is *simulated*: programmatic
`Item` slice permutations fed through `rx.Subject`, not a Gio drag-to-reorder
UI. The widget-rendering path (from `Map` return value to `gtx.Layout`) was not
exercised. What is confirmed: `keyed.Deferred` preserves state across Subject
emissions with the same call structure a Gio app would use. A real drag UI is
Phase 1 work once a drag-gesture recipe exists.
