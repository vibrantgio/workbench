# FEEDBACK-G5.1 — Dogfooding notes from the sitedocs build-out

This file collects observations made while building the `sitedocs/`
documentation app (Phase 5 sub-goal G5.1) against the Cadence + Spectrum +
Prism stack. Each entry names the milestone slice it surfaced under, the
package it touches, and the recommended follow-up.

## G5.1a — App skeleton + shell

### Sidebar API has no `Section` concept (cadence/sidebar)

`sidebar.Props.Items` is a flat slice of `Item`. The G5.1a milestone called
for "two collapsible placeholder sections"; the skeleton realises them as
non-interactive header items (OnClick nil → no focus, no Primary tint) with
leaf items underneath. This works but is brittle: visually grouping by
non-interactivity overloads what is otherwise a focus-traversal signal.

**Suggested follow-up:** add a `sidebar.Section{Header string, Items []Item}`
type and let `Props.Items` accept either a `Section` or a bare `Item` (via
a small interface). Per-section collapse state would belong to the section
struct, not the package-wide `Collapsed` observable.

### Spec referenced `prism/initial` for window bootstrap, but `initial` is `Value[T]`

The G5.1a Specific line in `PLAN.md` says the app "bootstraps a window via
`prism/initial`". `prism/initial` is the first-frame `Value[T]` sentinel
helper, not a window bootstrap. The actual bootstrap path is
`mvu.NewWindow` + `spectrum/window.New` + `spectrum/system.LiveTheme`,
which is what the skeleton uses.

**Suggested follow-up:** rewrite the G5.1a Specific line to name the
packages it actually depends on (`mvu`, `spectrum/window`, `spectrum/system`).
Leave the `prism/initial` reference for a future sub-goal that surfaces
genuine first-frame state.

### `rx.Subject[T]` is a function, not a type

The Specific line also refers to "a `currentPage rx.Subject[string]`". In
the rx library `Subject[T]` is a factory function returning
`(Observer[T], Observable[T])`. The skeleton uses
`send, obs := rx.Subject[string](0, 1)`; the buffered Subject (size 1)
lets the Main slot's late subscriber see the seed value on its first
frame. The plan's notation is descriptive, not the actual signature.

**Suggested follow-up:** in PLAN.md, replace `rx.Subject[string]` with
something like "an `rx.Subject` of `string` (buffered size 1)" so a reader
does not look for a type that does not exist.

### Workspace `use` does not resolve `github.com/vibrantgio/*` modules without per-module `replace`

`go.work` lists every sub-package as a workspace member, yet building a
new top-level consumer (sitedocs) fails with `git ls-remote ... Repository
not found` unless each `vibrantgio/*` dependency carries an explicit
`replace ... => ../<path>` directive in `sitedocs/go.mod`. The pattern is
already established in `cadence/*` and `coinviz/go.mod`, but the rationale
is invisible from `DESIGN.md`.

**Suggested follow-up:** document the per-module-`replace` pattern in
`DESIGN.md` (or in a `MIGRATION.md` follow-up), or collapse the per-module
`go.mod`s into one per-phase `go.mod` as already foreshadowed by GX.6.

## G5.1b — Landing page content (marketing patterns)

### `shell.Props.Main` is a single `layout.Widget`, so scroll is on the caller

`cadence/shell.Props.Main` accepts only a static `layout.Widget`. The
landing page is taller than the window's main slot at 1200 × 800, so
sitedocs has to wrap the four sections in its own `layout.List` to provide
scroll. This means every page that overflows reinvents the same list
glue — there is no shared "scroll-aware page slot" pattern.

**Suggested follow-up:** either add a `Scrollable bool` flag to
`shell.Props` (or a new `Page` slot type that internally uses
`layout.List`) so callers do not each maintain their own list state, or
document the expectation that pages own their scroll container.

### Marketing patterns disagree on outer inset

`cadence/hero` and `cadence/feature` wrap their content in `S6`
`UniformInset` so a flush-mounted parent still gets visible left/right
margins. `cadence/pricing` and `cadence/testimonial` skip the outer
inset — `drawPricing` and `drawGrid` flex straight to the canvas edges.
Stacking the four vertically (the G5.1b layout) produces a staggered
indentation: hero and feature sit S6 in from the edges, while pricing and
testimonial bleed to them. The golden image makes this visible.

**Suggested follow-up:** standardise on either always-or-never adding an
outer inset across the four marketing patterns. The "always inset"
variant is the more useful default because callers that need flush-mount
can wrap the pattern in a negative inset, while the inverse requires
caller-side padding for every adoption.

### `Render` signatures vary across patterns

`hero.Render`, `pricing.Render`, and `testimonial.Render` all take five
parameters: `(shaper, props, colors, spacing, radius, type)`. But
`feature.Render` takes only four: `(shaper, props, colors, spacing, type)`,
omitting `radius` (because the feature grid has no rounded chrome). When
composing the four into `renderLanding`, the asymmetry forces a
per-pattern argument list rather than a single tuple that fans out.

**Suggested follow-up:** either keep all `Render` signatures uniform (let
feature take and ignore `radius`) so a caller can spread the same token
tuple across all four, or expose a single `Compose` helper per pattern
that bundles the tokens.

### Each pattern subscribes the theme stream independently

`landingMain` calls `hero.Hero(th, ...)`, `feature.Feature(th, ...)`,
`pricing.Pricing(th, ...)`, and `testimonial.Testimonial(th, ...)`, each
of which immediately re-derives `(Color, Spacing, Radius, Type)` from the
shared theme observable via its own `SwitchMap`. Four near-identical
pipelines run for one page. Cheap, but redundant — the per-pattern
`Defer`-scoped `widget.Clickable` allocations also happen four times.

**Suggested follow-up:** publish a `theme.Resolved` observable from the
prism/theme package (or from each pattern's parent caller) that fans out
the resolved tuple once per theme change, and accept it as an optional
input to each pattern's constructor.

### Golden of "the rendered Home page" must use structural-only copy

The G5.1b Measurable line calls for "a golden of the rendered Home page
in light + dark". Following the convention established by the upstream
pattern goldens (hero/feature/pricing/testimonial), the sitedocs golden
uses blank/single-space text labels and sharp corner radii so the diff
does not depend on GPU font rasterisation or anti-aliased AA. The real
copy from `landing_content.go` is exercised only by the runtime
composition test (`TestLandingMainConstructs`).

**Suggested follow-up:** rewrite the G5.1b Measurable line to say "a
structural golden of the home-page composition", making the distinction
between layout-regression goldens and copy-review tools explicit.

## G5.1c — Multi-page docs

### `shell.Props.Sidebar` is typed as `sidebar.Props`, not `layout.Widget`

`cadence/shell.Props.Sidebar` accepts only a `sidebar.Props`, so the
shell internally wraps it via `sidebar.Sidebar(th, props)`. G5.1c needs
an accordion-grouped sidebar (phase sections with nested links), which
`sidebar.Props.Items` (a flat slice) cannot express. The G5.1c
implementation therefore bypasses `shell.Shell` entirely and replicates
`composeSidebarHeaderMain` locally so it can substitute a custom
accordion-based sidebar widget for the slot.

**Suggested follow-up:** either generalise `shell.Props.Sidebar` to
`layout.Widget` (with a helper to wrap `sidebar.Props` into the same
shape), or expose a `shell.ComposeSidebarHeaderMain(sb, nb, main)` Render
helper so a caller doing custom sidebars does not have to reimplement
the horizontal-flex composition.

### `cadence/accordion` body height is hard-coded at 96 dp

`cadence/accordion.bodyHDp = 96` is a package-level constant; every open
Section's Body is rendered with `Constraints.Exact(image.Pt(W, 96))`. The
G5.1c docs sidebar wants 2-3 nested links per phase, each ~28 dp tall,
so the Prism section (3 links -> 84 dp) just fits, the Cadence /
Spectrum / Pulse sections (2 links -> 56 dp) leave ~40 dp of empty
Surface beneath the last link. The constraint also rules out per-section
content of different heights without per-caller padding tricks.

**Suggested follow-up:** make `accordion.Props.SectionBodyHeight` an
optional override (defaulting to 96 dp), or measure the body's natural
height during a recording pass and lay the next header at the resulting
offset. The recording approach matches Gio's standard idiom for
variable-height content.

### `accordion.OnToggle` plus `Open rx.Observable` reinvent the Subject pattern per caller

`accordion.Props` separates the *current* open state (`Open` observable)
from the *intent to change* (`OnToggle` callback). Wiring this in
sitedocs takes ~25 lines: an `openController` struct holds the live map,
a mutex protects it against the rx-goroutine subscriber, and the
OnToggle handler mutates + republishes via an `rx.Subject(0, 1)` so the
subscriber sees the new map. SingleOpen amplifies the wiring cost
because the toggle handler is called once per peer closure plus once for
the activation.

**Suggested follow-up:** ship a thin `accordion.NewController(initially
int) Controller` helper that returns the open observable and the toggle
function pre-wired (including SingleOpen flipping), so callers do not
each reinvent the same controller plumbing.

### `card.Card` consumes the full vertical canvas constraint

`card.drawCard` paints its rounded surface across `gtx.Constraints.Max`
top-to-bottom — there is no shrink-to-fit pass. Stacking N cards in a
docs page therefore requires each card to be wrapped in a fixed-height
container (`fixedHeight(docsCardHeightDp, ...)` in `docs.go`), with the
height chosen to fit the longest expected sample. Cards with shorter
content leave Surface padding beneath the inner stack.

**Suggested follow-up:** add a `card.Props.HeightDp float32` field for
the fixed-height case, or measure the inner stack height during a
recording pass and constrain the card's surface to it. The marketing
patterns (hero, feature, pricing) all fit the full vertical canvas by
design, so the recording-and-resize idiom would be specific to card.

### "Running app" measurable is not checkable in CI

The G5.1c Measurable line includes "running app: clicking any sidebar
entry navigates…". Interactive verification is not reachable from
`go test ./sitedocs/...`; the implementation relies on a smoke test
plus a routedMain unit test plus the page-controller subject test to
cover the wiring. The interactive criterion is essentially "no
regressions, judged by eye" — useful for the human, untestable in CI.

**Suggested follow-up:** rewrite the running-app clause to name the unit
tests that stand in for it (e.g., "TestRoutedMainSelectsByPage plus
TestPageControllerSetAdvancesAtomic plus a smoke construct of each
page"), so the contract is fully discharged by `go test`.
