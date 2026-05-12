# VibrantGIO Implementation Plan

> **Source of truth:** [DESIGN.md](./DESIGN.md). This plan does not redefine architecture — it shards DESIGN.md into goals an LLM agent can execute one at a time.

## SMART contract for every goal

Each goal in this plan satisfies all five letters:

- **Specific** — names exact files/modules and acceptance criteria. No "improve X" goals.
- **Measurable** — completion is verified by `go build`, `go test`, a benchmark threshold, or a documented decision artefact (`MIGRATION.md`, `EXPERIMENT-A.md`, etc.). Subjective "looks good" is not acceptance.
- **Achievable** — scoped to one module slice. No goal opens design space that DESIGN.md leaves open; if a goal would require a new architectural decision, it is split into a *decision goal* (writes a doc) followed by an *implementation goal* (writes code).
- **Relevant** — every goal cites the DESIGN.md section it discharges, in the form `(DESIGN §<heading>)`. A goal with no citation does not belong here.
- **Timeboxed** — fits in one Claude Sonnet 4.6 session of **≤100 000 input + output tokens**. The token budget *is* the timebox.

### Token-budget arithmetic (the "T" in SMART)

A 100 K-token Sonnet 4.6 session typically allocates roughly:

| Bucket | Tokens |
|---|---:|
| System prompt + tools | ~10 K |
| Reading DESIGN.md once (590 lines) | ~10 K |
| Reading 3–5 referenced source files | ~15 K |
| Tool-call results (greps, builds, tests) | ~15 K |
| Model output (code + commentary) | ~30 K |
| Headroom for retries / debugging | ~20 K |
| **Total ceiling** | **100 K** |

This implies the practical code-output budget per goal is **~30 K tokens ≈ 600–900 lines of Go including tests**. Goals that cannot fit are split. Splits are listed inline as `Split:` sub-goals.

### Anti-goals (rules every goal inherits)

- **No goal spans more than one phase.** Phase boundaries in DESIGN.md exist for sequencing reasons.
- **No goal both designs and implements.** Decision goals output a doc; implementation goals consume that doc.
- **No goal depends on a later goal.** If you need something from a later goal, the dependency graph is wrong — fix this plan first.
- **No goal ships without tests** in the same session, except documentation goals.

---

## Dependency graph (top-down)

```
Phase −1 (Gio migration)
  └── Phase 00 experiments A, B, C  (parallelisable after −1)
        └── Phase 0 (token & theme contract)  (gated by 00 outcomes)
              └── Phase 1 (Prism)
                    ├── Phase 2 (theme runtime)         ┐
                    ├── Phase 3 (visual effects)        ├── parallelisable
                    └── Phase 4 (pattern library)       ┘
                          └── Phase 5 (example apps — pressure-test)
```

Goals are listed in topological order. Within a phase, goals marked **‖** are parallelisable.

---

## Phase −1 — Gio Migration

Discharges DESIGN §"Phase −1 — Gio Migration" and §"Known Fragilities".

### G−1.1 — Pin and audit current Gio usage

- [x] **Done**
- **Specific:** produce `MIGRATION.md` listing every Gio API call in the workspace that changed between v0.1.0 and current Gio (events channel, `pointer.InputOp`, `op.InvalidateOp`, ops-buffer internals).
- **Measurable:** `MIGRATION.md` exists with one row per call site (file:line, old API, new API, risk note). `grep -rn "Events()\|InputOp\|InvalidateOp{}\|Internal" --include="*.go"` matches every row.
- **Achievable:** read-only audit. No code changes.
- **Relevant:** DESIGN §"Phase −1" deliverable 6 (postmortem doc) prerequisite.
- **Budget:** audit + write ~400 lines of Markdown. Single session.

### G−1.2 — Migrate `mvu/window.go` event loop
- [x] **Done**
- **Specific:** rewrite `mvu/window.go` against `app.Window.Event()` (goroutine→channel→`rx.Recv`); fix `mvu/message.go` to use `event.Op` instead of the removed `pointer.InputOp`; fix `font/` `Variant` field (removed in v0.9). Preserve the `WithLatestFrom2` join exactly. The `unsafeOps` cast is corrected (`version uint32`) but not yet eliminated — that is G−1.3.
- **Measurable:** `go build ./mvu/...` is error-free; `go test ./mvu/...` passes.
- **Achievable:** `mvu/window.go` (~100 lines), `mvu/message.go` (~17 lines), `font/` leaf files (mechanical zero-value removal).
- **Relevant:** DESIGN §"Phase −1" deliverable 2.
- **Budget:** ~70 K tokens including reading current `mvu/window.go`, `traer/gio/...` example, and test rewrite.
### G−1.3 — Replace `unsafe.Pointer` MessageOp extraction
- [x] **Done**
- **Specific:** delete the `unsafeOps` cast in `mvu/window.go` (struct layout was corrected to `version uint32` in G−1.2, but the unsafe block must be removed entirely); route `MessageOp` through a mechanism that does not require `unsafe` — e.g. a per-frame accumulator slice threaded through the layout context, or another safe alternative.
- **Measurable:** `grep -rn "unsafe" mvu/` returns zero results; `go build ./mvu/...` still clean; existing `MessageOp` consumers in `todos` compile (once G−1.5 is done).
- **Achievable:** single-file refactor against the API stabilised in G−1.2; may require small changes to `mvu/message.go`.
- **Relevant:** DESIGN §"MessageOp extraction via `unsafe.Pointer`" repayment plan.
- **Budget:** ~50 K. Depends on G−1.2.
### G−1.4 ‖ — Migrate `coinviz`

- [x] **Done**
- **Specific:** update `coinviz/go.mod` to current Gio; fix every API breakage call site.
- **Measurable:** `go build ./coinviz/...` succeeds; app launches and renders one symbol on `BTC-USD` for ≥10 s without panic.
- **Achievable:** mechanical API renames; no architectural changes.
- **Relevant:** DESIGN §"Phase −1" deliverable 4.
- **Budget:** large but bounded. **Split** if first session ends mid-app:
  - **G−1.4a** data layer (`coinviz/data`, `coinviz/ws`)
  - **G−1.4b** chart panes (`coinviz/pane`)
  - **G−1.4c** app shell + main

### G−1.5 ‖ — Migrate `appviz`, `todos`, `mindchat`, `traer/gio/*`, `seen/gio/*`

- [x] **Done**
- **Specific:** one goal per app. Each goal updates `go.mod`, fixes API call sites, verifies the app launches.
- **Measurable:** `go build` succeeds in each app's directory; manual launch produces the expected first frame.
- **Achievable:** each app is smaller than `coinviz`; one session per app.
- **Relevant:** DESIGN §"Phase −1" deliverable 4.
- **Budget:** ~50 K each. Five separate goals: G−1.5a..G−1.5e.

### G−1.6 — Capture performance baseline

- [x] **Done**
- **Specific:** add `coinviz/bench/baseline_test.go` measuring per-pane `widget(gtx)` cost and allocation rate under "BTC-USD, 1h candles, all 16 panes, 1000 frames" load. Commit results to `BASELINE.md`.
- **Measurable:** benchmark runs to completion; numbers recorded with platform metadata (Apple Silicon model, Gio version, Go version).
- **Achievable:** one new file plus a Markdown report.
- **Relevant:** DESIGN §"Performance — Methodology" baseline measurement requirement.
- **Budget:** ~60 K. Depends on G−1.4.

### G−1.7 — Risk-gate review

- [x] **Done**
- **Specific:** verify each of the five DESIGN architectural patterns (FRP shape, MVU shape, `WithLatestFrom2`, `rx.Defer`, frame-driven physics) still holds on migrated Gio. Record findings in `MIGRATION.md`.
- **Measurable:** five sections in `MIGRATION.md`, each marked ✅ or ⚠ with a one-paragraph justification.
- **Achievable:** review-only; no new code.
- **Relevant:** DESIGN §"Phase −1" risk gate. Phase 00 cannot start until this is ✅ or downgraded patterns are documented.
- **Budget:** ~40 K.

---

### G−1.8 — Migrate `ivg/raster/gio` examples

- [x] **Done**
- **Specific:** port the 8 programs in `ivg/raster/gio/example/` (`arrow`, `blend`, `cowbell`, `favicon`, `gradients`, `icons`, `info`, `logo`) from the old Gio event loop (`app.NewWindow`, `window.Events()`, `system.FrameEvent`) to the current API (`app.Window.Event()`); bump `ivg/raster/gio/go.mod` to current Gio.
- **Measurable:** `go build ./ivg/raster/gio/...` succeeds; each example launches to its first rendered frame without panic.
- **Achievable:** 8 small files each using the same 3-call migration pattern established in G−1.2 and G−1.5. The raster library itself already uses no deprecated APIs — only the examples need updating. Non-blocking: does not gate Phase 0 or Phase 00.
- **Relevant:** DESIGN §"Phase −1" deliverable 4 (all Gio-dependent programs migrated).
- **Budget:** ~40 K.

## Phase 00 — Validation Experiments

Discharges DESIGN §"Phase 00 — Validation Experiments" and §"Architectural Limits". Gated by G−1.7.

### G00.A — Experiment A: Keyed identity

- [x] **Done**
- **Specific:** prototype `rx.KeyedDefer[K, V]` in a throwaway `experiments/keyed/` module. Build a reorderable todo list with per-row `Defer` state; verify state survives reorder.
- **Measurable:** `EXPERIMENT-A.md` records (a) the API tried, (b) whether it felt natural, (c) decision: adopt / reject / revise. `go test ./experiments/keyed/...` exercises reorder + insertion + deletion with state preservation assertions.
- **Achievable:** one module, ~300 lines code + 200 lines test.
- **Relevant:** DESIGN §"Architectural Limits" concern #1.
- **Budget:** ~80 K. **Split** if API revision needed:
  - **G00.A1** build prototype with first API draft
  - **G00.A2** revise API based on G00.A1 friction; finalise decision

### G00.B — Experiment B: Many-entity animation

- [x] **Done**
- **Specific:** force-directed graph in `experiments/manyentity/`, 200+ nodes via `traer`, sustained 60 FPS. Measure layout-pass cost and allocation rate per frame.
- **Measurable:** `EXPERIMENT-B.md` reports frame-time histogram and per-frame allocation count. Decision: op-cache pattern viable / dedicated scene primitive needed.
- **Achievable:** one prototype + benchmark.
- **Relevant:** DESIGN §"Architectural Limits" concern #2.
- **Budget:** ~80 K.

### G00.C1 — Experiment C1: Drag-drop with shared Subject

- [x] **Done**
- **Specific:** `experiments/coordination/` Gio module with a kanban board (≥2 columns) where cards are draggable between columns. An `rx.Subject[DragState]` is created at board level and passed explicitly to the drag source and all drop targets. Drop targets highlight in real time when an active drag hovers over them.
- **Measurable:** `go run ./experiments/coordination/` opens a window with working drag-and-drop. `EXPERIMENT-C.md` §C1 documents: (a) Subject creation and propagation pattern, (b) whether async Subject emissions integrate cleanly with Gio's synchronous frame model (the load-bearing question), (c) the candidate coordination primitive shape.
- **Achievable:** one Gio app; drag-drop only; no modal or tooltip yet.
- **Relevant:** DESIGN §"Architectural Limits" concern #3.
- **Budget:** ~60 K.

### G00.C2 — Experiment C2: Modal stacking + tooltip arbitration

- [x] **Done**
- **Depends on:** G00.C1.
- **Specific:** Extend `experiments/coordination/` with modal stacking (button opens a modal; a button inside opens a nested modal; Escape pops the stack) and tooltip arbitration (hovering any card shows exactly one tooltip while suppressing others). Both concerns use the coordination primitive established in C1.
- **Measurable:** `go run ./experiments/coordination/` demonstrates all three concerns (drag-drop, modal stack, tooltip arbitration) in a single running prototype. `EXPERIMENT-C.md` §C2 records findings; §Decision commits to the Phase 1 `prism.Coordination` package shape (type, fields, injection pattern).
- **Achievable:** extends C1 in the same module; no new module.
- **Relevant:** DESIGN §"Architectural Limits" concern #3.
- **Budget:** ~50 K.
### G00.D — Pre-Phase 0 decisions (concerns #4 and #5)

- [x] **Done**
- **Specific:** write `DECISIONS.md` recording the undo/redo decision (FRP-no-undo vs MessageOp replay buffer) and the plugin-system decision (no extensibility vs scripting layer).
- **Measurable:** two sections in `DECISIONS.md`, each with chosen path, rejected alternative, and rationale citing DESIGN §"Architectural Limits" concerns #4 and #5.
- **Achievable:** documentation only.
- **Relevant:** DESIGN §"Phase 00" pre-Phase 0 commitments.
- **Budget:** ~30 K.

---

## Phase 0 — Token & Theme Contract

Discharges DESIGN §"Phase 0 — Token & Theme Contract". Gated by G00.A–D.

### G0.1 ‖ — `prism/tokens/colors.go`

- [x] **Done**
- **Specific:** typed colour token structs (Background/OnBackground, Surface/OnSurface, etc.); align with Tailwind 50–950 shade scale per DESIGN §"Phase 4 — Token alignment".
- **Measurable:** `go test ./prism/tokens/...` includes contrast-ratio tests asserting WCAG AA compliance for every paired token.
- **Achievable:** one file + one test file.
- **Relevant:** DESIGN §"Phase 0" deliverable 1.
- **Budget:** ~60 K.

### G0.2 ‖ — `prism/tokens/{spacing,radius,elevation,motion}.go`

- [x] **Done**
- **Specific:** four files of typed scales. Spacing on 4-pt grid (Tailwind-aligned).
- **Measurable:** monotonicity tests per scale; `go test ./prism/tokens/...` passes.
- **Achievable:** one goal per file (G0.2a..G0.2d) if any goes long.
- **Relevant:** DESIGN §"Phase 0" deliverable 1.
- **Budget:** ~50 K total or 4 × ~25 K split.

### G0.3 — `prism/theme/theme.go`

- [x] **Done**
- **Specific:** `type Theme struct { Color rx.Observable[ColorTokens]; Type rx.Observable[TypeScale]; Motion rx.Observable[MotionTokens]; … }`.
- **Measurable:** `go test ./prism/theme/...` verifies emission shape via `rx.TestScheduler` per DESIGN §"Testing".
- **Achievable:** one file + tests.
- **Relevant:** DESIGN §"Phase 0" deliverable 2.
- **Budget:** ~60 K. Depends on G0.1, G0.2.

### G0.4 — `prism/theme/auto.go` (extract `AutoLightDark` from coinviz)

- [x] **Done**
- **Specific:** lift `AutoLightDark` out of `coinviz/theme/`; rewrite as `rx.Observable[Theme]`.
- **Measurable:** coinviz now imports `prism/theme` for this; `coinviz/theme/auto*` is deleted; tests in both packages still pass.
- **Achievable:** one mechanical lift.
- **Relevant:** DESIGN §"Phase 1 — Migration path" coinviz step.
- **Budget:** ~50 K.

### G0.5 — `prism/a11y/preferences.go`

- [x] **Done**
- **Specific:** `rx.Observable[A11yPrefs]` exposing reduced motion, high contrast, increased text size; backed by macOS `NSWorkspace` / Windows `SystemParametersInfo` / Linux best-effort.
- **Measurable:** unit tests against a fake OS source verify emissions; manual test on macOS confirms reduced-motion toggle propagates.
- **Achievable:** one file + cgo or platform shim files (one per OS).
- **Relevant:** DESIGN §"Accessibility" + §"Phase 0" deliverable 4.
- **Budget:** ~80 K. **Split** by OS if cgo balloons:
  - **G0.5a** macOS path + interface
  - **G0.5b** Windows path
  - **G0.5c** Linux fallback

### G0.6 — Conditional: `prism/keyed/` (only if G00.A adopted)

- [x] **Done**
- **Specific:** ship `KeyedDefer[K, V]` per the API decided in G00.A.
- **Measurable:** API matches `EXPERIMENT-A.md`; tests cover reorder/insert/delete state preservation.
- **Achievable:** API is already validated in G00.A.
- **Relevant:** DESIGN §"Phase 0" conditional deliverable 1.
- **Budget:** ~60 K.

### G0.7 — Conditional: `prism/cache/` (only if G00.B yields op-cache pattern)

- [x] **Done**
- **Specific:** documented `FrameCache` contract for animation-heavy widgets per `EXPERIMENT-B.md`.
- **Measurable:** contract documented; one reference implementation passes a regression test against G−1.6 baseline minus 30 % allocation rate.
- **Achievable:** API design constrained by experiment.
- **Relevant:** DESIGN §"Phase 0" conditional deliverable 2.
- **Budget:** ~70 K.

### G0.8 — Conditional: `prism/coordination/` shape (only if G00.C produced primitive)

- [x] **Done**
- **Specific:** lock the observable shape of the coordination primitive (types only — implementation lands in Phase 1).
- **Measurable:** `prism/coordination/types.go` compiles; downstream Phase 1 modal/popover/toast can import it.
- **Achievable:** types-only module.
- **Relevant:** DESIGN §"Phase 0" conditional deliverable 3.
- **Budget:** ~30 K.

---

## Phase 1 — Prism (component foundation)

Discharges DESIGN §"Phase 1 — Prism".

### G1.1 — `prism/initial/`

- [x] **Done**
- **Specific:** `Initial[T]` helper replacing ad-hoc first-frame sentinels (e.g., `offset.X = -1`).
- **Measurable:** unit tests cover first-frame and subsequent-frame paths; coinviz pane that currently uses the `-1` sentinel migrates to `Initial[T]` in the same session.
- **Achievable:** small generic helper.
- **Relevant:** DESIGN §"4. The `rx.Defer` Subscription-State Pattern — Initial-frame sentinels".
- **Budget:** ~50 K.

### G1.0 — Golden-image test harness

- [x] **Done**
- **Specific:** `prism/internal/golden/` with `Render(t, widget) -> diff(stored.png)`; CI gate.
- **Measurable:** harness produces a stable PNG for a fixed widget across two consecutive runs; a deliberate one-pixel change is detected.
- **Achievable:** Gio headless backend wrapper.
- **Relevant:** DESIGN §"Testing — Component golden-image tests".
- **Budget:** ~70 K.

### G1.2 — `prism/button/`

- [x] **Done**
- **Specific:** `Button(theme, ButtonProps{...})` with hover, focus, press; `Defer`-scoped state; full a11y (focus, keyboard, reduced motion, contrast); both `OnClick` callback and `MessageOp` paths per DESIGN §"Bridging FRP and MVU".
- **Measurable:** golden-image test (light + dark + focused + pressed); a11y tests; `gallery/` page demonstrates every variant; benchmark in `button_bench_test.go` per DESIGN §"Performance".
- **Achievable:** one component, well-scoped.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~95 K. Depends on G1.0.

### G1.3 ‖ — `prism/input/{textfield,checkbox,radio,dropdown}.go`

- [x] **Done** *(done when G1.3a–G1.3d all checked)*
- **Specific:** four input components split into G1.3a–G1.3d; one session each.
- **Measurable:** all four sub-goals checked.
- **Achievable:** parent tracking goal; implementation across G1.3a–G1.3d.
- **Relevant:** DESIGN §"Phase 1".
- **Budget:** ~80 K per sub-goal.

#### G1.3a — `prism/input/textfield.go`

- [x] **Done**
- **Specific:** `TextField(theme, TextFieldProps{...})` with focus, editing, placeholder text; `Defer`-scoped `widget.Editor` state; full a11y (focus ring, keyboard entry, 44 dp hit target, contrast, screen-reader description); both `OnChange` callback and `MessageOp` paths per DESIGN §"Bridging FRP and MVU".
- **Measurable:** golden-image test (`light-normal`, `dark-normal`, `light-focused`, `light-disabled`); a11y tests (44 dp min height, focus ring visually distinct, disabled visually distinct); `BenchmarkTextFieldRender` in `textfield_bench_test.go`. `go test ./...` green inside `prism/input/`.
- **Achievable:** one component, well-scoped.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~80 K. Depends on G1.2.

#### G1.3b — `prism/input/checkbox.go`

- [x] **Done**
- **Specific:** `Checkbox(theme, CheckboxProps{...})` with checked/unchecked states; `Defer`-scoped `widget.Bool` state; full a11y; both `OnChange` callback and `MessageOp` paths.
- **Measurable:** golden-image test (`light-unchecked`, `dark-unchecked`, `light-checked`, `light-focused`); a11y tests (44 dp hit target, focus ring distinct, checked visually distinct); `BenchmarkCheckboxRender` in `checkbox_bench_test.go`. `go test ./...` green inside `prism/input/`.
- **Achievable:** one component, well-scoped.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~80 K. Depends on G1.3a.

#### G1.3c — `prism/input/radio.go`

- [x] **Done**
- **Specific:** `Radio(theme, RadioProps{...})` with selected/unselected states; `Defer`-scoped `widget.Bool` state; full a11y; both `OnChange` callback and `MessageOp` paths.
- **Measurable:** golden-image test (`light-unselected`, `dark-unselected`, `light-selected`, `light-focused`); a11y tests (44 dp hit target, focus ring distinct, selected visually distinct); `BenchmarkRadioRender` in `radio_bench_test.go`. `go test ./...` green inside `prism/input/`.
- **Achievable:** one component, well-scoped.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~80 K. Depends on G1.3a.

#### G1.3d — `prism/input/dropdown.go`

- [x] **Done**
- **Specific:** `Dropdown(theme, DropdownProps{...})` with open/closed states and option selection; `Defer`-scoped state; full a11y; both `OnSelect` callback and `MessageOp` paths.
- **Measurable:** golden-image test (`light-closed`, `dark-closed`, `light-focused`, `light-open`); a11y tests (44 dp hit target, focus ring distinct, open state visually distinct); `BenchmarkDropdownRender` in `dropdown_bench_test.go`. `go test ./...` green inside `prism/input/`.
- **Achievable:** one component, well-scoped.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~80 K. Depends on G1.3a.

#### G1.3e — `prism/input/textfield.go` submit affordance

- [x] **Done**
- **Specific:** add `Submit bool`, `SubmitMessage func(text string) any`, and `OnSubmit func(text string)` fields to `TextFieldProps` in `prism/input/textfield.go`. When `Submit` is true, configure the inner `widget.Editor` with `Submit: true`. On `widget.SubmitEvent`: emit `mvu.MessageOp{Message: SubmitMessage(text)}` if `SubmitMessage` is non-nil; call `OnSubmit(text)` if non-nil; clear the editor (`editor.SetText("")`).
- **Measurable:** new golden-image test `light-focused-with-text` in `textfield_test.go`; behavioral test confirms (a) `widget.SubmitEvent` triggers `SubmitMessage`/`OnSubmit` with the editor's current text, (b) editor is cleared after submit, (c) callers without `Submit: true` still see ChangeEvent-driven `Message` (no behavioural regression); `BenchmarkTextFieldRender` updated to cover the submit-enabled state; `go test ./prism/input/...` green.
- **Achievable:** additive change to one file (`prism/input/textfield.go`) plus its tests; existing `TextField` callers unaffected; one session.
- **Relevant:** DESIGN §"Bridging FRP and MVU" — submit-style chat inputs need this affordance; unblocks G1.9d.
- **Budget:** ~70 K. Depends on G1.3a.

### G1.4 — `prism/list/`

- [x] **Done**
- **Specific:** virtual scrolling list generalised from `todos/list.go`; respects `KeyedDefer` if G00.A adopted.
- **Measurable:** golden tests for short / long / scrolled-mid lists; benchmark proves O(visible) layout cost not O(total).
- **Achievable:** one well-scoped module.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~95 K. **Split** if benchmark infra absent:
  - **G1.4a** core list mechanic
  - **G1.4b** virtualisation + benchmark

### G1.5 ‖ — `prism/icon/`

- [x] **Done**
- **Specific:** unified registry over existing `svg/` and `ivg/` packages; `Icon(name)` resolves either.
- **Measurable:** test asserts resolution for one SVG + one IVG icon; `gallery/` icon page renders both.
- **Achievable:** thin wrapper over existing modules.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~50 K.

### G1.6 ‖ — `prism/layout/`

- [x] **Done**
- **Specific:** spacing helpers, `FocusGroup`, basic flex/grid wrappers atop Gio's `layout`.
- **Measurable:** golden tests for each helper; `FocusGroup` keyboard traversal test.
- **Achievable:** modest module.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~80 K.

### G1.7 — `prism/coordination/` implementation (if G0.8 landed)

- [x] **Done**
- **Specific:** implement the types from G0.8: scoped subjects, drop-zone registry, modal stack, tooltip arbitration, gesture disambiguation.
- **Measurable:** `experiments/coordination/` G00.C prototype is rewritten on top of `prism/coordination/` and still passes its acceptance test.
- **Achievable:** types are pinned; only implementation work.
- **Relevant:** DESIGN §"Phase 1 — coordination/".
- **Budget:** ~95 K. **Split** along the five sub-concerns if needed.

### G1.8 — `prism/gallery/` (canonical reference app)

- [x] **Done**
- **Specific:** one page per Phase 1 component; every variant, every a11y mode visible.
- **Measurable:** every Phase 1 component appears in gallery; manual launch shows working interactions.
- **Achievable:** assembly only.
- **Relevant:** DESIGN §"Phase 1 — gallery/".
- **Budget:** ~80 K. **Split** by component group if late.

### G1.9 ‖ — Migrate reference apps to Prism

- [x] **Done**
- **Specific:** four sub-goals — one per app — each replacing bespoke widgets with Prism equivalents.
- **Measurable:** per app, no in-repo button/input/list duplication remains; all existing tests still pass.
- **Achievable:** mechanical replacement after Prism stabilises.
- **Relevant:** DESIGN §"Phase 1 — Migration path".
- **Budget:** ~70 K each (G1.9a coinviz, G1.9b appviz, G1.9c todos, G1.9d mindchat).

#### G1.9a ‖ — Migrate coinviz to Prism

- [x] **Done**
- **Specific:** coinviz already uses `prism/initial` and `prism/theme`; no bespoke button/input/list widget functions exist in the package.
- **Measurable:** `go build ./coinviz/...` passes; `grep -rn '^func.*Button\|^func.*Checkbox\|^func.*List' coinviz/` finds nothing.
- **Achievable:** verification only — already compliant.
- **Relevant:** DESIGN §"Phase 1 — Migration path".
- **Budget:** ~70 K.

#### G1.9b ‖ — Migrate appviz to Prism

- [x] **Done**
- **Specific:** appviz has no bespoke button/input/list widget functions.
- **Measurable:** `go build ./appviz/...` passes; `grep -rn '^func.*Button\|^func.*Checkbox\|^func.*List' appviz/` finds nothing.
- **Achievable:** verification only — already compliant.
- **Relevant:** DESIGN §"Phase 1 — Migration path".
- **Budget:** ~70 K.

#### G1.9c — Migrate todos to Prism

- [x] **Done**
- **Specific:** delete `todos/button.go`; replace bespoke `Button`, `IconBtn`, `Checkbox`, `Edit`, `List` in `todos/list.go` with `prism/button`, `prism/input`, `prism/list` via the MVU MessageOp callback path; remove `todos/theme.go` if no longer needed.
- **Measurable:** `todos/button.go` absent; no bespoke button/input/list function remains in `todos/`; `go build ./todos/...` passes.
- **Achievable:** mechanical replacement; one session.
- **Relevant:** DESIGN §"Phase 1 — Migration path".
- **Budget:** ~70 K.

#### G1.9d — Migrate mindchat to Prism

- [x] **Done**
- **Specific:** replace `EditWidget()` in `mindchat/view.go` with `prism/input.TextField` via MVU MessageOp callback path.
- **Measurable:** no bespoke editor-widget function remains in `mindchat/`; `go build ./mindchat/...` passes.
- **Achievable:** mechanical replacement; one session.
- **Relevant:** DESIGN §"Phase 1 — Migration path".
- **Budget:** ~70 K.

---

## Phase 2 — Spectrum (reactive theme runtime)

Discharges DESIGN §"Phase 2". Gated by G1 stability.

### G2.0 — Naming decision

- [x] **Done**
- **Specific:** decided **Spectrum** (rejected original candidates Pulse / Cadence / Resonance). DESIGN.md §"Naming Considerations" and this PLAN.md heading updated to match. Module path: `vibrantgio/spectrum`.
- **Measurable:** final name committed; module path reserved.
- **Achievable:** decision-only.
- **Relevant:** DESIGN §"Naming Considerations" — "Decide before implementation begins."
- **Budget:** ~20 K.

### G2.1 — User-preference persistence

- [x] **Done**
- **Specific:** persist chosen theme + a11y prefs to OS-appropriate config dir; emit on launch.
- **Measurable:** prefs survive app restart; verified by integration test.
- **Achievable:** one module.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~60 K.

### G2.2 — System-event bridging

- [x] **Done**
- **Specific:** subscribe to OS dark-mode and accent-colour change events; bridge to `Theme` observable.
- **Measurable:** macOS test asserts `defaults write -g AppleInterfaceStyle Dark` triggers an emission within 1 s.
- **Achievable:** per-OS shim files.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~80 K. **Split** by OS.

### G2.3 — Animated theme transitions

- [x] **Done**
- **Specific:** colour interpolation across theme switches via `Tween[Color]` (lands in Phase 3 or pre-Phase 3 stub here).
- **Measurable:** golden test of transitioning theme at frame 0/15/30; tween settles to target.
- **Achievable:** one tween implementation + integration.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~70 K.

### G2.4 — Per-window theme override

- [x] **Done**
- **Specific:** allow each `Window.Render` call to receive an independent `rx.Observable[Theme]`.
- **Measurable:** test launches two windows with different themes and verifies isolation.
- **Achievable:** mostly contract-level; small runtime change.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~50 K.

---

## Phase 3 — Pulse (visual effects layer)

Discharges DESIGN §"Phase 3". Parallelisable with Phase 2 once Phase 1 lands.

### G3.0 — Naming decision

- [x] **Done**
Same template as G2.0. Decided **Pulse** (rejected original candidates Verve / Vivace / Kinesis); repurposed from a Phase 2 candidate because rhythm fits motion more naturally than emissions. Module path: `vibrantgio/pulse`. Budget ~20 K.

### G3.1 ‖ — `pulse/tween/`

- [x] **Done**
- **Specific:** generic `Tween[T]` for fades, slides, colour interpolations.
- **Measurable:** deterministic settling-time tests per DESIGN §"Testing — Physics convergence tests".
- **Budget:** ~60 K.

### G3.2 ‖ — `pulse/spring/`

- [x] **Done**
- **Specific:** physics-based motion via `traer`; `Defer`-scoped `ParticleSystem`.
- **Measurable:** spring settles within tolerance under fixed seed.
- **Budget:** ~70 K.

### G3.3 ‖ — `pulse/glow/` and `pulse/depth/`

- [x] **Done** *(done when G3.3a–G3.3b all checked)*
- **Specific:** two visual-effect packages split into G3.3a (glow) and G3.3b (depth); one session each. Glow = luminance halo via gradient composition; depth = elevation-driven shadow layers.
- **Measurable:** both sub-goals checked.
- **Achievable:** parent tracking goal; implementation across G3.3a–G3.3b.
- **Relevant:** DESIGN §"Phase 3 — Pulse".
- **Budget:** ~60 K per sub-goal.

#### G3.3a — `pulse/glow/`

- [x] **Done**
- **Specific:** `pulse/glow/` package exposing a `Halo(gtx, bounds, opts)` primitive that renders a luminance halo around a rectangular region by composing 4 edge-band linear gradients (top/right/bottom/left) and 4 corner-triangle linear gradients, each clipped to its own region so corners do not double-paint. `Options` fields: `Color color.NRGBA`, `Radius int` (px halo extent beyond bounds), `Intensity float64` (peak alpha multiplier in [0,1]).
- **Measurable:** golden-image tests at four intensities (`intensity-zero`, `intensity-low`, `intensity-mid`, `intensity-high`); pixel-diff sanity tests assert successive intensities differ; `go test ./pulse/glow/...` green.
- **Achievable:** one package, one entry point, one Options struct; uses `gioui.org/op/paint.LinearGradientOp` directly — no new shader, no new gradient package.
- **Relevant:** DESIGN §"Phase 3 — Pulse" (`glow/ — luminance halos via gradient composition`).
- **Budget:** ~60 K. No code dependency on G3.1 / G3.2.

#### G3.3b — `pulse/depth/`

- [x] **Done**
- **Specific:** `pulse/depth/` package exposing a `Shadow(gtx, bounds, level tokens.ElevationLevel)` primitive that draws a Material-style cast shadow under a rectangular region by composing linear gradients whose offset and extent are derived from the elevation level (`prism/tokens.Elevation` Level0–Level5: 0/1/3/6/8/12 dp).
- **Measurable:** golden-image tests at all six elevation levels (`level-0` through `level-5`); pixel-diff test asserts adjacent levels differ; `go test ./pulse/depth/...` green.
- **Achievable:** one package, one entry point; reuses the gradient-composition technique established by G3.3a, parameterised by elevation rather than intensity.
- **Relevant:** DESIGN §"Phase 3 — Pulse" (`depth/ — elevation-driven shadow layers`); `prism/tokens/elevation.go` Level0–Level5.
- **Budget:** ~60 K. Depends on G3.3a (shared gradient-composition pattern); soft dependency only — no import.

### G3.4 — `pulse/motion/`

- [x] **Done**
- **Specific:** enter/exit/transition primitives composing tween + spring.
- **Measurable:** golden tests of enter, exit, swap on a `prism.Button` variant.
- **Budget:** ~80 K.

### G3.5 — `pulse/conductor/`

- [x] **Done**
- **Specific:** shared clock for coordinated animation across widgets.
- **Measurable:** test demonstrates staggered list reveal across N rows phase-locked.
- **Relevant:** DESIGN §"5. Frame-Driven Physics — Caveats — Independent simulations are not synchronised".
- **Budget:** ~80 K.

### G3.6 — Spring-variant components in Prism

- [x] **Done**
- **Specific:** for each Phase 1 interactive component, ship a `pulse.SpringX` variant per the composition mechanism in DESIGN §"Phase 3 — Composition mechanism".
- **Measurable:** `pulse.SpringButton` documented; `gallery/` shows side-by-side static vs spring.
- **Budget:** ~80 K per variant family. One goal per Phase 1 component (G3.6a..G3.6n).

---

## Phase 4 — Cadence (pattern library)

Discharges DESIGN §"Phase 4".

### G4.0 — Naming decision

- [x] **Done**
Decided **Cadence** (rejected original candidates Folio / Atelier / Suite); reassigned from a Phase 2 candidate because "rhythm of composition" fits patterns more naturally than theme emission. Module path: `vibrantgio/cadence`. Budget ~20 K.

### G4.1 ‖ — Static patterns: card, alert, breadcrumb, pagination

- [x] **Done** *(done when G4.1a–G4.1d all checked)*
- **Specific:** four static-content pattern packages split into G4.1a (card), G4.1b (alert), G4.1c (breadcrumb), G4.1d (pagination); one session each. "Static" = no coordination primitive (no popover/modal/toast); content layout only, composed from Prism primitives.
- **Measurable:** all four sub-goals checked.
- **Achievable:** parent tracking goal; implementation across G4.1a–G4.1d.
- **Relevant:** DESIGN §"Phase 4 — Cadence (pattern library)" — Composition contract.
- **Budget:** ~50 K per sub-goal.

#### G4.1a — `cadence/card/`

- [x] **Done**
- **Specific:** `cadence/card/` package exposing `Card(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget]` plus a static `Render(...) layout.Widget` for golden testing. `Props` carries `Header`, `Body`, `Footer layout.Widget` slots (any may be nil) and an optional `Elevated bool` flag selecting between flat outlined and shadowed surface variants. The visual is a rounded `Surface` rectangle with `Outline` border (or shadow, when elevated), padded `S4`, with the three slots stacked vertically separated by `S3`.
- **Measurable:** golden-image tests for `light-normal`, `dark-normal`, `light-header-only`, `light-elevated`; `go test ./cadence/card/...` green; copy-paste-friendly source per DESIGN §"Phase 4 — Composition contract".
- **Achievable:** one package, one entry point, one Props struct; pure layout composition — no event handling, no rx.Defer state, no coordination primitive.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`card/ — content cards with header/body/footer slots`).
- **Budget:** ~50 K. No dependency on G4.1b–d.

#### G4.1b — `cadence/alert/`

- [x] **Done**
- **Specific:** `cadence/alert/` package exposing `Alert(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props` carries `Variant` (`Info`, `Success`, `Warning`, `Error`), a `Title string`, and a `Body layout.Widget`. Visual is a tinted-`Surface` rounded rectangle with a leading icon slot (variant-dependent) and `OnSurface` typography.
- **Measurable:** golden-image tests for each variant × {light, dark} producing at least `info-light`, `info-dark`, `warning-light`, `error-light`; `go test ./cadence/alert/...` green.
- **Achievable:** one package, one Props struct; uses `prism/tokens` `Error` and `Primary` colour roles, plus locally-defined tint helpers. No icon dependency for the baseline goldens — start with a chevron rendered from `clip.Path`; richer icons can come from `prism/icon` once available.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`alert/ — info / success / warning / error banners`).
- **Budget:** ~50 K. No dependency on sibling sub-goals.

#### G4.1c — `cadence/breadcrumb/`

- [x] **Done**
- **Specific:** `cadence/breadcrumb/` package exposing `Breadcrumb(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props.Items []Item` where `Item` has `Label string` and `OnClick func()` (nil → non-interactive current segment). Visual is a horizontal row of labels separated by a chevron glyph; the last item rendered in `OnSurface` and remaining items in `OnSurfaceVariant`.
- **Measurable:** golden-image tests `light-three-segments`, `dark-three-segments`, `light-single-segment`; `go test ./cadence/breadcrumb/...` green.
- **Achievable:** one package; reuses `prism/button` interaction model for clickable segments. Chevron rendered from `clip.Path`.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`breadcrumb/ — hierarchical location indicators`).
- **Budget:** ~50 K. No dependency on sibling sub-goals.

#### G4.1d — `cadence/pagination/`

- [x] **Done**
- **Specific:** `cadence/pagination/` package exposing `Pagination(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props` carries `Page int`, `PageCount int`, `OnSelect func(page int)`. Visual is a horizontal row of numbered page buttons with prev/next chevrons; current page highlighted via `Primary`/`OnPrimary` tokens.
- **Measurable:** golden-image tests `light-page-1-of-5`, `light-page-3-of-5`, `dark-page-3-of-5`; `go test ./cadence/pagination/...` green.
- **Achievable:** one package; reuses `prism/button` for the page buttons. No virtualisation, no ellipsis collapse — that is deferred to G4.4 (table + pagination at scale).
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`pagination/ — page navigation controls`).
- **Budget:** ~50 K. No dependency on sibling sub-goals.

### G4.2 ‖ — Patterns depending on coordination primitive: modal, popover, tooltip, toast

- [x] **Done** *(done when G4.2a–G4.2d all checked)*
- **Specific:** four coordination-dependent pattern packages split into G4.2a (modal), G4.2b (popover), G4.2c (tooltip), G4.2d (toast); one session each. All depend on `prism/coordination/` (G1.7) for cross-widget arbitration (focus, dismissal, single-active). Modal is sequenced first because its acceptance criterion (focus trap + escape handling) is the hardest interaction proof and de-risks the remaining three.
- **Measurable:** all four sub-goals checked.
- **Achievable:** parent tracking goal; implementation across G4.2a–G4.2d.
- **Relevant:** DESIGN §"Phase 4 — Cadence (pattern library)" — Composition contract; §"Coordination ceiling".
- **Budget:** ~80 K per sub-goal.

#### G4.2a — `cadence/modal/`

- [x] **Done**
- **Specific:** `cadence/modal/` package exposing `Modal(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget]` plus a static `Render(...) layout.Widget` for golden testing. `Props` carries `Open rx.Observable[bool]`, `Title string`, `Body layout.Widget`, `OnClose func()`, and optional `Actions []layout.Widget` (footer button row, any may be nil). Visual: full-window scrim backdrop, centered elevated `Surface` with rounded corners, header row (title + close affordance), padded body, and footer action row. Uses `prism/coordination` `Subject` for modal-stack depth so nested modals layer correctly and only the topmost receives keyboard focus.
- **Measurable:** golden-image tests `light-open`, `dark-open`, `light-closed` (renders nothing), `light-with-actions`; interaction tests that prove (a) escape key invokes `OnClose`, (b) tab cycles focus within the modal and does not escape to background content, (c) backdrop click invokes `OnClose`; `go test ./cadence/modal/...` green.
- **Achievable:** one package; reuses `prism/button` for header close + footer actions, `prism/coordination` for stack depth. No animation in this goal — open/close is instantaneous; entrance/exit transitions are deferred to a later Pulse-integration goal.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`modal/ — dialog with backdrop, focus trap, escape handling`); §"Coordination ceiling".
- **Budget:** ~80 K. No dependency on G4.2b–d.

#### G4.2b — `cadence/popover/`

- [x] **Done**
- **Specific:** `cadence/popover/` package exposing `Popover(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props` carries `Open rx.Observable[bool]`, `Anchor layout.Widget`, `Content layout.Widget`, `Placement` (`Top`, `Bottom`, `Left`, `Right`), and `OnDismiss func()`. Visual: anchored elevated `Surface` with rounded corners positioned adjacent to the anchor per `Placement`, with a small triangular tail glyph pointing at the anchor. Uses `prism/coordination` `Subject` to coordinate outside-click dismissal so opening a second popover dismisses the first.
- **Measurable:** golden-image tests for each placement × {light, dark} producing at least `top-light`, `bottom-light`, `left-dark`, `right-dark`; interaction test that a pointer click outside the popover bounds invokes `OnDismiss`; `go test ./cadence/popover/...` green.
- **Achievable:** one package; positioning math relative to the anchor's last-recorded layout rect; coordination subject scoped to a popover-arbitration channel. No collision-aware reflow (i.e., no automatic flip when the placement would clip the viewport) — that is deferred.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`popover/ — anchored floating content`); §"Coordination ceiling".
- **Budget:** ~80 K. No dependency on sibling sub-goals.

#### G4.2c — `cadence/tooltip/`

- [x] **Done**
- **Specific:** `cadence/tooltip/` package exposing `Tooltip(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props` carries `Text string`, `Trigger layout.Widget`, an optional `Delay time.Duration` (default 500 ms before show after hover/focus entry), and `Placement` (`Top`, `Bottom`, `Left`, `Right`). Uses `prism/coordination` `Subject` for arbitration so only one tooltip is visible across the window at a time — showing a new tooltip cancels the previous.
- **Measurable:** golden-image tests `light-shown-top`, `dark-shown-bottom`; interaction tests that prove (a) hover entry after `Delay` shows the tooltip, (b) hover exit hides it, (c) a second tooltip becoming active hides the first; `go test ./cadence/tooltip/...` green.
- **Achievable:** one package; reuses gesture hover/focus primitives already used by `prism/button`; coordination subject for arbitration. Keyboard-focus-driven show is in scope; touch long-press is not.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`tooltip/ — small hover/focus annotations`); §"Coordination ceiling".
- **Budget:** ~80 K. No dependency on sibling sub-goals.

#### G4.2d — `cadence/toast/`

- [x] **Done**
- **Specific:** `cadence/toast/` package exposing `Stack(th, props) rx.Observable[layout.Widget]` rendering a positioned column of queued toasts, plus a `Notify(level, text)` entry point that emits a `Toast` value onto a package-scoped `prism/coordination` `Subject[Toast]`. `Props` carries `Position` (one of `TopRight`, `BottomRight`, `TopLeft`, `BottomLeft`), `Lifetime time.Duration` (default 4 s before auto-dismiss). Each toast renders as a tinted `Surface` (variant per level: info/success/warning/error) with text and an optional dismiss affordance.
- **Measurable:** golden-image tests `light-empty`, `light-three-stacked`, `dark-warning-toast`; interaction test that `Notify` adds a toast to the stack and the stack length returns to its prior value after `Lifetime` elapses; `go test ./cadence/toast/...` green.
- **Achievable:** one package; reuses `prism/coordination.Subject` for the queue and `pulse/tween` for fade-out near end of lifetime. Stack ordering is FIFO with newest nearest the position-anchored edge; no overflow collapse.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`toast/ — transient notification stack`); §"Coordination ceiling".
- **Budget:** ~80 K. No dependency on sibling sub-goals.

### G4.3 ‖ — Navigation: navbar, sidebar, tabs, accordion, shell

- [x] **Done** *(done when G4.3a–G4.3e all checked)*
- **Specific:** five navigation pattern packages split into G4.3a (navbar), G4.3b (sidebar), G4.3c (tabs), G4.3d (accordion), G4.3e (shell); one session each. G4.3e (shell) sequenced last because it composes G4.3a (navbar) and G4.3b (sidebar).
- **Measurable:** all five sub-goals checked.
- **Achievable:** parent tracking goal; implementation across G4.3a–G4.3e.
- **Relevant:** DESIGN §"Phase 4 — Cadence (pattern library)" — Composition contract.
- **Budget:** ~80 K per sub-goal.

#### G4.3a — `cadence/navbar/`

- [x] **Done**
- **Specific:** `cadence/navbar/` package exposing `Navbar(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget]` plus a static `Render(...) layout.Widget` for golden testing. `Props` carries `Brand layout.Widget` (logo/title slot, may be nil), `Links []Link` (each with `Label string`, `OnClick func()`, `Active bool`), and `Actions []layout.Widget` (trailing action buttons, any may be nil). Visual is a horizontal `Surface` bar with `S4` padding: brand on the leading edge, link row centered, action buttons on the trailing edge. The active link is rendered with a `Primary` underline.
- **Measurable:** golden-image tests `light-default`, `dark-default`, `light-active-second-link`; interaction test that proves Tab cycles focus through brand → links → actions in document order and Shift-Tab reverses, and clicking a link invokes its `OnClick`; `go test ./cadence/navbar/...` green.
- **Achievable:** one package; reuses `prism/button` for links and actions. No mobile collapse to hamburger menu; that responsive behaviour is deferred (would require viewport observation outside scope).
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`navbar/ — top navigation with branding, links, actions`).
- **Budget:** ~80 K. No dependency on sibling sub-goals.

#### G4.3b — `cadence/sidebar/`

- [x] **Done**
- **Specific:** `cadence/sidebar/` package exposing `Sidebar(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props` carries `Items []Item` (each with `Icon layout.Widget`, `Label string`, `OnClick func()`, `Active bool`), `Collapsed rx.Observable[bool]` for the expanded/collapsed state, and `OnToggleCollapse func()`. Visual is a vertical `Surface` column whose width swaps between expanded (~`S48`) and collapsed (~`S12`); when collapsed, labels are hidden and only icons remain. The active item is rendered with a `Primary` background tint.
- **Measurable:** golden-image tests `light-expanded`, `light-collapsed`, `dark-expanded-active-second`; interaction test that proves Arrow-Up/Arrow-Down move focus between items, Enter activates the focused item via `OnClick`, and triggering the toggle affordance dispatches `OnToggleCollapse`; `go test ./cadence/sidebar/...` green.
- **Achievable:** one package; reuses `prism/button` for items and toggle affordance; icons sourced from `prism/icon` if available, else `clip.Path` glyphs. Width transition is an instantaneous swap — Pulse-driven width tween is deferred.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`sidebar/ — collapsible side navigation`).
- **Budget:** ~80 K. No dependency on sibling sub-goals.

#### G4.3c — `cadence/tabs/`

- [x] **Done**
- **Specific:** `cadence/tabs/` package exposing `Tabs(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props` carries `Tabs []Tab` (each with `Label string`, `Content layout.Widget`), `Selected rx.Observable[int]`, and `OnSelect func(idx int)`. Visual is a horizontal tab strip with the selected tab underlined in `Primary`, followed by the selected tab's `Content` rendered below.
- **Measurable:** golden-image tests `light-three-tabs-first-selected`, `dark-three-tabs-second-selected`, `light-single-tab`; interaction test that proves Arrow-Left/Arrow-Right move selection between tabs (wrapping at the ends) and Home/End jump to first/last, with focus following selection per the WAI-ARIA tab pattern; `go test ./cadence/tabs/...` green.
- **Achievable:** one package; reuses `prism/button` for tab labels. No vertical tab orientation, no scrollable overflow when tab count exceeds available width — both deferred.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`tabs/ — tab strip + content panel`).
- **Budget:** ~80 K. No dependency on sibling sub-goals.

#### G4.3d — `cadence/accordion/`

- [x] **Done**
- **Specific:** `cadence/accordion/` package exposing `Accordion(th, props) rx.Observable[layout.Widget]` plus a static `Render`. `Props` carries `Sections []Section` (each with `Title string`, `Body layout.Widget`), `Open rx.Observable[map[int]bool]`, `OnToggle func(idx int)`, and `SingleOpen bool` (when true, opening one section closes the others). Each section renders as a header row with a chevron rotated per open state, followed by the body when open.
- **Measurable:** golden-image tests `light-three-sections-first-open`, `dark-three-sections-all-closed`, `light-single-open-mode`; interaction test that proves Arrow-Up/Arrow-Down move focus between section headers, Enter/Space toggles the focused section, and `SingleOpen=true` collapses prior sections when a new one opens; `go test ./cadence/accordion/...` green.
- **Achievable:** one package; reuses `prism/button` for header rows. Open/close is instantaneous; height-animated expand is deferred to a Pulse-integration goal.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`accordion/ — collapsible section groups`).
- **Budget:** ~80 K. No dependency on sibling sub-goals.

#### G4.3e — `cadence/shell/`

- [x] **Done**
- **Specific:** `cadence/shell/` package exposing `Shell(th, props) rx.Observable[layout.Widget]` plus a static `Render`. Two layout variants are selected via `Props.Layout` (`SidebarHeaderMain` or `SplitPane`): `SidebarHeaderMain` composes a `cadence/sidebar` on the leading edge, a `cadence/navbar` across the top of the remaining area, and a `Main layout.Widget` content slot below the navbar; `SplitPane` composes two slots (`Left`, `Right layout.Widget`) separated by a draggable vertical divider with configurable initial ratio. `Props` also carries `Sidebar sidebar.Props`, `Navbar navbar.Props`, `Main layout.Widget`, plus `Left`, `Right layout.Widget`, and `SplitRatio rx.Observable[float32]`, `OnSplitChange func(ratio float32)`.
- **Measurable:** golden-image tests `light-sidebar-header-main`, `dark-sidebar-header-main`, `light-split-pane-50-50`, `light-split-pane-30-70`; interaction test that proves the split-pane divider can be dragged with the pointer and emits ratio updates via `OnSplitChange`, and that Tab traversal flows sidebar → navbar → main in `SidebarHeaderMain` mode; `go test ./cadence/shell/...` green.
- **Achievable:** one package; composes `cadence/sidebar` (G4.3b) and `cadence/navbar` (G4.3a); split-pane divider uses pointer drag from `prism/button`'s gesture primitives. No keyboard-driven divider resize, no horizontal split orientation — both deferred.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`shell/ — application shells (sidebar+header+main, split-pane, etc.)`).
- **Budget:** ~80 K. Depends on G4.3a (navbar) and G4.3b (sidebar) — sequence last.

### G4.4 — Table and pagination

- [x] **Done**
- **Specific:** sortable, filterable, virtualised table consuming `prism/list` + `KeyedDefer` (if adopted) for per-row state.
- **Measurable:** benchmark proves O(visible-rows) cost on 10 000-row dataset.
- **Budget:** ~95 K. **Split** core / virtualisation / sort+filter if needed.

### G4.5 ‖ — Marketing sections

- [x] **Done** *(done when G4.5a–G4.5d all checked)*
- **Specific:** four marketing-section pattern packages split into G4.5a (hero), G4.5b (pricing), G4.5c (feature), G4.5d (testimonial); one session each. Pure layout composition — no coordination primitive, no focus arbitration, no per-row state — for app landing and onboarding pages. No inter-sub-goal dependencies; ordered alphabetically.
- **Measurable:** all four sub-goals checked.
- **Achievable:** parent tracking goal; implementation across G4.5a–G4.5d.
- **Relevant:** DESIGN §"Phase 4 — Cadence (pattern library)" — Composition contract; (`marketing/ — hero, pricing, feature, testimonial sections (for app landing/onboarding)`).
- **Budget:** ~50 K per sub-goal.

#### G4.5a — `cadence/hero/`

- [x] **Done**
- **Specific:** `cadence/hero/` package exposing `Hero(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget]` plus a static `Render(...) layout.Widget` for golden testing. `Props` carries `Eyebrow string` (small tag above the title, empty = omitted), `Title string`, `Subtitle string`, `PrimaryCTA, SecondaryCTA *CTA` (any may be nil), and `Visual layout.Widget` (optional illustration slot, nil = text-only centered layout). `CTA struct { Label string; OnClick func() }` is defined in this package — interactivity is rendered via `prism/button` internally but the cadence API does not leak the primitive type. Visual: when `Visual` is nil, content is centered in a single column with `S6` padding — eyebrow in `Primary` micro-cap typography, title in display typography (`OnSurface`), subtitle in body-large (`OnSurfaceVariant`), CTAs in a horizontal row with `S3` gap (primary filled, secondary outlined); when `Visual` is non-nil, layout splits into two equal columns (text leading, visual trailing) with `S6` gutter.
- **Measurable:** golden-image tests `light-text-only`, `dark-text-only`, `light-with-visual`, `light-eyebrow-and-dual-cta`; `go test ./cadence/hero/...` green; copy-paste-friendly source per DESIGN §"Phase 4 — Composition contract".
- **Achievable:** one package, one entry point, one Props struct; pure layout composition — no rx.Defer state, no coordination primitive. CTA buttons reuse `prism/button` for hit-testing and visual variants. The `Visual` slot is opaque — caller supplies any `layout.Widget` (image, illustration, video frame, etc.).
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`marketing/ — hero, pricing, feature, testimonial sections (for app landing/onboarding)`).
- **Budget:** ~50 K. No dependency on sibling sub-goals.

#### G4.5b — `cadence/pricing/`

- [x] **Done**
- **Specific:** `cadence/pricing/` package exposing `Pricing(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget]` plus a static `Render(...) layout.Widget` for golden testing. `Props.Tiers []Tier` where `Tier struct { Name string; Price string; Cadence string; Features []string; CTA *CTA; Highlighted bool }` (`Cadence` e.g. `"/mo"`; `Highlighted` selects the emphasised tier). `CTA struct { Label string; OnClick func() }` is defined in this package. Visual: horizontal row of tier cards (rounded `Surface` with `Outline` border, `S5` padding, `S4` inter-card gap), each containing — top to bottom — tier name in title typography (`OnSurface`), price + cadence in display typography (price prominent, cadence muted `OnSurfaceVariant`), a vertical feature list with a leading checkmark glyph rendered from `clip.Path`, and a footer CTA button. The `Highlighted` tier renders with a `Primary` border (2 px) and a small `Primary` chip above the tier name reading "Popular".
- **Measurable:** golden-image tests `light-three-tier`, `dark-three-tier`, `light-three-tier-highlighted`, `light-single-tier`; `go test ./cadence/pricing/...` green; copy-paste-friendly source per DESIGN §"Phase 4 — Composition contract".
- **Achievable:** one package; CTA buttons reuse `prism/button`. Checkmark glyph is a local `clip.Path` (no `prism/icon` dependency). No responsive breakpoint to stack tiers vertically — that responsive behaviour is deferred (would require viewport observation outside scope).
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`marketing/ — hero, pricing, feature, testimonial sections (for app landing/onboarding)`).
- **Budget:** ~50 K. **Split** core tier layout / highlighted-variant rendering if needed. No dependency on sibling sub-goals.

#### G4.5c — `cadence/feature/`

- [x] **Done**
- **Specific:** `cadence/feature/` package exposing `Feature(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget]` plus a static `Render(...) layout.Widget` for golden testing. `Props` carries `Columns int` (default 3 when zero) and `Items []Item` where `Item struct { Icon layout.Widget; Title string; Body string }` (`Icon` may be nil). Visual: a grid laying `Items` out in rows of `Columns` items each with `S5` cell gap and `S6` outer padding; each cell stacks icon (top, sized to `S8` square when present), title in title-medium typography (`OnSurface`), and body in body typography (`OnSurfaceVariant`).
- **Measurable:** golden-image tests `light-3-up`, `dark-3-up`, `light-2-up`, `light-6-items-3-up`; `go test ./cadence/feature/...` green; copy-paste-friendly source per DESIGN §"Phase 4 — Composition contract".
- **Achievable:** one package; pure layout composition — no interaction, no rx.Defer state. The `Icon` slot is opaque — caller supplies any `layout.Widget`. No responsive collapse from `Columns` to a smaller column count on narrow viewports — that responsive behaviour is deferred.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`marketing/ — hero, pricing, feature, testimonial sections (for app landing/onboarding)`).
- **Budget:** ~50 K. No dependency on sibling sub-goals.

#### G4.5d — `cadence/testimonial/`

- [x] **Done**
- **Specific:** `cadence/testimonial/` package exposing `Testimonial(th rx.Observable[theme.Theme], props Props) rx.Observable[layout.Widget]` plus a static `Render(...) layout.Widget` for golden testing. `Props.Variant` (`Single` or `Grid`) selects between a single-card centered layout and a horizontal row of cards; `Props.Items []Item` where `Item struct { Quote string; AuthorName string; AuthorRole string; AuthorAvatar layout.Widget }` (`AuthorAvatar` may be nil — when nil, an `Outline`-bordered circular placeholder with the first letter of `AuthorName` is rendered). Visual: each card is a rounded `Surface` with `Outline` border, `S5` padding — opening quotation glyph (rendered from `clip.Path`) in `Primary` at the top-leading edge, the quote body in body-large typography (`OnSurface`), then a horizontal author block (avatar, then name in `OnSurface` and role in `OnSurfaceVariant` stacked). `Single` variant uses one card centered with `S6` margin; `Grid` lays cards in a horizontal row with `S4` gap.
- **Measurable:** golden-image tests `light-single`, `dark-single`, `light-grid-three`, `dark-grid-three`; `go test ./cadence/testimonial/...` green; copy-paste-friendly source per DESIGN §"Phase 4 — Composition contract".
- **Achievable:** one package; pure layout composition — no interaction, no rx.Defer state. Quote glyph rendered from `clip.Path` (no `prism/icon` dependency). No responsive collapse from grid to vertical stack — that responsive behaviour is deferred.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`marketing/ — hero, pricing, feature, testimonial sections (for app landing/onboarding)`).
- **Budget:** ~50 K. No dependency on sibling sub-goals.

---

## Phase 5 — Example apps (pressure-test)

**Goal:** validate VibrantGIO end-to-end by building non-trivial apps in real composition. Each app's primary deliverable is a `FEEDBACK-G5.x.md` listing API friction, missing pieces, awkward compositions, and ergonomics wins discovered during the build — the apps themselves are the vehicle.

**No new framework code in this phase.** If a sub-goal needs missing functionality from Prism / Cadence / Spectrum / Pulse, append the finding to the relevant running `FEEDBACK-G5.x.md` and either work around or skip. Re-plan work is queued to a future phase once feedback is aggregated.

**Sequencing.** G5.1 depends on G4.5a–d (marketing patterns). G5.2 and G5.3 depend only on Phase 4 sub-goals already shipped (G4.1–G4.4 and the modal/popover/toast/tooltip + navigation packs) — they can start any time, including in parallel with G4.5a–d, if the developer prefers to start pressure-testing sooner.

**Per-session feedback discipline.** Every implementation sub-goal's `Measurable` requires appending discovered friction to the relevant `FEEDBACK-G5.x.md` before the session closes — even a single line is acceptable, including the explicit line "no findings yet". The dedicated feedback sub-goal at the end of each app ranks and rewrites those running notes; it does not invent them. If the running file does not exist when the feedback sub-goal starts, that is itself a process failure to surface.

### G5.1 ‖ — Docs site for VibrantGIO itself

- [ ] **Done** *(done when G5.1a–G5.1d all checked)*
- **Specific:** native desktop app rendering the VibrantGIO landing page and docs using VibrantGIO itself, in a new top-level Go module `vibrantgio/sitedocs/`. Split into G5.1a (skeleton + shell), G5.1b (landing-page content wiring), G5.1c (multi-page docs), G5.1d (feedback writeup). Dogfoods the marketing sub-goals in the use case for which they were created.
- **Measurable:** all four sub-goals checked; `FEEDBACK-G5.1.md` exists in repo root in the structured form defined in G5.1d.
- **Achievable:** parent tracking goal; implementation across G5.1a–G5.1d.
- **Relevant:** DESIGN §"Phase 4 — Cadence (pattern library)" — Composition contract; pressure-tests G4.5a–d in real composition.
- **Budget:** ~50–70 K per sub-goal.

#### G5.1a — App skeleton + shell

- [ ] **Done**
- **Specific:** new top-level Go module `vibrantgio/sitedocs/` (joined to `go.work`). `sitedocs/main.go` bootstraps a window via `prism/initial`, wires `spectrum` theme (light/dark auto), and renders `cadence/shell.Shell(SidebarHeaderMain)` with: navbar carrying brand "VibrantGIO" and three placeholder navigation links ("Home", "Docs", "About"); sidebar with two collapsible placeholder sections; Main showing placeholder text. A `currentPage rx.Subject[string]` (values `"home" | "docs"`) is wired through; only the placeholder text consumes it.
- **Measurable:** `go build ./sitedocs/...` green; `go test ./sitedocs/...` green (smoke test that constructs the root widget without panic); running `go run ./sitedocs/` opens a 1200×800 window with sidebar + navbar + placeholder Main visible in both light and dark themes; `FEEDBACK-G5.1.md` is created with first entries or the explicit line "no findings yet".
- **Achievable:** skeleton only. No content beyond placeholder text. Routing is the absolute minimum (one subject, two values). No persistence, no IPC, no networking. CTAs may be `func(){}` no-ops.
- **Relevant:** DESIGN §"Phase 4 — Cadence" — first real composition of `cadence/shell` + `cadence/sidebar` + `cadence/navbar` outside golden tests.
- **Budget:** ~60 K. Depends on G4.5a (only because `landing.go` in G5.1b consumes it; technically G5.1a could ship earlier, but no incentive to split).

#### G5.1b — Landing page content (marketing patterns)

- [ ] **Done**
- **Specific:** `sitedocs/landing.go` renders the Home page composed of the four marketing patterns wired with real content: `cadence/hero` (eyebrow "Native desktop · Go", title "VibrantGIO", subtitle naming the four phases Prism / Cadence / Spectrum / Pulse, primary CTA "Get started" routing to docs, secondary CTA "GitHub" no-op); `cadence/feature` (3-up grid: "Prism — component foundation", "Cadence — pattern library", "Pulse — motion + effects"); `cadence/pricing` (synthetic 3-tier: Free / Pro / Enterprise — realistic-enough copy distinguishing tiers); `cadence/testimonial` (3-card grid, synthetic-but-plausible quotes). Copy lives in `sitedocs/landing_content.go` for one-place editing.
- **Measurable:** `go test ./sitedocs/...` green (includes a golden of the rendered Home page in light + dark); running app: navbar "Home" route shows all four sections stacked vertically with scroll; primary CTA in hero advances `currentPage` to "docs" and the placeholder Docs panel from G5.1a appears; any rough edges from composing G4.5a–d together are appended to `FEEDBACK-G5.1.md`.
- **Achievable:** content-only sub-goal. Layout depends entirely on G4.5a–d shipping correctly — if any pattern doesn't compose well at this scale, log the finding and either work around or replace that section with placeholder text. No responsive layout, no anchor links.
- **Relevant:** highest-fidelity pressure test for G4.5a–d.
- **Budget:** ~50 K. Hard dependency on G4.5a, G4.5b, G4.5c, G4.5d.

#### G5.1c — Multi-page docs

- [ ] **Done**
- **Specific:** `sitedocs/docs.go` adds three docs pages reachable via the sidebar: "Getting started", "Phases overview", "Component reference". Each page composes `cadence/breadcrumb` at top, a scrollable prose section, and `cadence/card`-wrapped code samples (plain monospace text, no syntax highlight). The sidebar is reshaped via `cadence/accordion` to group entries per phase (Prism / Cadence / Spectrum / Pulse), each containing nested links. The `currentPage` subject from G5.1a is extended to include the docs-page identifiers and drives Main panel switching.
- **Measurable:** `go test ./sitedocs/...` green (smoke + light/dark golden of each docs page); running app: clicking any sidebar entry navigates to that page; breadcrumb reflects the path; navbar "Docs" link routes to the first docs page; any new friction appended to `FEEDBACK-G5.1.md`.
- **Achievable:** prose is brief — copying a sentence or two verbatim from DESIGN.md headings is acceptable. Code samples are plain text inside cards. No in-page anchors, no search, no narrow-viewport layout.
- **Relevant:** pressure-tests `cadence/breadcrumb` + `cadence/accordion` + `cadence/card` + multi-route shell composition in one place.
- **Budget:** ~70 K. Depends on G5.1a and (loosely) G5.1b.

#### G5.1d — Feedback writeup

- [ ] **Done**
- **Specific:** rewrite `FEEDBACK-G5.1.md` from the running notes left by G5.1a–c into its final structured form. Four sections: **Bugs**, **Missing API affordances**, **Awkward compositions / boilerplate**, **Ergonomics wins worth preserving**. Within each non-empty section, entries are ranked **blocker / major / minor**; each blocker and major carries a one-line remediation sketch. If a section is empty, it is explicitly noted as such with a half-sentence on whether that signals a coverage gap or real polish.
- **Measurable:** `FEEDBACK-G5.1.md` exists in repo root with all four section headings present; every non-empty entry severity-tagged; every blocker and major has a remediation sketch; the file is a coherent summary, not a stream of session-end hot takes.
- **Achievable:** pure documentation goal. No code changes. Sources are the running notes already in `FEEDBACK-G5.1.md` from G5.1a–c. If the running notes are missing, this goal stops and surfaces that as a process failure rather than fabricating findings.
- **Relevant:** primary Phase 5 deliverable for G5.1; feeds any follow-on framework-polish phase.
- **Budget:** ~20 K.

### G5.2 ‖ — RSS / reading-list app

- [ ] **Done** *(done when G5.2a–G5.2e all checked)*
- **Specific:** native desktop RSS / reading-list app in a new top-level Go module `vibrantgio/feeds/`. Pressure-tests the content-shaped slice of Cadence (sidebar groups, articles table, detail tabs, action modals). Split into G5.2a (skeleton + feeds sidebar), G5.2b (articles table), G5.2c (article detail view), G5.2d (CRUD actions), G5.2e (feedback writeup). Domain is RSS because it is content-rich, has natural tabs/accordion shape, and is self-contained — no networking, no parsing, fixtures only.
- **Measurable:** all five sub-goals checked; `FEEDBACK-G5.2.md` exists in repo root in the structured form from G5.2e.
- **Achievable:** parent tracking goal. No real RSS fetching in this phase.
- **Relevant:** pressure-tests the navigation + interaction patterns from G4.2 (modal/popover/toast) and G4.3 (sidebar/tabs/accordion/shell) in a non-trivial composition.
- **Budget:** ~50–70 K per sub-goal.

#### G5.2a — App skeleton + feeds sidebar

- [ ] **Done**
- **Specific:** new top-level Go module `vibrantgio/feeds/` (joined to `go.work`). `feeds/main.go` opens a window via `prism/initial` + `spectrum` theme. Layout is `cadence/shell.Shell(SidebarHeaderMain)`: navbar with brand "Feeds" and trailing "Add feed" action button (no-op for now); sidebar renders a `cadence/accordion` of feed groups (Tech / News / Personal — hard-coded) with feed names beneath each group. Main slot is placeholder text reading the currently-selected feed name. A `selectedFeed rx.Subject[FeedID]` drives selection.
- **Measurable:** `go build ./feeds/...` green; `go test ./feeds/...` green (smoke test); running `go run ./feeds/` opens a window with sidebar populated from hard-coded data in `feeds/fixtures.go`; clicking a feed updates the placeholder Main; `FEEDBACK-G5.2.md` is created with first entries or the explicit line "no findings yet".
- **Achievable:** no fetching, no parsing, no persistence. Hard-coded data in `feeds/fixtures.go`. Selection is wired but no consumer beyond the placeholder.
- **Relevant:** first composition of `cadence/shell` + `cadence/sidebar` + `cadence/accordion` + `cadence/navbar` outside the docs site.
- **Budget:** ~60 K.

#### G5.2b — Articles table

- [ ] **Done**
- **Specific:** `feeds/articles.go` adds a `cadence/table` to the Main slot showing the selected feed's article rows (columns: Title, Author, Published, Unread). Above the table sits a `prism/input/textfield` filter input. The table supports sort by Published or Title (header click), free-text filter on the input value, and `cadence/pagination` below the table (10 rows per page). Hard-coded fixtures contain ≥80 article rows distributed across feeds. Clicking a row emits `selectedArticle rx.Subject[ArticleID]` (consumed by G5.2c).
- **Measurable:** `go test ./feeds/...` green (includes a golden of one feed's table state in light + dark); running app: selecting a feed filters the table to that feed; sort, filter, pagination behaviours visibly correct; `FEEDBACK-G5.2.md` gets any new findings.
- **Achievable:** data in-memory; no persistence of read state across runs.
- **Relevant:** `cadence/table` + `cadence/pagination` + Prism input composition under realistic data volume.
- **Budget:** ~70 K. Depends on G5.2a.

#### G5.2c — Article detail view

- [ ] **Done**
- **Specific:** `feeds/detail.go` renders the selected article in a right-hand pane (use `cadence/shell.Shell(SplitPane)` mode, or stack below the table — pick whichever composes cleaner and log the choice + rationale to `FEEDBACK-G5.2.md`). Detail uses `cadence/tabs` with three tabs: "Reader" (formatted body, paragraph wrapping), "Raw" (same body in monospace), "Comments" (static placeholder list). A `cadence/popover` on a navbar "Share" button lists three share destinations (no-op). Hover tooltips (`cadence/tooltip`) on the table's icon-only column headers.
- **Measurable:** `go test ./feeds/...` green; running app: clicking an article populates detail pane; switching tabs swaps content; Share popover opens and dismisses correctly; tooltips appear on hover; any composition friction appended to `FEEDBACK-G5.2.md`.
- **Achievable:** no real formatting — Reader and Raw both render the same hard-coded body text, differing only in font. Comments tab is a static placeholder list.
- **Relevant:** `cadence/tabs` + `cadence/popover` + `cadence/tooltip` + split-pane composition.
- **Budget:** ~70 K. Depends on G5.2b.

#### G5.2d — CRUD actions

- [ ] **Done**
- **Specific:** wire the "Add feed" navbar action to open a `cadence/modal` containing a small form (a `cadence/card` wrapping a `prism/input/textfield` for URL + a `prism/button` submit). On submit: synthesise a feed entry, append to the in-memory list, fire a `cadence/toast` "Feed added". Delete-feed: hover-revealed trash icon on each sidebar entry → `cadence/popover` confirm ("Delete this feed?") → on confirm, remove + toast. Empty-URL submit displays a `cadence/alert` at the top of the modal.
- **Measurable:** `go test ./feeds/...` green (golden of the modal in light + dark, plus a small interaction test confirming submit flow); running app: full add-feed flow works end-to-end; delete-feed confirm works; alert fires on empty submit; toasts visible for both actions; findings appended to `FEEDBACK-G5.2.md`.
- **Achievable:** no persistence; additions/deletions live until app restart. No undo. No URL validation beyond non-empty.
- **Relevant:** `cadence/modal` + `cadence/popover` + `cadence/toast` + `cadence/alert` composed in a realistic CRUD flow.
- **Budget:** ~50 K. Depends on G5.2a (sidebar must exist for delete).

#### G5.2e — Feedback writeup

- [ ] **Done**
- **Specific:** rewrite `FEEDBACK-G5.2.md` from the running notes left by G5.2a–d into the same four-section structured form defined in G5.1d (Bugs / Missing API / Awkward compositions / Ergonomics wins), with severity tags and remediation sketches.
- **Measurable:** `FEEDBACK-G5.2.md` exists in repo root with all four section headings; every non-empty entry severity-tagged; every blocker and major carries a one-line remediation; empty sections are explicitly annotated.
- **Achievable:** pure documentation goal; reads existing running notes; halts and surfaces process failure if notes absent.
- **Relevant:** primary Phase 5 deliverable for G5.2.
- **Budget:** ~20 K.

### G5.3 ‖ — Coinviz watchlist editor (speculative)

- [ ] **Done** *(done when G5.3a–G5.3d all checked)*
- **Specific:** native desktop watchlist editor in a new top-level Go module `vibrantgio/watchlist/`. Coinviz today takes a single `-symbol` CLI flag and has no on-disk persistence — so this app is **speculative**: it designs a JSON watchlist format and produces watchlist files that a future coinviz multi-symbol feature would consume. The coinviz adoption is **out of scope for Phase 5** and is queued as a separate follow-on goal in a later phase. Split into G5.3a (format design + skeleton + watchlists sidebar), G5.3b (symbols table + edit modal), G5.3c (delete + bulk + tooltips + pagination), G5.3d (feedback writeup).
- **Measurable:** all four sub-goals checked; `FEEDBACK-G5.3.md` exists in repo root; `WATCHLIST-FORMAT.md` exists in repo root capturing the file-format schema produced.
- **Achievable:** parent tracking goal. No coinviz code changes in Phase 5. The app produces files on disk in a format we control; coinviz integration is a future-phase decision.
- **Relevant:** pressure-tests the same data-heavy interaction patterns as G5.2d while exercising a real on-disk-format design loop. Plants the seed for a future coinviz multi-symbol enhancement.
- **Budget:** ~50–70 K per sub-goal.

#### G5.3a — Format design + app skeleton + watchlists sidebar

- [ ] **Done**
- **Specific:** new top-level Go module `vibrantgio/watchlist/` (joined to `go.work`). Before any UI: write `WATCHLIST-FORMAT.md` at repo root specifying the JSON file format — fields per symbol (Symbol, Exchange, Timeframe, Notes), file path convention (`~/Library/Application Support/vibrantgio/watchlists.json` on macOS, XDG path on Linux), top-level shape (named watchlists each containing an ordered symbol list), versioning field. Then `watchlist/main.go` opens a window via `prism/initial` + `spectrum` theme. Layout is `cadence/shell.Shell(SidebarHeaderMain)`: navbar with brand "Watchlist editor" + "New watchlist" action button (no-op for now); sidebar lists the watchlist names loaded from the on-disk file (or shows an empty-state message if the file is absent). Selecting a watchlist exposes its name in Main as placeholder. On first run, if no file exists, write a starter watchlist (`"default"` containing 3 example symbols — e.g., BTC/USD, ETH/USD, SOL/USD) so the app has data to display.
- **Measurable:** `go build ./watchlist/...` green; `go test ./watchlist/...` green; running `go run ./watchlist/` opens a window with sidebar populated from the on-disk JSON; `WATCHLIST-FORMAT.md` exists and documents the schema completely enough that a coinviz adoption could implement against it; `FEEDBACK-G5.3.md` is created with first entries or the explicit line "no findings yet".
- **Achievable:** read+display only — no editing yet. Persistence is read-on-startup; the starter file is written once if absent. Format design is small and pragmatic: a flat JSON document, not a database, not a binary blob.
- **Relevant:** real file format + real persistence + first composition of `cadence/shell` + `cadence/sidebar` + `cadence/navbar` for this app.
- **Budget:** ~60 K.

#### G5.3b — Symbols table + edit modal

- [ ] **Done**
- **Specific:** Main slot renders a `cadence/table` of the selected watchlist's symbols (columns: Symbol, Exchange, Timeframe, Notes). Above the table, an "Add symbol" button opens a `cadence/modal` containing a form (`prism/input/textfield` per field + `prism/button` submit). Editing: row click or pencil icon → reopens the same modal pre-populated with the row's values. Save: mutates the in-memory watchlist and writes the full file back to disk atomically (write to temp + rename). Save success → `cadence/toast` "Saved". Empty-Symbol submit → `cadence/alert` at top of modal.
- **Measurable:** `go test ./watchlist/...` green (golden of the modal in light + dark; small interaction test of save round-trip via a temp directory); running app: add-symbol flow persists across restart; edit-symbol flow persists across restart; alert fires on empty Symbol; toast confirms saves; findings appended to `FEEDBACK-G5.3.md`.
- **Achievable:** persistence is full-file rewrite (no merge, no concurrency). One watchlist edited at a time. Form validation is non-empty Symbol only.
- **Relevant:** `cadence/table` + `cadence/modal` + `cadence/alert` + `cadence/toast` + real disk write.
- **Budget:** ~70 K. Depends on G5.3a.

#### G5.3c — Delete + bulk + tooltips + pagination

- [ ] **Done**
- **Specific:** row-level delete via trash icon → `cadence/popover` confirm. Bulk: a checkbox column allows multi-select; a "Delete N" action in the navbar opens `cadence/popover` confirm with the selection count. Column header tooltips (`cadence/tooltip`) explain each column. Right-clicking a watchlist in the sidebar opens a `cadence/popover` with "Rename" / "Delete" entries (rename uses a small modal; delete confirms). `cadence/pagination` is added below the table if a watchlist has more than 25 symbols.
- **Measurable:** `go test ./watchlist/...` green; running app: row delete confirms and persists; bulk delete confirms with count and persists; rename and delete watchlist flows persist; pagination renders only when needed; tooltips appear on hover; findings appended to `FEEDBACK-G5.3.md`.
- **Achievable:** scoped to interaction surfaces. No undo/redo. No drag-reorder.
- **Relevant:** `cadence/popover` + `cadence/tooltip` + `cadence/pagination` composed with persistence.
- **Budget:** ~70 K. Depends on G5.3b.

#### G5.3d — Feedback writeup

- [ ] **Done**
- **Specific:** rewrite `FEEDBACK-G5.3.md` from the running notes left by G5.3a–c into the same structured form defined in G5.1d (Bugs / Missing API / Awkward compositions / Ergonomics wins) with severity tags and remediation sketches. Add a final subsection **"Format-design notes for coinviz adoption"** capturing any format-design lessons relevant to a future coinviz multi-symbol feature (e.g., field naming, version migration considerations).
- **Measurable:** `FEEDBACK-G5.3.md` exists in repo root with all four standard section headings plus the "Format-design notes for coinviz adoption" subsection; severity-tagged entries; blocker/major remediations; the format-design subsection is at least a paragraph (even if it just says "no surprises — straight read of `WATCHLIST-FORMAT.md` is sufficient").
- **Achievable:** pure documentation.
- **Relevant:** primary Phase 5 deliverable for G5.3; seeds the future coinviz multi-symbol feature.
- **Budget:** ~20 K.

---

## Cross-cutting goals (run anytime they unblock work)

### GX.1 ‖ — Per-component benchmark in `prism/bench/`

- [ ] **Done**
- **Specific:** `prism/bench/` package exposing `BenchFrame(b *testing.B, widget layout.Widget)` per DESIGN §"Performance — Methodology — Benchmark harness". The helper drives `widget(gtx)` with synthesized constraints, calls `b.ReportAllocs()`, and standardises measurement across components. Three Phase 1 components plug into the harness via their own `*_bench_test.go` files: `prism/button` (idle render), `prism/input/textfield` (cursor-blinking frame), `prism/list` (1000-row render). Current numbers (ns/op, B/op) for each are captured in `BASELINE.md` under a new "Phase 1 component baseline" heading.
- **Measurable:** `go test -bench=. ./prism/bench/... ./prism/button/... ./prism/input/textfield/... ./prism/list/...` green; `BASELINE.md` contains a "Phase 1 component baseline" section with one ns/op + B/op row per named component.
- **Achievable:** the harness + three component benchmarks + baseline doc. No CI gate, no PR automation — regression detection is a manual `go test -bench` re-run by the developer when the relevant code changes. Solo-dev project.
- **Relevant:** DESIGN §"Performance — Methodology".
- **Budget:** ~70 K.

### GX.4 ‖ — Touch-up: `cadence/modal` close affordance uses `prism/button`

- [ ] **Done**
- **Specific:** Replace the `widget.Clickable` + custom `drawCross` glyph + modal-owned focus ring used for the close affordance in `cadence/modal/modal.go` with a `prism/button.Button` instance (icon-only or compact variant). Preserve the existing `Props` API and all interaction semantics (Escape, Tab focus trap, backdrop click). Refresh the four golden images.
- **Measurable:** `go test ./cadence/modal/...` green; `grep -n 'button.Button(' cadence/modal/modal.go` returns at least one match and `grep -n 'drawCross' cadence/modal/modal.go` returns no matches.
- **Achievable:** scoped to the close affordance and its golden refresh; do not touch focus-trap, stack, footer, or scrim logic.
- **Relevant:** PLAN.md G4.2a Achievable contract ("reuses `prism/button` for header close + footer actions") — recorded as a known deviation in the G4.2a session reply.
- **Budget:** ~30 K.

### GX.5 ‖ — Touch-up: `cadence/modal` footer actions own their own focus tags

- [ ] **Done**
- **Specific:** Remove the modal-owned focus tag and focus ring drawn around each `Props.Actions` entry in `cadence/modal/modal.go`. Action widgets register their own focus tags (e.g., `prism/button.Button` does); the modal must include those tags in its Tab cycle without wrapping them. Choose the smaller of two routes: (a) add `Props.ActionFocusTags []event.Tag` so callers declare their own tags, or (b) introspect a registered tag set after each action lays out. Document the choice in the package doc comment.
- **Measurable:** `go test ./cadence/modal/...` green, including a new interaction test confirming a focused `prism/button` action renders only the button's own focus ring (no doubled outer ring); existing `TestTabTrapsFocusWithinModal`, `TestShiftTabTrapsFocusWithinModal`, and `TestTabCyclesFocusAmongModalTags` still pass.
- **Achievable:** `Props` addition or small introspection helper plus one new test; do not touch scrim, stack depth, or close-button logic.
- **Relevant:** PLAN.md G4.2a follow-up — focus-ring composition between modal and `prism/button`-typed actions. Pair with GX.4 if the close-button swap surfaces the same doubled-ring issue at the header.
- **Budget:** ~40 K.

### GX.6 ‖ — Consolidate per-sub-package go.mod into per-repo go.mod

- [ ] **Done** *(done when GX.6a–GX.6d all checked)*
- **Specific:** parent tracking goal — collapse the embedded per-sub-package Go modules in each of the four design-system repos (`cadence/`, `prism/`, `pulse/`, `spectrum/`) into a single go.mod per repo. Each repo currently carries 4–16 sub-package go.mod files with `replace ../../...` directives stitching them back into a workspace; the consolidated layout has one root go.mod listing the union of dependencies and the sub-packages become normal Go packages within that module. Sibling design-system repos remain separately versioned at the repo boundary.
- **Measurable:** GX.6a, GX.6b, GX.6c, GX.6d all checked.
- **Achievable:** parent tracking goal; the work lives in the sub-goals. No source-code changes to the patterns themselves; mechanical migration plus `go mod tidy`.
- **Relevant:** the per-sub-package layout was carried over from an "each pattern is an island" intuition that Go's package model already provides for free — a consumer importing only `cadence/pricing` never compiles `cadence/hero` or pulls its transitive deps into the consumer's `go.sum`. The current layout produces visible drift (uncommitted `go mod tidy` edits across sibling sub-packages, missing `go.sum` files breaking `GOWORK=off` builds for `cadence/hero` and `cadence/pricing`) and grows linearly with each new sub-package added in Phase 4 and 5.
- **Budget:** parent — no direct work.

#### GX.6a — Consolidate `cadence/` to one go.mod

- [ ] **Done**
- **Specific:** in the embedded `cadence/` repo (16 sub-packages today), replace the 16 per-sub-package `go.mod` / `go.sum` files with a single root `cadence/go.mod` declaring `module github.com/vibrantgio/cadence` and a single `cadence/go.sum`. Each sub-package becomes a normal Go package — `cadence/pricing/pricing.go` keeps `package pricing` and continues to import siblings via `github.com/vibrantgio/cadence/<other>` paths, but those imports now resolve within the same module without `replace` plumbing. Intra-repo `replace` lines (e.g., `cadence/sidebar => ../sidebar`) are deleted; sibling-repo `replace` lines (`github.com/vibrantgio/prism/... => ../../prism/<x>`, `github.com/vibrantgio/pulse/depth => ../../pulse/depth`) consolidate at the root go.mod. The `go.work` `use (...)` block shrinks from 16 cadence entries to one (`./cadence`). The pre-existing uncommitted `go mod tidy` drift in `cadence/{alert,breadcrumb,navbar,toast}/go.mod` and the untracked `cadence/popover/` sub-package are absorbed by the consolidation: their `go.mod` / `go.sum` files are deleted, their Go source is committed as part of the consolidated module.
- **Measurable:** `find cadence -name go.mod | wc -l` returns `1`; `go test ./cadence/...` green from the workspace root; `GOWORK=off go test ./...` green from inside `cadence/` (proves the consolidated layout builds standalone, which is currently broken for `cadence/hero` and `cadence/pricing`).
- **Achievable:** mechanical migration: walk each sub-package, delete `go.mod`/`go.sum`, fix import paths only where they reference siblings (most don't — sub-packages already import via `github.com/vibrantgio/cadence/<x>`, just registered as separate modules). Refresh `go.work` and run `go mod tidy` once at the new root. No source-code changes to the patterns; no test changes.
- **Relevant:** parent GX.6. Cadence is the largest repo (16 sub-packages) and the one actively growing during Phase 4 — consolidating before G4.5c–d and Phase 5 lands marketing/example patterns directly in the new structure instead of requiring later migration.
- **Budget:** ~75 K. Largest of the four: 16 sub-packages to walk, two full test runs (workspace + `GOWORK=off`) across ~30 test files, plus absorbing two pre-existing drift situations (tidy edits in four sibling sub-packages and the fully untracked `popover/`).

#### GX.6b — Consolidate `prism/` to one go.mod

- [ ] **Done**
- **Specific:** in the embedded `prism/` repo (14 sub-packages today: `a11y`, `button`, `cache`, `coordination`, `gallery`, `icon`, `initial`, `input`, `internal/golden`, `keyed`, `layout`, `list`, `theme`, `tokens`), collapse the 14 sub-package `go.mod` files into a single `prism/go.mod` declaring `module github.com/vibrantgio/prism`. Sub-packages become normal Go packages; cross-package imports (e.g., `button` → `tokens`, `button` → `theme`) become same-module imports with no `replace` plumbing. The single `prism/go.mod` retains `replace` lines for downstream design-system repos it references at all (currently `mvu`). Update `go.work` to one prism entry.
- **Measurable:** `find prism -name go.mod | wc -l` returns `1`; `go test ./prism/...` green from the workspace root; `GOWORK=off go test ./...` green from inside `prism/`.
- **Achievable:** mechanical; same shape as GX.6a. Prism has more intra-module deps than cadence (button→tokens, button→theme, list→a11y, etc.) so the `replace` cleanup is the bulk of the diff. The `prism/internal/golden` package keeps its `internal/` path inside the consolidated module — it remains importable from `prism/<sub>` siblings and unreachable from outside the repo, the same access pattern it has today.
- **Relevant:** parent GX.6.
- **Budget:** ~65 K. 14 sub-packages and the deepest intra-repo dep graph of the four; the `internal/golden` path warrants an extra verification pass to confirm it still resolves from sibling test files after consolidation.

#### GX.6c — Consolidate `pulse/` to one go.mod

- [ ] **Done**
- **Specific:** in the embedded `pulse/` repo (7 sub-packages today: `conductor`, `depth`, `glow`, `motion`, `spring`, `springbutton`, `tween`), collapse the 7 sub-package `go.mod` files into a single `pulse/go.mod` declaring `module github.com/vibrantgio/pulse`. Sub-packages become normal Go packages within that module. Update `go.work` to one pulse entry.
- **Measurable:** `find pulse -name go.mod | wc -l` returns `1`; `go test ./pulse/...` green from the workspace root; `GOWORK=off go test ./...` green from inside `pulse/`.
- **Achievable:** mechanical; same shape as GX.6a. `springbutton` depends on `spring` and `tween`, so the `replace` cleanup pays off there.
- **Relevant:** parent GX.6.
- **Budget:** ~45 K. 7 sub-packages; mid-sized intra-repo dep graph (springbutton → spring + tween) and modest test surface.

#### GX.6d — Consolidate `spectrum/` to one go.mod

- [ ] **Done**
- **Specific:** in the embedded `spectrum/` repo (4 sub-packages today: `preferences`, `system`, `transition`, `window`), collapse the 4 sub-package `go.mod` files into a single `spectrum/go.mod` declaring `module github.com/vibrantgio/spectrum`. Update `go.work` to one spectrum entry.
- **Measurable:** `find spectrum -name go.mod | wc -l` returns `1`; `go test ./spectrum/...` green from the workspace root; `GOWORK=off go test ./...` green from inside `spectrum/`.
- **Achievable:** mechanical; same shape as GX.6a. Smallest of the four — natural starter sub-goal to validate the migration recipe before tackling the larger repos.
- **Relevant:** parent GX.6. Recommended to run first as a low-risk dress rehearsal.
- **Budget:** ~35 K. 4 sub-packages, no intra-repo `replace` chains, modest test surface — the cheapest validation pass for the consolidation recipe.

---

## Glossary

- **Goal** — one unit of work scoped to ≤100 K Sonnet 4.6 tokens.
- **Split** — break a goal into sub-goals when scope exceeds the budget.
- **‖** — parallelisable with siblings in the same phase.
- **Conditional** — runs only if a named experiment outcome triggers it.
- **Phase gate** — a goal whose completion unblocks an entire phase.

## Out of scope for this plan

- Web, mobile, embedded targets (DESIGN §"Non-goals").
- Anything not traceable to DESIGN.md.
- Open-ended exploration; experiments are explicit Phase 00 goals with defined exits.
