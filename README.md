# Vibrant Gio Workbench

Vibrant Gio is a design system for building beautiful, native desktop
applications on macOS, Windows, and Linux with [Gio](https://gioui.org) —
analogous to what Material Design is for Google, but built for a Functional
Reactive Programming application model on top of
[reactivego/rx](https://github.com/reactivego/rx).

This repository is the **workbench**: eight complete example applications
that exercise the design system end-to-end, and the launcher that opens
them.

```sh
go run github.com/vibrantgio/workbench@latest
```

That is the launcher: the package at this repository's root, with the apps
as cards on a live 3D field. Each card fetches and runs that app's own
latest release, because each app is a separate product on its own tags.

## The stack

The design system is layered — each layer only depends on the ones below it:

| Layer | Module | Role |
|---|---|---|
| Patterns | [`patterns`](https://github.com/vibrantgio/patterns) | Prebuilt application patterns: shells, tables, modals, popovers, tabs, toasts, navbars, sidebars, pagination, marketing sections |
| Effects | [`effects`](https://github.com/vibrantgio/effects) | Motion & vibrancy: tweens, spring physics, glow, depth, a shared animation conductor |
| Theme runtime | [`theme`](https://github.com/vibrantgio/theme) | Reactive theming: live OS dark-mode/accent tracking, preference persistence, animated theme transitions, window integration |
| Foundation | [`components`](https://github.com/vibrantgio/components) | Component catalogue: buttons, inputs, lists, icons, layout, focus/a11y, tokens, theme contract, keyed identity, coordination |
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

Each app is a full, runnable product built the way a real Vibrant Gio app is
meant to be built — MVU state, live theming from theme, patterns patterns:

- **[`todos/`](./todos)** — **start here**: the minimal canonical MVU app
  (~700 lines). One window, one Model, pure reducers, components widgets,
  live OS light/dark theming — the smallest complete demonstration of the
  bootstrap every other app follows.
- **[`iconbrowser/`](./iconbrowser)** — a searchable catalogue of the 961
  Material Design icons the apps draw from: type to filter the scrolling
  grid live, every glyph captioned with the name to import. Also the
  reference for components `TextField` + per-keystroke MVU updates.
- **[`sitedocs/`](./sitedocs)** — the documentation app, five tabs over
  `patterns/tabs`: **Docs** renders the application guide (`llms.txt`) as
  one markdown document with a `##`/`###` outline tree that scrolls it;
  **Theme** shows the whole theme — the colour the palette grew from,
  the ramps grid, the named picks and the type ladder — following the
  live one; **Components**, **Patterns** and
  **Markdown** are the component inventory's own groups, one per tab, as
  live, clickable controls.
- **[`feeds/`](./feeds)** — an RSS reading-list app: sortable/filterable/
  paginated article table, tabbed detail view in a split pane, modal CRUD
  forms with alerts and toasts, popovers and tooltips.
- **[`vaultview/`](./vaultview)** — a read-only viewer for a folder of
  Obsidian-style markdown notes: a disclosing file tree, frontmatter as a
  properties panel, `[[wikilinks]]` that resolve and navigate, history
  with back/forward, and a backlinks aside. Also the reference for
  document-centric navigation — a history stack, a nesting tree, and the
  shell's aside slot in use. See its [README](./vaultview/README.md).
- **[`themer/`](./themer)** — pick a brand colour out of a picture: drop an
  image anywhere on the window and the colours it is made of come back as a
  row of seed candidates, vivid ones first, each swatch beside the primary
  pair a palette derivation makes of it; click one and the window re-themes
  to it. The page under the row is four tabs — the theme itself, then
  components, patterns and markdown — and the last of them ends on a code
  specimen with the syntax bases listed beside it — the styles that ship and any style file dropped into the
  styles folder beside the kept theme, filtered to the half fitted to the
  scheme on screen — so the code can be judged too, and a base per scheme is
  kept with the seed. Also the reference for an OS file drop — a window-wide drop zone
  with its hover highlight, delivered into the MVU loop as messages — and
  for a theme observable the application itself re-seeds.
- **[`mindchat/`](./mindchat)** — an OpenAI chat client and the most
  feature-complete app: streaming completions routed through the MVU
  command loop, a resizable/collapsible split-pane shell, trash-backed
  undo with Cmd/Ctrl-Z, chat rename/delete/create, and per-chat streaming
  indicators. Set `OPENAI_API_KEY` to chat.
- **[`marketing/`](./marketing)** — a fictional SimpleApps landing on an
  outline field: one full-screen page for provenance and authenticity
  tools, with macOS full-size-content chrome so the traffic lights sit
  on the page.

Each app is its own Go module, so run it from inside its directory:

```sh
cd todos && go run .
```

Or open the launcher — `go run .` at this repository's root — and start
them from there.

The launcher itself is the root module's `main` package. It is the
reference for compositing a [`seen`](https://github.com/vibrantgio/seen)
3D scene as an mvu background layer — a noise-animated triangle field,
colour-keyed to the live theme — and for a single streaming `mvu.Command`
that reports one launched process (Started → Exited).

## Documentation

- **[DESIGN.md](https://github.com/vibrantgio/design/blob/master/DESIGN.md)**
  — the design system's architecture and rationale. It lived in this
  repository until G0E.3 (its pre-move history is in this repo's log) and now
  sits in [vibrantgio/design](https://github.com/vibrantgio/design) beside
  the published token bundle, together with its archived first edition
  `DESIGN-v1.md`.
- **[llms.txt](https://raw.githubusercontent.com/vibrantgio/workbench/master/llms.txt)**
  — a condensed guide for AI coding assistants (Claude, etc.) to write
  applications against the Vibrant Gio packages. It is maintained once, at the
  root of [vibrantgio/.github](https://github.com/vibrantgio/.github), and
  this repository links it rather than keeping a copy or a pointer file.
- Development planning lives once, in
  [vibrantgio/.github](https://github.com/vibrantgio/.github), not here. The
  finished plan this repository was built against, its performance baselines
  and its feedback notes were removed once that became true; they survive in
  this repository's git log.

## Requirements

Go 1.25+, and Gio's [platform dependencies](https://gioui.org/doc/install)
(on Linux: Wayland/X11 dev packages; macOS and Windows need nothing extra).

## License

MIT — see [LICENSE](./LICENSE). Individual library repositories carry their
own licenses.
