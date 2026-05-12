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
