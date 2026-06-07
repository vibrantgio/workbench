# FEEDBACK-G5.3 — Dogfooding notes from the watchlist editor

Running-notes style: severity-tagged `####` entries logged as the milestone is
built. G5.3a is the format design + app skeleton + watchlists sidebar (read +
display only).

## G5.3a

#### [Minor] Plan's `prism/initial` window-bootstrap citation is stale (carried over from FEEDBACK-G5.1/G5.2)

The G5.3a Specific says the window opens "via `prism/initial` + `spectrum`
theme". As logged in both FEEDBACK-G5.1 and FEEDBACK-G5.2, the real bootstrap
is `mvu.NewWindow` + `spectrum/window.New` + `spectrum/system.LiveTheme` — copy
`feeds/main.go`. `prism/initial` is not the live entry point. Re-logging here
so the citation is corrected for the whole G5.3 milestone. No code impact (the
recipe was followed); flagging for the eventual plan cleanup.

#### [Major] No `prism/storage` (or any framework persistence helper) — every app hand-rolls JSON load/save + path resolution

There is no framework primitive for "read/write a per-user JSON config file."
G5.3a hand-rolled `store.go`: `os.UserConfigDir()` + `filepath.Join` for the
path, `os.ReadFile`/`json.Unmarshal` to load, `os.MkdirAll`+`json.MarshalIndent`
+`os.WriteFile` to save, plus the first-run-starter and "missing file vs empty
document" branching, plus the injectable-path seam so tests use `t.TempDir()`
and never touch the real `~/Library/Application Support`.

None of this is hard, but it is boilerplate every persistent vibrantgio app
(coinviz adoption included) will re-implement identically, and the test-isolation
seam (path injection) is easy to forget — a default-path helper that wires the
real path in `main()` only is the kind of footgun a shared helper would prevent.
A small `prism/storage` (or `cadence/storage`) offering
`LoadJSON[T](path)` / `SaveJSON[T](path, v)` + a `UserConfigPath(app, file)`
resolver would remove the duplication and standardise the
absent-vs-empty-vs-newer-version contract that WATCHLIST-FORMAT.md spells out by
hand. Not a blocker — the format is small and pragmatic by design — but the
gap is real and will recur.

#### [Major] `cadence/sidebar.Props.Items` is a static slice — cannot drive a data-loaded, dynamic name list

`cadence/sidebar` takes `Items []sidebar.Item` fixed at construction (with
per-item `OnClick func(gtx)`, which IS gtx-aware, so MessageOp routing would
work). But the watchlists list is loaded from disk and will grow/shrink as the
user adds/renames lists, and `Active` is per-item-static too. There is no
observable item-list slot, so the component cannot reflect a model-derived list.

As feeds did before us (`feeds/sidebar.go`), the watchlist sidebar therefore
hand-draws its rows: per-name `widget.Clickable` keyed via `prism/keyed`, manual
row offsets, manual selected-row tint, manual empty-state. This is the right
call for a dynamic list, but it means the cadence/sidebar component is unusable
for the canonical "list of things loaded at runtime" case — exactly the case a
sidebar is for. A `Props.Items rx.Observable[[]Item]` (mirroring how
`shell.Props.Sidebar` is already an observable, the GX.7 remediation
FEEDBACK-G5.2 praised) would fix it.

#### [Minor] Layer-boundary atomic cell for shell's static `Main` slot — same boilerplate FEEDBACK-G5.2 flagged, recurs immediately in a brand-new app

`cadence/shell.Props.Main` is a static `layout.Widget`, but the Main content is
a model-derived `rx.Observable[layout.Widget]` (it shows the selected
watchlist's name). So G5.3a repeats the FEEDBACK-G5.2 "static slot over an
observable child" pattern: fold the Main stream onto the sidebar-driving
CombineLatest and read the latest from an `atomic.Value` cell in the static
slot. It is exactly the pattern feeds uses, and it worked first-try — but it is
the very first thing a new app built on this architecture has to reach for, which
underlines FEEDBACK-G5.2's point that observable-vs-static slot mismatch is the
single biggest source of app boilerplate. `shell.Props.Main` (and navbar
`Actions`) being observable would erase it.

#### [Preserve] `(gtx, value)` callbacks + `mvu.MessageOp` and the post-GX.10 MVU shape ported cleanly to a fresh app

Building a brand-new app directly on the post-GX.8/GX.10 architecture (Model +
pure Update + `mvu.MessageOp` per interaction, no rx.Subject controllers, no
atomic interaction mirrors) was frictionless: copy `feeds/main.go`'s run()
wiring, `feeds/model.go`'s reducer shape, and the layer-boundary-cell hand-off,
and the sidebar row click → `SelectWatchlist` → same-frame re-emit worked on the
first try. The measured `modelObsConsumers` for this small app is **3** (vs
feeds' 16); the count test caught it immediately.

#### [Preserve] Headless pixel verification against the real composed shell carried over directly

The `feeds/g52c_sim_test.go` pattern — Subject-driven model into the real shell
layer, `awaitStableWidget`, headless GPU capture, region pixel diffs — ported
verbatim and verified "the app opens with the sidebar populated" without a
window-server session. One gotcha worth noting for the next app: the sidebar is
a full-height Flex `Rigid` child, so the navbar overlays ONLY the Main column —
a sidebar pixel-assertion region must start at y=0, not below the navbar height,
or it samples pure Surface fill and the empty-vs-loaded diff reads 0.

## G5.3b

G5.3b is the symbols table + add/edit modal: a `cadence/table` of the selected
watchlist's symbols, an "Add symbol" button opening a `cadence/modal` form (four
`prism/input` textfields + a `prism/button`), row-click editing, atomic
full-file rewrite on save, a "Saved" `cadence/toast`, and an empty-Symbol
`cadence/alert`.

#### [Major] `prism/input.TextField` cannot be pre-populated — the single biggest friction in this task

The task requires "reopen the same modal **pre-populated** with the row's
values." `prism/input.TextField` is fully UNCONTROLLED: its `widget.Editor`
lives inside the component's `rx.Defer` scope, is allocated once per
subscription, and is never exposed. `TextFieldProps` has `Placeholder`,
`Description`, `OnChange`, `Message`, `Submit`, `SubmitMessage`, `OnSubmit`,
`Shaper` — and **no initial-value / value / SetText prop**. There is literally
no way to put the row's current text into the live editor. (The package even
documents this obliquely: `RenderState.Text` exists for the STATIC golden path
only, "has no effect on the live TextField.")

The chosen workaround (it works for the user flow, but it is a workaround):

1. **Rebuild the field fresh on every open.** The model carries an incrementing
   `modalEpoch` (bumped by `OpenAddSymbol`/`OpenEditSymbol`); each field is
   `rx.SwitchMap(editObs, …)` keyed on the epoch, so a new open re-subscribes
   the TextField and gets a FRESH (empty) editor. **This rebuild is mandatory,
   not cosmetic:** without it the editor persists across opens — open row A, type
   "ETH", close, open row B, and the field still visibly shows "ETH" while the
   seeded cell says B's value. Keying on the epoch (not `editIndex`) is required
   so reopening the SAME row after a cancel also rebuilds. `OnSubmit`'s internal
   `SetText("")` does NOT help: it only fires on Enter-submit success, not on a
   `prism/button` submit and not on close-without-submit.
2. **Show the current value as the Placeholder.** The fresh editor is empty, so
   the row's value is rendered as the field placeholder, and the field's text
   cell is *seeded* to the same value.
3. **"Empty field on submit = keep the seeded value."** Untouched field → cell
   still holds the seed → that value is submitted; typing replaces it. This is
   the load-bearing decision that makes "edit one field, the others survive"
   work.

**The honest limitation this forces** (front-and-centre, as requested): with
"empty keeps the seed", clearing a previously-set optional field (e.g. Notes
"foo" → "") is **un-discoverable, not impossible**. The cell is pre-seeded and
the editor renders empty, so focus+backspace on an already-empty editor fires no
change event and the seed survives; but type-any-char-then-delete-it DOES fire
`OnChange("")`, which clears the cell. So a user who knows to "type then erase"
can clear a field — but the obvious gesture (focus the field showing the old
value as a placeholder, hit delete) does nothing. And the placeholder hides on
focus, so the original value is not visible while typing into it. Both are direct consequences of the
uncontrolled field. A `TextFieldProps.InitialText string` (seed-once into the
editor, not a controlled binding) would erase the entire workaround AND restore
clear-to-empty semantics. This is the same uncontrolled-field gap FEEDBACK-G5.2d
flagged for the add-feed flow, but G5.3b is the first task where the gap
actually changes user-visible behaviour (you can't clear a field), not just
internal plumbing.

#### [Major] Save side-effect has no clean home — the reducer is pure but the run() Scan discards Commands

The save must "mutate the in-memory watchlist AND write the full file back."
Reducer purity (mandated post-GX.10) says the write can't go in `Update`.
`mvu.Command` exists (`mvu/command.go` has `Do`/`DoConcurrent`/…), but the
production seam ignores it: `main.go`'s `rx.Scan(...)` does
`next, _ := Update(model, msg)` — **the Command is discarded**, exactly as in
feeds. So a reducer-returned `mvu.Do(write)` is a dead path unless run()'s Scan
is rewired to execute Commands, which would disturb the load-bearing
`Publish().AutoConnect(modelObsConsumers)` seed-delivery the milestone is told
not to touch.

Resolution chosen (approach (a)): the write lives in the **submit callback**,
which (i) reads a model-mirror `atomic.Value` fed by `modelObs`, (ii) applies
the SAME pure `applyEdit(wls, selected, editIndex, sym)` helper the reducer
calls, (iii) writes the resulting full `Document` atomically, (iv) fires the
toast. The reducer and the callback can never diverge because both route through
`applyEdit`; the store path is injected through `buildLayers` →
`watchlistShellLayer` → `addSymbolModal` so tests write to `t.TempDir()`. The
cost: the mutation logic is *invoked* from two places (reducer for in-memory,
callback for disk), and the callback needs a model mirror just to see the full
watchlists the four form cells don't carry.

The model mirror is **correct only because the modal is exclusive and the fields
land no messages while open**: between `OpenEditSymbol` (which sets `editIndex`)
and the Save click, no message lands, so `modelObs` emits exactly once and the
mirror holds precisely the open-time model — there is no intervening emission to
make `editIndex`/`watchlists` stale at click time. A future non-exclusive modal,
or any background mutation of the model while the modal is open, would break this
assumption and the callback could persist a different mutation than the reducer
applies. The exclusivity invariant is load-bearing for this pattern. The framework gap is real: there is
no supported "reduce-then-effect" path because the canonical Scan throws
Commands away. Either run()'s Scan should execute the returned Command (a
`mvu` recipe change), or `mvu` should document the callback-effect pattern as
the blessed one and stop pretending `Command` is wired.

#### [Minor] `cadence/table` row-click + `RenderTextCell`-by-value patterns recur verbatim at four columns

Both FEEDBACK-G5.2 table frictions reproduced exactly: (1) no whole-row click
affordance — the row click is registered inside ONE column's `Cell` (the Symbol
column), keyed by row index via `prism/keyed.Defer`, and (2) `RenderTextCell`
takes tokens by value, so the cell closures read a per-frame atomic token
mirror. Nothing new, but worth noting these are now copy-pasted into a third app
(prism→feeds→watchlist) unchanged — strong signal they belong as a
`table.Props` row-click slot + a token-observable cell helper.

#### [Minor] Eight observables into a modal needs a manual `[4]layout.Widget` shim — `rx` tops out at `CombineLatest5`

The modal folds modal + card + 4 fields + submit + alert = 8 live widget
streams, but `reactivego/rx` provides `CombineLatest` only through arity 5. The
workaround: collapse the four fields into one `[4]layout.Widget` via
`CombineLatest4`, then `CombineLatest5(modal, card, fields4, submit, alert)`.
Functional, but the nesting obscures the topology and is easy to mis-index. A
variadic `CombineLatestSlice` (or higher arities) would help any app that
composes a non-trivial form.

#### [Preserve] The layer-boundary atomic-cell modal/overlay pattern scaled to four fields first-try

The FEEDBACK-G5.2 `addFeedModal` recipe — static `modal.Body`/`card.Body` slots
bridged to observable children through `atomic.Value` cells, the modal+toast
folded onto the shell stream and drawn as an overlay after the shell — ported to
a four-field form with no surprises. The headless verification harness
(`awaitStableWidget` + region pixel diffs against the REAL shell, plus the
`toast.Notify`→`Stack` render test and store-level round-trip for
"persists across restart") also ported verbatim. Measured `modelObsConsumers`
rose 3 → **11** (the model mirror + four epoch-keyed field SwitchMaps + the new
table/modal streams); the count test caught the hand-guess immediately and the
breakdown comment was updated to the measured value.

#### [Note] Verification was HEADLESS

No GUI driving is available (launching the Gio app from a shell has no
window-server session). "Persists across restart" is proven at the store level
(`TestSaveRoundTripPersistsEdits`: apply `applyEdit`, `saveStore`, then a FRESH
`loadStore` + deep-equal — plus the atomic-write crash-safety property that no
`.tmp` debris remains). The UI flow (modal opens on `OpenAddSymbol`, alert on
empty `SubmitSymbol`, the table updates after add and after edit) is proven by
pixel diffs against the real composed shell. The save toast is proven via the
`toast.Notify`→`Stack` render path. What no test drives end-to-end is the ~5
lines of submit-callback glue (read cells + model mirror → `applyEdit` →
`saveStore` → `toast.Notify`) and the seed-cell pre-population — the sim drives
`Update(SubmitSymbol{…})` directly, which exercises the reducer's `applyEdit`
but never the cells, the SwitchMap seed, the callback, or `saveStore`. Every
constituent pure piece IS tested (`applyEdit` aliasing + round-trip,
`saveStore` atomicity, the toast render path, the modal/alert/table pixel
states); only their in-callback composition is unverified. A pointer-driven
modal-submit test was judged not worth the cost (the Save button is buried deep
in the layer-boundary-cell composition, with no stable hit rect).

## G5.3c

G5.3c adds the interaction surfaces over the G5.3b symbols editor: row-level
delete (trash → `cadence/popover` confirm), bulk multi-select (a checkbox
column + a navbar "Delete N" `cadence/popover` confirm), per-header
`cadence/tooltip` overlays, a sidebar right-click context menu (Rename modal /
Delete confirm), and conditional `cadence/pagination` (only above `pageSize`
symbols).

#### [Major] First right-click composition in the codebase — a front-most hit area swallows the PRIMARY press unless wrapped in `pointer.PassOp`

The sidebar context menu needs a right-click to open the menu WITHOUT breaking
left-click-to-select. `widget.Clickable` does not expose the pressed button, so
a raw `pointer.Filter{Kinds: pointer.Press}` tag is registered over the row and
the drain checks `pe.Buttons.Contain(pointer.ButtonSecondary)`. The hit area
must be registered AFTER the select clickable (later = front-most) so it sees
the secondary press — but a plain front-most registration ABSORBS the primary
press too, and click-to-select silently breaks. A throwaway router probe proved
this immediately: secondary opened the menu, primary never reached the
clickable. The fix is `pointer.PassOp{}.Push(...)` around the tag registration:
pass-through delivers the press to BOTH the front tag (for the secondary filter)
AND the clickable behind it (for the primary). This is now guarded by
`TestRightClickPassesPrimaryReachesContextSecondary` (a real `input.Router`
queueing a secondary press then a primary press+release, asserting both land).
The friction is real: there is no `widget`/`gesture` helper for "right-click
this area", so every app needs the PassOp + manual-button-check recipe, and the
gotcha (front-most plain registration eats the primary press) is a footgun a
`gesture.Click`-with-button-filter would erase.

#### [Major] `cadence/popover` cannot open at the cursor — context menu opens centred on the row, not where the user right-clicked

`popover.Popover` centres its Anchor in the canvas and places the surface
adjacent per `Placement`; it has no "open at point" API. A right-click context
menu conventionally opens at the cursor, but here the menu can only anchor to
the row. The sidebar context popover therefore uses an INVISIBLE 1×1 anchor and
`Placement: Right`, so the menu floats off the row's centre regardless of where
inside the row the click landed. Acceptable for a watchlist sidebar (rows are
short), but it is the wrong affordance for a true context menu, and a third app
will hit it. A `popover.Props.AnchorPoint image.Point` (open the surface
relative to an explicit point, not the centred anchor) would fix both this and
the next entry.

#### [Major] popover-canvas coupling recurs ×3 — row trash, navbar Delete-N, sidebar context — each needs the Content-constraint override

Exactly as FEEDBACK-G5.2c logged for the Share popover: `popover.Popover`
centres the anchor in WHATEVER canvas it is handed and measures `Content` at
`canvas/2`. So each of the three new popovers wraps its anchor in the small cell
it lives in (the trash gutter, the navbar action slot, the sidebar row) and the
`Content` closure OVERRIDES its incoming `canvas/2` constraints with
`layout.Exact(self-sized)` because half of a 48 dp gutter cannot hold a confirm
prompt. The recipe ported cleanly from `feeds/sidebar.go`'s `deleteConfirm`
(copied verbatim for the row trash, adapted for the navbar action and the
context menu), but it is now pasted three more times in one task — the strongest
signal yet that `popover` should self-size its Content from the Content's OWN
measured size against the WINDOW, not against a caller canvas it has no business
constraining.

#### [Major] modelObs mirror MUST be subscribed eagerly per-layer — a lazy subscription inside `keyed.Defer` is invisible to the count test AND never seeded (silent data-loss path)

The first cut gave each per-row delete-confirm and each per-name context menu
its OWN `modelObs.Subscribe` (mirroring `addSymbolModal`'s mirror). That is a
TRAP. `keyed.Defer` constructors run LAZILY on the first `.For(key)` during a
LAYOUT frame — which is AFTER `Publish().AutoConnect(N)` has already fired
`StartWith(seed)`. `Publish()` does not replay, so the lazy mirror joins the hot
stream and receives NOTHING until the next message: its `modelCell` holds the
zero `Model{}` (watchlists=nil, selected="") until then. A user who right-clicks
a watchlist → Delete → Confirm BEFORE any other interaction would call
`deleteWatchlistNamed(nil, …)` → `saveStore` an EMPTY document over their file.
Data loss, and no model-driven sim catches it (they drive `Update` directly,
never the mirror seed path — the same untested-glue gap G5.3b flagged).

Worse, the lazy subscription is invisible to `TestModelObsConsumerCountMatches
Const` (whose subscribe callback never lays out, so `.For()` never runs), so the
count would silently UNDERCOUNT and grow unbounded as rows are interacted with —
the AutoConnect(N) seed-delivery invariant the whole app depends on.

Resolution: subscribe ONE eager mirror in each layer's function body
(`watchlistMain`, `watchlistSidebar`) and pass it down as a `func() Model`; the
per-row/per-name surfaces read through it and NEVER subscribe `modelObs`
themselves. This makes the consumer count STATIC (independent of watchlist/
symbol count) and seed-correct. **New invariant (now in app.go's
`modelObsConsumers` comment): never subscribe modelObs inside a keyed.Defer —
the count test can't see it and AutoConnect can't seed it.** Measured
`modelObsConsumers` rose 11 → **22**; the count test caught the const drift
immediately at both the broken (lazy, 20-and-climbing) and fixed (eager, static
22) topologies.

#### [Minor] Per-header tooltips at 4 columns = manual x-offset arithmetic over the SAME widths passed as `Column.Width`, mirroring the table's private `headerHDp`

`cadence/table` headers are string-only with no per-header widget slot (logged
in FEEDBACK-G5.2c for the single Unread tooltip), so each of the four labelled
columns gets a `tooltip.Tooltip` whose constraint-filling Trigger is overlaid on
its header cell by `overlayHeaderTooltips`. The x offsets are accumulated in
column order from the SAME dp widths fed to `Column.Width` (checkbox gutter →
flexing Symbol = tableWidth − pinned-sum → Exchange → Timeframe → Notes), and
the header height is the magic `tableHeaderHDp = 44` that mirrors the table's
PRIVATE `headerHDp` — doubly fragile now: a table internal-padding change breaks
the y, and a `Column.Width` edit that isn't mirrored into the overlay constants
breaks the x. Tooltip arbitration is global (one visible at a time) by design,
which is correct for headers. The fix is the same as G5.2c asked: a
`table.Column.HeaderTooltip string` (or a header-cell widget slot) would delete
all of this arithmetic and the fragility.

#### [Minor] Pagination conditional via `rx.Of[layout.Widget](nil)` in the SwitchMap; the rx `CombineLatest5` ceiling bites again (variadic `CombineLatest` rescued it)

"Rendered only when > pageSize symbols" is a `pageCountObs` derived from the
symbol slice; the `SwitchMap` over `(page, pageCount)` emits
`rx.Of[layout.Widget](nil)` when `pageCount <= 1` and the layout slot skips a
nil widget — clean. The Page/PageCount static-int workaround from G5.2b ported
verbatim. Separately, the Main pane folds selected + table + pagination + FOUR
header tooltips = 7 streams, past `CombineLatest5`; this time the VARIADIC
`rx.CombineLatest(obs...) Observable[[]T]` collapsed the four same-typed tooltip
streams into one `[]layout.Widget` (cleaner than G5.3b's manual `[4]` shim,
which is only needed for heterogeneous arities). Worth noting the variadic form
EXISTS and solves the homogeneous-fan-in case the G5.3b note worried about.

#### [Minor] Selection-set policy: absolute indices, cleared on EVERY mutation; page-relative checkbox maps via `pageOffset + row`

Selection is `map[int]bool` of ABSOLUTE indices into the full Symbols slice (not
page-relative, not Symbol-string identity — duplicate symbols are legal, so a
string key is ambiguous). Because indices shift under deletion, the reducer
clears the selection on EVERY symbol mutation (add/edit/delete/bulk) AND on
`SelectWatchlist` (different slice), and clamps `currentPage` to the new
`pageCount`. The paginated checkbox cell carries the absolute `idx`
(`pageOffset + page-relative row`) so a row selected on page 2 deletes the right
symbol. The trickiest edge — `SelectWatchlist` must reset selection AND page or
indices chosen in list A delete rows in list B — is a silent bug with no
per-surface pixel test; it's covered by reducer unit tests
(`TestSelectWatchlistClearsSelectionAndPage`, `TestPageClampsAfterShrink`,
`TestBulkDeleteRemovesSelectedRows`). The cost of index keys is the mandatory
clear-on-mutation; a stable per-row id (like feeds' FeedID) would let selection
survive deletion, but symbols have no id in WATCHLIST-FORMAT.md.

#### [Note] Model-state vs ephemeral-Subject choice (consistent, logged): modal/rename/selection/page in the Model; per-row & per-instance popover OPEN flags as ephemeral rx.Subjects

Following the feeds idiom and the G5.2 two-idioms note: the rename modal
(open/error/epoch/seed/target), the bulk selection set, and the current page
live in the MODEL (replayable, drive the count). The per-row delete-confirm,
the navbar Delete-N, and the per-name sidebar context popovers hold their OPEN
flag as ephemeral per-instance `rx.Subject[bool]` interaction state (NOT model
state) — they are transient, exclusive, and would otherwise bloat the model and
the consumer count. The navbar "Delete N" decides HIDE (not disable) at N=0: a
"Delete 0" affordance is meaningless, so the whole action renders to zero size
when nothing is selected (and auto-closes its confirm if the selection empties
out from under it).

#### [Note] Rename modal inherits the uncontrolled-field pre-population workaround AND its clear-to-empty limitation verbatim

The rename modal is the symbol modal scaled to one field, and it reuses the
identical `prism/input.TextField` workaround (epoch rebuild on open + current
name as placeholder + "empty keeps the seed" so an untouched field submits the
old name). The same honest limitation applies: you cannot rename a watchlist to
"" (correct here — empty names are rejected anyway), but the un-discoverable
focus+backspace-does-nothing behaviour is present. `TextFieldProps.InitialText`
would fix both modals at once.

#### [Note] Verification was HEADLESS

No GUI driving (no window-server session). The novel right-click composition and
one column-header tooltip are driven through a real `gioui.org/io/input.Router`
(`TestRightClickPassesPrimaryReachesContextSecondary`,
`TestColumnTooltipHoverHeadless`). The CRUD flows are pixel-diffed against the
REAL composed shell at each model state (`TestG53cShellStatesHeadless`: delete
removes a row, two-row select reveals the navbar Delete-N, pagination renders for
a 30-symbol fixture and NOT for a 2-symbol one, the rename modal scrim paints).
Persistence "across restart" is proven at the store level
(`TestG53cPersistenceRoundTrips`: each pure helper — `deleteSymbolAt`,
`bulkDeleteRows`, `renameWatchlistTo`, `deleteWatchlistNamed` — through
`saveStore` then a fresh `loadStore` + field asserts), the same durable proof
G5.3b used for edits, since the confirm/submit callbacks route through those SAME
helpers. The reducer edge cases (selection clearing, page clamping, rename
validation, delete-watchlist fallback) are unit-tested in `model_g53c_test.go`.
What no test drives end-to-end is the in-callback glue (read mirror → pure helper
→ `saveStore` → `toast.Notify`); every constituent piece is tested, only the
~5-line composition is not — the same judged-acceptable gap as G5.3b.
