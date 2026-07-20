# Primitives

Prism's widget packages are the foundation Cadence builds on: `button`,
`input`, `list`, `scrollbar` and `icon`, plus a11y helpers, layout
utilities, and coordination for cross-widget arbitration (which popover
closes when another opens).

`prism/keyed` gives list items stable identity so per-item state (focus,
hover, animation) survives reordering — the same mechanism `cadence/table`
uses for its rows.

## Layout utilities

```go
inset := pllayout.Inset(24)
gap := pllayout.VSpacer(12)
```
