# FEEDBACK-G5.3 — Dogfooding findings from the watchlist editor

Findings from building `watchlist/` (Phase 5 sub-goals G5.3a–c) against the
Cadence + Spectrum + Prism + mvu stack. The app is a from-scratch persistent
CRUD editor: a JSON-backed watchlists sidebar (G5.3a), a symbols table with an
add/edit modal (G5.3b), and the full interaction surface — row + bulk delete via
popover confirms, header tooltips, a right-click sidebar context menu with a
rename modal, and conditional pagination (G5.3c) — all driven by a single MVU
model on the post-GX.8/GX.10 architecture.

Entries are classified into four buckets and severity-tagged
**blocker / major / minor / preserve**. Blockers and majors each carry a
one-line remediation sketch. Each entry cites the milestone slice
(G5.3a / b / c) and the package it surfaced under. Findings that recur from
FEEDBACK-G5.2 are cross-referenced rather than re-litigated, but each recurrence
still gets its own severity-tagged entry so the re-plan can count frequency.

A process note worth keeping: this milestone built a brand-new app directly on
the architecture FEEDBACK-G5.2 left behind, so it is the cleanest available test
of whether the G5.2 remediations were enough. Most were; the most severe NEW
finding (the lazy-modelObs data-loss trap) is a fresh face on the same
AutoConnect-count fragility G5.2 flagged as [Major].

---

## Bugs

### Framework

#### [Major] A `modelObs` mirror subscribed lazily inside `keyed.Defer` is never seeded and is invisible to the count test — a silent data-loss path — `watchlist/{sidebar,sidebarcontext,rowdelete}.go` + reactivego/rx (G5.3c)

The first cut of the per-row delete-confirm and per-name context menu each took
its OWN `modelObs.Subscribe` (mirroring `addSymbolModal`'s mirror). That is a
trap. `keyed.Defer` constructors run LAZILY on the first `.For(key)` during a
LAYOUT frame — which is AFTER `Publish().AutoConnect(N)` has already fired
`StartWith(seed)`. `Publish()` does not replay, so a lazy mirror joins the hot
stream and receives NOTHING until the next message: its `modelCell` holds the
zero `Model{}` (watchlists=nil, selected="") until then. A user who right-clicks
a watchlist → Delete → Confirm BEFORE any other interaction calls
`deleteWatchlistNamed(nil, …)` → `saveStore` an EMPTY document over their file.
Data loss. Worse, the lazy subscription is invisible to
`TestModelObsConsumerCountMatchesConst` (whose subscribe callback never lays
out, so `.For()` never runs), so the count silently UNDERCOUNTS and grows
unbounded as rows are interacted with — corrupting the AutoConnect(N)
seed-delivery invariant the whole app depends on.

This is the same `Publish().AutoConnect(N)` / non-replaying-multicast fragility
FEEDBACK-G5.2 logged as [Major] ("launch correctness hostage to a hand-measured
subscription count"); here it does not merely blank a pane — it overwrites the
user's file with an empty document, because lazy late-joiners read the zero
Model.

**Remediation:** the same one G5.2 asked for — replace the non-replaying
Publish/AutoConnect seam with a replay-1 multicast so late subscribers receive
the current model and the count stops being load-bearing. As shipped, the
workaround is a hard invariant: subscribe ONE eager mirror in each layer's
function body (`watchlistMain`, `watchlistSidebar`) and pass it down as a
`func() Model`; per-row/per-name surfaces read through it and NEVER subscribe
`modelObs` themselves. This makes the consumer count STATIC (independent of
watchlist/symbol count) and seed-correct. New documented invariant in `app.go`'s
`modelObsConsumers` comment: never subscribe `modelObs` inside a `keyed.Defer` —
the count test can't see it and AutoConnect can't seed it. Measured
`modelObsConsumers` rose 11 → 22; the count test caught the const drift at both
the broken (lazy, 20-and-climbing) and fixed (eager, static 22) topologies.

### PLAN.md milestone-spec (no framework defect)

- **[Minor] G5.3a Specific cites `prism/initial` + `spectrum` as the window
  bootstrap** — the real bootstrap is `mvu.NewWindow` + `spectrum/window.New`
  + `spectrum/system.LiveTheme` (copy `feeds/main.go`); `prism/initial` is the
  first-frame `Value[T]` helper, not the live entry point. Same stale citation
  FEEDBACK-G5.1 and FEEDBACK-G5.2 already flagged; re-logged so the eventual
  plan cleanup corrects it for the whole G5.3 milestone. No code impact (the
  real recipe was followed).

---

## Missing API affordances

#### [Major] No `prism/storage` (or any framework persistence helper) — every persistent app hand-rolls JSON load/save + path resolution — `prism`/`cadence` (G5.3a)

There is no framework primitive for "read/write a per-user JSON config file."
G5.3a hand-rolled `store.go`: `os.UserConfigDir()` + `filepath.Join` for the
path, `os.ReadFile`/`json.Unmarshal` to load, an atomic `os.WriteFile`-to-tmp +
`os.Rename` to save, plus the first-run-starter and the
absent-file-vs-empty-document branching, plus the injectable-path seam so tests
write to `t.TempDir()` and never touch the real
`~/Library/Application Support`. None of this is hard, but it is boilerplate
every persistent vibrantgio app (coinviz adoption included) re-implements
identically, and the test-isolation seam (path injection) is easy to forget — a
default-path helper that wires the real path in `main()` only is exactly the
footgun a shared helper prevents.

**Remediation:** a small `prism/storage` (or `cadence/storage`) offering
`LoadJSON[T](path)` / `SaveJSON[T](path, v)` (atomic temp+rename) plus a
`UserConfigPath(app, file)` resolver. It would remove the duplication AND
standardise the absent-vs-empty-vs-newer-version contract WATCHLIST-FORMAT.md
currently spells out by hand.

#### [Major] `prism/input.TextField` is uncontrolled — cannot be pre-populated, which now changes user-visible behaviour — `prism/input` (G5.3b add/edit modal AND G5.3c rename modal)

The single biggest friction in this milestone, hit in two modals. The task
requires reopening a modal **pre-populated** with the row's (or watchlist's)
current values, but `prism/input.TextField` is fully UNCONTROLLED: its
`widget.Editor` lives inside the component's `rx.Defer` scope, is allocated once
per subscription, and is never exposed. `TextFieldProps` has `Placeholder`,
`Description`, `OnChange`, `Message`, `Submit`, `OnSubmit`, `Shaper` — and **no
initial-value / value / SetText prop**. (`RenderState.Text` exists for the
STATIC golden path only and "has no effect on the live TextField.")

This is the same uncontrolled-field gap FEEDBACK-G5.2d flagged as [Major] for
the add-feed flow — but G5.3 is the **first place where the gap changes
user-visible behaviour**, not just internal plumbing. The shipped workaround:

1. **Rebuild the field fresh on every open.** The model carries an incrementing
   `modalEpoch`; each field is `rx.SwitchMap` keyed on the epoch, so a new open
   re-subscribes the TextField and gets a fresh (empty) editor. Mandatory, not
   cosmetic: without it, open row A, type "ETH", close, open row B — and the
   field still shows "ETH". Keying on epoch (not `editIndex`) is required so
   reopening the SAME row after a cancel also rebuilds. `OnSubmit`'s internal
   `SetText("")` does not help (it fires only on Enter-submit success, not on a
   `prism/button` submit and not on close-without-submit).
2. **Show the current value as the Placeholder**, and *seed* the field's text
   cell to that same value.
3. **"Empty field on submit = keep the seeded value"** — untouched field → cell
   still holds the seed → that value is submitted; typing replaces it. This is
   the load-bearing decision that makes "edit one field, the others survive."

**The honest limitation this forces:** with "empty keeps the seed", clearing a
previously-set optional field (Notes "foo" → "") is *un-discoverable, not
impossible*. focus+backspace on an already-empty editor fires no change event,
so the seed survives; only type-any-char-then-delete-it fires `OnChange("")`.
The obvious gesture (focus the field showing the old value as a placeholder, hit
delete) does nothing, and the placeholder hides on focus so the original value
is invisible while typing. In the rename modal the same limitation is benign
(empty names are rejected anyway), but in the symbols modal it is a real,
user-visible loss of an affordance.

**Remediation:** a `TextFieldProps.InitialText string` (seed-once into the
editor, NOT a controlled binding) would erase the entire epoch-rebuild
workaround in BOTH modals at once AND restore clear-to-empty semantics.

#### [Major] `mvu.Command` returned by a reducer is a dead path — the canonical run() Scan discards it, so there is no supported reduce-then-effect seam — `mvu` + `watchlist/main.go` (G5.3b)

The save must mutate the in-memory watchlist AND write the full file back.
Reducer purity (mandated post-GX.10) forbids the write in `Update`. `mvu.Command`
exists (`mvu/command.go` has `Do`/`DoConcurrent`/…), but the production seam
ignores it: `main.go`'s `rx.Scan(...)` does `next, _ := Update(model, msg)` —
**the Command is discarded**, exactly as in feeds. So a reducer-returned
`mvu.Do(write)` is dead unless run()'s Scan is rewired, which would disturb the
load-bearing `Publish().AutoConnect(modelObsConsumers)` seed-delivery the
milestone is told not to touch.

Resolution chosen: the write lives in the **submit callback**, which reads a
model-mirror `atomic.Value` fed by `modelObs`, applies the SAME pure
`applyEdit(...)` helper the reducer calls, writes the resulting full `Document`
atomically, and fires the toast. Reducer and callback can never diverge (both
route through `applyEdit`); the store path is injected for tests. The cost: the
mutation logic is *invoked* from two places, and the callback needs a model
mirror just to see the full watchlists the four form cells don't carry. This is
correct ONLY because the modal is exclusive and the fields land no messages
while open (so the mirror holds precisely the open-time model); a non-exclusive
modal or any background mutation while open would break it — the exclusivity
invariant is load-bearing.

**Remediation:** either run()'s Scan should execute the returned Command (an
`mvu` recipe change), or `mvu` should bless and document the callback-effect
pattern as canonical and stop pretending `Command` is wired. The current state —
a `Command` type that the official Scan throws away — is a trap.

#### [Major] No `widget`/`gesture` helper for "right-click this area" — a front-most hit area swallows the PRIMARY press unless wrapped in `pointer.PassOp` — `gioui.org/io/pointer` + `widget` (G5.3c)

The first right-click composition in the codebase. The sidebar context menu
needs a right-click to open the menu WITHOUT breaking left-click-to-select.
`widget.Clickable` does not expose the pressed button, so a raw
`pointer.Filter{Kinds: pointer.Press}` tag is registered over the row and the
drain checks `pe.Buttons.Contain(pointer.ButtonSecondary)`. The tag must be
registered AFTER the select clickable (later = front-most) to see the secondary
press — but a plain front-most registration ABSORBS the primary press too, and
click-to-select silently breaks (proven with a throwaway router probe). The fix
is `pointer.PassOp{}.Push(...)` around the tag registration: pass-through
delivers the press to BOTH the front tag (secondary filter) AND the clickable
behind it (primary). Guarded by
`TestRightClickPassesPrimaryReachesContextSecondary`.

**Remediation:** a `gesture.Click`-with-button-filter (or a `widget` helper that
reports the pressed button) would erase both the PassOp recipe and the
front-most-eats-the-primary-press footgun every app otherwise re-derives.

#### [Major] `cadence/popover` cannot open at the cursor — a context menu opens centred on the row, not where the user right-clicked — `cadence/popover` (G5.3c)

`popover.Popover` centres its Anchor in the canvas and places the surface
adjacent per `Placement`; it has no "open at point" API. A right-click context
menu conventionally opens at the cursor, but here it can only anchor to the row.
The sidebar context popover therefore uses an INVISIBLE 1×1 anchor and
`Placement: Right`, so the menu floats off the row's centre regardless of where
inside the row the click landed. Acceptable for short sidebar rows, but the
wrong affordance for a true context menu.

**Remediation:** a `popover.Props.AnchorPoint image.Point` (open the surface
relative to an explicit point, not the centred anchor) would fix both this and
the canvas-coupling entry below.

#### [Major] `cadence/popover` couples anchor placement and content sizing to the caller's canvas — recurs ×3 in one task — `cadence/popover` (G5.3c)

Exactly as FEEDBACK-G5.2c logged as [Major] for the Share popover:
`popover.Popover` centres the anchor in WHATEVER canvas it is handed and
measures `Content` at `canvas/2`. So each of the three new popovers (row trash,
navbar Delete-N, sidebar context) wraps its anchor in the small cell it lives in
and its `Content` closure OVERRIDES the incoming `canvas/2` constraints with
`layout.Exact(self-sized)`, because half of a 48 dp gutter cannot hold a confirm
prompt. The recipe ported cleanly from `feeds/sidebar.go`'s `deleteConfirm` but
is now pasted three more times in a single task — the strongest signal yet for
the G5.2 remediation.

**Remediation:** the same as FEEDBACK-G5.2 — decouple the concerns: wrap the
anchor in place instead of centring it, register the dismissal absorber against
window extents, and measure `Content` against an explicit `MaxSize` prop (or
against the Content's own measured size against the WINDOW), not against a caller
canvas it has no business constraining.

#### [Minor] `cadence/sidebar.Props.Items` is a static slice — cannot drive a data-loaded, dynamic name list — `cadence/sidebar` (G5.3a)

`cadence/sidebar` takes `Items []sidebar.Item` fixed at construction (the
per-item `OnClick(gtx)` IS gtx-aware, so MessageOp routing works), but the
watchlists list is loaded from disk and grows/shrinks as the user
adds/renames/deletes, and `Active` is per-item-static too. There is no
observable item-list slot, so the component cannot reflect a model-derived list.
As feeds did, the sidebar therefore hand-draws its rows (per-name
`widget.Clickable` keyed via `prism/keyed`, manual offsets/tint/empty-state) —
the right call for a dynamic list, but it means cadence/sidebar is unusable for
the canonical "list of things loaded at runtime" case a sidebar is FOR.

**Remediation:** `Props.Items rx.Observable[[]Item]`, mirroring how
`shell.Props.Sidebar` is already an observable (the GX.7 remediation
FEEDBACK-G5.2 praised).

#### [Minor] `cadence/table` row-click and `RenderTextCell`-by-value patterns recur verbatim at four columns — `cadence/table` (G5.3b)

Both FEEDBACK-G5.2b table frictions ([Major] no row-click affordance; [Major]
`RenderTextCell` takes tokens by value) reproduced exactly: the row click is
registered inside ONE column's `Cell` keyed by row index via `keyed.Defer`, and
the cell closures read a per-frame atomic token mirror. Nothing new, but now
copy-pasted into a third app (prism→feeds→watchlist) unchanged — strong signal
they belong as a `table.Props` row-click slot + a token-observable cell helper,
as G5.2 already proposed.

#### [Minor] `cadence/table` headers are string-only — per-header tooltips need x-offset arithmetic over the SAME widths passed as `Column.Width`, plus the table's PRIVATE `headerHDp` — `cadence/table` (G5.3c)

Logged in FEEDBACK-G5.2c as [Major] for the single Unread tooltip; here four
labelled columns each get a `tooltip.Tooltip` overlaid on its header cell by
`overlayHeaderTooltips`. The x offsets are accumulated in column order from the
SAME dp widths fed to `Column.Width`, and the header height is the magic
`tableHeaderHDp = 44` mirroring the table's PRIVATE `headerHDp` — doubly fragile:
a table internal-padding change breaks the y, a `Column.Width` edit not mirrored
into the overlay constants breaks the x.

**Remediation:** the same as G5.2 asked — a `table.Column.HeaderTooltip string`
(or a header-cell widget slot) deletes all the arithmetic and the fragility.

#### [Minor] `reactivego/rx` `CombineLatest` tops out at arity 5 — non-trivial form/pane compositions need manual shims — `reactivego/rx` (G5.3b, G5.3c)

The G5.3b modal folds 8 live widget streams (modal + card + 4 fields + submit +
alert), past `CombineLatest5`; the workaround collapses the four fields into one
`[4]layout.Widget` via `CombineLatest4`, then `CombineLatest5(...)` — functional
but easy to mis-index. The G5.3c Main pane folds 7 streams (selected + table +
pagination + four header tooltips); there the **variadic**
`rx.CombineLatest(obs...) Observable[[]T]` collapsed the four same-typed tooltip
streams cleanly (see the Ergonomics win below). The heterogeneous-arity case in
G5.3b still needs the manual `[N]` shim.

**Remediation:** higher fixed arities, or lean on the existing variadic
`CombineLatest` for homogeneous fan-in and document it as the blessed escape
hatch above arity 5.

---

## Awkward compositions / boilerplate

#### [Minor] Static `layout.Widget` slots over observable children — the layer-boundary-cell pattern is the first thing a brand-new app reaches for — `cadence/{shell,navbar,modal,card,pagination}` (G5.3a–c)

FEEDBACK-G5.2 logged this as its single biggest boilerplate source ([Major]),
and it recurs immediately in a from-scratch app. `cadence/shell.Props.Main`
(and navbar `Actions`) are static `layout.Widget` slots, but the Main content is
a model-derived `rx.Observable[layout.Widget]` (it shows the selected
watchlist). So G5.3a folds the Main stream onto the sidebar-driving
CombineLatest and reads the latest from an `atomic.Value` cell in the static
slot. The same recipe scaled to the four-field add/edit modal (static
`modal.Body`/`card.Body` bridged to observable children through cells) and the
pagination static-int `Page`/`PageCount` (worked around by SwitchMap-rebuilding
the slot). It is the very first thing a new app built on this architecture has
to reach for — underlining that the observable-vs-static slot mismatch is the
top source of app boilerplate.

**Remediation:** the G5.2 policy decision — cadence slots accept
`rx.Observable[layout.Widget]` (and scalar props accept observables) the way
`shell.Sidebar` and `table.Sort` already do, or prism ships a single
`slot.Bridge` helper packaging the fold-and-cell dance.

#### [Minor] Two idioms for "is this overlay open" — model state vs ephemeral per-instance Subjects — `watchlist` (G5.3c)

Following the feeds idiom and the FEEDBACK-G5.2 [Minor] two-idioms note: the
rename modal (open/error/epoch/seed/target), the bulk selection set, and the
current page live in the MODEL (replayable, drive the count). The per-row
delete-confirm, the navbar Delete-N, and the per-name sidebar context popovers
hold their OPEN flag as ephemeral per-instance `rx.Subject[bool]` interaction
state — transient, exclusive, and would otherwise bloat the model and the
consumer count. The navbar "Delete N" HIDES (renders to zero size) rather than
disables at N=0, and auto-closes its confirm if the selection empties out from
under it. Both idioms are defensible; as G5.2 asked, the framework should pick
and document one before every app re-invents the split.

#### [Minor] Selection-set policy: absolute indices, cleared on EVERY mutation — symbols have no stable id — `watchlist` (G5.3c)

Selection is `map[int]bool` of ABSOLUTE indices into the full Symbols slice (not
page-relative, not Symbol-string identity — duplicate symbols are legal, so a
string key is ambiguous). Because indices shift under deletion, the reducer
clears the selection on EVERY symbol mutation AND on `SelectWatchlist`, and
clamps `currentPage` to the new `pageCount`; the paginated checkbox cell carries
the absolute `idx = pageOffset + row`. The trickiest edge — `SelectWatchlist`
must reset selection AND page or indices chosen in list A delete rows in list B
— is a silent bug with no per-surface pixel test, covered by reducer unit tests
(`TestSelectWatchlistClearsSelectionAndPage`, `TestPageClampsAfterShrink`,
`TestBulkDeleteRemovesSelectedRows`). A stable per-row id (like feeds' FeedID)
would let selection survive deletion and remove the mandatory clear-on-mutation,
but symbols have no id in WATCHLIST-FORMAT.md.

---

## Ergonomics wins worth preserving

#### [Preserve] `(gtx, value)` callbacks + `mvu.MessageOp` and the post-GX.10 MVU shape ported cleanly to a fresh app (G5.3a–c)

Building a brand-new app directly on the post-GX.8/GX.10 architecture (Model +
pure Update + `mvu.MessageOp` per interaction, no rx.Subject controllers, no
atomic interaction mirrors) was frictionless across all three sub-goals: copy
`feeds/main.go`'s run() wiring and `feeds/model.go`'s reducer shape, and sidebar
selection, table row-click, modal submit, popover dismiss, right-click menu, and
pagination all routed through `mvu.MessageOp` with same-frame repaint and zero
controller code. Reducer-owned policy stayed pure and table-tested as the Model
grew real (selection clearing, page clamping, rename validation, delete-watchlist
fallback in `model_g53c_test.go`). The G5.1/G5.2 blocker remediation has fully
paid off in a clean-room app.

#### [Preserve] The layer-boundary atomic-cell overlay pattern scaled from one slot to a four-field modal + three popovers first-try (G5.3a–c)

The FEEDBACK-G5.2 `addFeedModal` recipe (static `modal.Body`/`card.Body` slots
bridged to observable children through `atomic.Value` cells, modal+toast folded
onto the shell stream and drawn as an overlay after the shell) ported to a
four-field form, a rename modal, and three confirm popovers with no surprises.
Overlays still fold onto the shell stream with no extra window layers.

#### [Preserve] The variadic `rx.CombineLatest(obs...) Observable[[]T]` solves homogeneous fan-in above arity 5 (G5.3c)

The G5.3b note worried the `CombineLatest5` ceiling would force manual shims
everywhere; G5.3c found the variadic form EXISTS and collapses the four
same-typed header-tooltip streams into one `[]layout.Widget` cleaner than a
manual `[4]` shim. It is the blessed escape hatch for the homogeneous-fan-in
case (only heterogeneous arities still need the hand-rolled `[N]` shim). Worth
preserving and documenting.

#### [Preserve] Headless pixel + `input.Router` verification against the real composed shell carried over directly (G5.3a–c)

All verification was HEADLESS — no GUI driving is available (launching the Gio
app from a shell has no window-server session). The `feeds/g52c_sim_test.go`
pattern (Subject-driven model into the REAL shell layer, `awaitStableWidget`,
headless GPU capture, region pixel diffs) ported verbatim and caught real
geometry bugs reducer tests cannot see: the sidebar populated state (gotcha: the
sidebar is a full-height Flex `Rigid`, so a sidebar pixel region must start at
y=0, not below the navbar, or it samples pure Surface fill); the CRUD pixel
states (delete removes a row, two-row select reveals navbar Delete-N, pagination
renders for a 30-symbol fixture and NOT a 2-symbol one, modal/alert/rename scrim
paint); and the novel right-click composition + a header tooltip driven through
a real `gioui.org/io/input.Router`. Persistence "across restart" is proven at
the store level (`saveStore` → fresh `loadStore` + deep-equal, plus the
atomic-write property that no `.tmp` debris remains), since every confirm/submit
callback routes through the SAME pure helpers (`applyEdit`, `deleteSymbolAt`,
`bulkDeleteRows`, `renameWatchlistTo`, `deleteWatchlistNamed`).

**One acknowledged gap (consistent across G5.3b/c, judged acceptable):** what no
test drives end-to-end is the ~5 lines of in-callback glue (read model mirror →
pure helper → `saveStore` → `toast.Notify`) and the SwitchMap seed-cell
pre-population. The sim drives `Update(...)` directly, exercising the reducer's
pure helpers but never the cells, the SwitchMap seed, the callback, or
`saveStore` in composition. Every constituent pure piece IS tested; only their
in-callback composition is not. A pointer-driven modal-submit test was judged
not worth the cost (the Save button is buried deep in the layer-boundary-cell
composition with no stable hit rect). Notably this is the SAME untested-glue
path the lazy-mirror data-loss [Major] above hides in — a reason to close it by
making the seed seam testable (a replay-1 model multicast), not just by
documenting the eager-mirror invariant.

---

## Format-design notes for coinviz adoption

The on-disk format (`WATCHLIST-FORMAT.md`) and the `store.go` experience held up
with no surprises, and a future coinviz multi-symbol feature can adopt them by a
straight read of WATCHLIST-FORMAT.md — but five design decisions are
load-bearing and worth carrying forward explicitly rather than re-deciding.
**(1) The top level is an ordered array, not a name-keyed object.** Both
`watchlists` and each watchlist's `symbols` are arrays whose order is
significant and round-trip-preserved, precisely because JSON object key order is
not guaranteed and Go map iteration is randomized; coinviz must keep this if its
multi-symbol view is to render in a stable, user-controlled order. **(2) Version
is a refuse-to-overwrite contract, not just a tag.** `formatVersion = 1` is
emitted on every write, and a reader that encounters a version it does not
recognise MUST refuse to overwrite (fall back to read-only/empty) so a newer
tool's data is never clobbered by an older one — this is exactly the
forward-compatibility seam that lets coinviz and the watchlist editor share one
file safely, and the G5.3 data-loss [Major] above is a reminder that "write an
empty document over the user's file" is the worst-case failure these guards
exist to prevent. **(3) Optional fields default to the empty string and readers
must treat omitted and `""` identically** (`exchange`, `timeframe`, `notes`);
any field coinviz adds (a chart type, an alert threshold) should follow the same
omit-or-empty-is-unspecified rule so old files load without migration and old
readers ignore unknown fields gracefully — additive, optional, empty-defaulting
fields are a version bump-free migration path, and only a breaking change should
increment `version`. **(4) The absent-file-vs-empty-document distinction is
semantic:** an absent file triggers one-time starter creation; a present-but-empty
file is a valid, never-overwritten document. coinviz must preserve this so it
does not re-seed starter data over a user who deliberately emptied their lists.
**(5) Two engineering seams should be copied, not re-derived:** the
injectable-path seam (the default platform path via `os.UserConfigDir()` is
wired only in `main()`; everything else takes an explicit path) so tests use
`t.TempDir()` and never touch the real config dir, and the atomic write
(marshal → sibling `.tmp` → `os.Rename`, with the temp removed on any pre-rename
failure) so a crash mid-save never corrupts or leaves debris. The strongest
single recommendation for coinviz: if a `prism/storage` helper lands (see the
Missing-API [Major]), it should encode decisions (2)–(5) as its default contract
so multi-tool file sharing is correct by construction rather than by each tool
re-implementing the guards by hand.

---

*No section is empty; all four standard buckets accumulated findings, plus the
coinviz format-design notes — consistent with a from-scratch persistent CRUD app
that exercised the sidebar, table, modal, popover, tooltip, pagination, and a
hand-rolled JSON store in earnest.*
