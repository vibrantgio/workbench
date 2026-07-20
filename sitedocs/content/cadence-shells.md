# Shells

`cadence/shell` provides the top-level application layouts, in four
variants:

| Variant | Composition |
| --- | --- |
| `SidebarHeaderMain` | full-height sidebar on the leading edge, navbar across the remaining top, main below |
| `SplitPane` | draggable divider on either axis |
| `ThreeColumn` | full-width navbar, sidebar, main, optional resizable aside, optional footer |
| `StackedPage` | a pinned navbar over a shell-owned scroll of page sections |

This app eats the dog food: the landing page is a `StackedPage` whose
sections are the Cadence marketing patterns, and the page you are reading
renders in a `ThreeColumn` shell with the accordion sidebar in the leading
column.

## StackedPage — marketing shell

```go
shell.Shell(th, shell.Props{
    Layout:   shell.StackedPage,
    Navbar:   nav,
    Sections: []rx.Observable[layout.Widget]{heroObs, featObs},
})
```

## ThreeColumn — resizable aside

```go
shell.Shell(th, shell.Props{
    Layout: shell.ThreeColumn,
    Sidebar: sb, Main: main, Aside: comments,
    OnAsideResize: saveWidth,
})
```
