# FEEDBACK-G5.2 — Dogfooding notes from the feeds app skeleton

Findings from building `feeds/` — the first non-docs app to exercise
`cadence/shell` with the fixed `rx.Observable[layout.Widget]` sidebar slot
(GX.7 remediation). The app composes accordion-grouped sidebar + navbar +
placeholder main from hard-coded fixture data driven by an `rx.Subject[FeedID]`.

---

## Bugs

### Framework

#### [Blocker] Same as FEEDBACK-G5.1 [Blocker] "Cadence interactive-pattern callbacks lack `gtx` → consumers cannot route through mvu `MessageOp` → invalidation contract broken" — `cadence/{accordion,table,pagination,sidebar}` (G5.2a–b)

The `feeds/` app exhibits the same bug surfaced in sitedocs and traced in FEEDBACK-G5.1's [Blocker] entry. Reproduction in feeds:

- Clicking a sidebar feed entry: the table contents update, but **not until the user moves the mouse**. The `selectionController` Subject emits and stores into an `atomic.Pointer` on `rx.Goroutine`; the outer view observable doesn't re-emit because its inputs (theme + shape) haven't changed; mvu's invalidation hook never fires.
- Clicking a pagination button: same chain. `pageSend.Next(p)` runs on the click goroutine, the table re-derives on rx.Goroutine, but no Gio frame is requested.
- Clicking a sortable column header: same chain via `OnSort`.

Every interactive control in feeds reaches for the same `rx.Subject + atomic.Pointer + rx.Goroutine` workaround pattern because cadence's callback signatures (`OnSelect func(p int)`, `OnSort func(col int)`, `OnRowClick` — not yet exposed) don't carry `gtx.Ops`, so consumers cannot emit `mvu.MessageOp`. Three more entries in this file (`pagination.Props.Page is static int`, `table has no row-click affordance`, `table.OnSort delegates full state machine`) describe the cadence-side gaps that *compound* the same root issue: callback APIs decoupled from `layout.Context` force consumers to bypass mvu and rebuild the framework's plumbing, badly.

**Remediation:** see FEEDBACK-G5.1 [Blocker]. The Phase-5 dogfooding lesson is the same in both apps and gets stronger with each example — fix the cadence callback shape, fix every Phase-5 app's perceived-lag bug at once.

---

## PLAN.md milestone-spec (no framework defect)

- **[Minor] G5.2a Specific cites `prism/initial` as part of "opens a window via
  `prism/initial + spectrum` theme"** — also noted in G5.1 feedback. The actual
  bootstrap is `mvu.NewWindow` + `spectrum/window.New` + `spectrum/system.LiveTheme`.
  `prism/initial` is the first-frame `Value[T]` sentinel helper, not a window API.
- **[Minor] G5.2a Budget "Depends on GX.7"** — GX.7 was already discharged before
  this milestone (shell.Props.Sidebar is now `rx.Observable[layout.Widget]`).
  The dependency note is stale but harmless; the composition worked first-try.

---

## Missing API affordances

#### [Major] `go mod tidy` in a new workspace module requires multi-pass replace discovery — repo infra (G5.2a)

Adding a new module (`feeds/`) to the workspace required discovering transitive
indirect deps (`cadence/sidebar`, `prism/layout`, `prism/internal/golden`) that
fail `go mod tidy` with git-remote errors before `replace` directives exist.
Each missing dep requires an extra tidy pass. This repeats the finding from
G5.1 feedback ("Workspace `use` requires per-module `replace`"). Three manual
iterations were needed before tidy succeeded.

**Remediation:** the FEEDBACK-G5.1 suggestion stands — document the
multi-pass bootstrap in `MIGRATION.md` (or a CONTRIBUTING.md), or collapse
per-module go.mods into per-phase modules (GX.6) so new apps inherit the
full replace graph from a single parent.

---

## Awkward compositions / boilerplate

#### [Minor] `openController` is re-implemented in feeds/sidebar.go verbatim from sitedocs/docs_sidebar.go

The 30-line `openController` struct (mutex, open-map, Subject, toggle) is
copy-pasted unchanged. This is the same issue logged as a major in G5.1
("accordion.OnToggle + Open rx.Observable reinvents the Subject pattern per
caller"). Extracting `accordion.NewController` would eliminate the copy for
both callers.

#### [Major] `pagination.Props.Page` / `PageCount` are static ints, not observables — every page change re-subscribes (G5.2b)

`cadence/pagination.Props` declares `Page int` and `PageCount int`. Both are
captured by value inside the `rx.Defer` body and the `rx.Map` projector, so
the emitted widget closures permanently bind whatever values were passed at
construction time. There is no in-band way to advance the page after that.

The feeds composition works around this by re-constructing the entire
Pagination observable on every page or pageCount change:

```go
paginationWidgetObs := rx.SwitchMap(
    rx.CombineLatest2(pageObs, pageCountObs),
    func(t rx.Tuple2[int, int]) rx.Observable[layout.Widget] {
        return pagination.Pagination(th, pagination.Props{
            Page: t.First, PageCount: t.Second,
            OnSelect: func(p int) { pageSend.Next(p) },
            Shaper:   shaper,
        })
    },
)
```

Each click forces a fresh subscription, a fresh `widget.Clickable` slice, and
a fresh shaper-defaulting branch in the `rx.Defer` body. The user just clicked
something so no pending events are lost in practice, but the cost-per-click is
real, the API is inconsistent with `cadence/table.Props.Sort`
(`rx.Observable[Sort]`), and the workaround pattern is non-obvious.

**Remediation:** make `pagination.Props.Page` and `PageCount` accept
`rx.Observable[int]` (or accept a single `rx.Observable[pagination.State]`).
Combine them with the theme observable inside `Pagination` so the inner
closures observe their latest values without re-subscription.

#### [Major] `cadence/table` has no row-click affordance; click must be smuggled into one cell (G5.2b)

`table.Column[T].Cell` produces a `layout.Widget` per cell — there is no
per-row hook, no `OnRowClick`, and no `Selected` highlight state. The feeds
articles table satisfies "clicking a row emits `selectedArticle`" by wiring
a `widget.Clickable` only inside the Title column's Cell closure and keying it
by `ArticleID` via `keyed.Defer`:

```go
titleCell := func(a article) layout.Widget {
    click := rowClicks.For(a.ID)
    body := table.RenderTextCell(shaper, *colorPtr.Load(), *typePtr.Load(), a.Title)
    return func(gtx layout.Context) layout.Dimensions {
        if click.Clicked(gtx) { selectArticle.Next(a.ID) }
        return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
            // …
            return body(gtx)
        })
    }
}
```

This means only the Title column is actually clickable — clicks in Author /
Published / Unread cells are inert. A real "select-row" UX needs the click on
the whole row (and visual feedback). Replicating that today requires either
duplicating the same `widget.Clickable` across every column's Cell (which Gio
forbids — one `Clickable.Layout` per frame) or building a parallel row-level
pointer-event layer outside the table.

**Remediation:** extend `table.Props[T]` with `OnRowClick func(item T)` and
optionally `Selected rx.Observable[K]` for highlight state. The table can
register a per-row hit area inside its `drawRow` once it knows the row exists,
which is information the consumer cannot reconstruct from the Cell-level API.

#### [Major] `table.RenderTextCell` requires colour/type tokens at construction, forcing atomic-pointer mirrors in the consumer (G5.2b)

`table.RenderTextCell(shaper, colors, typeScale, s)` takes tokens by value.
Inside a reactive composition the Cell closures (built once, called on every
emission) live outside any `rx.Defer` scope and outside the table's own
`rx.Defer`. To honour theme switching they have to read current colours and
typography on every frame — which means the consumer keeps two
`atomic.Pointer` mirrors and subscribes `theme.Color` / `theme.Type` on
`rx.Goroutine`:

```go
var colorPtr atomic.Pointer[tokens.ColorTokens]
var typePtr atomic.Pointer[tokens.TypeScale]
_ = rx.SwitchMap(th, …t.Color).Subscribe(…)
_ = rx.SwitchMap(th, …t.Type).Subscribe(…)
cell := func(a article) layout.Widget {
    return table.RenderTextCell(shaper, *colorPtr.Load(), *typePtr.Load(), s)
}
```

That is a lot of plumbing for the most common cell — plain text. It also
duplicates work the table is already doing internally (the table subscribes
to the same tokens for its header and divider).

**Remediation:** either (a) expose a `table.TextCell(s)` helper that resolves
tokens against an ambient context the table has already injected, or (b)
pass a `Cell` API that receives a `(item, tok)` pair instead of just `item`,
so the table hands current tokens to the cell every emission.

#### [Minor] `table.OnSort` delegates the full Asc → Desc → None state machine to every consumer (G5.2b)

`table.Sort` carries `(Column, Asc)`; `OnSort` is fired with a column index
and nothing else. The package doc suggests cycling
`None → Asc → Desc → None`, but the consumer has to implement that cycle
itself, including reading the current sort state to decide what comes next.
The feeds composition does the simpler Asc/Desc toggle on the same column,
which is a different state machine from the documented suggestion. Two
consumers will almost certainly diverge.

**Remediation:** either ship a stock `table.CycleSort(cur, col) Sort` helper
that returns the next state, or have the table own the cycle internally and
emit `OnSort func(sk Sort)` with the resolved next state.

---

## Ergonomics wins worth preserving

#### [Preserve] `shell.Props.Sidebar` as `rx.Observable[layout.Widget]` (GX.7)

The accordion-grouped sidebar wired directly into `shell.Shell` without a
`composeSidebarHeaderMain` workaround. The G5.1 blocker is fully resolved —
the composition worked first-try with zero local reimplementation.

#### [Preserve] `rx.Subject[T](0, 1)` for selection state

`selectionController` follows the same Subject + atomic.Pointer pattern as
`pageController` in sitedocs. The buffer-1 replay and goroutine subscriber
worked correctly; `TestSelectionControllerAdvancesAtomic` validated the
full chain (set → Subject → rx.Goroutine → atomic.Pointer) in < 100 ms.

---

# G5.2c — article detail view (tabs + popover + tooltip + split)

Findings from building `feeds/detail.go`, the Share popover, the Unread
header tooltip, and the articles/detail split — the first G5.2 sub-goal
composed entirely on the post-GX.8/10 architecture (model-derived
observables + `mvu.MessageOp` callbacks + layer-boundary cells).

## Layout choice (required log)

**Chose: right-hand detail pane via a split pane; planned
`cadence/shell.Shell(SplitPane)`, shipped a feeds-local replacement
(`feeds/split.go`) after `-race` flagged the component.** Rationale for the
right-hand pane over stack-below: the detail view is tall (header + tab
strip + wrapped body), and stacking it under a 10-row table pushes it below
the fold at 800 px window height; side-by-side keeps both halves usable and
pressure-tests SplitPane, which no Phase-5 app had touched yet. That
pressure-test immediately found the blocker below, which is the point of
Phase 5.

## Bugs

### Framework

#### [Blocker] `shell.Shell(SplitPane)` dragState races between the emission projector and frame layout — `cadence/shell` (G5.2c)

`splitPaneObservable` allocates a per-subscription `dragState` and writes
`ds.current = clampRatio(r)` inside the `rx.Map` projector (shell.go:219),
which runs on the rx scheduler goroutine each time a colour token or
SplitRatio value emits. The emitted widget reads `ds.current` during layout
on the frame goroutine (shell.go:226). `go test -race` fails on the first
model-driven SplitRatio emission that overlaps a frame:

```
WARNING: DATA RACE
Write at 0x… by goroutine 102: cadence/shell.splitPaneObservable.func2.1()  shell.go:219
Previous read at 0x… by goroutine 8:  cadence/shell.splitPaneObservable.func2.1.1()  shell.go:226
```

Feeding `SplitRatio` from the MVU model — the composition the framework
itself steers consumers toward — makes every model emission a write. The
race is not theoretical: it reproduced on every run of the feeds re-emission
test once SplitRatio was model-derived.

**Workaround (shipped):** `feeds/split.go` re-implements the two-pane
divider MVU-pure: the emitted widget closes over the ratio *value* carried
by the emission, divider drags land `SetSplitRatio` messages through the mvu
loop, and the transient drag tracker is touched only during layout. ~150
lines, mostly a copy of the component — exactly the duplication cadence
exists to prevent.

**Remediation:** apply the same shape inside `cadence/shell`: close over the
ratio per emission instead of mutating `ds.current` from the projector, and
keep `ds.active`/grab state frame-side only. The mid-drag "external update
wins" branch then becomes unnecessary (drags emit through OnSplitChange and
flow back in-band). Audit the other rx.Defer-scoped state structs for
projector-side writes; popover's `st.opened` transitions also run in the
projector but are read nowhere frame-side, so it escapes by luck, not by
design.

## Missing API affordances

#### [Major] `cadence/popover` couples anchor placement, dismissal area, and content sizing to one canvas — (G5.2c)

Three couplings, all to `gtx.Constraints.Max` of wherever the popover widget
happens to be laid out:

1. the anchor is **centred** in the canvas (`anchorPos = (canvas − anchor)/2`);
2. the outside-press dismissal absorber covers exactly the canvas;
3. `Content` is measured against `canvas/2 × canvas/2`.

A popover anchored to a navbar action button therefore cannot be composed
correctly at any canvas size. Button-sized canvas: anchor lands right, but
the dismissal absorber shrinks to the button (outside-clicks elsewhere in
the window do NOT dismiss) and Content gets *half a button* to lay out in.
Window-sized canvas: dismissal works, but the anchor renders in the middle
of the window. feeds ships the button-sized canvas (an `Exact` 160×28 dp
wrapper in the navbar action slot) with both degradations papered over:
the Content widget **ignores its incoming constraints** and sizes itself
(returning its own dims, which popover pads into the surface rect), and
dismissal relies on anchor-toggle plus destination-click instead of
outside-press. The 160 dp wrapper width is itself a hand-tuned collision
workaround — Placement: Bottom centres the surface under the anchor, and a
right-edge button would push half the surface off-screen.

**Remediation:** decouple the three concerns: accept the anchor's laid-out
position (popover as a wrapper around the anchor, not a canvas that centres
it), register the outside-press absorber against the window extents (the
modal scrim already solves this), and measure Content against an explicit
`MaxSize` prop or the window, not `canvas/2`.

#### [Major] `cadence/table` headers are string-only — no widget slot, so header tooltips need coordinate arithmetic — (G5.2c)

`table.Column[T].Header` is a `string` and the header row is drawn
internally by `drawTable`; there is no per-header widget hook (the
row-click finding from G5.2b, one level up). The G5.2c "tooltips on
icon-only column headers" requirement was met by overlaying the tooltip's
trigger canvas on top of the table widget at hand-computed coordinates:
trailing pinned column ⇒ `x ∈ [tableW − 96dp, tableW]`, header ⇒
`y ∈ [0, 44dp]`, with 96 mirroring the column's `Width` and 44 mirroring the
table's **private** `headerHDp`. Any future change to the table's header
height silently misaligns the hit area — there is no compile-time tether.

Also: the articles table has exactly **one** plausibly icon-only header
(Unread, now "•"); the plan's plural "headers" overestimates how many
icon-only columns a content table naturally has.

**Remediation:** either `Column.HeaderWidget layout.Widget` (string Header
as the simple case) or an exported `table.HeaderHeight` constant plus a
documented overlay recipe. The former also unlocks header-level affordances
the plan keeps asking for (tooltips here, filter chips later).

## Awkward compositions / boilerplate

#### [Major] `tabs.Tab.Content` is a static `layout.Widget` captured at construction — model-dependent content needs cell bridges — (G5.2c)

`tabs.Props.Tabs` is a plain slice read once; `Selected` is an
`rx.Observable[int]` but each `Tab.Content` is a static widget. Detail-pane
content depends on the selected article (model state), so the three Content
closures read an `atomic.Value` article cell that the detail layer's
combine-map stores synchronously before each emission — the mainCell
pattern again, now one component deeper. The alternative (rebuilding the
whole Tabs instance per article via SwitchMap) re-subscribes `Selected` on
every article click against a non-replaying published model observable,
which would miss the in-flight emission — subtle enough that the cell
bridge is the *safer* of two workarounds for what should be a directly
expressible composition ("tab content renders current model state").

**Remediation:** same one-prop fix as pagination in G5.2b: accept
`Content rx.Observable[layout.Widget]` (or give Cell-style closures an
in-band value), so consumers fold model-derived content without a cell.

#### [Minor] `navbar.Props.Actions` are static widgets — live action widgets (the Share popover) need the same cell bridge

`Actions []layout.Widget` is captured once. The Share popover is an
observable widget stream, so it reaches its action slot via the shareCell
layer-boundary adapter + an `Exact`-canvas wrapper. Fourth instance of the
static-slot-needs-a-cell pattern in this app (Main, SplitPane Left/Right,
tabs Content, navbar Actions). The pattern is now well-understood
boilerplate, which is precisely the argument for the framework absorbing
it.

## Ergonomics wins worth preserving

#### [Preserve] `(gtx, value)` callback signatures + `mvu.MessageOp` — every G5.2c interaction wired first-try

`tabs.OnSelect(gtx, idx)`, `popover.OnDismiss(gtx)`, and plain
`widget.Clickable` anchors all routed through `mvu.MessageOp` with zero
controller code and same-frame repaint. The G5.1/G5.2 [Blocker] callback
remediation has fully paid off: five new message types (SelectTab,
ToggleShare, CloseShare, SetSplitRatio, plus the now-consumed
SelectArticle) and not one atomic mirror in the interaction path.

#### [Preserve] `tooltip.Tooltip` self-contained hover machinery composes cleanly as an overlay

The tooltip needed no model wiring at all: hover detection, show-delay
(`gtx.Now` + `InvalidateCmd`), and arbitration are all internal, so the
overlay composition (position a trigger-sized canvas, done) worked
first-try — including headless router-driven verification. This is the
right amount of encapsulation; the popover's split of Open-state ownership
(consumer) vs dismissal detection (component) is the same idea and also
worked, modulo the canvas coupling above.

#### [Preserve] Layer-boundary cells stay mechanical

All four new cells (detail article, splitCell, shareCell, articles/detail
pane cells) follow the identical store-in-projector / read-at-frame shape
with no surprises. Boilerplate, but *predictable* boilerplate — a good sign
the eventual framework affordance can be a single helper.

---

# G5.2d — CRUD actions (modal + toast + alert + delete-confirm)

Findings from wiring the Add-feed modal (cadence/modal + cadence/card +
prism/input/textfield + prism/button + cadence/alert), the toast stack, and
the hover-revealed per-row delete-confirm popover. The feed tree moved from
a static fixture into the Model (`Model.feeds`), with SubmitFeed /
ConfirmDelete reducer policies (empty-URL alert, selection fallback on
deleting the selected feed).

## Bugs

### Infrastructure

#### [Blocker] reactivego/rx delivery is unreliable under load — stalls, deadlocks, and dropped emissions (G5.2d)

Three distinct failure signatures surfaced while stabilising the feeds
suite, all inside reactivego/rx / reactivego/scheduler, none in app wiring:

1. **Delivery stall on a cold chain.** A freshly subscribed, fully cold
   chain (`backdropLayer` over `rx.Of` token observables — no Subjects, no
   multicast) intermittently delivers nothing for seconds. One full-suite
   `go test` run hung indefinitely (killed at 3.5 min; the identical suite
   then passed in 0.4 s); under `-race` the suite flaked ~1-in-3 at its
   FIRST `collectOne` ("layer 0 produced no widget"); 10/10 isolated runs
   of the same test pass.
2. **Subject send deadlock.** With the shell graph's 16 consumers on a
   buffer-1 `rx.Subject[Model]`, `send.Next(m)` blocked FOREVER
   (subject.go:163) while a delivery goroutine sat in `sync.Cond.Wait`
   (subject.go:242) — the suite died on the 3-minute test timeout, in two
   different tests on consecutive runs. The cursor that never advances is
   plausibly the documented unsubscribe-path race (multicast.go:68/100 vs
   :28, see feeds/wiring_test.go) triggered by pagination's SwitchMap
   re-subscribing — and therefore unsubscribing — on every emission.
3. **Completion without emission.** Once, the same cold token chain
   delivered `done` (nil error) having emitted NO value at all — the
   emission was simply lost.

**Workarounds (shipped, test-side):** model Subjects in shell-graph tests
get a deep buffer (`rx.Subject[Model](0, 1, 256)`) so `Next` cannot block
on a wedged reader — this eliminated the hangs; `collectOne` retries the
subscription (3 × 2 s) on both timeout AND spurious completion — a fresh
Subscribe sidesteps a wedged one. 0/8 `-race` full-suite failures after
both. The production app shares the exposure (mvuWin.Messages drains a
channel rather than a Subject, so signature 2 does not apply, but 1 and 3
do: a stalled/dropped launch emission = a blank layer until an unrelated
re-emission).

**Remediation:** this is now the third-and-strongest reliability finding
against the rx substrate (unsubscribe race, AutoConnect count fragility,
and this delivery cluster). Queue a consolidated re-plan item: diagnose or
vendor reactivego/rx, or front the layer graph with a delivery-guaranteed
adapter owned by spectrum.

## Missing API affordances

#### [Major] `toast.Notify` is a package-global side-channel — toast policy cannot live in the reducer (G5.2d)

`cadence/toast` exposes `Notify(level, text)` writing to a package-scoped
Subject. Toasts are therefore fired from view callbacks, not from `Update`:
the success toast for Add-feed fires in the button's OnClick, which must
duplicate the reducer's "non-empty URL" validity check to decide whether a
toast is warranted — two sources of truth for the same policy, and they can
drift. An mvu app wants toast emission to be a reducer-owned effect (e.g. a
`mvu.Command`), or at minimum an instance-scoped sink, not a global.

**Remediation:** either toast support for `mvu.Command` (reducer returns
`toast.Show(...)` as the command), or `toast.NewStack()` returning a
`(Notify, Stack)` pair so apps can scope and route notification emission.
The global Subject also leaks across windows in any future multi-window
app.

#### [Major] `prism/input.TextField` is uncontrolled — the app can never clear or set the field (G5.2d)

The textfield's `widget.Editor` lives in the component's rx.Defer scope and
is not reachable from outside; the only output is `OnChange(gtx, string)`.
feeds mirrors the latest text into an atomic urlCell for the submit
callback to read — and after a successful submit the field CANNOT be
cleared: reopening the Add-feed modal shows the previously submitted URL.
There is no value-in prop (`Value rx.Observable[string]`) and no reset
affordance. A controlled-input option is table stakes for form CRUD.

**Remediation:** accept an optional `Value rx.Observable[string]` that
overwrites editor content on emission (with the usual caret-preservation
caveats), or expose a `Clear()`/handle in the props.

## Awkward compositions / boilerplate

#### [Minor] `modal.Props.Body` / `card.Props.Body` are static slots over observable children — four more cell bridges (G5.2d)

modal Body and card Body are `layout.Widget`, while the children that fill
them (card, textfield, button, alert) are `rx.Observable[layout.Widget]`.
The Add-feed modal needed cardCell + fieldCell + submitCell + alertCell plus
an errorCell bool mirror, all folded through a CombineLatest5 — the same
layer-boundary-cell pattern for the sixth, seventh, eighth time in this app
(Main, Left/Right, tabs Content, navbar Actions, now modal/card Bodies).
The pattern is mechanical and reliable, but its volume is now the app's
single biggest boilerplate source. (Same remediation as G5.2c: observable
slots, or a framework helper that folds-and-bridges in one call.)

#### [Minor] Per-row delete-confirm popovers re-hit the popover canvas coupling, ×N rows (G5.2d)

Each sidebar row wraps its own `cadence/popover` in an Exact trash-gutter
canvas (the G5.2c Share-popover workaround, now multiplied across rows and
keyed by FeedID via prism/keyed). Outside-press dismissal is again limited
to the tiny canvas; dismissal relies on trash-toggle + confirm-click.
Also a state-ownership wrinkle: the confirm-open flag is EPHEMERAL per-row
interaction state held in a per-row rx.Subject, while the Add-feed modal's
open flag is Model state — two idioms for "is this overlay open" in one
app. Defensible (N rows × open-flag messages would bloat the reducer), but
the framework should pick and document one idiom.

## Ergonomics wins worth preserving

#### [Preserve] `gesture.Hover` composes hover-reveal cleanly under clickable rows

The trash icon reveals on row hover with `gesture.Hover` (Enter/Leave only,
never Press), so the row's select-click and the trash click coexist without
event-claim conflicts — verified by a router-driven regression test
(TestHoverGutterDoesNotSwallowSelectPress). No framework affordance needed;
this is a good recipe to document.

#### [Preserve] Reducer-owned CRUD policy stayed pure and testable

SubmitFeed (empty → alert, non-empty → append+close) and ConfirmDelete
(remove + selection fallback to first remaining feed + page reset) are pure
reducer cases with table-driven tests; moving the feed tree into the Model
was mechanical. The MVU shape held up well under real mutation — the first
sub-goal where the Model is not just view state.

#### [Preserve] Overlay layers fold onto the shell stream without new buildLayers entries

The modal scrim and toast stack draw over the whole window by folding onto
the shell observable and painting after the shell widget (reporting the
shell's dims) — no third buildLayers layer, no z-order machinery. Cheap and
predictable; candidate for the documented overlay recipe.
