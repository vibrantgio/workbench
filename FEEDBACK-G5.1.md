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

#### [Major] `accordion.OnToggle` + `Open rx.Observable` reinvents the Subject pattern per caller — `cadence/accordion` (G5.1c)

`accordion.Props` separates current open state (`Open` observable)
from the intent to change (`OnToggle` callback). Wiring this in
sitedocs takes ~25 lines: an `openController` struct holds the live
map, a mutex protects it against the rx-goroutine subscriber, and the
toggle handler mutates + republishes via `rx.Subject(0, 1)` so the
subscriber sees the new map. SingleOpen amplifies the cost — the
handler fires once per peer closure plus once for the activation.

**Remediation:** ship `accordion.NewController(initiallyOpen int) Controller`
returning the open observable and the toggle function pre-wired
(including SingleOpen flipping). Eliminates the per-caller plumbing.

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
