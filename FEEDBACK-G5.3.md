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
