# Reactive window

`mvu.NewWindow` owns the Gio event loop and exposes the message stream;
`Render` composes one or more `rx.Observable[layout.Widget]` layers, and
every layer emission invalidates the window — a theme change or model
update repaints without manual wiring.

Layers stack back to front. This app renders a backdrop layer and a shell
layer; apps with overlays (modals, toasts, undo bars) append more.

## Compose window layers

```go
w.Render(func(th rx.Observable[theme.Theme]) []rx.Observable[layout.Widget] {
    return []rx.Observable[layout.Widget]{
        backdropLayer(th), shellLayer(th),
    }
})
```
