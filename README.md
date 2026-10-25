# Vibrant Gio Workbench

Vibrant Gio is a design system for building beautiful, native desktop
applications on macOS, Windows, and Linux with [Gio](https://gioui.org) —
analogous to what a platform's own design system is for that platform, but built for a Functional
Reactive Programming application model on top of
[reactivego/rx](https://github.com/reactivego/rx).

This repository is the **workbench**: eight complete example applications
that exercise the design system end-to-end, and the workbench window that opens
them.

```sh
go run github.com/vibrantgio/workbench@latest
```

That is the workbench window: the package at this repository's root, with
each app as a group over a live 3D field. A group's launch button runs
`go run github.com/vibrantgio/workbench/<app>@latest` when the workbench
itself was installed that way; `workbench` carries no tags, so `@latest`
is the newest commit on its default branch. Started from a checkout, the
workbench runs `go run .` in the app's own directory instead.

## The stack

The design system is layered — each layer imports only the layers below it
and the supporting libraries:

| Layer | Module | Role |
|---|---|---|
| Patterns | [`patterns`](https://github.com/vibrantgio/patterns) | Application patterns: shells, panes, splitters, tables, dialogs, popovers, tabs, notifications, navbars, sidebars, groups, cards, accordions, marketing sections |
| Effects | [`effects`](https://github.com/vibrantgio/effects) | Motion: tweens, spring physics, glow, depth, blur, theme transitions, a shared animation conductor |
| Components | [`components`](https://github.com/vibrantgio/components) | Component catalogue: buttons, inputs, lists, pickers, chips, badges, alerts, toasts, tooltips, pagination, progress, scrollbars, icons, layout, keyed identity, coordination |
| Theme | [`theme`](https://github.com/vibrantgio/theme) | The theme contract, design tokens, colour, typography, accessibility, live OS appearance and accent tracking, preference persistence, window integration |
| Runtime | [`mvu`](https://github.com/vibrantgio/mvu) | Model-View-Update runtime for Gio: `NewWindow`, `MessageOp` component protocol, commands |

Supporting libraries: [`seen`](https://github.com/vibrantgio/seen) (3D scenes
to SVG/Gio), [`traer`](https://github.com/vibrantgio/traer) (particle
physics), [`svg`](https://github.com/vibrantgio/svg) and
[`ivg`](https://github.com/vibrantgio/ivg) (vector graphics),
[`backdrop`](https://github.com/vibrantgio/backdrop),
[`noise`](https://github.com/vibrantgio/noise),
[`style`](https://github.com/vibrantgio/style),
[`textdraw`](https://github.com/vibrantgio/textdraw),
[`font`](https://github.com/vibrantgio/font),
[`markdown`](https://github.com/vibrantgio/markdown) (markdown rendering),
[`kiwi`](https://github.com/vibrantgio/kiwi) (constraint solving),
[`csg`](https://github.com/vibrantgio/csg),
[`gradient`](https://github.com/vibrantgio/gradient),
[`circle`](https://github.com/vibrantgio/circle).

## The example apps

Each app is a full, runnable product built the way a real Vibrant Gio app is
meant to be built — MVU state, live theming from theme, layouts from patterns:

- **[`todos/`](./todos)** — **start here**: the minimal canonical MVU app.
  One window, one Model, pure reducers, library components,
  live OS light/dark theming — the smallest complete demonstration of the
  bootstrap every other app follows.
- **[`iconbrowser/`](./iconbrowser)** — a searchable catalogue of the symbols the apps
  draw from, in two sets: the design system's own marks from
  `components/icons` above the Material Design icons. Type to filter both
  live; every symbol is captioned with the name to write. Also the
  reference for `components/input`'s `SearchField` with per-keystroke MVU
  updates.
- **[`sitedocs/`](./sitedocs)** — the documentation app, six tabs over
  `patterns/tabs`: **Docs** renders the application guide (`llms.txt`) as
  one markdown document with a `##`/`###` outline tree that scrolls it;
  **Colours** shows the platform's colour set, name by name with each
  value in both appearances, and **Typography** the type scale, both
  following the live theme; **Components**, **Patterns** and
  **Markdown** are the component inventory's own groups, one per tab, as
  live, clickable controls.
- **[`feeds/`](./feeds)** — an RSS reading-list app: sortable/filterable/
  paginated article table, tabbed detail view in a split pane, dialog CRUD
  forms with alerts and toasts, popovers and tooltips.
- **[`themer/`](./themer)** — the chooser for the theme colour and for
  what a theme says about code, laid out as a column of titled groups in
  reading order: the picture and the colours it gave, the theme colour, the
  code face. The platform's own accent colour leads the swatches as the
  default; drop an image anywhere on the window and the colours it is made
  of come back beside it, vivid ones first; or write the colour into the
  field. Under the choices stands the row they are judged in: a scrolling
  list of every syntax highlighter style fitted to the appearance on screen,
  and beside it the preview — one picture of an application in the
  platform's colour set with the chosen colour standing in where the
  platform uses its accent, carrying a sidebar with the selection pill, a
  toolbar with the platform's window controls, a heading with a badge,
  prose, a fenced code block in the chosen face and style, a text field and
  two push buttons. One appearance at a time, and one switch at the top of
  the window that moves the whole of it. Under that, the window's one
  default button keeps the theme to the shared file every application reads.
  Also the reference for an OS file drop — a window-wide drop zone with its
  hover highlight, delivered into the MVU loop as messages.
- **[`mindchat/`](./mindchat)** — an OpenAI chat client and the most
  feature-complete app: streaming completions routed through the MVU
  command loop, a resizable/collapsible split-pane shell, trash-backed
  undo with Cmd/Ctrl-Z, chat rename/delete/create, and per-chat streaming
  indicators. Set `OPENAI_API_KEY` to chat.
- **[`marketing/`](./marketing)** — a fictional SimpleApps landing on an
  outline field: one full-screen page for provenance and authenticity
  tools, with macOS full-size-content chrome so the traffic lights sit
  on the page.
- **[`sk150/`](./sk150)** — a controller for the XY-SK150 buck-boost
  converter over Modbus RTU: a live monitor with charts, a presets tab
  over the device's memory slots, and a device tab for its settings; runs
  against a simulated device with `sk150 demo`.

Each app is its own Go module, so run it from inside its directory:

```sh
cd todos && go run .
```

Or open the workbench — `go run .` at this repository's root — and start
them from there.

The workbench itself is the root module's `main` package. It is the
reference for compositing a [`seen`](https://github.com/vibrantgio/seen)
3D scene as an mvu background layer — a noise-animated triangle field,
colour-keyed to the live theme — and for a single streaming `mvu.Command`
that reports one launched process (Started → Exited).

## Documentation

- **[DESIGN.md](https://github.com/vibrantgio/design/blob/master/DESIGN.md)**
  — the design system's architecture and rationale, in
  [vibrantgio/design](https://github.com/vibrantgio/design) beside the
  published token bundle.
- **[llms.txt](https://raw.githubusercontent.com/vibrantgio/workbench/master/llms.txt)**
  — a condensed guide for AI coding assistants (Claude, etc.) to write
  applications against the Vibrant Gio packages. It is kept here, at this
  repository's root, and the Site Docs app's Docs tab renders it.
- Development planning is in one place,
  [vibrantgio/organisation](https://github.com/vibrantgio/organisation), not here.

## Requirements

Go 1.25+, and Gio's [platform dependencies](https://gioui.org/doc/install)
(on Linux: Wayland/X11 dev packages; macOS and Windows need nothing extra).

## License

MIT — see [LICENSE](./LICENSE). Individual library repositories carry their
own licenses.
