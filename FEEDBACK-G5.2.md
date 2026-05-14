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
