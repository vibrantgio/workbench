# VibrantGio Workbench

VibrantGio is a design system for building beautiful, native desktop
applications on macOS, Windows, and Linux with [Gio](https://gioui.org) —
analogous to what Material Design is for Google, but built for a Functional
Reactive Programming application model on top of
[reactivego/rx](https://github.com/reactivego/rx).

This repository is the **workbench**: it holds the architecture and design
documentation, and three complete example applications that exercise the
design system end-to-end.

## The stack

The design system is layered — each layer only depends on the ones below it:

| Layer | Module | Role |
|---|---|---|
| Patterns | [`cadence`](https://github.com/vibrantgio/cadence) | Prebuilt application patterns: shells, tables, modals, popovers, tabs, toasts, navbars, sidebars, pagination, marketing sections |
| Effects | [`pulse`](https://github.com/vibrantgio/pulse) | Motion & vibrancy: tweens, spring physics, glow, depth, a shared animation conductor |
| Theme runtime | [`spectrum`](https://github.com/vibrantgio/spectrum) | Reactive theming: live OS dark-mode/accent tracking, preference persistence, animated theme transitions, window integration |
| Foundation | [`prism`](https://github.com/vibrantgio/prism) | Component catalogue: buttons, inputs, lists, icons, layout, focus/a11y, tokens, theme contract, keyed identity, coordination |
| Runtime | [`mvu`](https://github.com/vibrantgio/mvu) | Model-View-Update runtime for Gio: `NewWindow`, `MessageOp` widget protocol, commands |

Supporting libraries: [`seen`](https://github.com/vibrantgio/seen) (3D scenes
to SVG/Gio), [`traer`](https://github.com/vibrantgio/traer) (particle
physics), [`svg`](https://github.com/vibrantgio/svg) and
[`ivg`](https://github.com/vibrantgio/ivg) (vector graphics),
[`backdrop`](https://github.com/vibrantgio/backdrop),
[`noise`](https://github.com/vibrantgio/noise),
[`style`](https://github.com/vibrantgio/style),
[`textdraw`](https://github.com/vibrantgio/textdraw),
[`font`](https://github.com/vibrantgio/font).

## The example apps

Each app is a full, runnable product built the way a real VibrantGio app is
meant to be built — MVU state, spectrum theming, cadence patterns:

- **[`sitedocs/`](./sitedocs)** — a documentation & marketing site app:
  application shell, hero/feature/pricing/testimonial sections,
  accordion-grouped sidebar navigation, breadcrumbs, light/dark theming.
- **[`feeds/`](./feeds)** — an RSS reading-list app: sortable/filterable/
  paginated article table, tabbed detail view in a split pane, modal CRUD
  forms with alerts and toasts, popovers and tooltips.
- **[`watchlist/`](./watchlist)** — a persistent watchlist editor: JSON-backed
  storage, sidebar with right-click context menu, add/edit modals, bulk
  delete with confirmation popovers, conditional pagination.
  Its on-disk format is specified in [WATCHLIST-FORMAT.md](./WATCHLIST-FORMAT.md).

Run one:

```sh
go run ./watchlist/
```

## Documentation

- **[DESIGN.md](./DESIGN.md)** — the architecture document: vision, the five
  core patterns (including the `WithLatestFrom2` frame-synchronisation model
  and the `rx.Defer` subscription-state pattern), threading rules,
  accessibility, performance methodology, and the phase plan that produced
  Prism, Spectrum, Pulse, and Cadence.
- **[llm.txt](./llm.txt)** — a condensed guide for AI coding assistants
  (Claude, etc.) to write applications against the VibrantGio packages.
- **[PLAN.md](./PLAN.md)** — the executed implementation plan, kept as the
  historical record of how the system was built and validated.
- **[BASELINE.md](./BASELINE.md)** — measured performance baselines the
  component benchmarks compare against.

## Requirements

Go 1.25+, and Gio's [platform dependencies](https://gioui.org/doc/install)
(on Linux: Wayland/X11 dev packages; macOS and Windows need nothing extra).

## License

MIT — see [LICENSE](./LICENSE). Individual library repositories carry their
own licenses.
