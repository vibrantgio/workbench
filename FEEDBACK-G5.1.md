# FEEDBACK-G5.1 — Dogfooding notes from the sitedocs build-out

Findings from building the `sitedocs/` documentation app (Phase 5 sub-goal
G5.1) against the Cadence + Spectrum + Prism stack. The app exercised the
shell, marketing patterns (hero / feature / pricing / testimonial),
accordion-grouped sidebar, and card-stacked content pages — i.e., the
canonical "compose pre-built pieces into a real product" path the framework
is meant to serve.

Entries below are classified into four buckets and severity-tagged
**blocker / major / minor**. Blockers and majors each carry a one-line
remediation sketch. Each entry cites the milestone slice (G5.1a / b / c)
and package it surfaced under, so reviewers can pivot back to the running
context if needed.

---

## Bugs

### Framework

#### [Blocker] Pill widgets pass unclamped `Radius.Full` (9999 dp) into `clip.RRect`, flooding the canvas — `cadence/{hero,pricing}` (G5.1b)

`hero.eyebrowWidget` and `pricing.popularChipWidget` build their pill via
`rad := gtx.Dp(unit.Dp(tok.radius.Full))` (= 9999 px at PxPerDp=1) and
hand `rad` directly to `clip.RRect{SE:rad, SW:rad, NE:rad, NW:rad}`.
`clip.RRect` does **not** clamp corner radii to the rect — a radius
larger than `min(w,h)/2` produces a clip path that sprays paint over a
region far beyond the pill bounds.

In the Pro pricing tier (Highlighted = `true`), the chip is filled with
pure `tok.color.Primary`, so the flood is immediately visible: the live
sitedocs landing renders the entire Pro column and large adjacent areas
in bright blue, hiding the Free tier card, the feature row above, and
the hero text. The structural goldens use a zero-valued `RadiusScale`
(Full = 0), so the bug is invisible to them.

The hero eyebrow has the same defect, but its pill is filled with
`tintColor(Primary, Surface)` (≈12%/88% blend) which is visually almost
identical to Surface, so the flood is masked — still a latent bug, now
caught by `sitedocs/landing_radius_regression_test.go`.

**Remediation:** clamp `rad` to `min(w, h) / 2` before constructing the
`clip.RRect`. Applied at both call sites
(`cadence/{hero/hero.go,pricing/pricing.go}`) plus a regression test in
`sitedocs/landing_radius_regression_test.go` that renders each pattern
with the real radius scale and asserts no flood. Longer-term, a small
`prism/layout.Pill(w, h, rad) clip.Op` helper would centralise the
clamp so future pill callers cannot reintroduce the bug.

#### [Major] Marketing patterns disagree on outer inset — `cadence/{hero,feature,pricing,testimonial}` (G5.1b)

`hero` and `feature` wrap their content in an `S6` `UniformInset`;
`pricing` and `testimonial` flex straight to the canvas edges. Stacking
the four vertically (as the landing page does) produces a staggered
indentation visible in the golden: hero/feature sit `S6` in from the
edges, pricing/testimonial bleed flush. This is a real layout defect
when the patterns are composed, not just a stylistic difference.

**Remediation:** standardise on always-inset across the four marketing
patterns. Callers that need flush-mount can wrap in a negative inset; the
inverse (every caller pads) is strictly worse.

#### [Blocker] Cadence interactive-pattern callbacks lack `gtx` → consumers cannot route through mvu `MessageOp` → invalidation contract broken — `cadence/{accordion,button,navbar,pricing,...}` (G5.1c)

**Surfaced as:** clicking the accordion header (or any other cadence-pattern interactive control in sitedocs) updates state but does **not** repaint until the next input event arrives — typically the user moving the mouse. The visual lag is the gap between `click` and `mouse-move`; functionally the state has already changed.

**Root cause.** Every cadence interactive pattern declares its event callback as a plain function with no `layout.Context`:

```
accordion.Props.OnToggle   func(idx int)
button.Props.OnClick       func()
navbar.Link.OnClick        func()
pricing.CTA.OnClick        func()
pagination.Props.OnSelect  func(p int)
table.Props.OnSort         func(col int)
```

`mvu.Window`'s entire invalidation path (`mvu/window.go:59-66`) fires `window.Invalidate()` from exactly one place: the rx subscription that watches the top-level layer observables. The framework's intended way for a click to land a state change is to add a `mvu.MessageOp{Message: ...}` to `gtx.Ops` (collected per-frame at `mvu/window.go:92-102`, drained to `Messages()`, dispatched into Model/Update, producing a new view emission, which triggers Invalidate). All four pre-Phase-5 mvu apps (`todos/`, `appviz/`, `mindchat/`, `coinviz/`) do exactly this — see `todos/list.go:64-65`, `mindchat/view.go:190`, `appviz/periodpanel.go:63`.

But the cadence callbacks don't carry `gtx`, so the consumer's handler cannot construct or add a `MessageOp`. Sitedocs (and now feeds) work around the API gap by mutating `rx.Subject`s directly inside the callback closure (`openController.toggle`, `pageController.set`), with the value re-published on `rx.Goroutine`. That subject network lives parallel to the mvu loop — emissions never enter `frameMessages` (`mvu/window.go:92`), so the only Invalidate hook the framework owns is never tripped. The atomic-pointer mirrors (`mirrorWidget`, `pageController.cur`) hold the new widget, but Gio doesn't know it needs to paint, and so it doesn't.

**Why the previous accordion entry was off-target.** I previously logged this as "accordion.OnToggle + Open rx.Observable produces user-visible click-to-paint lag in SingleOpen mode" and attributed the lag to N+1 emissions per cross-section click hopping the rx.Goroutine scheduler. That entry was treating a symptom: the SingleOpen amplification produces N+1 *missed Invalidates*, which makes the lag more obvious, but a single-toggle click has the same bug — the user simply may not click again before moving the mouse. Sitedocs's `SingleOpen: false` workaround (commit `598336e`) reduces the wasted work but doesn't fix the lag; it only makes the worst case match the best case.

**Why this is in scope as a Cadence bug, not a sitedocs bug.** Sitedocs *correctly* chose mvu + cadence + spectrum as its substrate. The framework provides a Model/Update/Messages loop with a single, automatic invalidation hook. The framework also provides interactive patterns whose callback signatures *prevent* consumers from emitting into that loop. Picking either piece alone is fine; picking both, as sitedocs and feeds did, is a trap. Every Phase-5 app built on cadence will hit this and reach for the same workaround, because the cadence callback shape leaves no other choice.

**Remediation:** thread `layout.Context` through every interactive callback in cadence. The smallest change is to widen each `OnX func(...)` to `OnX func(gtx layout.Context, ...)`. Callers then write the canonical:

```go
OnToggle: func(gtx layout.Context, idx int) {
    mvu.MessageOp{Message: ToggleAccordion{idx}}.Add(gtx.Ops)
}
```

The `mirrorWidget`, `openController`, `pageController`, and (in feeds) `selectionController` plumbing collapses into a Model + Update function — no mutexes, no atomic.Pointers, no parallel Subject network, automatic invalidation.

This is the gating remediation for Phase 5 dogfooding: until it lands, every example app pays the "rebuild MVU state plumbing badly" tax that produced this entire FEEDBACK file's Awkward Compositions section. See related entries in this file ("accordion.OnToggle + Open rx.Observable", "theme stream re-subscribed per pattern") and FEEDBACK-G5.2 ("openController re-implemented verbatim", "pagination.Props.Page is static int").

#### [Major] `spectrum/system` polls system appearance via fork+exec — `~10%` CPU floor per 1 s tick — `spectrum/system` (G5.1c)

`spectrum/system.LiveTheme(interval)` is implemented as
`rx.Ticker(interval) → darwinSource.Read()`, where each `Read` spawns
**two** `defaults` subprocesses (`AppleInterfaceStyle` and
`AppleAccentColor`). The package's own comment puts the cost at "~50 ms
per call" — so the recommended 1 s interval imposes a baseline of
~100 ms CPU per second (~10% of a core) before the app does any
rendering. `sample 67518` against the live sitedocs process caught a
95.9% CPU first-second burst and a 21% steady-state floor with no user
interaction. The fork+exec also wakes `cfprefsd`, `launchd`, and the
pipe-allocation/teardown path on every tick, which is what surfaces as
cursor sluggishness over the window (WindowServer competes for the
same CPU these spawned processes are using).

The strategy is reasonable for correctness — `cfprefsd` always returns
fresh state, unlike NSUserDefaults' in-process cache (see
`system_darwin.go:11–17`) — but is the wrong mechanism for a UI poll
loop. Worked around in sitedocs by raising the interval to 5 s
(`sitedocs/main.go:65–71`), a 5× CPU reduction at the cost of slower
Light/Dark toggle response.

**Remediation:** replace the poll with a push subscription to
`NSDistributedNotificationCenter` for
`AppleInterfaceThemeChangedNotification`; fall back to a slow (≥10 s)
poll only for the accent colour (which doesn't have an equivalent
notification). Eliminates the idle CPU floor entirely and removes the
user-visible cursor lag without sacrificing freshness.

### PLAN.md milestone-spec (no framework defect)

These are bugs in `PLAN.md` Specific/Measurable lines, surfaced while
implementing against them. The framework behaves correctly; the spec
referred to it inaccurately.

- **[Minor] G5.1a Specific cites `prism/initial` as "window bootstrap"** but
  `prism/initial` is the first-frame `Value[T]` sentinel helper. The
  actual bootstrap is `mvu.NewWindow` + `spectrum/window.New` +
  `spectrum/system.LiveTheme`.
- **[Minor] G5.1a Specific writes `rx.Subject[string]` as a type** —
  `rx.Subject[T]` is a factory function returning
  `(Observer[T], Observable[T])`. The skeleton uses
  `send, obs := rx.Subject[string](0, 1)`.
- **[Minor] G5.1b Measurable says "golden of the rendered Home page"** but
  the upstream pattern-goldens convention requires structural-only copy
  (blank labels, sharp corners) so the diff does not depend on GPU font
  rasterisation. The sitedocs golden follows that convention; real copy is
  exercised only by `TestLandingMainConstructs`.
- **[Minor] G5.1c Measurable "running app: clicking any sidebar entry
  navigates…"** is not reachable from `go test`. The implementation
  stands in with unit tests (`TestRoutedMainSelectsByPage`,
  `TestPageControllerSetAdvancesAtomic`, per-page smoke construct).

These four collectively suggest a tightening pass on phase-5 spec
language — name packages that exist, types as types and factories as
factories, and CI-checkable acceptance for any "running app" clause.

---

## Missing API affordances

#### [Blocker] `shell.Props.Sidebar` is typed as `sidebar.Props`, not `layout.Widget` — `cadence/shell` (G5.1c)

`shell.Shell` internally wraps the sidebar via `sidebar.Sidebar(th, props)`,
so callers can only supply a flat `sidebar.Props`. G5.1c needs an
accordion-grouped sidebar (phase sections with nested links), which
`sidebar.Props.Items` (a flat slice) cannot express. The implementation
therefore **bypasses `shell.Shell` entirely** and re-implements
`composeSidebarHeaderMain` locally to substitute a custom accordion
widget into the slot. The framework's top-level shell composition was
unusable for a non-trivial sidebar.

**Remediation:** generalise `shell.Props.Sidebar` to `layout.Widget`
(with a thin helper to wrap a `sidebar.Props` for callers using the
default), or expose `shell.ComposeSidebarHeaderMain(sb, nb, main)` so a
caller doing custom sidebars does not have to reimplement the
horizontal-flex composition.

#### [Major] `sidebar.Props.Items` has no `Section` concept — `cadence/sidebar` (G5.1a)

`Props.Items` is a flat `[]Item`. G5.1a called for "two collapsible
placeholder sections"; the skeleton fakes them as non-interactive
header items (`OnClick nil` → no focus, no Primary tint) with leaf
items underneath. This overloads non-interactivity — normally a
focus-traversal signal — as a visual-grouping signal.

**Remediation:** add `sidebar.Section{Header string, Items []Item}` and
let `Props.Items` accept either a `Section` or a bare `Item` (small
interface). Per-section collapse state lives on the section struct, not
on a package-wide observable.

#### [Major] `shell.Props.Main` is a single `layout.Widget` — no built-in scroll — `cadence/shell` (G5.1b)

Landing page content is taller than the main slot at 1200 × 800, so
sitedocs wraps its four sections in its own `layout.List` to provide
scroll. Every page that overflows reinvents the same list glue; there
is no shared "scroll-aware page slot" pattern.

**Remediation:** add `Scrollable bool` to `shell.Props` (or a new
`Page` slot type that wraps an internal `layout.List`) so callers do
not maintain per-page list state. If shell deliberately stays
scroll-agnostic, document the "pages own their scroll container"
expectation.

#### [Major] `cadence/accordion` body height is hard-coded at 96 dp — `cadence/accordion` (G5.1c)

`bodyHDp = 96` is a package-level constant; every open Section's Body
renders with `Constraints.Exact(image.Pt(W, 96))`. The docs sidebar
fits the Prism section (3 × ~28 dp links → 84 dp) but leaves ~40 dp of
empty Surface beneath the last link in 2-link sections, and rules out
heterogeneous section bodies entirely.

**Remediation:** make `accordion.Props.SectionBodyHeight` an optional
override (default 96 dp). Longer-term, measure body natural height
during a recording pass and lay the next header at the resulting
offset — Gio's standard idiom for variable-height content.

#### [Major] `card.Card` consumes the full vertical canvas constraint — `cadence/card` (G5.1c)

`card.drawCard` paints its rounded surface across `Constraints.Max`
top-to-bottom — no shrink-to-fit pass. Stacking N cards in a docs
page requires each one to be wrapped in `fixedHeight(docsCardHeightDp, ...)`
sized for the longest expected sample, leaving Surface padding under
shorter cards.

**Remediation:** add `card.Props.HeightDp` for the fixed-height case;
or measure inner-stack height during a recording pass and constrain
the surface to it. The marketing patterns fit the full vertical canvas
by design, so the recording-and-resize idiom is specific to `card`.

---

## Awkward compositions / boilerplate

#### [Major] `accordion` `SingleOpen` mode amplifies emission cost — N+1 callbacks per cross-section click — `cadence/accordion` (G5.1c)

`processInput.activate` (accordion.go:160–169) closes every currently-open peer by issuing a separate `OnToggle(j)` call for each one, then issues `OnToggle(i)` for the activation. With N peers open before the click, one user click triggers N+1 `OnToggle` invocations. Each invocation in the current sitedocs/feeds wiring round-trips through `openController.toggle` → `Subject.Next(copyOfMap)` → `CombineLatest2(resolved, open)` → `Map` → `atomic.Pointer.Store`, hopping the `rx.Goroutine` scheduler each time. The work is wasted: the accordion already knows the full target open-map at activation time and could push it in a single shot.

This is wasted work, not the lag root cause (see the [Blocker] above for the invalidation bug that produces user-visible lag). Once invalidation is fixed via the MessageOp route, the N+1 amplification is a perf and allocation cost on the click hot path, but no longer a UX defect.

**Workaround applied in sitedocs:** `docs_sidebar.go` sets `SingleOpen: false` (commit `598336e`) — every section openable independently. This collapses the worst case (N+1 calls per cross-section click) to the same one-call cost as a same-section toggle. The user-visible lag remains until the [Blocker] above is addressed.

**Remediation:** add a single-shot `SetOpen func(gtx layout.Context, m map[int]bool)` callback on the accordion, dispatched once per click in SingleOpen mode with the full target map. The existing `OnToggle` stays for the non-SingleOpen path. Couples naturally with `accordion.NewController(initiallyOpen int)` that returns the `(Open observable, SetOpen, toggle)` triple pre-wired so callers don't reimplement the controller plumbing — but neither half is useful without the [Blocker]'s gtx-carrying callback widening first.

#### [Major] Each marketing pattern subscribes the theme stream independently — `cadence/{hero,feature,pricing,testimonial}` (G5.1b)

`landingMain` calls `hero.Hero(th, ...)`, `feature.Feature(th, ...)`,
`pricing.Pricing(th, ...)`, `testimonial.Testimonial(th, ...)`, each
of which re-derives `(Color, Spacing, Radius, Type)` from the shared
theme observable via its own `SwitchMap`. Four near-identical pipelines
fire for one page; per-pattern `Defer`-scoped `widget.Clickable`
allocations also happen four times.

**Remediation:** publish a `theme.Resolved` observable from
`prism/theme` that fans out the resolved tuple once per theme change,
and accept it as an optional input to each pattern's constructor —
callers that already have it share one subscription.

#### [Major] Workspace `use` requires per-module `replace` for `github.com/vibrantgio/*` — repo infra (G5.1a)

`go.work` lists every sub-package as a workspace member, yet a new
top-level consumer (sitedocs) still fails with
`git ls-remote ... Repository not found` unless each `vibrantgio/*`
dependency carries an explicit `replace ... => ../<path>` directive
in `sitedocs/go.mod`. The pattern is established in `cadence/*` and
`coinviz/go.mod`, but the rationale is invisible from `DESIGN.md`,
so every new module bootstraps with copy-paste boilerplate.

**Remediation:** either document the per-module-`replace` pattern in
`DESIGN.md` (or `MIGRATION.md`), or — better — collapse the
per-module `go.mod`s into one per-phase `go.mod` as already
foreshadowed by GX.6.

#### [Minor] Docs sidebar labels mismatch the page titles they route to — `sitedocs/docs_sidebar.go` (G5.1c)

The accordion sidebar lists per-phase entries — Prism → "Tokens &
theme", Cadence → "Patterns overview" / "Pattern reference", Spectrum
→ "System glue" / "Live theme", Pulse → "Motion overview" / "Effects
reference" — but every non-"Getting started" entry routes to one of
only three underlying pages (`pageDocsPhasesOverview` or
`pageDocsComponentRef`). Clicking "Patterns overview" lands on a page
whose breadcrumb + `<h1>` read "Phases overview", which reads as a
broken link rather than a deliberate design choice.

The implementation is consistent with G5.1c's "three docs pages
reachable via the sidebar" spec; the friction is that the spec asked
for three pages but the sidebar invites N labels, so the per-label
specificity sets an expectation the page content cannot satisfy.

**Remediation:** either collapse the per-phase entries down to the
three canonical labels (Getting started / Phases overview / Component
reference) so the sidebar mirrors the page set; or expand to N real
pages so each label has a matching destination. Don't keep the current
1:N hybrid.

#### [Minor] `Render` signatures vary across marketing patterns — `cadence/{hero,feature,pricing,testimonial}` (G5.1b)

`hero.Render`, `pricing.Render`, `testimonial.Render` take
`(shaper, props, colors, spacing, radius, type)`; `feature.Render`
takes only `(shaper, props, colors, spacing, type)` because the
feature grid has no rounded chrome. The asymmetry forces a
per-pattern argument list rather than a single tuple that fans out
in `renderLanding`.

---

## Ergonomics wins worth preserving

These are inferred from the running notes by absence: each of the
following patterns was used repeatedly during the build-out without
generating a single complaint, despite being load-bearing. They are
candidates to keep stable when adjacent APIs churn.

Entries here are tagged `[Preserve]` rather than blocker/major/minor —
the severity scheme used elsewhere ranks problems, which does not fit
wins. The tag exists so every non-empty entry in the file carries a
classification.

#### [Preserve] Buffered `rx.Subject[T]` for seeded late-subscriber semantics

`rx.Subject[string](0, 1)` (buffer size 1) lets the Main slot's late
subscriber see the seed value on its first frame. Used to drive
`currentPage` in the skeleton (G5.1a) and re-used for the
accordion open-map republish (G5.1c). The only complaint about
`rx.Subject` in the notes was a notational issue in PLAN.md — the
semantics worked first-try.

#### [Preserve] `theme.Observable` → `SwitchMap` → `(Color, Spacing, Radius, Type)` tuple

Every marketing and content pattern derives its design tokens via the
same `SwitchMap` pattern off the theme observable. The G5.1b complaint
is that this fires four times for one page — but that complaint
*presupposes* the per-pattern shape is correct; nobody questioned the
pattern itself, only its redundancy. Keep the tuple shape stable when
factoring out `theme.Resolved`.

#### [Preserve] Structural-only goldens (blank text + sharp radii)

Established upstream for the per-pattern goldens and adopted unchanged
by the sitedocs landing-page golden (G5.1b). Diffs do not depend on
GPU font rasterisation or AA, so goldens stay deterministic across
machines. The convention scaled cleanly from single-pattern goldens
to a composed-page golden.

#### [Preserve] `Defer`-scoped widget allocation (e.g., `widget.Clickable`)

Mentioned across multiple patterns (hero, feature, pricing,
testimonial, accordion) as the standard place to allocate per-frame
state. Surfaced only as "this allocation happens N times" in the
theme-redundancy complaint — never as a misuse risk or lifecycle bug.
Keep the idiom stable; it is the load-bearing pattern for every
interactive pattern in the kit.
