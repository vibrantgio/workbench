# Tokens & theme

`prism/tokens` holds the semantic scales: `ColorTokens` pairs every ground
with its "On" foreground (`Surface`/`OnSurface`, `Primary`/`OnPrimary`, …)
plus `TypeScale`, `SpacingScale`, `RadiusScale`, `MotionScale` and
`ElevationScale`. `DefaultLight` and `DefaultDark` ship ready to use.

`prism/theme` carries one observable per token category. Components
subscribe to exactly the categories they consume, so a theme change
re-emits only the widgets it affects.

## Consume a token category

```go
colors := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] {
    return t.Color
})
```

## Always pair grounds with their On colour

```go
paint.ColorOp{Color: c.OnSurface} // text on Surface
paint.ColorOp{Color: c.OnPrimary} // text on Primary
```
