# Window & system

`spectrum/window` wraps an MVU window so every rendered layer receives the
live per-window theme; `spectrum/system` reads the OS appearance (dark
mode, accent colour) and republishes it as that theme observable.

> On macOS each appearance read forks a `defaults` process, so `LiveTheme`
> polls at a configurable cadence — 5 s keeps idle cost near zero while
> still reacting to a dark-mode toggle in under a second.

## A live system-driven theme

```go
th := specsystem.LiveTheme(5 * time.Second)
w := specwin.New(mvuWin, th)
```
