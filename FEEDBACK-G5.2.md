# FEEDBACK-G5.2 — Dogfooding findings from the feeds app (RSS / reading-list)

Findings from building `feeds/` (Phase 5 sub-goals G5.2a–d) against the
Cadence + Spectrum + Prism + mvu stack. The app exercised the
content-shaped slice of the framework end-to-end: accordion-grouped
sidebar, sortable/filterable/paginated articles table, tabbed article
detail in a draggable split pane, navbar popover, header tooltip, and a
full CRUD flow (modal form + alert + toasts + hover-revealed
delete-confirm popovers), all driven by a single MVU model.

Entries are classified into four buckets and severity-tagged
**blocker / major / minor**. Blockers and majors each carry a one-line
remediation sketch. Each entry cites the milestone slice (G5.2a / b / c /
d) and package it surfaced under. Findings that were already discharged
mid-milestone (by GX.6, GX.8, GX.10) are kept with an explicit
*Resolved* note so the re-plan does not double-queue them; the milestone
shipped against their fixes.

A process note worth keeping: the deepest defects (the SplitPane data
race, the rx delivery cluster) were found not by writing new framework
code but by composing existing components in the configuration the
framework itself recommends — which is exactly what Phase 5 is for.

---

## Bugs

### Framework

#### [Blocker] `shell.Shell(SplitPane)` dragState races between the emission projector and frame layout — `cadence/shell` (G5.2c)

`splitPaneObservable` writes `ds.current = clampRatio(r)` inside its
`rx.Map` projector (shell.go:219, rx scheduler goroutine) while the emitted
widget reads `ds.current` during layout (shell.go:226, frame goroutine).
Feeding `Props.SplitRatio` from the MVU model — the composition the
framework steers consumers toward — makes every model emission a write;
`go test -race` failed on every run of the feeds re-emission test once
SplitRatio was model-derived.

Context: feeds chose the right-hand detail pane (over stack-below) because
the detail view is tall and 800 px windows put a stacked pane below the
fold — making feeds the first app to pressure-test SplitPane, which
surfaced this immediately.

**Workaround (shipped):** `feeds/split.go` re-implements the divider
MVU-pure — the widget closes over the ratio value carried by the emission,
drags land `SetSplitRatio` messages, drag-grab state stays frame-side.
~150 lines, mostly a copy of the component: exactly the duplication
cadence exists to prevent.

**Remediation:** restructure `splitPaneObservable` the same way (close
over the per-emission ratio; keep grab state frame-side; drop the mid-drag
"external update wins" branch — drags flow back in-band via
OnSplitChange). Then audit every rx.Defer-scoped state struct in cadence
for projector-side writes; popover's `st.opened` transitions also run in
the projector and escape only because nothing frame-side reads them.

#### [Blocker] *(Resolved mid-milestone by GX.8/GX.10)* Cadence interactive-pattern callbacks lacked `gtx` — consumers could not route through mvu `MessageOp`; invalidation contract broken — `cadence/{accordion,table,pagination,sidebar}` (G5.2a–b)

The defect FEEDBACK-G5.1 traced reproduced identically in feeds: sidebar
selection, pagination, and sort clicks updated state on `rx.Goroutine`
through Subject + atomic-pointer controllers, no Gio frame was requested,
and the UI repainted only on the next unrelated input ("click does nothing
until the mouse moves"). GX.8 gave cadence callbacks `(gtx, value)`
signatures and GX.10 migrated feeds onto Model/Update/MessageOp; G5.2c–d
were then built on the fixed architecture with zero recurrence (see the
first Ergonomics win below). Kept for the record; nothing left to queue.

### Infrastructure

#### [Blocker] reactivego/rx delivery is unreliable under load — stalls, deadlocks, and dropped emissions — `reactivego/{rx,scheduler}` (G5.2d)

Three distinct failure signatures while stabilising the feeds suite, none
attributable to app wiring:

1. **Delivery stall on a cold chain.** A freshly subscribed, fully cold
   chain (`backdropLayer` over `rx.Of` token observables — no Subjects, no
   multicast) intermittently delivers nothing for seconds: one full-suite
   `go test` run hung indefinitely (killed at 3.5 min; the identical suite
   then passed in 0.4 s); under `-race` the suite flaked ~1-in-3 at its
   first `collectOne`; 10/10 isolated runs of the same test pass.
2. **Subject send deadlock.** With the shell graph's 16 consumers on a
   buffer-1 `rx.Subject[Model]`, `send.Next(m)` blocked forever
   (subject.go:163) against a delivery goroutine parked in
   `sync.Cond.Wait` (subject.go:242); the suite died on the 3-minute test
   timeout in two different tests on consecutive runs. The never-advancing
   cursor is plausibly the documented unsubscribe-path race
   (multicast.go:68/100 vs :28; see feeds/wiring_test.go) triggered by
   pagination's SwitchMap unsubscribing on every emission.
3. **Completion without emission.** Once, the same cold token chain
   delivered `done` (nil error) having emitted no value at all.

**Workarounds (shipped, test-side):** deep-buffered model Subjects
(`rx.Subject[Model](0, 1, 256)`) so `Next` cannot block on a wedged
reader, plus `collectOne` retrying (3 × 2 s) on timeout AND on spurious
completion. 0/8 full-suite `-race` failures after both. Production shares
the exposure for signatures 1 and 3 (a stalled or dropped launch emission
= a blank layer until an unrelated re-emission); signature 2 does not
apply because mvuWin.Messages drains a channel, not a Subject.

**Remediation:** queue a consolidated re-plan item — diagnose or vendor
reactivego/rx, or front the layer graph with a delivery-guaranteed adapter
owned by spectrum. With the unsubscribe race and AutoConnect-count
fragility (below) this is the third-and-strongest reliability finding
against the rx substrate.

#### [Major] `Publish().AutoConnect(N)` makes the app's launch correctness hostage to a hand-measured subscription count — `feeds/app.go` + reactivego/rx (G5.2b–d)

`modelObsConsumers` must equal the EXACT number of cold subscriptions the
shell graph makes (9 → 13 → 16 across G5.2b/c/d): too low and late
consumers miss the seed (blank launch panes), too high and Connect never
fires (frozen app). The count is not derivable from the source by
inspection — it includes per-derivation fan-out — so feeds carries a
measuring test (`TestModelObsConsumerCountMatchesConst`) whose failure
message is the only reliable way to learn the new value after any
topology edit.

**Remediation:** replace the non-replaying Publish/AutoConnect seam with a
replay-1 multicast for the model observable (late subscribers receive the
current model; the count stops being load-bearing), or have spectrum own a
model-distribution adapter that hides the counting.

### PLAN.md milestone-spec (no framework defect)

- **[Minor] G5.2a Specific cites `prism/initial` as part of the window
  bootstrap** — the actual bootstrap is `mvu.NewWindow` +
  `spectrum/window.New` + `spectrum/system.LiveTheme`; `prism/initial` is
  the first-frame `Value[T]` helper. Same stale citation as G5.1.
- **[Minor] G5.2a Budget says "Depends on GX.7"** — GX.7 was discharged
  before the milestone started; the sidebar slot composed first-try.
- **[Minor] G5.2c Specific says "tooltips on the table's icon-only column
  headers" (plural)** — a content table naturally has about one icon-only
  column (Unread "•"); the plural overestimates. One tooltip shipped.

---

## Missing API affordances

#### [Major] `cadence/popover` couples anchor placement, dismissal area, and content sizing to one canvas — `cadence/popover` (G5.2c, ×N rows in G5.2d)

Three couplings, all to `gtx.Constraints.Max` of wherever the popover is
laid out: (1) the anchor is centred in the canvas; (2) the outside-press
dismissal absorber covers exactly the canvas; (3) `Content` is measured
against `canvas/2 × canvas/2`. A popover anchored to a navbar button (or a
sidebar trash icon) therefore composes correctly at NO canvas size:
button-sized canvases break dismissal and content measurement,
window-sized canvases re-centre the anchor mid-window. feeds ships
button-sized `Exact` wrapper canvases with the degradations papered over —
Content ignores its incoming constraints and self-sizes; dismissal falls
back to anchor-toggle + item-click; the Share wrapper's 160 dp width is a
hand-tuned guard against the Bottom-placed surface clipping the window
edge. G5.2d multiplied the workaround across every sidebar row's
delete-confirm popover (keyed by FeedID via prism/keyed).

**Remediation:** decouple the three concerns — wrap the anchor in place
instead of centring it, register the dismissal absorber against window
extents (the modal scrim already does), and measure Content against an
explicit `MaxSize` prop.

#### [Major] `cadence/table` has no row-click affordance; click must be smuggled into one cell — `cadence/table` (G5.2b)

`Column[T].Cell` is the only per-row hook — no `OnRowClick`, no `Selected`
highlight state. feeds wires a `widget.Clickable` inside the Title
column's Cell (keyed by ArticleID via `keyed.Defer`), so only Title-column
clicks select an article; Author/Published/Unread cells are inert. Doing
better requires either reusing one Clickable across cells (Gio forbids
two `Clickable.Layout` calls per frame) or a parallel row-level pointer
layer outside the table.

**Remediation:** extend `table.Props[T]` with
`OnRowClick func(gtx, item T)` and optionally `Selected rx.Observable[K]`;
the table's `drawRow` is the only place with the per-row geometry needed
to register the hit area.

#### [Major] `cadence/table` headers are string-only — header tooltips need coordinate arithmetic against private constants — `cadence/table` (G5.2c)

`Column.Header` is a `string` and the header row is drawn internally, so
the Unread ("•") header tooltip is an overlay positioned by hand: trailing
pinned column ⇒ `x ∈ [tableW − 96dp, tableW]`, header ⇒ `y ∈ [0, 44dp]`,
where 96 mirrors the column `Width` and 44 mirrors the table's **private**
`headerHDp`. A future change to the header height silently misaligns the
hit area — no compile-time tether exists.

**Remediation:** `Column.HeaderWidget layout.Widget` (string stays the
simple case), or at minimum export `table.HeaderHeight` and document the
overlay recipe.

#### [Major] `table.RenderTextCell` takes tokens by value, forcing atomic token mirrors in every consumer — `cadence/table` (G5.2b)

Cell closures are built once and run on every frame, outside any rx.Defer
scope — honouring theme switches means each consumer subscribes
`theme.Color`/`theme.Type` on `rx.Goroutine` into atomic mirrors and reads
them per cell render. That is heavy plumbing for the most common cell
(plain text) and duplicates subscriptions the table already holds for its
header and dividers.

**Remediation:** a `table.TextCell(s)` helper resolved against tokens the
table already injects, or a Cell API receiving `(item, tok)` per
emission.

#### [Major] `toast.Notify` is a package-global side-channel — toast policy cannot live in the reducer — `cadence/toast` (G5.2d)

Toasts fire from view callbacks, not from `Update`: the Add-feed success
toast fires in the submit button's OnClick, which must duplicate the
reducer's "non-empty URL" validity check — two sources of truth for one
policy. The global Subject would also leak across windows in a
multi-window app.

**Remediation:** toast support for `mvu.Command` (reducer returns
`toast.Show(...)`) or `toast.NewStack()` returning an instance-scoped
`(Notify, Stack)` pair.

#### [Major] `prism/input.TextField` is uncontrolled — the app can never clear or set the field — `prism/input` (G5.2d)

The `widget.Editor` lives unexported in the component's rx.Defer scope;
the only output is `OnChange`. feeds mirrors the latest text into an
atomic cell for the submit callback — and after a successful submit the
field cannot be cleared, so reopening the Add-feed modal shows the stale
URL. A controlled-input option is table stakes for form CRUD.

**Remediation:** accept an optional `Value rx.Observable[string]` that
overwrites editor content on emission, or expose a reset handle in the
props.

#### [Major] *(Resolved mid-milestone by GX.6)* `go mod tidy` in a new workspace module required multi-pass replace discovery — repo infra (G5.2a)

Bootstrapping `feeds/` required three manual tidy passes to discover
transitive `replace` directives. GX.6a–d consolidated the per-package
go.mods into one module per top-level project, after which new apps
inherit the full replace graph from four parents. Nothing left to queue.

#### [Minor] `table.OnSort` delegates the full sort-cycle state machine to every consumer — `cadence/table` (G5.2b)

`OnSort` delivers only a column index; the documented None→Asc→Desc cycle
must be reimplemented per consumer (feeds ships a simpler Asc/Desc toggle
— two consumers will diverge). Ship `table.CycleSort(cur, col) Sort` or
emit the resolved next state.

---

## Awkward compositions / boilerplate

#### [Major] Static `layout.Widget` slots over observable children — the layer-boundary-cell pattern is the app's single biggest boilerplate source — `cadence/{shell,tabs,navbar,modal,card,pagination}` (G5.2b–d)

The recurring shape: a component exposes a static `layout.Widget` slot,
but the thing that belongs in the slot is model-derived and therefore an
`rx.Observable[layout.Widget]`. The consumer folds the child stream into a
CombineLatest that drives some live slot (the shell's Sidebar), stores the
latest child widget into an `atomic.Value` cell in the projector, and
reads the cell from the static slot at frame time. Instances in feeds:
shell `Main`; SplitPane `Left`/`Right`; tabs `Tab.Content` (selected
article's body — the SwitchMap alternative re-subscribes `Selected`
against a non-replaying published model and would miss in-flight
emissions, so the cell is the *safer* workaround); navbar `Actions` (the
Share popover); modal `Body` + card `Body` (cardCell + fieldCell +
submitCell + alertCell + an errorCell mirror). `pagination.Props.Page` /
`PageCount` are the same defect in scalar form — static ints captured at
construction, worked around by SwitchMap-rebuilding the whole component
per page change (fresh subscription and Clickable slice per click, and
inconsistent with `table.Props.Sort`, which IS an observable).

**Remediation:** one policy decision, applied across cadence: slots accept
`rx.Observable[layout.Widget]` (and scalar props accept observables) the
way shell.Sidebar and table.Sort already do — or prism ships a single
`slot.Bridge` helper that packages the fold-and-cell dance. Either ends
eight-plus hand-rolled instances in one app.

#### [Minor] *(Resolved mid-milestone by GX.10)* `openController` was copy-pasted verbatim from sitedocs — `feeds/sidebar.go` (G5.2a)

The 30-line Subject-based accordion controller duplicated sitedocs'. GX.10
replaced it with the `ToggleSection` reducer case (which also owns the
single-open invariant in one message instead of N+1 OnToggle calls).
Nothing left to queue.

#### [Minor] Two idioms for "is this overlay open" in one app — model state vs ephemeral per-row Subjects (G5.2d)

The Add-feed modal's open flag is Model state (`addFeedOpen` + messages);
the per-row delete-confirm flags are ephemeral interaction state in
per-row rx.Subjects (N rows of open-flag messages would bloat the
reducer). Both are defensible; the framework should pick and document one
idiom before every app invents its own split.

---

## Ergonomics wins worth preserving

#### [Preserve] `shell.Props.Sidebar` as `rx.Observable[layout.Widget]` — the GX.7 remediation held up across the whole milestone

The accordion-grouped sidebar wired into `shell.Shell` first-try in G5.2a
and absorbed every later change (model-derived open state, mutable feed
list, hover trash gutters) with no shell-side friction. This slot is the
proof-of-shape for the static-slots remediation above: make the other
slots look like this one. (G5.2a's other early win — `rx.Subject[T](0, 1)`
selection controllers — was retired by GX.10's model loop and is further
deprecated by the rx delivery findings; it is intentionally not carried
forward.)

#### [Preserve] `(gtx, value)` callbacks + `mvu.MessageOp` — every post-GX.10 interaction wired first-try

Across G5.2c–d: tabs.OnSelect, popover.OnDismiss, shell-replacement drag,
modal OnClose, button OnClick, and plain Clickable anchors all routed
through `mvu.MessageOp` with zero controller code and same-frame repaint —
ten new message types and not one atomic mirror in any interaction path.
The G5.1/G5.2 blocker remediation has fully paid off.

#### [Preserve] Reducer-owned policy stayed pure and testable as the Model grew real

SubmitFeed (empty → alert, non-empty → append + close), ConfirmDelete
(remove + selection fallback + page reset), ToggleSection (single-open
invariant), SelectFeed (page reset) are all pure reducer cases with
table-driven tests. Moving the feed tree from fixture into `Model.feeds`
was mechanical — the MVU shape held up under real mutation.

#### [Preserve] `tooltip.Tooltip`'s self-contained hover machinery composes as an overlay with zero model wiring

Hover detection, show-delay (`gtx.Now` + `InvalidateCmd`), and arbitration
are all internal, so positioning a trigger-sized canvas over the header
cell was the whole job — verified headlessly through a real input.Router.
This is the right amount of encapsulation for hover affordances.

#### [Preserve] `gesture.Hover` (Enter/Leave only) under clickable rows — hover-reveal without press conflicts

The sidebar trash gutter reveals on row hover without ever claiming the
row's select press; a router-driven regression test
(TestHoverGutterDoesNotSwallowSelectPress) pins the behaviour. A recipe to
document, not an API gap.

#### [Preserve] Overlays fold onto the shell stream — no extra window layers

The modal scrim and toast stack draw over the whole window by combining
onto the shell observable and painting after the shell widget (returning
the shell's dims) — no third buildLayers layer, no z-order machinery, and
every model change still re-emits the stream for same-frame repaint.

#### [Preserve] Headless pixel + input.Router verification against the real composed shell

The G5.2c/d sim tests render the actual shell at successive model states
and assert pixel deltas (detail pane populates, tabs swap, popover
opens/dismisses-exactly, modal/alert/toast/delete flows), and drive
hover/press through `gioui.org/io/input.Router` where timing matters.
This caught real geometry bugs that reducer tests cannot see, runs in CI
without a window server, and is the verification pattern Phase 5 apps
should standardise on.

*No section is empty; all four buckets accumulated findings — consistent
with an app slice that exercised eleven cadence/prism components in
earnest.*
