# FEEDBACK-G5.2 — Dogfooding notes from the feeds app skeleton

Findings from building `feeds/` — the first non-docs app to exercise
`cadence/shell` with the fixed `rx.Observable[layout.Widget]` sidebar slot
(GX.7 remediation). The app composes accordion-grouped sidebar + navbar +
placeholder main from hard-coded fixture data driven by an `rx.Subject[FeedID]`.

---

## Bugs

no findings yet

---

## PLAN.md milestone-spec (no framework defect)

- **[Minor] G5.2a Specific cites `prism/initial` as part of "opens a window via
  `prism/initial + spectrum` theme"** — also noted in G5.1 feedback. The actual
  bootstrap is `mvu.NewWindow` + `spectrum/window.New` + `spectrum/system.LiveTheme`.
  `prism/initial` is the first-frame `Value[T]` sentinel helper, not a window API.
- **[Minor] G5.2a Budget "Depends on GX.7"** — GX.7 was already discharged before
  this milestone (shell.Props.Sidebar is now `rx.Observable[layout.Widget]`).
  The dependency note is stale but harmless; the composition worked first-try.

---

## Missing API affordances

#### [Major] `go mod tidy` in a new workspace module requires multi-pass replace discovery — repo infra (G5.2a)

Adding a new module (`feeds/`) to the workspace required discovering transitive
indirect deps (`cadence/sidebar`, `prism/layout`, `prism/internal/golden`) that
fail `go mod tidy` with git-remote errors before `replace` directives exist.
Each missing dep requires an extra tidy pass. This repeats the finding from
G5.1 feedback ("Workspace `use` requires per-module `replace`"). Three manual
iterations were needed before tidy succeeded.

**Remediation:** the FEEDBACK-G5.1 suggestion stands — document the
multi-pass bootstrap in `MIGRATION.md` (or a CONTRIBUTING.md), or collapse
per-module go.mods into per-phase modules (GX.6) so new apps inherit the
full replace graph from a single parent.

---

## Awkward compositions / boilerplate

#### [Minor] `openController` is re-implemented in feeds/sidebar.go verbatim from sitedocs/docs_sidebar.go

The 30-line `openController` struct (mutex, open-map, Subject, toggle) is
copy-pasted unchanged. This is the same issue logged as a major in G5.1
("accordion.OnToggle + Open rx.Observable reinvents the Subject pattern per
caller"). Extracting `accordion.NewController` would eliminate the copy for
both callers.

---

## Ergonomics wins worth preserving

#### [Preserve] `shell.Props.Sidebar` as `rx.Observable[layout.Widget]` (GX.7)

The accordion-grouped sidebar wired directly into `shell.Shell` without a
`composeSidebarHeaderMain` workaround. The G5.1 blocker is fully resolved —
the composition worked first-try with zero local reimplementation.

#### [Preserve] `rx.Subject[T](0, 1)` for selection state

`selectionController` follows the same Subject + atomic.Pointer pattern as
`pageController` in sitedocs. The buffer-1 replay and goroutine subscriber
worked correctly; `TestSelectionControllerAdvancesAtomic` validated the
full chain (set → Subject → rx.Goroutine → atomic.Pointer) in < 100 ms.
