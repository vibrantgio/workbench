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

- [ ] **Done** *(done when G4.2a–G4.2d all checked)*
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

- [ ] **Done**
- **Specific:** `cadence/toast/` package exposing `Stack(th, props) rx.Observable[layout.Widget]` rendering a positioned column of queued toasts, plus a `Notify(level, text)` entry point that emits a `Toast` value onto a package-scoped `prism/coordination` `Subject[Toast]`. `Props` carries `Position` (one of `TopRight`, `BottomRight`, `TopLeft`, `BottomLeft`), `Lifetime time.Duration` (default 4 s before auto-dismiss). Each toast renders as a tinted `Surface` (variant per level: info/success/warning/error) with text and an optional dismiss affordance.
- **Measurable:** golden-image tests `light-empty`, `light-three-stacked`, `dark-warning-toast`; interaction test that `Notify` adds a toast to the stack and the stack length returns to its prior value after `Lifetime` elapses; `go test ./cadence/toast/...` green.
- **Achievable:** one package; reuses `prism/coordination.Subject` for the queue and `pulse/tween` for fade-out near end of lifetime. Stack ordering is FIFO with newest nearest the position-anchored edge; no overflow collapse.
- **Relevant:** DESIGN §"Phase 4 — Cadence" (`toast/ — transient notification stack`); §"Coordination ceiling".
- **Budget:** ~80 K. No dependency on sibling sub-goals.

### G4.3 ‖ — Navigation: navbar, sidebar, shell, tabs, accordion

- [ ] **Done**
- **Specific:** one goal per pattern.
- **Measurable:** keyboard traversal test for each; `shell` demonstrates sidebar+header+main and split-pane.
- **Budget:** ~80 K each.

### G4.4 — Table and pagination

- [ ] **Done**
- **Specific:** sortable, filterable, virtualised table consuming `prism/list` + `KeyedDefer` (if adopted) for per-row state.
- **Measurable:** benchmark proves O(visible-rows) cost on 10 000-row dataset.
- **Budget:** ~95 K. **Split** core / virtualisation / sort+filter if needed.

### G4.5 ‖ — Marketing sections

- [ ] **Done**
- **Specific:** hero, pricing, feature, testimonial — one goal each.
- **Measurable:** golden tests per section.
- **Budget:** ~50 K each.

---

## Cross-cutting goals (run anytime they unblock work)

### GX.1 ‖ — Per-component benchmark in `prism/bench/`

- [ ] **Done**
- **Specific:** `BenchFrame(b, widget)` helper per DESIGN §"Performance — Methodology — Benchmark harness".
- **Measurable:** at least three Phase 1 components plug into it; CI rejects >5 % regression.
- **Budget:** ~70 K.

### GX.2 ‖ — Documentation: `MIGRATION.md`, `BASELINE.md`, `DECISIONS.md`, `EXPERIMENT-{A,B,C}.md`

- [ ] **Done**
- These are produced as artefacts of the goals above; this entry exists to make sure no goal closes without writing the relevant doc.

### GX.3 — Pin and review `reactivego/rx` version on every upgrade

- [ ] **Done**
- **Specific:** add a CI check that fails if `go.mod`'s `reactivego/rx` version changes without a corresponding diff in DESIGN §"3. The `WithLatestFrom2` Frame Synchronisation Model".
- **Measurable:** CI script exists and triggers on the synthetic test PR.
- **Relevant:** DESIGN §"`reactivego/rx` semantic coupling".
- **Budget:** ~30 K.

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
