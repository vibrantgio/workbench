# Getting started

Vibrant Gio is a design system for building native desktop applications in
Go with [Gio](https://gioui.org). Five layers stack on each other:

- **Prism** — tokens and primitives
- **Cadence** — application patterns
- **Spectrum** — platform glue
- **Pulse** — motion
- **MVU** — the reactive runtime

Each layer is its own Go module under
[github.com/vibrantgio](https://github.com/vibrantgio) — add only the ones
you need. Every visual component consumes the same Prism theme observable,
so a light/dark or accent change flows through the whole tree without
manual wiring.

## Install the layers you need

```sh
go get github.com/vibrantgio/prism@latest
go get github.com/vibrantgio/cadence@latest
go get github.com/vibrantgio/mvu@latest
```

## Bootstrap a themed window

```go
mvuWin := mvu.NewWindow(app.Title("My App"))
w := specwin.New(mvuWin, specsystem.LiveTheme(5*time.Second))
w.Render(buildLayers).Wait()
```
