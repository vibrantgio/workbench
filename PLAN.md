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

- [ ] **Done**
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

- [ ] **Done**
- **Specific:** `type Theme struct { Color rx.Observable[ColorTokens]; Type rx.Observable[TypeScale]; Motion rx.Observable[MotionTokens]; … }`.
- **Measurable:** `go test ./prism/theme/...` verifies emission shape via `rx.TestScheduler` per DESIGN §"Testing".
- **Achievable:** one file + tests.
- **Relevant:** DESIGN §"Phase 0" deliverable 2.
- **Budget:** ~60 K. Depends on G0.1, G0.2.

### G0.4 — `prism/theme/auto.go` (extract `AutoLightDark` from coinviz)

- [ ] **Done**
- **Specific:** lift `AutoLightDark` out of `coinviz/theme/`; rewrite as `rx.Observable[Theme]`.
- **Measurable:** coinviz now imports `prism/theme` for this; `coinviz/theme/auto*` is deleted; tests in both packages still pass.
- **Achievable:** one mechanical lift.
- **Relevant:** DESIGN §"Phase 1 — Migration path" coinviz step.
- **Budget:** ~50 K.

### G0.5 — `prism/a11y/preferences.go`

- [ ] **Done**
- **Specific:** `rx.Observable[A11yPrefs]` exposing reduced motion, high contrast, increased text size; backed by macOS `NSWorkspace` / Windows `SystemParametersInfo` / Linux best-effort.
- **Measurable:** unit tests against a fake OS source verify emissions; manual test on macOS confirms reduced-motion toggle propagates.
- **Achievable:** one file + cgo or platform shim files (one per OS).
- **Relevant:** DESIGN §"Accessibility" + §"Phase 0" deliverable 4.
- **Budget:** ~80 K. **Split** by OS if cgo balloons:
  - **G0.5a** macOS path + interface
  - **G0.5b** Windows path
  - **G0.5c** Linux fallback

### G0.6 — Conditional: `prism/keyed/` (only if G00.A adopted)

- [ ] **Done**
- **Specific:** ship `KeyedDefer[K, V]` per the API decided in G00.A.
- **Measurable:** API matches `EXPERIMENT-A.md`; tests cover reorder/insert/delete state preservation.
- **Achievable:** API is already validated in G00.A.
- **Relevant:** DESIGN §"Phase 0" conditional deliverable 1.
- **Budget:** ~60 K.

### G0.7 — Conditional: `prism/cache/` (only if G00.B yields op-cache pattern)

- [ ] **Done**
- **Specific:** documented `FrameCache` contract for animation-heavy widgets per `EXPERIMENT-B.md`.
- **Measurable:** contract documented; one reference implementation passes a regression test against G−1.6 baseline minus 30 % allocation rate.
- **Achievable:** API design constrained by experiment.
- **Relevant:** DESIGN §"Phase 0" conditional deliverable 2.
- **Budget:** ~70 K.

### G0.8 — Conditional: `prism/coordination/` shape (only if G00.C produced primitive)

- [ ] **Done**
- **Specific:** lock the observable shape of the coordination primitive (types only — implementation lands in Phase 1).
- **Measurable:** `prism/coordination/types.go` compiles; downstream Phase 1 modal/popover/toast can import it.
- **Achievable:** types-only module.
- **Relevant:** DESIGN §"Phase 0" conditional deliverable 3.
- **Budget:** ~30 K.

---

## Phase 1 — Prism (component foundation)

Discharges DESIGN §"Phase 1 — Prism".

### G1.1 — `prism/initial/`

- [ ] **Done**
- **Specific:** `Initial[T]` helper replacing ad-hoc first-frame sentinels (e.g., `offset.X = -1`).
- **Measurable:** unit tests cover first-frame and subsequent-frame paths; coinviz pane that currently uses the `-1` sentinel migrates to `Initial[T]` in the same session.
- **Achievable:** small generic helper.
- **Relevant:** DESIGN §"4. The `rx.Defer` Subscription-State Pattern — Initial-frame sentinels".
- **Budget:** ~50 K.

### G1.0 — Golden-image test harness

- [ ] **Done**
- **Specific:** `prism/internal/golden/` with `Render(t, widget) -> diff(stored.png)`; CI gate.
- **Measurable:** harness produces a stable PNG for a fixed widget across two consecutive runs; a deliberate one-pixel change is detected.
- **Achievable:** Gio headless backend wrapper.
- **Relevant:** DESIGN §"Testing — Component golden-image tests".
- **Budget:** ~70 K.

### G1.2 — `prism/button/`

- [ ] **Done**
- **Specific:** `Button(theme, ButtonProps{...})` with hover, focus, press; `Defer`-scoped state; full a11y (focus, keyboard, reduced motion, contrast); both `OnClick` callback and `MessageOp` paths per DESIGN §"Bridging FRP and MVU".
- **Measurable:** golden-image test (light + dark + focused + pressed); a11y tests; `gallery/` page demonstrates every variant; benchmark in `button_bench_test.go` per DESIGN §"Performance".
- **Achievable:** one component, well-scoped.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~95 K. Depends on G1.0.

### G1.3 ‖ — `prism/input/{textfield,checkbox,radio,dropdown}.go`

- [ ] **Done**
- **Specific:** four input components, each its own goal: G1.3a..G1.3d.
- **Measurable:** per-component golden + a11y + benchmark, same template as G1.2.
- **Achievable:** one component per session.
- **Relevant:** DESIGN §"Phase 1".
- **Budget:** ~80 K each.

### G1.4 — `prism/list/`

- [ ] **Done**
- **Specific:** virtual scrolling list generalised from `todos/list.go`; respects `KeyedDefer` if G00.A adopted.
- **Measurable:** golden tests for short / long / scrolled-mid lists; benchmark proves O(visible) layout cost not O(total).
- **Achievable:** one well-scoped module.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~95 K. **Split** if benchmark infra absent:
  - **G1.4a** core list mechanic
  - **G1.4b** virtualisation + benchmark

### G1.5 ‖ — `prism/icon/`

- [ ] **Done**
- **Specific:** unified registry over existing `svg/` and `ivg/` packages; `Icon(name)` resolves either.
- **Measurable:** test asserts resolution for one SVG + one IVG icon; `gallery/` icon page renders both.
- **Achievable:** thin wrapper over existing modules.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~50 K.

### G1.6 ‖ — `prism/layout/`

- [ ] **Done**
- **Specific:** spacing helpers, `FocusGroup`, basic flex/grid wrappers atop Gio's `layout`.
- **Measurable:** golden tests for each helper; `FocusGroup` keyboard traversal test.
- **Achievable:** modest module.
- **Relevant:** DESIGN §"Phase 1" deliverable.
- **Budget:** ~80 K.

### G1.7 — `prism/coordination/` implementation (if G0.8 landed)

- [ ] **Done**
- **Specific:** implement the types from G0.8: scoped subjects, drop-zone registry, modal stack, tooltip arbitration, gesture disambiguation.
- **Measurable:** `experiments/coordination/` G00.C prototype is rewritten on top of `prism/coordination/` and still passes its acceptance test.
- **Achievable:** types are pinned; only implementation work.
- **Relevant:** DESIGN §"Phase 1 — coordination/".
- **Budget:** ~95 K. **Split** along the five sub-concerns if needed.

### G1.8 — `prism/gallery/` (canonical reference app)

- [ ] **Done**
- **Specific:** one page per Phase 1 component; every variant, every a11y mode visible.
- **Measurable:** every Phase 1 component appears in gallery; manual launch shows working interactions.
- **Achievable:** assembly only.
- **Relevant:** DESIGN §"Phase 1 — gallery/".
- **Budget:** ~80 K. **Split** by component group if late.

### G1.9 ‖ — Migrate reference apps to Prism

- [ ] **Done**
- **Specific:** four sub-goals — one per app — each replacing bespoke widgets with Prism equivalents.
- **Measurable:** per app, no in-repo button/input/list duplication remains; all existing tests still pass.
- **Achievable:** mechanical replacement after Prism stabilises.
- **Relevant:** DESIGN §"Phase 1 — Migration path".
- **Budget:** ~70 K each (G1.9a coinviz, G1.9b appviz, G1.9c todos, G1.9d mindchat).

---

## Phase 2 — Reactive theme runtime *(name TBD: Cadence)*

Discharges DESIGN §"Phase 2". Gated by G1 stability.

### G2.0 — Naming decision

- [ ] **Done**
- **Specific:** decide between Pulse / Cadence / Resonance per DESIGN §"Naming Considerations". Update DESIGN.md and this PLAN.md.
- **Measurable:** final name committed; module path reserved.
- **Achievable:** decision-only.
- **Relevant:** DESIGN §"Naming Considerations" — "Decide before implementation begins."
- **Budget:** ~20 K.

### G2.1 — User-preference persistence

- [ ] **Done**
- **Specific:** persist chosen theme + a11y prefs to OS-appropriate config dir; emit on launch.
- **Measurable:** prefs survive app restart; verified by integration test.
- **Achievable:** one module.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~60 K.

### G2.2 — System-event bridging

- [ ] **Done**
- **Specific:** subscribe to OS dark-mode and accent-colour change events; bridge to `Theme` observable.
- **Measurable:** macOS test asserts `defaults write -g AppleInterfaceStyle Dark` triggers an emission within 1 s.
- **Achievable:** per-OS shim files.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~80 K. **Split** by OS.

### G2.3 — Animated theme transitions

- [ ] **Done**
- **Specific:** colour interpolation across theme switches via `Tween[Color]` (lands in Phase 3 or pre-Phase 3 stub here).
- **Measurable:** golden test of transitioning theme at frame 0/15/30; tween settles to target.
- **Achievable:** one tween implementation + integration.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~70 K.

### G2.4 — Per-window theme override

- [ ] **Done**
- **Specific:** allow each `Window.Render` call to receive an independent `rx.Observable[Theme]`.
- **Measurable:** test launches two windows with different themes and verifies isolation.
- **Achievable:** mostly contract-level; small runtime change.
- **Relevant:** DESIGN §"Phase 2".
- **Budget:** ~50 K.

---

## Phase 3 — Visual effects layer *(name TBD: Vivace)*

Discharges DESIGN §"Phase 3". Parallelisable with Phase 2 once Phase 1 lands.

### G3.0 — Naming decision

- [ ] **Done**
Same template as G2.0. Pick Verve / Vivace / Kinesis. Budget ~20 K.

### G3.1 ‖ — `<phase3>/tween/`

- [ ] **Done**
- **Specific:** generic `Tween[T]` for fades, slides, colour interpolations.
- **Measurable:** deterministic settling-time tests per DESIGN §"Testing — Physics convergence tests".
- **Budget:** ~60 K.

### G3.2 ‖ — `<phase3>/spring/`

- [ ] **Done**
- **Specific:** physics-based motion via `traer`; `Defer`-scoped `ParticleSystem`.
- **Measurable:** spring settles within tolerance under fixed seed.
- **Budget:** ~70 K.

### G3.3 ‖ — `<phase3>/glow/` and `<phase3>/depth/`

- [ ] **Done**
- **Specific:** one goal each (G3.3a glow, G3.3b depth). Glow = luminance halo via gradient composition; depth = elevation-driven shadow layers.
- **Measurable:** golden tests at multiple elevations.
- **Budget:** ~60 K each.

### G3.4 — `<phase3>/motion/`

- [ ] **Done**
- **Specific:** enter/exit/transition primitives composing tween + spring.
- **Measurable:** golden tests of enter, exit, swap on a `prism.Button` variant.
- **Budget:** ~80 K.

### G3.5 — `<phase3>/conductor/`

- [ ] **Done**
- **Specific:** shared clock for coordinated animation across widgets.
- **Measurable:** test demonstrates staggered list reveal across N rows phase-locked.
- **Relevant:** DESIGN §"5. Frame-Driven Physics — Caveats — Independent simulations are not synchronised".
- **Budget:** ~80 K.

### G3.6 — Spring-variant components in Prism

- [ ] **Done**
- **Specific:** for each Phase 1 interactive component, ship a `<phase3>.SpringX` variant per the composition mechanism in DESIGN §"Phase 3 — Composition mechanism".
- **Measurable:** `<phase3>.SpringButton` documented; `gallery/` shows side-by-side static vs spring.
- **Budget:** ~80 K per variant family. One goal per Phase 1 component (G3.6a..G3.6n).

---

## Phase 4 — Pattern library *(name TBD: Folio)*

Discharges DESIGN §"Phase 4".

### G4.0 — Naming decision

- [ ] **Done**
Pick Folio / Atelier / Suite. Budget ~20 K.

### G4.1 ‖ — Static patterns: card, alert, breadcrumb, pagination

- [ ] **Done**
- **Specific:** one goal per pattern (G4.1a..G4.1d). Each is a function consuming `rx.Observable[Theme]` and returning `layout.Widget`.
- **Measurable:** golden tests in light + dark; copy-paste-friendly source documented per DESIGN §"Phase 4 — Composition contract".
- **Budget:** ~50 K each.

### G4.2 ‖ — Patterns depending on coordination primitive: modal, popover, tooltip, toast

- [ ] **Done**
- **Specific:** one goal per pattern. Gated by G1.7.
- **Measurable:** golden + interaction tests; modal proves focus trap + escape handling.
- **Budget:** ~80 K each.

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
